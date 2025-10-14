package identification

import (
    "github.com/gin-gonic/gin"
    "github.com/yourusername/tenant-control-plane/internal/domain"
)

type TenantContext struct {
    TenantID string
    Strategy domain.IdentificationStrategy
    Metadata map[string]interface{}
}

type RequestContext struct {
    Host    string
    Path    string
    Headers map[string]string
}

func NewRequestContextFromGin(c *gin.Context) *RequestContext {
    headers := make(map[string]string)
    for name, values := range c.Request.Header {
        if len(values) > 0 {
            headers[name] = values[0]
        }
    }
    return &RequestContext{
        Host:    c.Request.Host,
        Path:    c.Request.URL.Path,
        Headers: headers,
    }
}