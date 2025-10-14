package main

import (
    "log"
    
    "github.com/yourusername/tenant-control-plane/internal/controlplane"
    "github.com/yourusername/tenant-control-plane/internal/domain"
    apihttp "github.com/yourusername/tenant-control-plane/api/http"
)

func main() {
    // Initialize control plane
    cp := controlplane.NewTenantControlPlane()
    
    // Demo: Create and setup a tenant
    setupDemoTenant(cp)
    
    // Setup Gin routes
    router := apihttp.SetupRoutes(cp)
    
    // Start server
    port := ":8080"
    log.Printf("Starting Tenant Control Plane server on %s", port)
    log.Printf("\nAPI Endpoints:")
    log.Printf("  POST   /api/v1/tenants - Create new tenant")
    log.Printf("  POST   /api/v1/tenants/:id/activate - Activate tenant")
    log.Printf("  POST   /api/v1/tenants/:id/suspend - Suspend tenant")
    log.Printf("  GET    /api/v1/tenant - Get current tenant (requires auth)")
    log.Printf("  GET    /api/v1/capabilities - List capabilities (requires auth)")
    log.Printf("  POST   /api/v1/capabilities - Provision capability (requires auth)")
    log.Printf("  POST   /api/v1/authorize - Check authorization (requires auth)")
    log.Printf("  POST   /api/v1/usage - Record usage (requires auth)")
    log.Printf("  GET    /health - Health check")
    
    if err := router.Run(port); err != nil {
        log.Fatal(err)
    }
}

func setupDemoTenant(cp *controlplane.TenantControlPlane) {
    // Create demo tenant
    tenant := cp.OnboardTenant("Demo Corp")
    log.Printf("Demo tenant created: %s", tenant.TenantID)
    
    // Activate tenant
    if err := cp.ActivateTenant(tenant.TenantID); err != nil {
        log.Printf("Failed to activate tenant: %v", err)
        return
    }
    log.Printf("Demo tenant activated")
    
    // Provision capabilities
    if err := cp.ProvisionCapability(tenant.TenantID, domain.CapabilityStorage, "storage", 1000); err != nil {
        log.Printf("Failed to provision storage: %v", err)
    }
    if err := cp.ProvisionCapability(tenant.TenantID, domain.CapabilityAPICalls, "api", 10000); err != nil {
        log.Printf("Failed to provision API calls: %v", err)
    }
    log.Printf("Demo capabilities provisioned")
    
    // Register identification methods for demo tenant
    resolver := cp.GetResolver()
    apiKey := "demo_api_key_12345"
    resolver.RegisterAPIKey(tenant.TenantID, apiKey)
    resolver.RegisterSubdomain(tenant.TenantID, "demo")
    
    log.Printf("\n" + repeat("=", 60))
    log.Printf("Demo tenant ready!")
    log.Printf(repeat("=", 60))
    log.Printf("Tenant ID: %s", tenant.TenantID)
    log.Printf("API Key: %s", apiKey)
    log.Printf("\nTry these curl commands:")
    log.Printf("\n# Health check")
    log.Printf("curl http://localhost:8080/health")
    log.Printf("\n# Get tenant info")
    log.Printf("curl -H 'X-Api-Key: %s' http://localhost:8080/api/v1/tenant", apiKey)
    log.Printf("\n# Get capabilities")
    log.Printf("curl -H 'X-Api-Key: %s' http://localhost:8080/api/v1/capabilities", apiKey)
    log.Printf("\n# Check authorization")
    log.Printf("curl -X POST -H 'X-Api-Key: %s' -H 'Content-Type: application/json' \\", apiKey)
    log.Printf("  -d '{\"capability\":\"storage\",\"resource_units\":100}' \\")
    log.Printf("  http://localhost:8080/api/v1/authorize")
    log.Printf("\n# Record usage")
    log.Printf("curl -X POST -H 'X-Api-Key: %s' -H 'Content-Type: application/json' \\", apiKey)
    log.Printf("  -d '{\"capability\":\"storage\",\"usage\":250}' \\")
    log.Printf("  http://localhost:8080/api/v1/usage")
    log.Printf("\n" + repeat("=", 60) + "\n")
}

func repeat(s string, count int) string {
    result := ""
    for i := 0; i < count; i++ {
        result += s
    }
    return result
}