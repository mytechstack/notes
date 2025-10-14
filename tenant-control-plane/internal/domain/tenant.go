package domain

import "time"

type Tenant struct {
    TenantID  string
    Name      string
    Status    TenantStatus
    CreatedAt time.Time
    Metadata  map[string]string
}

func NewTenant(tenantID, name string) *Tenant {
    return &Tenant{
        TenantID:  tenantID,
        Name:      name,
        Status:    StatusProvisioning,
        CreatedAt: time.Now(),
        Metadata:  make(map[string]string),
    }
}