package hosts

import (
	"context"
	"time"
)

type HostStatus string

const (
	HostOnline   HostStatus = "ONLINE"
	HostDegraded HostStatus = "DEGRADED"
	HostOffline  HostStatus = "OFFLINE"
	HostUnknown  HostStatus = "UNKNOWN"
)

// HostInfo represents metadata and state of a hypervisor node.
type HostInfo struct {
	ID           string     `json:"id"`
	Hostname     string     `json:"hostname"`
	IPAddress    string     `json:"ip_address"`
	OS           string     `json:"os"`
	Kernel       string     `json:"kernel"`
	Architecture string     `json:"architecture"`
	CPUCores     int        `json:"cpu_cores"`
	TotalRAMBytes int64     `json:"total_ram_bytes"`
	Status       HostStatus `json:"status"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// HostInspector defines boundaries for host system inspection.
type HostInspector interface {
	InspectHost(ctx context.Context) (*HostInfo, error)
}
