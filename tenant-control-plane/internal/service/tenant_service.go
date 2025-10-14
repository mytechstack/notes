package service

import (
    "fmt"
    "github.com/google/uuid"
    "github.com/yourusername/tenant-control-plane/internal/domain"
    "github.com/yourusername/tenant-control-plane/internal/repository"
)

type TenantService struct {
    repository *repository.TenantRepository
}

func NewTenantService(repo *repository.TenantRepository) *TenantService {
    return &TenantService{repository: repo}
}

func (s *TenantService) CreateTenant(name string) *domain.Tenant {
    tenantID := fmt.Sprintf("tenant-%s", uuid.New().String()[:8])
    tenant := domain.NewTenant(tenantID, name)
    s.repository.Save(tenant)
    return tenant
}

func (s *TenantService) ActivateTenant(tenantID string) error {
    tenant, exists := s.repository.FindByID(tenantID)
    if !exists {
        return fmt.Errorf("tenant not found: %s", tenantID)
    }
    tenant.Status = domain.StatusActive
    s.repository.Save(tenant)
    return nil
}

func (s *TenantService) SuspendTenant(tenantID string) error {
    tenant, exists := s.repository.FindByID(tenantID)
    if !exists {
        return fmt.Errorf("tenant not found: %s", tenantID)
    }
    tenant.Status = domain.StatusSuspended
    s.repository.Save(tenant)
    return nil
}

func (s *TenantService) GetTenant(tenantID string) (*domain.Tenant, error) {
    tenant, exists := s.repository.FindByID(tenantID)
    if !exists {
        return nil, fmt.Errorf("tenant not found: %s", tenantID)
    }
    return tenant, nil
}

func (s *TenantService) GetAllTenants() []*domain.Tenant {
    return s.repository.FindAll()
}