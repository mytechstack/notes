package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// FetchAll calls all requested services in parallel. Each result captures its
// own error so a single failing service does not cancel the others.
func (b *Backend) FetchAll(ctx context.Context, userID string, services []ServiceName) []ServiceResult {
	results := make([]ServiceResult, len(services))
	var wg sync.WaitGroup

	for i, svc := range services {
		wg.Add(1)
		go func(idx int, name ServiceName) {
			defer wg.Done()
			results[idx] = b.fetch(ctx, name, userID)
		}(i, svc)
	}

	wg.Wait()
	return results
}
