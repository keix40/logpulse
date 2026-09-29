package redisx

import (
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

// NewClientFromEnv returns a Redis client using REDIS_URL when set, otherwise REDIS_ADDR.
func NewClientFromEnv() (*redis.Client, error) {
	if raw := os.Getenv("REDIS_URL"); raw != "" {
		opts, err := redis.ParseURL(raw)
		if err != nil {
			return nil, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		return redis.NewClient(opts), nil
	}
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "redis:6379"
	}
	return redis.NewClient(&redis.Options{Addr: addr}), nil
}
