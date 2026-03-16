package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port        string `envconfig:"PORT" default:"8080"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat   string `envconfig:"LOG_FORMAT" default:"json"`

	RedisAddr     string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	RedisPassword string `envconfig:"REDIS_PASSWORD" default:""`
	RedisDB       int    `envconfig:"REDIS_DB" default:"0"`

	ProfileServiceURL     string `envconfig:"PROFILE_SERVICE_URL" required:"true"`
	PreferencesServiceURL string `envconfig:"PREFERENCES_SERVICE_URL" required:"true"`
	PermissionsServiceURL string `envconfig:"PERMISSIONS_SERVICE_URL" required:"true"`
	ResourcesServiceURL   string `envconfig:"RESOURCES_SERVICE_URL" required:"true"`

	BackendTimeoutSecs int `envconfig:"BACKEND_TIMEOUT_SECS" default:"4"`

	CookieSecret   string `envconfig:"COOKIE_SECRET" default:""`
	CookieEncoding string `envconfig:"COOKIE_ENCODING" default:"base64json"` // "base64json" or "jwt"

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
