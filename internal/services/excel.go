package services

import (
	"bytes"
	"employee-management/internal/config"
	"employee-management/internal/models"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// JobStatus represents the status of an async job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// JobResult represents the result of an async job
type JobResult struct {
	ID        string                      `json:"id"`
	Status    JobStatus                   `json:"status"`
	Result    *models.ExcelUploadResponse `json:"result,omitempty"`
	Error     string                      `json:"error,omitempty"`
	CreatedAt time.Time                   `json:"created_at"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

// ExcelService handles Excel file processing
type ExcelService struct {
	employeeService *EmployeeService
	config          *config.Config
	mu              sync.RWMutex
	processResults  map[string]*models.ExcelUploadResponse
	jobs            map[string]*JobResult
}

// NewExcelService creates a new Excel service
func NewExcelService(employeeService *EmployeeService, cfg *config.Config) *ExcelService {
	return &ExcelService{
		employeeService: employeeService,
		config:          cfg,
		processResults:  make(map[string]*models.ExcelUploadResponse),
		jobs:            make(map[string]*JobResult),
	}
}

// StartAsyncExcelProcessing starts async processing of an Excel file
func (s *ExcelService) StartAsyncExcelProcessing(file *multipart.FileHeader) (string, error) {
	// Validate file first
	if err := s.validateExcelFile(file); err != nil {
		return "", fmt.Errorf("file validation failed: %w", err)
	}

	// Generate job ID
	jobID := uuid.New().String()

	// Create job record
	job := &JobResult{
		ID:        jobID,
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.mu.Lock()
	s.jobs[jobID] = job
	s.mu.Unlock()

	// Start processing in background
	go s.processExcelAsync(jobID, file)

	return jobID, nil
}

// GetJobStatus returns the status of a job
func (s *ExcelService) GetJobStatus(jobID string) (*JobResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found")
	}

	return job, nil
}

// processExcelAsync processes Excel file asynchronously
func (s *ExcelService) processExcelAsync(jobID string, file *multipart.FileHeader) {
	// Update job status to running
	s.updateJobStatus(jobID, JobStatusRunning, nil, "")

	// Process the Excel file
	result, err := s.ProcessExcelFile(file)

	if err != nil {
		s.updateJobStatus(jobID, JobStatusFailed, nil, err.Error())
		return
	}

	s.updateJobStatus(jobID, JobStatusCompleted, result, "")
}

// updateJobStatus updates the status of a job
func (s *ExcelService) updateJobStatus(jobID string, status JobStatus, result *models.ExcelUploadResponse, errorMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, exists := s.jobs[jobID]; exists {
		job.Status = status
		job.Result = result
		job.Error = errorMsg
		job.UpdatedAt = time.Now()
	}
}

// ProcessExcelFile processes uploaded Excel file asynchronously
func (s *ExcelService) ProcessExcelFile(file *multipart.FileHeader) (*models.ExcelUploadResponse, error) {
	// Validate file
	if err := s.validateExcelFile(file); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Read file content
	content, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	// Parse Excel file
	employees, validationErrors, err := s.parseExcelContent(content, file.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Excel file: %w", err)
	}

	// Prepare response
	response := &models.ExcelUploadResponse{
		TotalRecords:    len(employees) + len(validationErrors),
		ValidRecords:    len(employees),
		InvalidRecords:  len(validationErrors),
		InsertedRecords: 0,
		SkippedRecords:  0,
		DuplicateEmails: []string{},
	}

	// Process valid employees
	if len(employees) > 0 {
		// Save valid employees to database with detailed results
		inserted, skipped, duplicateEmails, err := s.employeeService.repo.CreateEmployeesInBatchWithResult(employees)
		if err != nil {
			log.Printf("Error saving employees to database: %v", err)
			response.Message = fmt.Sprintf("Processed %d records, but failed to save to database: %v",
				response.TotalRecords, err)
		} else {
			// Update response with actual results
			response.InsertedRecords = inserted
			response.SkippedRecords = skipped
			response.ValidRecords = inserted // Update to show only actually inserted records

			// Include sample duplicate emails (limit to first 10 for readability)
			maxDuplicatesToShow := 10
			if len(duplicateEmails) > maxDuplicatesToShow {
				response.DuplicateEmails = duplicateEmails[:maxDuplicatesToShow]
			} else {
				response.DuplicateEmails = duplicateEmails
			}

			if skipped > 0 {
				duplicateEmailsText := ""
				if len(duplicateEmails) > 0 {
					if len(duplicateEmails) > maxDuplicatesToShow {
						duplicateEmailsText = fmt.Sprintf(" (examples: %s and %d more)",
							strings.Join(response.DuplicateEmails, ", "), len(duplicateEmails)-maxDuplicatesToShow)
					} else {
						duplicateEmailsText = fmt.Sprintf(" (%s)", strings.Join(response.DuplicateEmails, ", "))
					}
				}

				response.Message = fmt.Sprintf("Successfully processed %d records. Inserted: %d new employees, Skipped: %d duplicates%s, Invalid: %d",
					response.TotalRecords, inserted, skipped, duplicateEmailsText, response.InvalidRecords)

				// Log duplicate emails for debugging
				if len(duplicateEmails) > 0 {
					maxShow := 5
					if len(duplicateEmails) < maxShow {
						maxShow = len(duplicateEmails)
					}
					log.Printf("Duplicate emails encountered: %v", duplicateEmails[:maxShow])
					if len(duplicateEmails) > maxShow {
						log.Printf("... and %d more duplicate emails", len(duplicateEmails)-maxShow)
					}
				}
			} else {
				response.Message = fmt.Sprintf("Successfully processed %d records. Inserted: %d new employees, Invalid: %d",
					response.TotalRecords, inserted, response.InvalidRecords)
			}
		}

		// Invalidate cache since we added new data
		if err := s.employeeService.cache.InvalidateEmployeeListCache(); err != nil {
			log.Printf("Warning: Failed to invalidate employee list cache after batch insert: %v", err)
		}
	} else {
		response.Message = "No valid employee records found in the Excel file"
	}

	return response, nil
}

// validateExcelFile validates the uploaded Excel file
func (s *ExcelService) validateExcelFile(file *multipart.FileHeader) error {
	// Check file size using config value
	maxSize := s.config.Server.MaxFileSize
	if file.Size > maxSize {
		return fmt.Errorf("file size %d bytes exceeds maximum allowed size %d bytes", file.Size, maxSize)
	}

	// Check file extension
	filename := strings.ToLower(file.Filename)
	if !strings.HasSuffix(filename, ".xlsx") && !strings.HasSuffix(filename, ".xls") {
		return fmt.Errorf("invalid file format. Only .xlsx and .xls files are supported")
	}

	return nil
}

// parseExcelContent parses Excel file content and returns employees and validation errors
func (s *ExcelService) parseExcelContent(content []byte, filename string) ([]models.Employee, []models.ValidationError, error) {
	// Open Excel file from bytes using excelize
	xlFile, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer xlFile.Close()

	// Get the first sheet name
	sheetName := xlFile.GetSheetName(0)
	if sheetName == "" {
		return nil, nil, fmt.Errorf("Excel file has no sheets")
	}

	// Get all rows from the first sheet
	rows, err := xlFile.GetRows(sheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read Excel sheet: %w", err)
	}

	if len(rows) <= 1 {
		return nil, nil, fmt.Errorf("Excel file appears to be empty or has no data rows")
	}

	var employees []models.Employee
	var validationErrors []models.ValidationError

	// Define expected headers (as per your Excel structure)
	expectedHeaders := []string{
		"first_name", "last_name", "company_name", "address",
		"city", "county", "postal", "phone", "email", "web",
	}

	// Read header row (first row)
	headerRow := rows[0]

	// Debug: Log actual headers found
	log.Printf("Excel headers found: %v", headerRow)

	// Validate headers
	headerMap, err := s.validateAndMapHeaders(headerRow, expectedHeaders)
	if err != nil {
		return nil, nil, fmt.Errorf("header validation failed: %w", err)
	}

	// Debug: Log header mapping
	log.Printf("Header mapping: %v", headerMap)

	// Process data rows
	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]

		// Skip empty rows
		if s.isRowEmpty(row) {
			continue
		}

		// Parse employee from row
		employee, rowErrors := s.parseEmployeeFromRow(row, headerMap, rowIndex+1)
		if len(rowErrors) > 0 {
			validationErrors = append(validationErrors, rowErrors...)
		} else if employee != nil {
			employees = append(employees, *employee)
		}
	}

	log.Printf("Parsed Excel file '%s': %d total rows, %d valid employees, %d validation errors",
		filename, len(rows)-1, len(employees), len(validationErrors))

	return employees, validationErrors, nil
}

// validateAndMapHeaders validates Excel headers and creates a mapping
func (s *ExcelService) validateAndMapHeaders(headerRow []string, expectedHeaders []string) (map[string]int, error) {
	headerMap := make(map[string]int)

	// Convert headers to lowercase and map to column indices
	for i, header := range headerRow {
		cleanHeader := strings.TrimSpace(strings.ToLower(header))
		headerMap[cleanHeader] = i
		log.Printf("Header %d: '%s' -> cleaned: '%s'", i, header, cleanHeader)
	}

	// Check for required headers
	missingHeaders := []string{}
	for _, expectedHeader := range expectedHeaders {
		if _, found := headerMap[expectedHeader]; !found {
			// Check if it's a required field
			if expectedHeader == "first_name" || expectedHeader == "last_name" || expectedHeader == "email" {
				missingHeaders = append(missingHeaders, expectedHeader)
			}
		}
	}

	if len(missingHeaders) > 0 {
		return nil, fmt.Errorf("required headers not found: %v. Available headers: %v", missingHeaders, headerRow)
	}

	return headerMap, nil
}

// parseEmployeeFromRow parses an employee from an Excel row
func (s *ExcelService) parseEmployeeFromRow(row []string, headerMap map[string]int, rowNumber int) (*models.Employee, []models.ValidationError) {
	var validationErrors []models.ValidationError

	// Helper function to get cell value safely
	getCellValue := func(columnName string) string {
		if colIndex, exists := headerMap[columnName]; exists && colIndex < len(row) {
			return strings.TrimSpace(row[colIndex])
		}
		return ""
	}

	// Create employee
	employee := &models.Employee{
		FirstName:   getCellValue("first_name"),
		LastName:    getCellValue("last_name"),
		CompanyName: getCellValue("company_name"),
		Address:     getCellValue("address"),
		City:        getCellValue("city"),
		County:      getCellValue("county"),
		Postal:      getCellValue("postal"),
		Phone:       getCellValue("phone"),
		Email:       getCellValue("email"),
		Web:         getCellValue("web"),
	}

	// Debug: Log first few employees
	if rowNumber <= 3 {
		log.Printf("Row %d parsed employee: FirstName='%s', LastName='%s', Email='%s'",
			rowNumber, employee.FirstName, employee.LastName, employee.Email)
	}

	// Validate employee using the service validator
	fieldErrors := s.employeeService.ValidateEmployeeData(employee)
	for _, fieldError := range fieldErrors {
		validationErrors = append(validationErrors, models.ValidationError{
			Field:   fmt.Sprintf("Row %d - %s", rowNumber, fieldError.Field),
			Message: fieldError.Message,
		})
	}

	// Note: We don't check for duplicate emails here during parsing.
	// The database layer will handle duplicates during batch insert,
	// which is more efficient and provides proper skip behavior.

	if len(validationErrors) > 0 {
		return nil, validationErrors
	}

	return employee, nil
}

// isRowEmpty checks if a row is empty
func (s *ExcelService) isRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// GetProcessingResult retrieves processing result by ID
func (s *ExcelService) GetProcessingResult(processingID string) (*models.ExcelUploadResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, exists := s.processResults[processingID]
	if !exists {
		return nil, fmt.Errorf("processing result with ID %s not found", processingID)
	}

	return result, nil
}

// ValidateExcelStructure validates Excel file structure and format only (no database operations)
func (s *ExcelService) ValidateExcelStructure(file *multipart.FileHeader) (*models.ExcelValidationResponse, error) {
	// Basic file validation
	if err := s.validateExcelFile(file); err != nil {
		return nil, err
	}

	// Open and check structure
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	xlFile, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Excel file: %w", err)
	}
	defer xlFile.Close()

	// Get the first sheet name
	sheetName := xlFile.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("Excel file has no sheets")
	}

	// Get all rows from the first sheet
	rows, err := xlFile.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel sheet: %w", err)
	}

	if len(rows) <= 1 {
		return &models.ExcelValidationResponse{
			Message:      "Excel file appears to be empty",
			TotalRecords: 0,
		}, nil
	}

	// Check headers only
	headerRow := rows[0]
	expectedHeaders := []string{
		"first_name", "last_name", "company_name", "address",
		"city", "county", "postal", "phone", "email", "web",
	}

	log.Printf("Validating Excel headers: %v", headerRow)
	_, err = s.validateAndMapHeaders(headerRow, expectedHeaders)
	if err != nil {
		return nil, err
	}

	// Count data rows (simple validation - just check if rows exist and are not empty)
	dataRowCount := 0
	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		if !s.isRowEmpty(rows[rowIndex]) {
			dataRowCount++
		}
	}

	message := fmt.Sprintf("Excel validation successful. File structure is valid with %d data rows and correct headers", dataRowCount)

	log.Printf("Excel format validation complete: %d data rows found", dataRowCount)

	return &models.ExcelValidationResponse{
		Message:      message,
		TotalRecords: dataRowCount,
	}, nil
}
