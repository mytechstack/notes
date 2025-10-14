package identification

import (
    "strings"
    "github.com/yourusername/tenant-control-plane/internal/domain"
)

type TenantResolver struct {
    apiKeyIdentifier       *APIKeyIdentifier
    jwtIdentifier          *JWTIdentifier
    subdomainIdentifier    *SubdomainIdentifier
    customDomainIdentifier *CustomDomainIdentifier
    pathIdentifier         *PathIdentifier
    headerIdentifier       *HeaderIdentifier
    resolutionOrder        []domain.IdentificationStrategy
}

func NewTenantResolver() *TenantResolver {
    return &TenantResolver{
        apiKeyIdentifier:       NewAPIKeyIdentifier(),
        jwtIdentifier:          NewJWTIdentifier(),
        subdomainIdentifier:    NewSubdomainIdentifier(),
        customDomainIdentifier: NewCustomDomainIdentifier(),
        pathIdentifier:         NewPathIdentifier(),
        headerIdentifier:       NewHeaderIdentifier(),
        resolutionOrder: []domain.IdentificationStrategy{
            domain.StrategyHeader,
            domain.StrategyJWT,
            domain.StrategyAPIKey,
            domain.StrategyCustomDomain,
            domain.StrategySubdomain,
            domain.StrategyPath,
        },
    }
}

func (r *TenantResolver) Resolve(request *RequestContext) (*TenantContext, bool) {
    for _, strategy := range r.resolutionOrder {
        if tenantID, ok := r.tryStrategy(strategy, request); ok {
            return &TenantContext{
                TenantID: tenantID,
                Strategy: strategy,
                Metadata: make(map[string]interface{}),
            }, true
        }
    }
    return nil, false
}

func (r *TenantResolver) tryStrategy(strategy domain.IdentificationStrategy, request *RequestContext) (string, bool) {
    switch strategy {
    case domain.StrategyHeader:
        return r.headerIdentifier.IdentifyTenant(request.Headers)
    case domain.StrategyJWT:
        if bearer, ok := request.Headers["Authorization"]; ok && strings.HasPrefix(bearer, "Bearer ") {
            return r.jwtIdentifier.IdentifyTenant(bearer[7:])
        }
    case domain.StrategyAPIKey:
        if apiKey, ok := request.Headers["X-Api-Key"]; ok {
            return r.apiKeyIdentifier.IdentifyTenant(apiKey)
        }
    case domain.StrategyCustomDomain:
        return r.customDomainIdentifier.IdentifyTenant(request.Host)
    case domain.StrategySubdomain:
        return r.subdomainIdentifier.IdentifyTenant(request.Host)
    case domain.StrategyPath:
        return r.pathIdentifier.IdentifyTenant(request.Path)
    }
    return "", false
}

func (r *TenantResolver) RegisterAPIKey(tenantID, apiKey string) {
    r.apiKeyIdentifier.RegisterAPIKey(tenantID, apiKey)
}

func (r *TenantResolver) RegisterSubdomain(tenantID, subdomain string) {
    r.subdomainIdentifier.RegisterSubdomain(tenantID, subdomain)
}

func (r *TenantResolver) RegisterCustomDomain(tenantID, domain string) {
    r.customDomainIdentifier.RegisterCustomDomain(tenantID, domain)
}