package hydrator

import (
	"context"
	"errors"
	"log/slog"

	"github.com/yourorg/context-hydrator/internal/cache"
	"github.com/yourorg/context-hydrator/internal/services"
)

// ResolveResources returns the resource list from the access pattern cache,
// falling back to all resources defined in appConfig if not found or on error.
func ResolveResources(ctx context.Context, store *cache.Store, appConfig *services.AppConfig, contextKey string, log *slog.Logger) []services.ServiceName {
	resources, err := store.GetAccessPattern(ctx, appConfig.AppID, contextKey)
	if err != nil {
		if !errors.Is(err, cache.ErrCacheMiss) {
			log.WarnContext(ctx, "failed to load access pattern, using defaults",
				"app_id", appConfig.AppID, "context_key", contextKey, "error", err)
		}
		return allConfiguredResources(appConfig)
	}

	// Validate against app's configured resources
	valid := make([]services.ServiceName, 0, len(resources))
	for _, r := range resources {
		svc := services.ServiceName(r)
		if _, ok := appConfig.Resources[svc]; ok {
			valid = append(valid, svc)
		} else {
			log.WarnContext(ctx, "unknown resource in access pattern, skipping",
				"app_id", appConfig.AppID, "context_key", contextKey, "resource", r)
		}
	}

	if len(valid) == 0 {
		return allConfiguredResources(appConfig)
	}
	return valid
}

func allConfiguredResources(appConfig *services.AppConfig) []services.ServiceName {
	out := make([]services.ServiceName, 0, len(appConfig.Resources))
	for name := range appConfig.Resources {
		out = append(out, name)
	}
	return out
}
