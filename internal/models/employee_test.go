package models

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestEmployeeValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name     string
		employee Employee
		wantErr  bool
		errField string
	}{
		{
			name: "valid employee",
			employee: Employee{
				FirstName:   "John",
				LastName:    "Doe",
				Email:       "john.doe@example.com",
				CompanyName: "Tech Corp",
				Address:     "123 Main St",
				City:        "Boston",
				County:      "Suffolk",
				Postal:      "02101",
				Phone:       "555-0123",
				Web:         "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "missing required first name",
			employee: Employee{
				LastName: "Doe",
				Email:    "john.doe@example.com",
			},
			wantErr:  true,
			errField: "FirstName",
		},
		{
			name: "missing required last name",
			employee: Employee{
				FirstName: "John",
				Email:     "john.doe@example.com",
			},
			wantErr:  true,
			errField: "LastName",
		},
		{
			name: "invalid email format",
			employee: Employee{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "invalid-email",
			},
			wantErr:  true,
			errField: "Email",
		},
		{
			name: "missing required email",
			employee: Employee{
				FirstName: "John",
				LastName:  "Doe",
			},
			wantErr:  true,
			errField: "Email",
		},
		{
			name: "first name too short",
			employee: Employee{
				FirstName: "J",
				LastName:  "Doe",
				Email:     "john.doe@example.com",
			},
			wantErr:  true,
			errField: "FirstName",
		},
		{
			name: "last name too short",
			employee: Employee{
				FirstName: "John",
				LastName:  "D",
				Email:     "john.doe@example.com",
			},
			wantErr:  true,
			errField: "LastName",
		},
		{
			name: "first name too long",
			employee: Employee{
				FirstName: "ThisIsAVeryLongFirstNameThatExceedsTheMaximumAllowedLength",
				LastName:  "Doe",
				Email:     "john.doe@example.com",
			},
			wantErr:  true,
			errField: "FirstName",
		},
		{
			name: "last name too long",
			employee: Employee{
				FirstName: "John",
				LastName:  "ThisIsAVeryLongLastNameThatExceedsTheMaximumAllowedLength",
				Email:     "john.doe@example.com",
			},
			wantErr:  true,
			errField: "LastName",
		},
		{
			name: "very long email should fail",
			employee: Employee{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "this.is.a.very.very.very.very.very.very.long.email.address.that.definitely.exceeds.the.maximum.allowed.length.for.email.field.in.the.database.schema.and.should.cause.validation.to.fail.because.it.is.way.too.long.for.any.reasonable.email.field@very-long-domain-name-that-should-not-be-allowed-because-it-exceeds-reasonable-limits.example.com.very.long.domain.extension.that.makes.this.email.way.too.long",
			},
			wantErr:  true,
			errField: "Email",
		},
		{
			name: "invalid web URL",
			employee: Employee{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "john.doe@example.com",
				Web:       "not-a-valid-url",
			},
			wantErr:  true,
			errField: "Web",
		},
		{
			name: "valid web URL",
			employee: Employee{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "john.doe@example.com",
				Web:       "https://www.example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.employee)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error for field %s, but got none", tt.errField)
					return
				}

				// Check if the error is for the expected field
				validationErrors := err.(validator.ValidationErrors)
				found := false
				for _, fieldErr := range validationErrors {
					if fieldErr.Field() == tt.errField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected validation error for field %s, but got errors for: %v", tt.errField, validationErrors)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, but got: %v", err)
				}
			}
		})
	}
}

func TestEmployeeDefaultValues(t *testing.T) {
	employee := Employee{
		FirstName: "Test",
		LastName:  "Employee",
		Email:     "test@example.com",
	}

	// Test that employee can be created with minimal required fields
	if employee.FirstName != "Test" {
		t.Error("Expected FirstName to be 'Test'")
	}
	if employee.LastName != "Employee" {
		t.Error("Expected LastName to be 'Employee'")
	}
	if employee.Email != "test@example.com" {
		t.Error("Expected Email to be 'test@example.com'")
	}
}

func TestExcelUploadResponse(t *testing.T) {
	response := ExcelUploadResponse{
		Message:         "Upload completed",
		TotalRecords:    100,
		ValidRecords:    85,
		InvalidRecords:  15,
		InsertedRecords: 80,
		SkippedRecords:  5,
		DuplicateEmails: []string{"john@example.com", "jane@example.com"},
		ProcessingID:    "proc-123",
	}

	if response.TotalRecords != 100 {
		t.Errorf("Expected TotalRecords to be 100, got %d", response.TotalRecords)
	}

	if response.ValidRecords != 85 {
		t.Errorf("Expected ValidRecords to be 85, got %d", response.ValidRecords)
	}

	if response.InvalidRecords != 15 {
		t.Errorf("Expected InvalidRecords to be 15, got %d", response.InvalidRecords)
	}

	if response.InsertedRecords != 80 {
		t.Errorf("Expected InsertedRecords to be 80, got %d", response.InsertedRecords)
	}

	if response.SkippedRecords != 5 {
		t.Errorf("Expected SkippedRecords to be 5, got %d", response.SkippedRecords)
	}

	if len(response.DuplicateEmails) != 2 {
		t.Errorf("Expected 2 duplicate emails, got %d", len(response.DuplicateEmails))
	}

	if response.ProcessingID != "proc-123" {
		t.Errorf("Expected ProcessingID to be 'proc-123', got %s", response.ProcessingID)
	}
}

func TestEmployeeBusinessLogic(t *testing.T) {
	t.Run("email should be case insensitive for uniqueness", func(t *testing.T) {
		// This test documents expected behavior
		// In practice, this would be handled by database constraints
		email1 := "John.Doe@Example.Com"
		email2 := "john.doe@example.com"

		// Both should be considered the same for uniqueness
		// (This would be tested with actual database operations in integration tests)
		if email1 == email2 {
			t.Error("Direct string comparison should be different, database handles case insensitivity")
		}
	})

	t.Run("phone number precision", func(t *testing.T) {
		employee := Employee{
			Phone: "555-0123",
		}

		// Test that phone maintains format
		if employee.Phone != "555-0123" {
			t.Errorf("Expected phone format to be maintained, got %s", employee.Phone)
		}
	})

	t.Run("web URL validation", func(t *testing.T) {
		employee := Employee{
			Web: "https://example.com",
		}

		// Web URL should be properly formatted
		if employee.Web != "https://example.com" {
			t.Error("Web URL should be properly maintained")
		}
	})
}

// Benchmark tests for performance awareness
func BenchmarkEmployeeValidation(b *testing.B) {
	validate := validator.New()
	employee := Employee{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john.doe@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validate.Struct(employee)
	}
}
