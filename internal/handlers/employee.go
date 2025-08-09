package handlers

import (
	"employee-management/internal/models"
	"employee-management/internal/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// EmployeeHandler handles HTTP requests for employees
type EmployeeHandler struct {
	employeeService *services.EmployeeService
	excelService    *services.ExcelService
}

// NewEmployeeHandler creates a new employee handler
func NewEmployeeHandler(employeeService *services.EmployeeService, excelService *services.ExcelService) *EmployeeHandler {
	return &EmployeeHandler{
		employeeService: employeeService,
		excelService:    excelService,
	}
}

// UploadExcel handles Excel file upload and processing
// POST /api/employees/upload
func (h *EmployeeHandler) UploadExcel(c *gin.Context) {
	// Parse multipart form
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "No file uploaded",
			Details: []models.ValidationError{
				{Field: "file", Message: "Please select an Excel file to upload"},
			},
		})
		return
	}

	// Process Excel file
	response, err := h.excelService.ProcessExcelFile(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Failed to process Excel file",
			Details: []models.ValidationError{
				{Field: "file", Message: err.Error()},
			},
		})
		return
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// ValidateExcel validates Excel file structure without processing
// POST /api/employees/validate-excel
func (h *EmployeeHandler) ValidateExcel(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "No file uploaded",
		})
		return
	}

	response, err := h.excelService.ValidateExcelStructure(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// GetEmployees retrieves all employees with pagination
// GET /api/employees?page=1&limit=10&search=john
func (h *EmployeeHandler) GetEmployees(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	search := c.Query("search")

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	var employees []models.EmployeeResponse
	var total int64
	var err error

	// Check if search query is provided
	if search != "" {
		// Search employees
		empList, totalCount, searchErr := h.employeeService.SearchEmployees(search, limit, offset)
		if searchErr != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to search employees",
			})
			return
		}

		// Convert to response format
		employees = make([]models.EmployeeResponse, len(empList))
		for i, emp := range empList {
			employees[i] = emp.ToResponse()
		}
		total = totalCount
	} else {
		// Get all employees
		employees, total, err = h.employeeService.GetEmployeeListResponse(limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to retrieve employees",
			})
			return
		}
	}

	// Calculate pagination info
	totalPages := (total + int64(limit) - 1) / int64(limit)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"employees": employees,
			"pagination": gin.H{
				"page":        page,
				"limit":       limit,
				"total":       total,
				"total_pages": totalPages,
				"has_next":    page < int(totalPages),
				"has_prev":    page > 1,
			},
			"search": search,
		},
	})
}

// GetEmployee retrieves a single employee by ID
// GET /api/employees/:id
func (h *EmployeeHandler) GetEmployee(c *gin.Context) {
	// Parse employee ID
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid employee ID",
		})
		return
	}

	// Get employee
	employee, err := h.employeeService.GetEmployeeResponse(id)
	if err != nil {
		if err.Error() == "employee with ID "+idStr+" not found" {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "Employee not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to retrieve employee",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    employee,
	})
}

// CreateEmployee creates a new employee
// POST /api/employees
func (h *EmployeeHandler) CreateEmployee(c *gin.Context) {
	var employee models.Employee

	// Bind JSON to employee struct
	if err := c.ShouldBindJSON(&employee); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request data",
			Details: []models.ValidationError{
				{Field: "body", Message: err.Error()},
			},
		})
		return
	}

	// Validate employee data
	validationErrors := h.employeeService.ValidateEmployeeData(&employee)
	if len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Details: validationErrors,
		})
		return
	}

	// Create employee
	if err := h.employeeService.CreateEmployee(&employee); err != nil {
		if err.Error() == "employee with email "+employee.Email+" already exists" {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error: "Employee with this email already exists",
			})
		} else {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to create employee",
			})
		}
		return
	}

	// Return created employee
	response := employee.ToResponse()
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    response,
		"message": "Employee created successfully",
	})
}

// UpdateEmployee updates an existing employee
// PUT /api/employees/:id
func (h *EmployeeHandler) UpdateEmployee(c *gin.Context) {
	// Parse employee ID
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid employee ID",
		})
		return
	}

	var updateData models.Employee

	// Bind JSON to employee struct
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request data",
			Details: []models.ValidationError{
				{Field: "body", Message: err.Error()},
			},
		})
		return
	}

	// Update employee
	updatedEmployee, err := h.employeeService.UpdateEmployee(id, &updateData)
	if err != nil {
		if err.Error() == "employee with ID "+idStr+" not found" {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "Employee not found",
			})
		} else if err.Error() == "employee with email "+updateData.Email+" already exists" {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error: "Employee with this email already exists",
			})
		} else {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to update employee",
			})
		}
		return
	}

	// Return updated employee
	response := updatedEmployee.ToResponse()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
		"message": "Employee updated successfully",
	})
}

// DeleteEmployee deletes an employee
// DELETE /api/employees/:id
func (h *EmployeeHandler) DeleteEmployee(c *gin.Context) {
	// Parse employee ID
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid employee ID",
		})
		return
	}

	// Delete employee
	deletedEmployee, err := h.employeeService.DeleteEmployee(id)
	if err != nil {
		if err.Error() == "employee with ID "+idStr+" not found" {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "Employee not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to delete employee",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Employee deleted successfully",
		"data":    deletedEmployee,
	})
}

// HealthCheck checks if the service is healthy
// GET /api/health
func (h *EmployeeHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "Employee Management Service is running",
		"version": "1.0.0",
	})
}
