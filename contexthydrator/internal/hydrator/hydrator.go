package hydrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/yourorg/context-hydrator/internal/cache"
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

// RunHydration executes the full hydration pipeline for a context key.
// It is designed to be called in a goroutine (fire-and-forget).
//
// appConfig defines which resources to fetch and their URL templates.
// contextKey is the namespaced identity (e.g. "user-123" or "user-123:profile-456").
// claims are the key-value pairs substituted into URL templates (e.g. {"user_id": "123"}).
func (h *Hydrator) RunHydration(bgCtx context.Context, appConfig *services.AppConfig, contextKey string, claims map[string]string) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(bgCtx, h.backendTimeout)
	defer cancel()

	// Step 1: resolve which resources to fetch
	resourcesToFetch := ResolveResources(ctx, h.store, appConfig, contextKey, h.log)

	// Step 2: parallel backend calls using URL templates
	results := h.backend.FetchWithConfig(ctx, appConfig, resourcesToFetch, claims)

	// Step 3: write successful results to cache
	var successCount, failCount int
	for _, result := range results {
		if result.Err != nil {
			failCount++
			h.log.WarnContext(bgCtx, "backend fetch failed",
				"app_id", appConfig.AppID,
				"context_key", contextKey,
				"service", result.Service,
				"error", result.Err)
			continue
		}

		resCfg, ok := appConfig.Resources[result.Service]
		if !ok {
			failCount++
			continue
		}

		cacheKey := cache.ResourceCacheKey(appConfig.AppID, string(result.Service), contextKey)
		if err := h.store.Set(bgCtx, cacheKey, result.Data, resCfg.TTL); err != nil {
			failCount++
			h.log.WarnContext(bgCtx, "cache write failed",
				"app_id", appConfig.AppID,
				"context_key", contextKey,
				"service", result.Service,
				"error", err)
			continue
		}
		successCount++
	}

	h.log.InfoContext(bgCtx, "hydration complete",
		"app_id", appConfig.AppID,
		"context_key", contextKey,
		"success_count", successCount,
		"fail_count", failCount,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
}
