package services

import (
	"encoding/json"
	"time"
)

type ServiceName string

const (
	ServiceProfile     ServiceName = "profile"
	ServicePreferences ServiceName = "preferences"
	ServicePermissions ServiceName = "permissions"
	ServiceResources   ServiceName = "resources"
)

var AllServices = []ServiceName{
	ServiceProfile,
	ServicePreferences,
	ServicePermissions,
	ServiceResources,
}

type ServiceResult struct {
	Service ServiceName
	Data    json.RawMessage
	Err     error
}

// ResourceConfig defines how to fetch and cache a single resource.
type ResourceConfig struct {
	URLTemplate string        // e.g. "http://svc/users/{user_id}/profile"
	TTL         time.Duration
}

// AppConfig holds per-app hydration configuration.
type AppConfig struct {
	AppID     string
	Resources map[ServiceName]ResourceConfig
	Secret    []byte
}

// HydrationMapping maps an opaque hyd_token to a context key and claims.
// Stored in Redis at login time by the issuing application.
type HydrationMapping struct {
	ContextKey string            `json:"context_key"`
	Claims     map[string]string `json:"claims"`
}
