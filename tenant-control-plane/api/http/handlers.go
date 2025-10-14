package http

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/yourusername/tenant-control-plane/internal/controlplane"
    "github.com/yourusername/tenant-control-plane/internal/domain"
    "github.com/yourusername/tenant-control-plane/pkg/middleware"
)

type TenantHandler struct {
    controlPlane *controlplane.TenantControlPlane
}

func NewTenantHandler(cp *controlplane.TenantControlPlane) *TenantHandler {
    return &TenantHandler{controlPlane: cp}
}

func (h *TenantHandler) CreateTenant(c *gin.Context) {
    var req struct {
        Name string `json:"name" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    tenant := h.controlPlane.OnboardTenant(req.Name)
    c.JSON(http.StatusCreated, tenant)
}

func (h *TenantHandler) GetTenant(c *gin.Context) {
    tenantCtx, ok := middleware.GetTenantContext(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant context not found"})
        return
    }
    
    tenant, err := h.controlPlane.GetTenant(tenantCtx.TenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, tenant)
}

func (h *TenantHandler) ActivateTenant(c *gin.Context) {
    tenantID := c.Param("tenantId")
    
    if err := h.controlPlane.ActivateTenant(tenantID); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "activated"})
}

func (h *TenantHandler) SuspendTenant(c *gin.Context) {
    tenantID := c.Param("tenantId")
    
    if err := h.controlPlane.SuspendTenant(tenantID); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "suspended"})
}

func (h *TenantHandler) ProvisionCapability(c *gin.Context) {
    tenantCtx, ok := middleware.GetTenantContext(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant context not found"})
        return
    }
    
    var req struct {
        Type  domain.CapabilityType `json:"type" binding:"required"`
        Name  string                `json:"name" binding:"required"`
        Quota int64                 `json:"quota" binding:"required,min=1"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if err := h.controlPlane.ProvisionCapability(tenantCtx.TenantID, req.Type, req.Name, req.Quota); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{"status": "provisioned"})
}

func (h *TenantHandler) GetCapabilities(c *gin.Context) {
    tenantCtx, ok := middleware.GetTenantContext(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant context not found"})
        return
    }
    
    capabilities := h.controlPlane.GetTenantCapabilities(tenantCtx.TenantID)
    c.JSON(http.StatusOK, capabilities)
}

func (h *TenantHandler) AuthorizeRequest(c *gin.Context) {
    tenantCtx, ok := middleware.GetTenantContext(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant context not found"})
        return
    }
    
    var req struct {
        Capability    string `json:"capability" binding:"required"`
        ResourceUnits int64  `json:"resource_units" binding:"required,min=1"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    authorized := h.controlPlane.AuthorizeRequest(tenantCtx.TenantID, req.Capability, req.ResourceUnits)
    
    c.JSON(http.StatusOK, gin.H{
        "authorized": authorized,
        "tenant_id":  tenantCtx.TenantID,
        "capability": req.Capability,
        "units":      req.ResourceUnits,
    })
}

func (h *TenantHandler) RecordUsage(c *gin.Context) {
    tenantCtx, ok := middleware.GetTenantContext(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant context not found"})
        return
    }
    
    var req struct {
        Capability string `json:"capability" binding:"required"`
        Usage      int64  `json:"usage" binding:"required,min=0"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if err := h.controlPlane.RecordUsage(tenantCtx.TenantID, req.Capability, req.Usage); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

func SetupRoutes(cp *controlplane.TenantControlPlane) *gin.Engine {
    // Set Gin to release mode for production
    // gin.SetMode(gin.ReleaseMode)
    
    router := gin.Default()
    handler := NewTenantHandler(cp)
    
    // API v1 group
    v1 := router.Group("/api/v1")
    {
        // Public routes - no authentication required
        public := v1.Group("/tenants")
        {
            public.POST("", handler.CreateTenant)
            public.POST("/:tenantId/activate", handler.ActivateTenant)
            public.POST("/:tenantId/suspend", handler.SuspendTenant)
        }
        
        // Protected routes - require tenant identification
        protected := v1.Group("")
        protected.Use(middleware.TenantIdentification(cp.GetResolver()))
        {
            protected.GET("/tenant", handler.GetTenant)
            protected.GET("/capabilities", handler.GetCapabilities)
            protected.POST("/capabilities", handler.ProvisionCapability)
            protected.POST("/authorize", handler.AuthorizeRequest)
            protected.POST("/usage", handler.RecordUsage)
        }
    }
    
    // Health check endpoint
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "status": "healthy",
            "service": "tenant-control-plane",
        })
    })
    
    return router
}