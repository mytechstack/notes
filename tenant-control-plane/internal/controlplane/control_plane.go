package controlplane

import (
    "github.com/yourusername/tenant-control-plane/internal/domain"
    "github.com/yourusername/tenant-control-plane/internal/repository"
    "github.com/yourusername/tenant-control-plane/internal/service"
    "github.com/yourusername/tenant-control-plane/internal/identification"
)

type TenantControlPlane struct {
    tenantService     *service.TenantService
    capabilityService *service.CapabilityService
    resolver          *identification.TenantResolver
}

func NewTenantControlPlane() *TenantControlPlane {
    repo := repository.NewTenantRepository()
    return &TenantControlPlane{
        tenantService:     service.NewTenantService(repo),
        capabilityService: service.NewCapabilityService(repo),
        resolver:          identification.NewTenantResolver(),
    }
}

func (cp *TenantControlPlane) OnboardTenant(name string) *domain.Tenant {
    return cp.tenantService.CreateTenant(name)
}

func (cp *TenantControlPlane) ProvisionCapability(tenantID string, capType domain.CapabilityType, name string, quota int64) error {
    return cp.capabilityService.EnableCapability(tenantID, capType, name, quota)
}

func (cp *TenantControlPlane) ActivateTenant(tenantID string) error {
    return cp.tenantService.ActivateTenant(tenantID)
}

func (cp *TenantControlPlane) SuspendTenant(tenantID string) error {
    return cp.tenantService.SuspendTenant(tenantID)
}

func (cp *TenantControlPlane) AuthorizeRequest(tenantID, capability string, resourceUnits int64) bool {
    tenant, err := cp.tenantService.GetTenant(tenantID)
    if err != nil || tenant.Status != domain.StatusActive {
        return false
    }
    return cp.capabilityService.CheckQuota(tenantID, capability, resourceUnits)
}

func (cp *TenantControlPlane) RecordUsage(tenantID, capability string, usage int64) error {
    return cp.capabilityService.UpdateUsage(tenantID, capability, usage)
}

func (cp *TenantControlPlane) GetTenant(tenantID string) (*domain.Tenant, error) {
    return cp.tenantService.GetTenant(tenantID)
}

func (cp *TenantControlPlane) GetTenantCapabilities(tenantID string) []*domain.Capability {
    return cp.capabilityService.GetCapabilities(tenantID)
}

func (cp *TenantControlPlane) GetResolver() *identification.TenantResolver {
    return cp.resolver
}