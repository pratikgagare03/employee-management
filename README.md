# Employee Management System

A Go-based employee management system that handles Excel file uploads, stores data in MySQL, and uses Redis for caching.

## Features

This system provides:
- Excel file import for employee data
- MySQL database storage with proper schema
- Redis caching with 5-minute expiration
- Complete REST API for CRUD operations
- Input validation and error handling

## Technology Stack

- **Backend**: Go with Gin web framework
- **Database**: MySQL with GORM ORM
- **Cache**: Redis for performance optimization
- **Excel Processing**: Excelize library
- **API**: RESTful endpoints with JSON responses

## Excel File Format

The Excel file should have these columns:
- first_name (required)
- last_name (required) 
- email (required, must be unique)
- company_name
- address
- city
- county
- postal
- phone
- web

## Setup and Installation

### Prerequisites
- Go 1.21 or higher
- MySQL 8.0 or higher  
- Redis server
- Git (for cloning)

### Database Configuration
```sql
CREATE DATABASE employee_management;
CREATE USER 'emp_user'@'localhost' IDENTIFIED BY 'secure_password';
GRANT ALL PRIVILEGES ON employee_management.* TO 'emp_user'@'localhost';
FLUSH PRIVILEGES;
```

### Application Setup
1. Clone the repository
2. Install dependencies:
   ```bash
   go mod tidy
   ```
3. Configure environment variables by copying `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```
4. Update the `.env` file with your database credentials:
   ```
   DB_HOST=localhost
   DB_PORT=3306
   DB_USER=emp_user
   DB_PASSWORD=secure_password
   DB_NAME=employee_management

   REDIS_HOST=localhost
   REDIS_PORT=6379
   SERVER_PORT=8081
   GIN_MODE=release
   ```

### Starting the Application
```bash
go run cmd/main.go
```

The server will start on the configured port (default: 8081).

You should see output similar to:
```
🚀 Employee Management Server starting on port 8081
📋 API Documentation: http://localhost:8081/api/health
```
## API Reference

Base URL: `http://localhost:8081`

### System Endpoints
- **GET** `/api/health` - Health check endpoint
- **GET** `/` - API documentation and welcome message

### Excel Import Endpoints
- **POST** `/api/employees/upload` - Upload and process Excel file
- **POST** `/api/employees/validate-excel` - Validate Excel file structure

### Employee Management Endpoints
- **GET** `/api/employees` - List employees with pagination and search
- **GET** `/api/employees/:id` - Retrieve specific employee
- **POST** `/api/employees` - Create new employee record
- **PUT** `/api/employees/:id` - Update existing employee
- **DELETE** `/api/employees/:id` - Remove employee record

## Usage Examples

### Excel File Upload
```bash
curl -X POST http://localhost:8081/api/employees/upload \
  -F "file=@employee_data.xlsx"
```

### List Employees with Pagination
```bash
curl "http://localhost:8081/api/employees?page=1&limit=20"
```

### Search Employees
```bash
curl "http://localhost:8081/api/employees?search=john&page=1&limit=10"
```

### Create New Employee
```bash
curl -X POST http://localhost:8081/api/employees \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "John",
    "last_name": "Doe", 
    "email": "john.doe@company.com",
    "company_name": "Tech Corp"
  }'
```

## Architecture Overview

The application follows a layered architecture pattern:

### Project Structure
```
cmd/main.go                 # Application entry point
internal/
  ├── config/              # Configuration management
  ├── database/            # Database and cache connections
  ├── handlers/            # HTTP request handlers
  ├── models/              # Data structures and DTOs
  └── services/            # Business logic layer
```

### Design Principles
- **Separation of Concerns**: Clear separation between HTTP handlers, business logic, and data access
- **Dependency Injection**: Services are injected into handlers for better testability
- **Cache-First Strategy**: Redis cache is checked before database queries
- **Error Handling**: Comprehensive error handling with meaningful messages
- **Input Validation**: Both structural and business rule validation

## Performance Features

### Caching Strategy
- Redis caching with 5-minute TTL as per requirements
- Automatic cache invalidation on data changes
- Cache-first approach for read operations
- Separate caching for individual records and paginated lists

### Database Optimizations
- Connection pooling for better resource management
- Batch processing for Excel imports
- Indexed email field for unique constraint
- Efficient pagination with LIMIT/OFFSET

### Scalability Considerations
- Stateless application design for horizontal scaling
- Asynchronous Excel processing
- Optimized database queries
- Environment-based configuration

## Configuration

### Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | MySQL server hostname | localhost |
| `DB_PORT` | MySQL server port | 3306 |
| `DB_USER` | Database username | - |
| `DB_PASSWORD` | Database password | - |
| `DB_NAME` | Database name | employee_management |
| `REDIS_HOST` | Redis server hostname | localhost |
| `REDIS_PORT` | Redis server port | 6379 |
| `SERVER_PORT` | Application server port | 8081 |
| `GIN_MODE` | Gin framework mode | release |

### File Upload Limits
- Maximum file size: 10MB
- Supported formats: .xlsx, .xls
- Processing timeout: 30 seconds

## Troubleshooting

### Common Issues

**Database Connection Failed**
```bash
# Check MySQL service
sudo systemctl status mysql
# Verify connection
mysql -u emp_user -p -e "SELECT 1"
```

**Redis Connection Failed** 
```bash
# Check Redis service
redis-cli ping
# Should return PONG
```

**Port Already in Use**
```bash
# Check what's using the port
lsof -i :8081
# Change port in .env file if needed
```
