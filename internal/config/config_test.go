package config

import (
	"os"
	"testing"
	"time"
)

func TestGetDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   DatabaseConfig
		expected string
	}{
		{
			name: "standard configuration",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     3306,
				User:     "testuser",
				Password: "testpass",
				DBName:   "testdb",
			},
			expected: "testuser:testpass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name: "remote database",
			config: DatabaseConfig{
				Host:     "db.example.com",
				Port:     3307,
				User:     "admin",
				Password: "secret123",
				DBName:   "production",
			},
			expected: "admin:secret123@tcp(db.example.com:3307)/production?charset=utf8mb4&parseTime=True&loc=Local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetDSN()
			if result != tt.expected {
				t.Errorf("GetDSN() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetRedisAddr(t *testing.T) {
	tests := []struct {
		name     string
		config   RedisConfig
		expected string
	}{
		{
			name: "default redis",
			config: RedisConfig{
				Host: "localhost",
				Port: 6379,
			},
			expected: "localhost:6379",
		},
		{
			name: "custom redis",
			config: RedisConfig{
				Host: "redis.example.com",
				Port: 6380,
			},
			expected: "redis.example.com:6380",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetRedisAddr()
			if result != tt.expected {
				t.Errorf("GetRedisAddr() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	// Clear environment variables to test defaults
	envVars := []string{
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"REDIS_HOST", "REDIS_PORT", "SERVER_PORT", "GIN_MODE",
	}

	// Store original values
	originalValues := make(map[string]string)
	for _, env := range envVars {
		originalValues[env] = os.Getenv(env)
		os.Unsetenv(env)
	}

	// Restore environment after test
	defer func() {
		for env, value := range originalValues {
			if value != "" {
				os.Setenv(env, value)
			}
		}
	}()

	config := Load()

	// Test database defaults
	if config.Database.Host != "localhost" {
		t.Errorf("Expected DB host 'localhost', got '%s'", config.Database.Host)
	}
	if config.Database.Port != 3306 {
		t.Errorf("Expected DB port 3306, got %d", config.Database.Port)
	}
	if config.Database.User != "root" {
		t.Errorf("Expected DB user 'root', got '%s'", config.Database.User)
	}

	// Test Redis defaults
	if config.Redis.Host != "localhost" {
		t.Errorf("Expected Redis host 'localhost', got '%s'", config.Redis.Host)
	}
	if config.Redis.Port != 6379 {
		t.Errorf("Expected Redis port 6379, got %d", config.Redis.Port)
	}
	if config.Redis.CacheExpiry != 5*time.Minute {
		t.Errorf("Expected cache expiry 5m, got %v", config.Redis.CacheExpiry)
	}

	// Test server defaults
	if config.Server.Port != "8080" {
		t.Errorf("Expected server port '8080', got '%s'", config.Server.Port)
	}
	if config.Server.MaxFileSize != 10*1024*1024 {
		t.Errorf("Expected max file size 10MB, got %d", config.Server.MaxFileSize)
	}
}

func TestLoad_WithEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	testEnvVars := map[string]string{
		"DB_HOST":     "testhost",
		"DB_PORT":     "3307",
		"DB_USER":     "testuser",
		"REDIS_HOST":  "redishost",
		"REDIS_PORT":  "6380",
		"SERVER_PORT": "9000",
	}

	// Store original values and set test values
	originalValues := make(map[string]string)
	for key, value := range testEnvVars {
		originalValues[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	// Restore environment after test
	defer func() {
		for key, value := range originalValues {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	config := Load()

	// Test that environment variables are used
	if config.Database.Host != "testhost" {
		t.Errorf("Expected DB host 'testhost', got '%s'", config.Database.Host)
	}
	if config.Database.Port != 3307 {
		t.Errorf("Expected DB port 3307, got %d", config.Database.Port)
	}
	if config.Database.User != "testuser" {
		t.Errorf("Expected DB user 'testuser', got '%s'", config.Database.User)
	}
	if config.Redis.Host != "redishost" {
		t.Errorf("Expected Redis host 'redishost', got '%s'", config.Redis.Host)
	}
	if config.Redis.Port != 6380 {
		t.Errorf("Expected Redis port 6380, got %d", config.Redis.Port)
	}
	if config.Server.Port != "9000" {
		t.Errorf("Expected server port '9000', got '%s'", config.Server.Port)
	}
}

func TestGetEnvHelpers(t *testing.T) {
	t.Run("getEnv with existing value", func(t *testing.T) {
		os.Setenv("TEST_STRING", "testvalue")
		defer os.Unsetenv("TEST_STRING")

		result := getEnv("TEST_STRING", "default")
		if result != "testvalue" {
			t.Errorf("Expected 'testvalue', got '%s'", result)
		}
	})

	t.Run("getEnv with default", func(t *testing.T) {
		os.Unsetenv("NONEXISTENT_VAR")

		result := getEnv("NONEXISTENT_VAR", "default")
		if result != "default" {
			t.Errorf("Expected 'default', got '%s'", result)
		}
	})

	t.Run("getEnvAsInt with valid value", func(t *testing.T) {
		os.Setenv("TEST_INT", "42")
		defer os.Unsetenv("TEST_INT")

		result := getEnvAsInt("TEST_INT", 10)
		if result != 42 {
			t.Errorf("Expected 42, got %d", result)
		}
	})

	t.Run("getEnvAsInt with invalid value", func(t *testing.T) {
		os.Setenv("TEST_INT_INVALID", "notanumber")
		defer os.Unsetenv("TEST_INT_INVALID")

		result := getEnvAsInt("TEST_INT_INVALID", 10)
		if result != 10 {
			t.Errorf("Expected default 10, got %d", result)
		}
	})

	t.Run("getEnvAsDuration with valid value", func(t *testing.T) {
		os.Setenv("TEST_DURATION", "30s")
		defer os.Unsetenv("TEST_DURATION")

		result := getEnvAsDuration("TEST_DURATION", time.Minute)
		if result != 30*time.Second {
			t.Errorf("Expected 30s, got %v", result)
		}
	})

	t.Run("getEnvAsDuration with invalid value", func(t *testing.T) {
		os.Setenv("TEST_DURATION_INVALID", "invalid")
		defer os.Unsetenv("TEST_DURATION_INVALID")

		result := getEnvAsDuration("TEST_DURATION_INVALID", time.Minute)
		if result != time.Minute {
			t.Errorf("Expected default 1m, got %v", result)
		}
	})
}
