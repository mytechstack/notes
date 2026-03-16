package hydrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/yourorg/context-hydrator/internal/cache"
	redisc "github.com/yourorg/context-hydrator/internal/redis"
	"github.com/yourorg/context-hydrator/internal/services"
)

type Hydrator struct {
	store          *cache.Store
	backend        *services.Backend
	log            *slog.Logger
	backendTimeout time.Duration
}

func New(store *cache.Store, backend *services.Backend, log *slog.Logger, backendTimeout time.Duration) *Hydrator {
	return &Hydrator{
		store:          store,
		backend:        backend,
		log:            log,
		backendTimeout: backendTimeout,
	}
}

// RunHydration executes the full hydration pipeline for a user.
// It is designed to be called in a goroutine (fire-and-forget).
func (h *Hydrator) RunHydration(bgCtx context.Context, userID string) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(bgCtx, h.backendTimeout)
	defer cancel()

	// Step 1: resolve which resources to fetch
	resourcesToFetch := ResolveResources(ctx, h.store, userID, h.log)

	// Step 2: parallel backend calls
	results := h.backend.FetchAll(ctx, userID, resourcesToFetch)

	// Step 3: write successful results to cache
	var successCount, failCount int
	for _, result := range results {
		if result.Err != nil {
			failCount++
			h.log.WarnContext(bgCtx, "backend fetch failed",
				"user_id", userID,
				"service", result.Service,
				"error", result.Err)
			continue
		}

		ttl, cacheKey := ttlAndKey(result.Service, userID)
		if err := h.store.Set(bgCtx, cacheKey, result.Data, ttl); err != nil {
			failCount++
			h.log.WarnContext(bgCtx, "cache write failed",
				"user_id", userID,
				"service", result.Service,
				"error", err)
			continue
		}
		successCount++
	}

	h.log.InfoContext(bgCtx, "hydration complete",
		"user_id", userID,
		"success_count", successCount,
		"fail_count", failCount,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
}

func ttlAndKey(svc services.ServiceName, userID string) (time.Duration, string) {
	switch svc {
	case services.ServiceProfile:
		return redisc.TTLProfile, cache.ProfileKey(userID)
	case services.ServicePreferences:
		return redisc.TTLPreferences, cache.PreferencesKey(userID)
	case services.ServicePermissions:
		return redisc.TTLPermissions, cache.PermissionsKey(userID)
	case services.ServiceResources:
		return redisc.TTLResources, cache.ResourcesKey(userID)
	default:
		return redisc.TTLProfile, cache.ProfileKey(userID)
	}
}
