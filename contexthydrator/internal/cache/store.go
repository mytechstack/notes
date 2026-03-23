package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	redisc "github.com/yourorg/context-hydrator/internal/redis"
	"github.com/yourorg/context-hydrator/internal/services"
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

// StoreMapping persists the hyd_token → {contextKey, claims} mapping in Redis.
// Called at login time by the issuing application (via SDK).
func (s *Store) StoreMapping(ctx context.Context, appID, hydToken string, mapping *services.HydrationMapping) error {
	b, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshal mapping: %w", err)
	}
	key := MappingKey(appID, hydToken)
	return s.client.Set(ctx, key, b, redisc.TTLMapping).Err()
}

// ResolveMapping retrieves the mapping for a given hyd_token.
func (s *Store) ResolveMapping(ctx context.Context, appID, hydToken string) (*services.HydrationMapping, error) {
	b, err := s.client.Get(ctx, MappingKey(appID, hydToken)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get mapping: %w", err)
	}
	var m services.HydrationMapping
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unmarshal mapping: %w", err)
	}
	return &m, nil
}

// GetAccessPattern returns the list of resources from the access pattern key,
// or nil if not found.
func (s *Store) GetAccessPattern(ctx context.Context, appID, contextKey string) ([]string, error) {
	data, err := s.Get(ctx, AccessPatternKey(appID, contextKey))
	if err != nil {
		return nil, err
	}
	var resources []string
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, fmt.Errorf("unmarshal access pattern: %w", err)
	}
	return resources, nil
}

// ── Key builders ──────────────────────────────────────────────────────────────

// ResourceCacheKey returns the namespaced cache key for a resource.
// Format: {appID}:{resource}:{contextKey}
func ResourceCacheKey(appID, resource, contextKey string) string {
	return appID + ":" + resource + ":" + contextKey
}

// MappingKey returns the Redis key for a hydration token mapping.
func MappingKey(appID, hydToken string) string {
	return redisc.KeyPrefixMapping + appID + ":" + hydToken
}

// AccessPatternKey returns the Redis key for a user's access pattern.
func AccessPatternKey(appID, contextKey string) string {
	return appID + ":access_pattern:" + contextKey
}

// KeyForResource returns the cache key for a given resource name and validates it.
func KeyForResource(appID, contextKey, resource string) (string, bool) {
	switch services.ServiceName(resource) {
	case services.ServiceProfile, services.ServicePreferences,
		services.ServicePermissions, services.ServiceResources:
		return ResourceCacheKey(appID, resource, contextKey), true
	default:
		return "", false
	}
}
