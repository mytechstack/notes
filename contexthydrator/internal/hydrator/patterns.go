package hydrator

import (
	"context"
	"errors"
	"log/slog"

	"github.com/yourorg/context-hydrator/internal/cache"
	"github.com/yourorg/context-hydrator/internal/services"
)

var DefaultResources = []services.ServiceName{
	services.ServiceProfile,
	services.ServicePreferences,
	services.ServicePermissions,
	services.ServiceResources,
}

// ResolveResources returns the resource list from the access pattern cache,
// falling back to DefaultResources if not found or on error.
func ResolveResources(ctx context.Context, store *cache.Store, userID string, log *slog.Logger) []services.ServiceName {
	resources, err := store.GetAccessPattern(ctx, userID)
	if err != nil {
		if !errors.Is(err, cache.ErrCacheMiss) {
			log.WarnContext(ctx, "failed to load access pattern, using defaults",
				"user_id", userID, "error", err)
		}
		return DefaultResources
	}

	// Validate and convert to ServiceName slice
	valid := make([]services.ServiceName, 0, len(resources))
	for _, r := range resources {
		svc := services.ServiceName(r)
		switch svc {
		case services.ServiceProfile, services.ServicePreferences,
			services.ServicePermissions, services.ServiceResources:
			valid = append(valid, svc)
		default:
			log.WarnContext(ctx, "unknown resource in access pattern, skipping",
				"user_id", userID, "resource", r)
		}
	}

	if len(valid) == 0 {
		return DefaultResources
	}
	return valid
}
