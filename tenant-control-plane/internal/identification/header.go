package identification

type HeaderIdentifier struct{}

func NewHeaderIdentifier() *HeaderIdentifier {
    return &HeaderIdentifier{}
}

func (i *HeaderIdentifier) IdentifyTenant(headers map[string]string) (string, bool) {
    tenantID, exists := headers["X-Tenant-Id"]
    return tenantID, exists
}