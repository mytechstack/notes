package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache key TTLs — per resource, tuned by sensitivity and change frequency.
const (
	TTLProfile     = 12 * time.Hour
	TTLPreferences = 4 * time.Hour
	TTLPermissions = 15 * time.Minute
	TTLResources   = 30 * time.Minute
	TTLMapping     = 30 * 24 * time.Hour // persistent hydration token lifetime
)

// Key prefixes
const (
	KeyPrefixMapping = "hyd:mapping:"
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
