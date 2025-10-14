package identification

import "strings"

type PathIdentifier struct{}

func NewPathIdentifier() *PathIdentifier {
    return &PathIdentifier{}
}

func (i *PathIdentifier) IdentifyTenant(path string) (string, bool) {
    if !strings.HasPrefix(path, "/tenants/") {
        return "", false
    }
    parts := strings.Split(path, "/")
    if len(parts) >= 3 {
        return parts[2], true
    }
    return "", false
}