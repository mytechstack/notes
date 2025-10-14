package identification

import (
    "encoding/base64"
    "encoding/json"
    "strings"
)

type JWTIdentifier struct{}

func NewJWTIdentifier() *JWTIdentifier {
    return &JWTIdentifier{}
}

func (i *JWTIdentifier) IdentifyTenant(token string) (string, bool) {
    parts := strings.Split(token, ".")
    if len(parts) != 3 {
        return "", false
    }

    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return "", false
    }

    var claims map[string]interface{}
    if err := json.Unmarshal(payload, &claims); err != nil {
        return "", false
    }

    if tenantID, ok := claims["tenant_id"].(string); ok {
        return tenantID, true
    }
    return "", false
}