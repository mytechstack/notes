package repository

import (
    "sync"
    "github.com/yourusername/tenant-control-plane/internal/domain"
)

type TenantRepository struct {
    mu                 sync.RWMutex
    tenants            map[string]*domain.Tenant
    tenantCapabilities map[string][]*domain.Capability
}

func NewTenantRepository() *TenantRepository {
    return &TenantRepository{
        tenants:            make(map[string]*domain.Tenant),
        tenantCapabilities: make(map[string][]*domain.Capability),
    }
}

func (r *TenantRepository) Save(tenant *domain.Tenant) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.tenants[tenant.TenantID] = tenant
    if _, exists := r.tenantCapabilities[tenant.TenantID]; !exists {
        r.tenantCapabilities[tenant.TenantID] = make([]*domain.Capability, 0)
    }
}

func (r *TenantRepository) FindByID(tenantID string) (*domain.Tenant, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    tenant, exists := r.tenants[tenantID]
    return tenant, exists
}

func (r *TenantRepository) FindAll() []*domain.Tenant {
    r.mu.RLock()
    defer r.mu.RUnlock()
    tenants := make([]*domain.Tenant, 0, len(r.tenants))
    for _, tenant := range r.tenants {
        tenants = append(tenants, tenant)
    }
    return tenants
}

func (r *TenantRepository) AddCapability(tenantID string, capability *domain.Capability) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.tenantCapabilities[tenantID] = append(r.tenantCapabilities[tenantID], capability)
}

func (r *TenantRepository) GetCapabilities(tenantID string) []*domain.Capability {
    r.mu.RLock()
    defer r.mu.RUnlock()
    capabilities := r.tenantCapabilities[tenantID]
    if capabilities == nil {
        return make([]*domain.Capability, 0)
    }
    return capabilities
}

func (r *TenantRepository) FindCapability(tenantID, capabilityName string) (*domain.Capability, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    capabilities := r.tenantCapabilities[tenantID]
    for _, cap := range capabilities {
        if cap.Name == capabilityName {
            return cap, true
        }
    }
    return nil, false
}