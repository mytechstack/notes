package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
	redisc "github.com/yourorg/context-hydrator/internal/redis"
	"github.com/yourorg/context-hydrator/internal/services"
)

type Config struct {
	// Hydration service port
	Port string `envconfig:"PORT" default:"8080"`
	// Context reader service port (used by cmd/context-reader)
	ReaderPort string `envconfig:"READER_PORT" default:"8081"`

	LogLevel  string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat string `envconfig:"LOG_FORMAT" default:"json"`

	// App identifier — used to namespace Redis keys and JWT claims.
	// Defaults to "default" for local development.
	AppID string `envconfig:"APP_ID" default:"default"`

	RedisAddr     string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	RedisPassword string `envconfig:"REDIS_PASSWORD" default:""`
	RedisDB       int    `envconfig:"REDIS_DB" default:"0"`

	// Base URLs for backend services. URL templates are derived from these:
	// {SERVICE_URL}/users/{user_id}/{resource}
	ProfileServiceURL     string `envconfig:"PROFILE_SERVICE_URL" required:"true"`
	PreferencesServiceURL string `envconfig:"PREFERENCES_SERVICE_URL" required:"true"`
	PermissionsServiceURL string `envconfig:"PERMISSIONS_SERVICE_URL" required:"true"`
	ResourcesServiceURL   string `envconfig:"RESOURCES_SERVICE_URL" required:"true"`

	BackendTimeoutSecs int `envconfig:"BACKEND_TIMEOUT_SECS" default:"4"`

	// Cookie decoding: "base64json" (local dev) or "jwt" (production)
	CookieSecret   string `envconfig:"COOKIE_SECRET" default:""`
	CookieEncoding string `envconfig:"COOKIE_ENCODING" default:"base64json"`

	ReadTimeout  time.Duration `envconfig:"READ_TIMEOUT" default:"5s"`
	WriteTimeout time.Duration `envconfig:"WRITE_TIMEOUT" default:"10s"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DefaultAppConfig builds an AppConfig from the environment-based service URLs.
// URL templates are derived from base service URLs, compatible with the mock backend
// which serves at /{resource} paths under /users/{user_id}.
func (c *Config) DefaultAppConfig() *services.AppConfig {
	return &services.AppConfig{
		AppID: c.AppID,
		Resources: map[services.ServiceName]services.ResourceConfig{
			services.ServiceProfile: {
				URLTemplate: fmt.Sprintf("%s/users/{user_id}/profile", c.ProfileServiceURL),
				TTL:         redisc.TTLProfile,
			},
			services.ServicePreferences: {
				URLTemplate: fmt.Sprintf("%s/users/{user_id}/preferences", c.PreferencesServiceURL),
				TTL:         redisc.TTLPreferences,
			},
			services.ServicePermissions: {
				URLTemplate: fmt.Sprintf("%s/users/{user_id}/permissions", c.PermissionsServiceURL),
				TTL:         redisc.TTLPermissions,
			},
			services.ServiceResources: {
				URLTemplate: fmt.Sprintf("%s/users/{user_id}/resources", c.ResourcesServiceURL),
				TTL:         redisc.TTLResources,
			},
		},
		Secret: []byte(c.CookieSecret),
	}
}
