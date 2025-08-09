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
	// Load configuration from .env file
	cfg := config.Load()

	// Set Gin mode based on environment
	gin.SetMode(cfg.Server.Mode)

	// Initialize database connection
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Auto-migrate database schema
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize Redis cache
	cache, err := database.NewRedisClient(&cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer cache.Close()

	// Initialize repositories
	employeeRepo := database.NewEmployeeRepository(db)

	// Initialize services
	employeeService := services.NewEmployeeService(employeeRepo, cache)
	excelService := services.NewExcelService(employeeService)

	// Initialize handlers
	employeeHandler := handlers.NewEmployeeHandler(employeeService, excelService)

	// Setup routes
	router := setupRoutes(employeeHandler)

	// Configure server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	log.Printf("🚀 Employee Management Server starting on port %s", cfg.Server.Port)
	log.Printf("📋 API Documentation: http://localhost:%s/api/health", cfg.Server.Port)
	log.Printf("🔍 Environment: %s", cfg.Server.Mode)

	// Start server
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupRoutes configures all API routes
func setupRoutes(employeeHandler *handlers.EmployeeHandler) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// API routes
	api := router.Group("/api")
	{
		// Health check
		api.GET("/health", employeeHandler.HealthCheck)

		// Employee routes
		employees := api.Group("/employees")
		{
			// Excel upload
			employees.POST("/upload", employeeHandler.UploadExcel)
			employees.POST("/validate-excel", employeeHandler.ValidateExcel)

			// CRUD operations
			employees.GET("", employeeHandler.GetEmployees)          // GET /api/employees?page=1&limit=10&search=john
			employees.POST("", employeeHandler.CreateEmployee)       // POST /api/employees
			employees.GET("/:id", employeeHandler.GetEmployee)       // GET /api/employees/1
			employees.PUT("/:id", employeeHandler.UpdateEmployee)    // PUT /api/employees/1
			employees.DELETE("/:id", employeeHandler.DeleteEmployee) // DELETE /api/employees/1
		}
	}

	// Welcome route
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to Employee Management API",
			"version": "1.0.0",
			"endpoints": gin.H{
				"health":          "GET /api/health",
				"upload_excel":    "POST /api/employees/upload",
				"validate_excel":  "POST /api/employees/validate-excel",
				"list_employees":  "GET /api/employees",
				"get_employee":    "GET /api/employees/:id",
				"create_employee": "POST /api/employees",
				"update_employee": "PUT /api/employees/:id",
				"delete_employee": "DELETE /api/employees/:id",
			},
		})
	})

	return router
}

// corsMiddleware adds CORS headers for API access
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
