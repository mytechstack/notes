package identification

import "sync"

type CustomDomainIdentifier struct {
    mu             sync.RWMutex
    domainToTenant map[string]string
}

func NewCustomDomainIdentifier() *CustomDomainIdentifier {
    return &CustomDomainIdentifier{
        domainToTenant: make(map[string]string),
    }
}

func (i *CustomDomainIdentifier) RegisterCustomDomain(tenantID, domain string) {
    i.mu.Lock()
    defer i.mu.Unlock()
    i.domainToTenant[domain] = tenantID
}

func (i *CustomDomainIdentifier) IdentifyTenant(domain string) (string, bool) {
    i.mu.RLock()
    defer i.mu.RUnlock()
    tenantID, exists := i.domainToTenant[domain]
    return tenantID, exists
}