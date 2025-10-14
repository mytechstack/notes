package identification

import "sync"

type APIKeyIdentifier struct {
    mu             sync.RWMutex
    apiKeyToTenant map[string]string
}

func NewAPIKeyIdentifier() *APIKeyIdentifier {
    return &APIKeyIdentifier{
        apiKeyToTenant: make(map[string]string),
    }
}

func (i *APIKeyIdentifier) RegisterAPIKey(tenantID, apiKey string) {
    i.mu.Lock()
    defer i.mu.Unlock()
    i.apiKeyToTenant[apiKey] = tenantID
}

func (i *APIKeyIdentifier) IdentifyTenant(apiKey string) (string, bool) {
    i.mu.RLock()
    defer i.mu.RUnlock()
    tenantID, exists := i.apiKeyToTenant[apiKey]
    return tenantID, exists
}