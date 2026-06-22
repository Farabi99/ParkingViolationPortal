package idempotency

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type Checker struct {
	client *redis.Client
}

func NewChecker(redisURL string) *Checker {
	opt, _ := redis.ParseURL(redisURL)
	client := redis.NewClient(opt)
	return &Checker{client: client}
}

// CheckAndSet checks if the key exists. If it does not exist, it sets it with the given TTL and returns true (meaning safe to process).
// If it exists, it returns false (meaning already processed or in progress).
func (c *Checker) CheckAndSet(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := "idempotency:" + key
	set, err := c.client.SetNX(ctx, fullKey, "processing", ttl).Result()
	if err != nil {
		return false, err
	}
	return set, nil
}

func (c *Checker) MarkComplete(ctx context.Context, key string, result string, ttl time.Duration) error {
	fullKey := "idempotency:" + key
	return c.client.Set(ctx, fullKey, result, ttl).Err()
}

func (c *Checker) GetResult(ctx context.Context, key string) (string, error) {
	fullKey := "idempotency:" + key
	return c.client.Get(ctx, fullKey).Result()
}
