package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// RedisClient Redis缓存客户端
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(host, port, password string, db int) (*RedisClient, error) {
	addr := fmt.Sprintf("%s:%s", host, port)

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		PoolSize: 10,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rdb.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	fmt.Printf("✅ Connected to Redis: %s\n", addr)

	return &RedisClient{client: rdb}, nil
}

// Close 关闭连接
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Get 获取缓存
func (r *RedisClient) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("cache miss")
		}
		return err
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("failed to unmarshal cache value: %w", err)
	}

	return nil
}

// Set 设置缓存
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

// Delete 删除缓存
func (r *RedisClient) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Exists 检查key是否存在
func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	return count > 0, err
}

// MemoryCachedRetriever 内存缓存装饰器（优化性能）
type MemoryCachedRetriever struct {
	cache    map[string]interface{}
	expiry   map[string]time.Time
	mu       sync.RWMutex
	defaultTTL time.Duration
}

// NewMemoryCachedRetriever 创建内存缓存
func NewMemoryCachedRetriever(defaultTTL time.Duration) *MemoryCachedRetriever {
	return &MemoryCachedRetriever{
		cache:     make(map[string]interface{}),
		expiry:    make(map[string]time.Time),
		defaultTTL: defaultTTL,
	}
}

// Get 获取内存缓存
func (m *MemoryCachedRetriever) Get(ctx context.Context, key string, dest interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查是否过期
	if expiry, exists := m.expiry[key]; exists {
		if time.Now().Before(expiry) {
			// 使用类型断言
			if val, ok := m.cache[key]; ok {
				if strVal, ok := val.(string); ok {
					if err := json.Unmarshal([]byte(strVal), dest); err == nil {
						return nil
					}
				}
			}
		}
	}

	return fmt.Errorf("cache miss")
}

// Set 设置内存缓存
func (m *MemoryCachedRetriever) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	m.cache[key] = string(data)
	m.expiry[key] = time.Now().Add(ttl)

	return nil
}

// Delete 删除内存缓存
func (m *MemoryCachedRetriever) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.cache, key)
	delete(m.expiry, key)

	return nil
}

// Exists 检查key是否存在
func (m *MemoryCachedRetriever) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if expiry, exists := m.expiry[key]; exists {
		if time.Now().Before(expiry) {
			return true, nil
		}
	}

	return false, nil
}

// CleanupExpired 清理过期缓存
func (m *MemoryCachedRetriever) CleanupExpired(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, expiry := range m.expiry {
		if now.After(expiry) {
			delete(m.cache, key)
			delete(m.expiry, key)
		}
	}
}
