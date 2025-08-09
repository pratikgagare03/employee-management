-- Initialize database for employee management system
CREATE DATABASE IF NOT EXISTS employee_management;
USE employee_management;

-- Grant privileges to the user
GRANT ALL PRIVILEGES ON employee_management.* TO 'user'@'%';
FLUSH PRIVILEGES;

-- The application will handle table creation through GORM migrations
