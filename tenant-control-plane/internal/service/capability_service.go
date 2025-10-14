package service

import (
    "fmt"
    "github.com/yourusername/tenant-control-plane/internal/domain"
    "github.com/yourusername/tenant-control-plane/internal/repository"
)

type CapabilityService struct {
    repository *repository.TenantRepository
}

func NewCapabilityService(repo *repository.TenantRepository) *CapabilityService {
    return &CapabilityService{repository: repo}
}

func (s *CapabilityService) EnableCapability(tenantID string, capType domain.CapabilityType, name string, quota int64) error {
    tenant, exists := s.repository.FindByID(tenantID)
    if !exists {
        return fmt.Errorf("tenant not found: %s", tenantID)
    }
    if tenant.Status != domain.StatusActive {
        return fmt.Errorf("tenant must be active to enable capabilities")
    }

    capability := domain.NewCapability(capType, name, quota)
    s.repository.AddCapability(tenantID, capability)
    return nil
}

func (s *CapabilityService) GetCapabilities(tenantID string) []*domain.Capability {
    return s.repository.GetCapabilities(tenantID)
}

func (s *CapabilityService) CheckQuota(tenantID, capabilityName string, requestedUsage int64) bool {
    capability, exists := s.repository.FindCapability(tenantID, capabilityName)
    if !exists || !capability.Enabled {
        return false
    }
    return capability.Quota.CanAllocate(requestedUsage)
}

func (s *CapabilityService) UpdateUsage(tenantID, capabilityName string, usage int64) error {
    capability, exists := s.repository.FindCapability(tenantID, capabilityName)
    if !exists {
        return fmt.Errorf("capability not found: %s", capabilityName)
    }
    capability.Quota.CurrentUsage = usage
    return nil
}