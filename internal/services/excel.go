package services

import (
	"bytes"
	"employee-management/internal/models"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"strings"
	"sync"

	"github.com/xuri/excelize/v2"
)

// ExcelService handles Excel file processing
type ExcelService struct {
	employeeService *EmployeeService
	mu              sync.RWMutex
	processResults  map[string]*models.ExcelUploadResponse
}

// NewExcelService creates a new Excel service
func NewExcelService(employeeService *EmployeeService) *ExcelService {
	return &ExcelService{
		employeeService: employeeService,
		processResults:  make(map[string]*models.ExcelUploadResponse),
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
	// Check file size (max 10MB as configured)
	maxSize := int64(10 * 1024 * 1024) // 10MB
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

	// Validate headers
	headerMap, err := s.validateAndMapHeaders(headerRow, expectedHeaders)
	if err != nil {
		return nil, nil, fmt.Errorf("header validation failed: %w", err)
	}

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
	}

	// Check for required headers
	for _, expectedHeader := range expectedHeaders {
		if _, found := headerMap[expectedHeader]; !found && (expectedHeader == "first_name" || expectedHeader == "last_name" || expectedHeader == "email") {
			// Required fields
			return nil, fmt.Errorf("required header '%s' not found in Excel file", expectedHeader)
		}
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

	// Validate employee using the service validator
	fieldErrors := s.employeeService.ValidateEmployeeData(employee)
	for _, fieldError := range fieldErrors {
		validationErrors = append(validationErrors, models.ValidationError{
			Field:   fmt.Sprintf("Row %d - %s", rowNumber, fieldError.Field),
			Message: fieldError.Message,
		})
	}

	// Additional business validations
	if employee.Email != "" {
		// Check if email already exists (this is expensive, but necessary for data integrity)
		existingEmployee, err := s.employeeService.repo.GetEmployeeByEmail(employee.Email)
		if err == nil && existingEmployee != nil {
			validationErrors = append(validationErrors, models.ValidationError{
				Field:   fmt.Sprintf("Row %d - Email", rowNumber),
				Message: fmt.Sprintf("Email '%s' already exists in database", employee.Email),
			})
		}
	}

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

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

// ValidateExcelStructure validates Excel file structure without processing data
func (s *ExcelService) ValidateExcelStructure(file *multipart.FileHeader) (*models.ExcelUploadResponse, error) {
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
		return &models.ExcelUploadResponse{
			Message:         "Excel file appears to be empty",
			TotalRecords:    0,
			ValidRecords:    0,
			InvalidRecords:  0,
			InsertedRecords: 0,
			SkippedRecords:  0,
		}, nil
	}

	// Check headers
	headerRow := rows[0]
	// Use the same expected headers as upload process for consistency
	expectedHeaders := []string{
		"first_name", "last_name", "company_name", "address",
		"city", "county", "postal", "phone", "email", "web",
	}
	headerMap, err := s.validateAndMapHeaders(headerRow, expectedHeaders)
	if err != nil {
		return nil, err
	}

	// Perform detailed validation analysis
	var potentialDuplicates []string
	var validCount, invalidCount int
	emailsSeen := make(map[string]bool)

	log.Printf("Starting validation analysis for %d rows", len(rows)-1)

	// Analyze each row for validation and duplicates
	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]

		// Skip empty rows
		if s.isRowEmpty(row) {
			continue
		}

		// Create employee for validation (same as upload process)
		employee := &models.Employee{
			FirstName:   getCellValue(row, headerMap, "first_name"),
			LastName:    getCellValue(row, headerMap, "last_name"),
			CompanyName: getCellValue(row, headerMap, "company_name"),
			Address:     getCellValue(row, headerMap, "address"),
			City:        getCellValue(row, headerMap, "city"),
			County:      getCellValue(row, headerMap, "county"),
			Postal:      getCellValue(row, headerMap, "postal"),
			Phone:       getCellValue(row, headerMap, "phone"),
			Email:       getCellValue(row, headerMap, "email"),
			Web:         getCellValue(row, headerMap, "web"),
		}

		// Basic field validation (same logic as upload)
		fieldErrors := s.employeeService.ValidateEmployeeData(employee)

		// Check for database duplicates (same logic as upload)
		if employee.Email != "" {
			existingEmployee, err := s.employeeService.repo.GetEmployeeByEmail(employee.Email)
			if err == nil && existingEmployee != nil {
				fieldErrors = append(fieldErrors, models.ValidationError{
					Field:   fmt.Sprintf("Row %d - Email", rowIndex+1),
					Message: fmt.Sprintf("Email '%s' already exists in database", employee.Email),
				})
				// Track this email as a duplicate
				if !emailsSeen[employee.Email] {
					potentialDuplicates = append(potentialDuplicates, employee.Email)
					emailsSeen[employee.Email] = true
				}
			}
		}

		// Count valid vs invalid (same logic as upload)
		if len(fieldErrors) > 0 {
			invalidCount++
		} else {
			validCount++
		}
	}

	log.Printf("Validation complete: valid=%d, invalid=%d, duplicates=%d", validCount, invalidCount, len(potentialDuplicates))

	// Prepare detailed response
	totalRows := len(rows) - 1
	duplicateCount := len(potentialDuplicates)

	message := fmt.Sprintf("Excel validation complete. Found %d data rows: %d valid, %d invalid",
		totalRows, validCount, invalidCount)

	if duplicateCount > 0 {
		message += fmt.Sprintf(", %d potential duplicates detected", duplicateCount)
	}

	// Limit duplicate emails shown
	maxDuplicatesToShow := 10
	duplicateEmailsToShow := potentialDuplicates
	if len(potentialDuplicates) > maxDuplicatesToShow {
		duplicateEmailsToShow = potentialDuplicates[:maxDuplicatesToShow]
		message += fmt.Sprintf(" (showing first %d)", maxDuplicatesToShow)
	}

	return &models.ExcelUploadResponse{
		Message:         message,
		TotalRecords:    totalRows,
		ValidRecords:    validCount,
		InvalidRecords:  invalidCount,
		InsertedRecords: 0, // Not applicable for validation
		SkippedRecords:  duplicateCount,
		DuplicateEmails: duplicateEmailsToShow,
	}, nil
}

// Helper function to get cell value safely
func getCellValue(row []string, headerMap map[string]int, columnName string) string {
	if colIndex, exists := headerMap[columnName]; exists && colIndex < len(row) {
		return strings.TrimSpace(row[colIndex])
	}
	return ""
}
