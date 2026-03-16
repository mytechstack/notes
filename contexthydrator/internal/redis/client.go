package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache key TTLs
const (
	TTLProfile     = 3600 * time.Second
	TTLPreferences = 3600 * time.Second
	TTLPermissions = 3600 * time.Second
	TTLResources   = 3600 * time.Second
)

// Key prefixes
const (
	KeyPrefixProfile       = "user:profile:"
	KeyPrefixPreferences   = "user:preferences:"
	KeyPrefixPermissions   = "user:permissions:"
	KeyPrefixResources     = "user:resources:"
	KeyPrefixAccessPattern = "user:access_pattern:"
)

func NewClient(addr, password string, db int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return client, nil
}
