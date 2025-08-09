package database

import (
	"employee-management/internal/config"
	"employee-management/internal/models"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB holds the database connection
type DB struct {
	*gorm.DB
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.DatabaseConfig) (*DB, error) {
	// Configure GORM logger
	logLevel := logger.Silent
	if cfg.SSLMode == "debug" {
		logLevel = logger.Info
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	}

	// Create database connection
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &DB{db}, nil
}

// AutoMigrate runs database migrations
func (db *DB) AutoMigrate() error {
	log.Println("Running database migrations...")

	err := db.DB.AutoMigrate(
		&models.Employee{},
	)

	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Health checks database connectivity
func (db *DB) Health() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Repository interface defines database operations
type Repository interface {
	// Employee operations
	CreateEmployee(employee *models.Employee) error
	GetEmployeeByID(id int) (*models.Employee, error)
	GetEmployeeByEmail(email string) (*models.Employee, error)
	GetAllEmployees(limit, offset int) ([]models.Employee, int64, error)
	UpdateEmployee(employee *models.Employee) error
	DeleteEmployee(id int) error

	// Batch operations for Excel import
	CreateEmployeesInBatch(employees []models.Employee) error
	CreateEmployeesInBatchWithResult(employees []models.Employee) (int, int, []string, error)
	SearchEmployees(query string, limit, offset int) ([]models.Employee, int64, error)
}

// EmployeeRepository implements Repository interface
type EmployeeRepository struct {
	db *DB
}

// NewEmployeeRepository creates a new employee repository
func NewEmployeeRepository(db *DB) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

// CreateEmployee creates a new employee
func (r *EmployeeRepository) CreateEmployee(employee *models.Employee) error {
	return r.db.Create(employee).Error
}

// GetEmployeeByID retrieves an employee by ID
func (r *EmployeeRepository) GetEmployeeByID(id int) (*models.Employee, error) {
	var employee models.Employee
	err := r.db.First(&employee, id).Error
	if err != nil {
		return nil, err
	}
	return &employee, nil
}

// GetEmployeeByEmail retrieves an employee by email
func (r *EmployeeRepository) GetEmployeeByEmail(email string) (*models.Employee, error) {
	var employee models.Employee
	err := r.db.Where("email = ?", email).First(&employee).Error
	if err != nil {
		return nil, err
	}
	return &employee, nil
}

// GetAllEmployees retrieves all employees with pagination
func (r *EmployeeRepository) GetAllEmployees(limit, offset int) ([]models.Employee, int64, error) {
	var employees []models.Employee
	var total int64

	// Count total records
	if err := r.db.Model(&models.Employee{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated records
	err := r.db.Limit(limit).Offset(offset).Find(&employees).Error
	if err != nil {
		return nil, 0, err
	}

	return employees, total, nil
}

// UpdateEmployee updates an existing employee
func (r *EmployeeRepository) UpdateEmployee(employee *models.Employee) error {
	return r.db.Save(employee).Error
}

// DeleteEmployee deletes an employee by ID
func (r *EmployeeRepository) DeleteEmployee(id int) error {
	return r.db.Delete(&models.Employee{}, id).Error
}

// CreateEmployeesInBatch creates multiple employees in a single transaction
func (r *EmployeeRepository) CreateEmployeesInBatch(employees []models.Employee) error {
	if len(employees) == 0 {
		return nil
	}

	// Use transaction to ensure data consistency
	return r.db.Transaction(func(tx *gorm.DB) error {
		batchSize := 100

		// Process in batches
		for i := 0; i < len(employees); i += batchSize {
			end := i + batchSize
			if end > len(employees) {
				end = len(employees)
			}

			batch := employees[i:end]

			// Try batch insert first
			err := tx.CreateInBatches(batch, batchSize).Error
			if err != nil {
				// If batch insert fails, try individual inserts to handle duplicates
				for _, employee := range batch {
					err := tx.Create(&employee).Error
					if err != nil {
						// Skip duplicate email errors, log others
						if !isDuplicateKeyError(err) {
							log.Printf("Failed to insert employee %s %s (%s): %v",
								employee.FirstName, employee.LastName, employee.Email, err)
							return err
						}
						// Log duplicate but continue
						log.Printf("Skipping duplicate email: %s", employee.Email)
					}
				}
			}
		}
		return nil
	})
}

// CreateEmployeesInBatchWithResult creates multiple employees and returns detailed results
func (r *EmployeeRepository) CreateEmployeesInBatchWithResult(employees []models.Employee) (int, int, []string, error) {
	if len(employees) == 0 {
		return 0, 0, nil, nil
	}

	var inserted, skipped int
	var duplicateEmails []string

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, employee := range employees {
			err := tx.Create(&employee).Error
			if err != nil {
				if isDuplicateKeyError(err) {
					skipped++
					duplicateEmails = append(duplicateEmails, employee.Email)
				} else {
					return err
				}
			} else {
				inserted++
			}
		}
		return nil
	})

	return inserted, skipped, duplicateEmails, err
}

// isDuplicateKeyError checks if the error is a duplicate key constraint violation
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Duplicate entry") ||
		strings.Contains(errStr, "UNIQUE constraint failed") ||
		strings.Contains(errStr, "duplicate key")
}

// SearchEmployees searches employees by name, email, or company
func (r *EmployeeRepository) SearchEmployees(query string, limit, offset int) ([]models.Employee, int64, error) {
	var employees []models.Employee
	var total int64

	// Build search query
	searchQuery := "%" + query + "%"
	whereClause := r.db.Where("first_name LIKE ? OR last_name LIKE ? OR email LIKE ? OR company_name LIKE ?",
		searchQuery, searchQuery, searchQuery, searchQuery)

	// Count total matching records
	if err := whereClause.Model(&models.Employee{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated matching records
	err := whereClause.Limit(limit).Offset(offset).Find(&employees).Error
	if err != nil {
		return nil, 0, err
	}

	return employees, total, nil
}
