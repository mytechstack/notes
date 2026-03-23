package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type BackendConfig struct {
	ProfileURL     string
	PreferencesURL string
	PermissionsURL string
	ResourcesURL   string
}

type Backend struct {
	cfg    BackendConfig
	client *http.Client
}

func NewBackend(cfg BackendConfig, client *http.Client) *Backend {
	return &Backend{cfg: cfg, client: client}
}

// FetchWithConfig fetches the given resources in parallel using URL templates
// from appConfig, substituting claims into each template.
func (b *Backend) FetchWithConfig(ctx context.Context, appConfig *AppConfig, resources []ServiceName, claims map[string]string) []ServiceResult {
	results := make([]ServiceResult, len(resources))
	var wg sync.WaitGroup

	for i, svcName := range resources {
		resCfg, ok := appConfig.Resources[svcName]
		if !ok {
			results[i] = ServiceResult{Service: svcName, Err: fmt.Errorf("no config for resource %s", svcName)}
			continue
		}

		wg.Add(1)
		go func(idx int, name ServiceName, cfg ResourceConfig) {
			defer wg.Done()
			results[idx] = b.fetchWithTemplate(ctx, name, cfg.URLTemplate, claims)
		}(i, svcName, resCfg)
	}

	wg.Wait()
	return results
}

func (b *Backend) fetchWithTemplate(ctx context.Context, name ServiceName, urlTemplate string, claims map[string]string) ServiceResult {
	url := resolveTemplate(urlTemplate, claims)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ServiceResult{Service: name, Err: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return ServiceResult{Service: name, Err: fmt.Errorf("http get: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ServiceResult{Service: name, Err: fmt.Errorf("upstream %s: status %d", name, resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ServiceResult{Service: name, Err: fmt.Errorf("read body: %w", err)}
	}

	if !json.Valid(body) {
		return ServiceResult{Service: name, Err: fmt.Errorf("invalid JSON from %s", name)}
	}

	return ServiceResult{Service: name, Data: json.RawMessage(body)}
}

// resolveTemplate substitutes {claim} placeholders in a URL template.
func resolveTemplate(template string, claims map[string]string) string {
	url := template
	for k, v := range claims {
		url = strings.ReplaceAll(url, "{"+k+"}", v)
	}
	return url
}

func (b *Backend) serviceURL(name ServiceName, userID string) (string, error) {
	switch name {
	case ServiceProfile:
		return fmt.Sprintf("%s/users/%s/profile", b.cfg.ProfileURL, userID), nil
	case ServicePreferences:
		return fmt.Sprintf("%s/users/%s/preferences", b.cfg.PreferencesURL, userID), nil
	case ServicePermissions:
		return fmt.Sprintf("%s/users/%s/permissions", b.cfg.PermissionsURL, userID), nil
	case ServiceResources:
		return fmt.Sprintf("%s/users/%s/resources", b.cfg.ResourcesURL, userID), nil
	default:
		return "", fmt.Errorf("unknown service: %s", name)
	}
}

func (b *Backend) fetch(ctx context.Context, name ServiceName, userID string) ServiceResult {
	url, err := b.serviceURL(name, userID)
	if err != nil {
		return ServiceResult{Service: name, Err: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ServiceResult{Service: name, Err: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return ServiceResult{Service: name, Err: fmt.Errorf("http get: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ServiceResult{Service: name, Err: fmt.Errorf("upstream %s: status %d", name, resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ServiceResult{Service: name, Err: fmt.Errorf("read body: %w", err)}
	}

	if !json.Valid(body) {
		return ServiceResult{Service: name, Err: fmt.Errorf("invalid JSON from %s", name)}
	}

	return ServiceResult{Service: name, Data: json.RawMessage(body)}
}

// FetchAll calls all requested services in parallel using legacy BackendConfig URLs.
// Kept for backward compatibility.
func (b *Backend) FetchAll(ctx context.Context, userID string, svcs []ServiceName) []ServiceResult {
	results := make([]ServiceResult, len(svcs))
	var wg sync.WaitGroup

	for i, svc := range svcs {
		wg.Add(1)
		go func(idx int, name ServiceName) {
			defer wg.Done()
			results[idx] = b.fetch(ctx, name, userID)
		}(i, svc)
	}

	wg.Wait()
	return results
}
