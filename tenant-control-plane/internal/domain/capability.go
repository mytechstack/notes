package domain

type ResourceQuota struct {
    MaxLimit     int64
    CurrentUsage int64
    Unit         string
}

func NewResourceQuota(maxLimit int64) *ResourceQuota {
    return &ResourceQuota{
        MaxLimit:     maxLimit,
        CurrentUsage: 0,
        Unit:         "units",
    }
}

func (q *ResourceQuota) IsQuotaExceeded() bool {
    return q.CurrentUsage >= q.MaxLimit
}

func (q *ResourceQuota) CanAllocate(requested int64) bool {
    return q.CurrentUsage+requested <= q.MaxLimit
}

type Capability struct {
    Type    CapabilityType
    Name    string
    Enabled bool
    Config  map[string]interface{}
    Quota   *ResourceQuota
}

func NewCapability(capType CapabilityType, name string, quota int64) *Capability {
    return &Capability{
        Type:    capType,
        Name:    name,
        Enabled: true,
        Config:  make(map[string]interface{}),
        Quota:   NewResourceQuota(quota),
    }
}