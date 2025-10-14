package domain

type TenantStatus string

const (
    StatusActive       TenantStatus = "ACTIVE"
    StatusSuspended    TenantStatus = "SUSPENDED"
    StatusInactive     TenantStatus = "INACTIVE"
    StatusProvisioning TenantStatus = "PROVISIONING"
)

type CapabilityType string

const (
    CapabilityStorage   CapabilityType = "STORAGE"
    CapabilityCompute   CapabilityType = "COMPUTE"
    CapabilityAPICalls  CapabilityType = "API_CALLS"
    CapabilityDatabase  CapabilityType = "DATABASE"
    CapabilityMessaging CapabilityType = "MESSAGING"
)

type IdentificationStrategy string

const (
    StrategyAPIKey       IdentificationStrategy = "API_KEY"
    StrategyJWT          IdentificationStrategy = "JWT_TOKEN"
    StrategySubdomain    IdentificationStrategy = "SUBDOMAIN"
    StrategyCustomDomain IdentificationStrategy = "CUSTOM_DOMAIN"
    StrategyPath         IdentificationStrategy = "PATH"
    StrategyHeader       IdentificationStrategy = "HEADER"
)