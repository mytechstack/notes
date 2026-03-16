package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	redisc "github.com/yourorg/context-hydrator/internal/redis"
)

var ErrCacheMiss = errors.New("cache miss")

type Store struct {
	client *redis.Client
}

func NewStore(client *redis.Client) *Store {
	return &Store{client: client}
}

func (s *Store) Set(ctx context.Context, key string, data json.RawMessage, ttl time.Duration) error {
	return s.client.Set(ctx, key, []byte(data), ttl).Err()
}

func (s *Store) Get(ctx context.Context, key string) (json.RawMessage, error) {
	b, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	return json.RawMessage(b), nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Key builders

func ProfileKey(userID string) string {
	return redisc.KeyPrefixProfile + userID
}

func PreferencesKey(userID string) string {
	return redisc.KeyPrefixPreferences + userID
}

func PermissionsKey(userID string) string {
	return redisc.KeyPrefixPermissions + userID
}

func ResourcesKey(userID string) string {
	return redisc.KeyPrefixResources + userID
}

func AccessPatternKey(userID string) string {
	return redisc.KeyPrefixAccessPattern + userID
}

func KeyForResource(userID, resource string) (string, bool) {
	switch resource {
	case "profile":
		return ProfileKey(userID), true
	case "preferences":
		return PreferencesKey(userID), true
	case "permissions":
		return PermissionsKey(userID), true
	case "resources":
		return ResourcesKey(userID), true
	default:
		return "", false
	}
}

// GetAccessPattern returns the list of resources from the access pattern key,
// or nil if not found.
func (s *Store) GetAccessPattern(ctx context.Context, userID string) ([]string, error) {
	data, err := s.Get(ctx, AccessPatternKey(userID))
	if err != nil {
		return nil, err
	}
	var resources []string
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, fmt.Errorf("unmarshal access pattern: %w", err)
	}
	return resources, nil
}
