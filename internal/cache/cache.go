package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"recommendation-service/internal/model"
)

const defaultTTL = 10 * time.Minute

// Cache wraps Redis with typed helpers for recommendations.
type Cache struct {
	client *redis.Client
}

func New(client *redis.Client) *Cache {
	return &Cache{client: client}
}

// NewRedisClient creates a Redis client from a URL (e.g. redis://localhost:6379).
func NewRedisClient(redisURL string) *redis.Client {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		// Fall back to default if URL is malformed
		opts = &redis.Options{Addr: "localhost:6379"}
	}
	return redis.NewClient(opts)
}

func (c *Cache) Close() error {
	return c.client.Close()
}

// cacheKey returns the structured key for a user's recommendations.
func cacheKey(userID int64, limit int) string {
	return fmt.Sprintf("rec:user:%d:limit:%d", userID, limit)
}

// Get retrieves cached recommendations. Returns nil, nil on cache miss.
func (c *Cache) Get(ctx context.Context, userID int64, limit int) ([]model.ScoredContent, error) {
	val, err := c.client.Get(ctx, cacheKey(userID, limit)).Result()
	if err == redis.Nil {
		return nil, nil // cache miss — not an error
	}
	if err != nil {
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var recs []model.ScoredContent
	if err := json.Unmarshal([]byte(val), &recs); err != nil {
		return nil, fmt.Errorf("cache unmarshal: %w", err)
	}
	return recs, nil
}

// Set stores recommendations in Redis with the default TTL.
func (c *Cache) Set(ctx context.Context, userID int64, limit int, recs []model.ScoredContent) error {
	data, err := json.Marshal(recs)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	if err := c.client.Set(ctx, cacheKey(userID, limit), data, defaultTTL).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

// Invalidate removes all cached recommendation keys for a user.
// Called when a user's watch history is updated.
func (c *Cache) Invalidate(ctx context.Context, userID int64) error {
	pattern := fmt.Sprintf("rec:user:%d:limit:*", userID)
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("cache scan: %w", err)
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache del: %w", err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
