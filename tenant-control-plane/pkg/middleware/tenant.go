package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/yourusername/tenant-control-plane/internal/identification"
)

const TenantContextKey = "tenant_context"

func TenantIdentification(resolver *identification.TenantResolver) gin.HandlerFunc {
    return func(c *gin.Context) {
        reqCtx := identification.NewRequestContextFromGin(c)
        tenantCtx, ok := resolver.Resolve(reqCtx)
        
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant identification failed"})
            c.Abort()
            return
        }
        
        c.Set(TenantContextKey, tenantCtx)
        c.Next()
    }
}

func GetTenantContext(c *gin.Context) (*identification.TenantContext, bool) {
    value, exists := c.Get(TenantContextKey)
    if !exists {
        return nil, false
    }
    tenantCtx, ok := value.(*identification.TenantContext)
    return tenantCtx, ok
}