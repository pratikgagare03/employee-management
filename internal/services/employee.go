package services

import (
	"employee-management/internal/database"
	"employee-management/internal/models"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// EmployeeService handles business logic for employees
type EmployeeService struct {
	repo     database.Repository
	cache    database.CacheInterface
	validate *validator.Validate
}

// NewEmployeeService creates a new employee service
func NewEmployeeService(repo database.Repository, cache database.CacheInterface) *EmployeeService {
	return &EmployeeService{
		repo:     repo,
		cache:    cache,
		validate: validator.New(),
	}
}

// CreateEmployee creates a new employee
func (s *EmployeeService) CreateEmployee(employee *models.Employee) error {
	// Validate the employee data
	if err := s.validate.Struct(employee); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if email already exists
	existingEmployee, err := s.repo.GetEmployeeByEmail(employee.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing employee: %w", err)
	}
	if existingEmployee != nil {
		return fmt.Errorf("employee with email %s already exists", employee.Email)
	}

	// Create employee in database
	if err := s.repo.CreateEmployee(employee); err != nil {
		return fmt.Errorf("failed to create employee: %w", err)
	}

	// Cache the employee
	if err := s.cache.SetEmployee(employee); err != nil {
		log.Printf("Warning: Failed to cache employee %d: %v", employee.ID, err)
	}

	// Invalidate list caches since we added a new employee
	if err := s.cache.InvalidateEmployeeListCache(); err != nil {
		log.Printf("Warning: Failed to invalidate employee list cache: %v", err)
	}

	return nil
}

// GetEmployeeByID retrieves an employee by ID (cache-first strategy)
func (s *EmployeeService) GetEmployeeByID(id int) (*models.Employee, error) {
	// Try cache first
	employee, err := s.cache.GetEmployee(id)
	if err != nil {
		log.Printf("Warning: Cache error for employee %d: %v", id, err)
	} else if employee != nil {
		log.Printf("Cache hit for employee %d", id)
		return employee, nil
	}

	// Cache miss, get from database
	log.Printf("Cache miss for employee %d, fetching from database", id)
	employee, err = s.repo.GetEmployeeByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("employee with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get employee: %w", err)
	}

	// Cache the result
	if err := s.cache.SetEmployee(employee); err != nil {
		log.Printf("Warning: Failed to cache employee %d: %v", id, err)
	}

	return employee, nil
}

// GetAllEmployees retrieves all employees with pagination (cache-first strategy)
func (s *EmployeeService) GetAllEmployees(limit, offset int) ([]models.Employee, int64, error) {
	// Generate cache key
	cacheKey := database.GenerateListCacheKey(limit, offset, "")

	// Try cache first
	employees, total, err := s.cache.GetEmployeeList(cacheKey)
	if err != nil {
		log.Printf("Warning: Cache error for employee list: %v", err)
	} else if employees != nil {
		log.Printf("Cache hit for employee list (limit: %d, offset: %d)", limit, offset)
		return employees, total, nil
	}

	// Cache miss, get from database
	log.Printf("Cache miss for employee list, fetching from database (limit: %d, offset: %d)", limit, offset)
	employees, total, err = s.repo.GetAllEmployees(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get employees: %w", err)
	}

	// Cache the result
	if err := s.cache.SetEmployeeList(cacheKey, employees, total); err != nil {
		log.Printf("Warning: Failed to cache employee list: %v", err)
	}

	return employees, total, nil
}

// UpdateEmployee updates an existing employee
func (s *EmployeeService) UpdateEmployee(id int, updateData *models.Employee) (*models.Employee, error) {
	// Get existing employee
	existingEmployee, err := s.repo.GetEmployeeByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("employee with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get employee: %w", err)
	}

	// Check if email is being changed and if new email already exists
	if updateData.Email != "" && updateData.Email != existingEmployee.Email {
		emailEmployee, err := s.repo.GetEmployeeByEmail(updateData.Email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to check existing email: %w", err)
		}
		if emailEmployee != nil {
			return nil, fmt.Errorf("employee with email %s already exists", updateData.Email)
		}
	}

	// Update fields
	if updateData.FirstName != "" {
		existingEmployee.FirstName = updateData.FirstName
	}
	if updateData.LastName != "" {
		existingEmployee.LastName = updateData.LastName
	}
	if updateData.Email != "" {
		existingEmployee.Email = updateData.Email
	}
	if updateData.CompanyName != "" {
		existingEmployee.CompanyName = updateData.CompanyName
	}
	if updateData.Address != "" {
		existingEmployee.Address = updateData.Address
	}
	if updateData.City != "" {
		existingEmployee.City = updateData.City
	}
	if updateData.County != "" {
		existingEmployee.County = updateData.County
	}
	if updateData.Postal != "" {
		existingEmployee.Postal = updateData.Postal
	}
	if updateData.Phone != "" {
		existingEmployee.Phone = updateData.Phone
	}
	if updateData.Web != "" {
		existingEmployee.Web = updateData.Web
	}

	// Validate updated employee
	if err := s.validate.Struct(existingEmployee); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Update in database
	if err := s.repo.UpdateEmployee(existingEmployee); err != nil {
		return nil, fmt.Errorf("failed to update employee: %w", err)
	}

	// Update cache
	if err := s.cache.SetEmployee(existingEmployee); err != nil {
		log.Printf("Warning: Failed to update employee cache %d: %v", id, err)
	}

	// Invalidate list caches since data changed
	if err := s.cache.InvalidateEmployeeListCache(); err != nil {
		log.Printf("Warning: Failed to invalidate employee list cache: %v", err)
	}

	return existingEmployee, nil
}

// DeleteEmployee deletes an employee
func (s *EmployeeService) DeleteEmployee(id int) error {
	// Check if employee exists
	_, err := s.repo.GetEmployeeByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("employee with ID %d not found", id)
		}
		return fmt.Errorf("failed to get employee: %w", err)
	}

	// Delete from database
	if err := s.repo.DeleteEmployee(id); err != nil {
		return fmt.Errorf("failed to delete employee: %w", err)
	}

	// Remove from cache
	if err := s.cache.DeleteEmployee(id); err != nil {
		log.Printf("Warning: Failed to delete employee from cache %d: %v", id, err)
	}

	// Invalidate list caches since data changed
	if err := s.cache.InvalidateEmployeeListCache(); err != nil {
		log.Printf("Warning: Failed to invalidate employee list cache: %v", err)
	}

	return nil
}

// SearchEmployees searches employees by query
func (s *EmployeeService) SearchEmployees(query string, limit, offset int) ([]models.Employee, int64, error) {
	// Sanitize search query
	query = strings.TrimSpace(query)
	if query == "" {
		return s.GetAllEmployees(limit, offset)
	}

	// Generate cache key for search
	cacheKey := database.GenerateListCacheKey(limit, offset, query)

	// Try cache first
	employees, total, err := s.cache.GetEmployeeList(cacheKey)
	if err != nil {
		log.Printf("Warning: Cache error for search: %v", err)
	} else if employees != nil {
		log.Printf("Cache hit for search: %s (limit: %d, offset: %d)", query, limit, offset)
		return employees, total, nil
	}

	// Cache miss, search in database
	log.Printf("Cache miss for search, querying database: %s (limit: %d, offset: %d)", query, limit, offset)
	employees, total, err = s.repo.SearchEmployees(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search employees: %w", err)
	}

	// Cache the search result
	if err := s.cache.SetEmployeeList(cacheKey, employees, total); err != nil {
		log.Printf("Warning: Failed to cache search result: %v", err)
	}

	return employees, total, nil
}

// GetEmployeeResponse converts employee to response format
func (s *EmployeeService) GetEmployeeResponse(id int) (*models.EmployeeResponse, error) {
	employee, err := s.GetEmployeeByID(id)
	if err != nil {
		return nil, err
	}

	response := employee.ToResponse()
	return &response, nil
}

// GetEmployeeListResponse converts employee list to response format
func (s *EmployeeService) GetEmployeeListResponse(limit, offset int) ([]models.EmployeeResponse, int64, error) {
	employees, total, err := s.GetAllEmployees(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]models.EmployeeResponse, len(employees))
	for i, emp := range employees {
		responses[i] = emp.ToResponse()
	}

	return responses, total, nil
}

// ValidateEmployeeData validates employee data
func (s *EmployeeService) ValidateEmployeeData(employee *models.Employee) []models.ValidationError {
	var validationErrors []models.ValidationError

	if err := s.validate.Struct(employee); err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors = append(validationErrors, models.ValidationError{
				Field:   err.Field(),
				Message: getValidationMessage(err),
			})
		}
	}

	return validationErrors
}

// getValidationMessage returns user-friendly validation messages
func getValidationMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", err.Field())
	case "email":
		return "Invalid email format"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", err.Field(), err.Param())
	case "max":
		return fmt.Sprintf("%s must not exceed %s characters", err.Field(), err.Param())
	case "url":
		return "Invalid URL format"
	default:
		return fmt.Sprintf("%s is invalid", err.Field())
	}
}
