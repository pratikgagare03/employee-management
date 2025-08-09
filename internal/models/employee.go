package models

import (
	"time"
)

// Employee represents the structure of employee data from Excel file
type Employee struct {
	ID          int       `json:"id" gorm:"primaryKey;autoIncrement"`
	FirstName   string    `json:"first_name" gorm:"column:first_name;type:varchar(50);not null" validate:"required,min=2,max=50"`
	LastName    string    `json:"last_name" gorm:"column:last_name;type:varchar(50);not null" validate:"required,min=2,max=50"`
	CompanyName string    `json:"company_name" gorm:"column:company_name;type:varchar(100)" validate:"max=100"`
	Address     string    `json:"address" gorm:"column:address;type:varchar(255)" validate:"max=255"`
	City        string    `json:"city" gorm:"column:city;type:varchar(50)" validate:"max=50"`
	County      string    `json:"county" gorm:"column:county;type:varchar(50)" validate:"max=50"`
	Postal      string    `json:"postal" gorm:"column:postal;type:varchar(20)" validate:"max=20"`
	Phone       string    `json:"phone" gorm:"column:phone;type:varchar(20)" validate:"max=20"`
	Email       string    `json:"email" gorm:"column:email;type:varchar(255);uniqueIndex" validate:"required,email,max=255"`
	Web         string    `json:"web" gorm:"column:web;type:varchar(255)" validate:"omitempty,url"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName specifies the table name for GORM
func (Employee) TableName() string {
	return "employees"
}

// EmployeeResponse represents the response structure for API
type EmployeeResponse struct {
	ID          int    `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	CompanyName string `json:"company_name"`
	Address     string `json:"address"`
	City        string `json:"city"`
	County      string `json:"county"`
	Postal      string `json:"postal"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Web         string `json:"web"`
	FullName    string `json:"full_name"`
}

// ToResponse converts Employee to EmployeeResponse
func (e *Employee) ToResponse() EmployeeResponse {
	return EmployeeResponse{
		ID:          e.ID,
		FirstName:   e.FirstName,
		LastName:    e.LastName,
		CompanyName: e.CompanyName,
		Address:     e.Address,
		City:        e.City,
		County:      e.County,
		Postal:      e.Postal,
		Phone:       e.Phone,
		Email:       e.Email,
		Web:         e.Web,
		FullName:    e.FirstName + " " + e.LastName,
	}
}

// ExcelUploadResponse represents the response after Excel upload
type ExcelUploadResponse struct {
	Message         string   `json:"message"`
	TotalRecords    int      `json:"total_records"`
	ValidRecords    int      `json:"valid_records"`
	InvalidRecords  int      `json:"invalid_records"`
	InsertedRecords int      `json:"inserted_records"`
	SkippedRecords  int      `json:"skipped_records"`
	DuplicateEmails []string `json:"duplicate_emails,omitempty"`
	ProcessingID    string   `json:"processing_id,omitempty"`
}

// ValidationError represents validation errors
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorResponse represents error response structure
type ErrorResponse struct {
	Error   string            `json:"error"`
	Details []ValidationError `json:"details,omitempty"`
}
