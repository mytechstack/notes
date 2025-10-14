package identification

import (
    "strings"
    "sync"
)

type SubdomainIdentifier struct {
    mu                sync.RWMutex
    subdomainToTenant map[string]string
}

func NewSubdomainIdentifier() *SubdomainIdentifier {
    return &SubdomainIdentifier{
        subdomainToTenant: make(map[string]string),
    }
}

func (i *SubdomainIdentifier) RegisterSubdomain(tenantID, subdomain string) {
    i.mu.Lock()
    defer i.mu.Unlock()
    i.subdomainToTenant[subdomain] = tenantID
}

func (i *SubdomainIdentifier) IdentifyTenant(host string) (string, bool) {
    i.mu.RLock()
    defer i.mu.RUnlock()
    subdomain := extractSubdomain(host)
    if subdomain == "" {
        return "", false
    }
    tenantID, exists := i.subdomainToTenant[subdomain]
    return tenantID, exists
}

func extractSubdomain(host string) string {
    parts := strings.Split(host, ".")
    if len(parts) > 2 {
        return parts[0]
    }
    return ""
}