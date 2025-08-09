package database

import (
	"context"
	"employee-management/internal/config"
	"employee-management/internal/models"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client
type RedisClient struct {
	client *redis.Client
	ctx    context.Context
	expiry time.Duration
}

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg *config.RedisConfig) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisAddr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	ctx := context.Background()

	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client: rdb,
		ctx:    ctx,
		expiry: cfg.CacheExpiry, // 5 minutes as required
	}, nil
}

// CacheInterface defines Redis operations
type CacheInterface interface {
	// Employee caching
	SetEmployee(employee *models.Employee) error
	GetEmployee(id int) (*models.Employee, error)
	DeleteEmployee(id int) error

	// Employee list caching
	SetEmployeeList(key string, employees []models.Employee, total int64) error
	GetEmployeeList(key string) ([]models.Employee, int64, error)

	// Cache invalidation
	InvalidateEmployeeCache() error
	InvalidateEmployeeListCache() error

	// Health check
	Health() error
	Close() error
}

// SetEmployee caches a single employee
func (r *RedisClient) SetEmployee(employee *models.Employee) error {
	key := fmt.Sprintf("employee:%d", employee.ID)

	data, err := json.Marshal(employee)
	if err != nil {
		return fmt.Errorf("failed to marshal employee: %w", err)
	}

	err = r.client.Set(r.ctx, key, data, r.expiry).Err()
	if err != nil {
		return fmt.Errorf("failed to cache employee: %w", err)
	}

	return nil
}

// GetEmployee retrieves a cached employee
func (r *RedisClient) GetEmployee(id int) (*models.Employee, error) {
	key := fmt.Sprintf("employee:%d", id)

	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get cached employee: %w", err)
	}

	var employee models.Employee
	err = json.Unmarshal([]byte(data), &employee)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached employee: %w", err)
	}

	return &employee, nil
}

// DeleteEmployee removes an employee from cache
func (r *RedisClient) DeleteEmployee(id int) error {
	key := fmt.Sprintf("employee:%d", id)
	return r.client.Del(r.ctx, key).Err()
}

// EmployeeListData represents cached employee list with metadata
type EmployeeListData struct {
	Employees []models.Employee `json:"employees"`
	Total     int64             `json:"total"`
	CachedAt  time.Time         `json:"cached_at"`
}

// SetEmployeeList caches employee list with pagination info
func (r *RedisClient) SetEmployeeList(key string, employees []models.Employee, total int64) error {
	cacheKey := fmt.Sprintf("employee_list:%s", key)

	listData := EmployeeListData{
		Employees: employees,
		Total:     total,
		CachedAt:  time.Now(),
	}

	data, err := json.Marshal(listData)
	if err != nil {
		return fmt.Errorf("failed to marshal employee list: %w", err)
	}

	err = r.client.Set(r.ctx, cacheKey, data, r.expiry).Err()
	if err != nil {
		return fmt.Errorf("failed to cache employee list: %w", err)
	}

	return nil
}

// GetEmployeeList retrieves cached employee list
func (r *RedisClient) GetEmployeeList(key string) ([]models.Employee, int64, error) {
	cacheKey := fmt.Sprintf("employee_list:%s", key)

	data, err := r.client.Get(r.ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, 0, nil // Cache miss
		}
		return nil, 0, fmt.Errorf("failed to get cached employee list: %w", err)
	}

	var listData EmployeeListData
	err = json.Unmarshal([]byte(data), &listData)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal cached employee list: %w", err)
	}

	return listData.Employees, listData.Total, nil
}

// InvalidateEmployeeCache removes all individual employee caches
func (r *RedisClient) InvalidateEmployeeCache() error {
	pattern := "employee:*"
	keys, err := r.client.Keys(r.ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get employee cache keys: %w", err)
	}

	if len(keys) > 0 {
		err = r.client.Del(r.ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("failed to delete employee cache keys: %w", err)
		}
	}

	return nil
}

// InvalidateEmployeeListCache removes all employee list caches
func (r *RedisClient) InvalidateEmployeeListCache() error {
	pattern := "employee_list:*"
	keys, err := r.client.Keys(r.ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get employee list cache keys: %w", err)
	}

	if len(keys) > 0 {
		err = r.client.Del(r.ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("failed to delete employee list cache keys: %w", err)
		}
	}

	return nil
}

// GenerateListCacheKey creates a cache key for employee lists based on parameters
func GenerateListCacheKey(limit, offset int, searchQuery string) string {
	if searchQuery != "" {
		return fmt.Sprintf("search:%s:limit:%d:offset:%d", searchQuery, limit, offset)
	}
	return fmt.Sprintf("all:limit:%d:offset:%d", limit, offset)
}

// Health checks Redis connectivity
func (r *RedisClient) Health() error {
	_, err := r.client.Ping(r.ctx).Result()
	return err
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetCacheStats returns cache statistics
func (r *RedisClient) GetCacheStats() (map[string]interface{}, error) {
	info, err := r.client.Info(r.ctx, "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis stats: %w", err)
	}

	// Count cached employees
	employeeKeys, err := r.client.Keys(r.ctx, "employee:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to count employee keys: %w", err)
	}

	// Count cached employee lists
	listKeys, err := r.client.Keys(r.ctx, "employee_list:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to count employee list keys: %w", err)
	}

	stats := map[string]interface{}{
		"redis_info":            info,
		"cached_employees":      len(employeeKeys),
		"cached_employee_lists": len(listKeys),
		"cache_expiry_minutes":  r.expiry.Minutes(),
	}

	return stats, nil
}
