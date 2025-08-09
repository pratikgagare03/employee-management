package main

import (
	"employee-management/internal/config"
	"employee-management/internal/database"
	"employee-management/internal/handlers"
	"employee-management/internal/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize Redis
	cache, err := database.NewRedisClient(&cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer cache.Close()

	// Initialize services
	employeeRepo := database.NewEmployeeRepository(db)
	employeeService := services.NewEmployeeService(employeeRepo, cache)
	excelService := services.NewExcelService(employeeService, cfg)
	employeeHandler := handlers.NewEmployeeHandler(employeeService, excelService)

	// Setup router
	router := setupRoutes(employeeHandler)

	// Start server
	log.Printf("🚀 Server starting on port %s", cfg.Server.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Server.Port, router))
}

// setupRoutes configures all API routes
func setupRoutes(employeeHandler *handlers.EmployeeHandler) *gin.Engine {
	router := gin.Default()

	// API routes
	api := router.Group("/api")
	{
		api.GET("/health", employeeHandler.HealthCheck)

		employees := api.Group("/employees")
		{
			employees.POST("/upload", employeeHandler.UploadExcel)
			employees.POST("/validate-excel", employeeHandler.ValidateExcel)
			employees.GET("", employeeHandler.GetEmployees)
			employees.POST("", employeeHandler.CreateEmployee)
			employees.GET("/:id", employeeHandler.GetEmployee)
			employees.PUT("/:id", employeeHandler.UpdateEmployee)
			employees.DELETE("/:id", employeeHandler.DeleteEmployee)
		}

		// Job status routes
		jobs := api.Group("/jobs")
		{
			jobs.GET("/:id", employeeHandler.GetJobStatus)
		}
	}

	return router
}
