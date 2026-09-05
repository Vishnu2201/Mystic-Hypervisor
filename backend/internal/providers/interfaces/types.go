package interfaces

import (
	"errors"
	"time"
)

// Standardized Provider Errors
var (
	ErrUnsupportedOperation = errors.New("operation not supported by virtualization provider")
	ErrInstanceNotFound     = errors.New("instance not found")
	ErrInstanceExists       = errors.New("instance with given name already exists")
	ErrProviderUnavailable  = errors.New("virtualization provider is unavailable")
	ErrInvalidConfiguration = errors.New("invalid instance configuration")
)

// InstanceType defines whether the workload is a VM or Container.
type InstanceType string

const (
	InstanceTypeVM        InstanceType = "vm"
	InstanceTypeContainer InstanceType = "container"
)

// InstanceState represents the authoritative live lifecycle state reported by the provider.
type InstanceState string

const (
	StateRunning  InstanceState = "running"
	StateStopped  InstanceState = "stopped"
	StateFrozen   InstanceState = "frozen"
	StateError    InstanceState = "error"
	StateCreating InstanceState = "creating"
	StateDeleting InstanceState = "deleting"
	StateUnknown  InstanceState = "unknown"
)

// ResourceLimits specifies CPU, Memory, and Disk constraints.
type ResourceLimits struct {
	CPUCores    int   `json:"cpu_cores"`
	MemoryBytes int64 `json:"memory_bytes"`
	DiskBytes   int64 `json:"disk_bytes"`
}

// Instance represents provider-level instance details.
type Instance struct {
	ID        string                       `json:"id"`
	Name      string                       `json:"name"`
	Type      InstanceType                 `json:"type"`
	State     InstanceState                `json:"state"`
	Provider  string                       `json:"provider"`
	Node      string                       `json:"node"`
	IPAddress string                       `json:"ip_address,omitempty"`
	Limits    ResourceLimits               `json:"limits"`
	Labels    map[string]string            `json:"labels,omitempty"`
	Devices   map[string]map[string]string `json:"devices,omitempty"`
	CreatedAt time.Time                    `json:"created_at"`
	UpdatedAt time.Time                    `json:"updated_at"`
}

// InstanceMetrics represents live telemetry collected from the provider.
type InstanceMetrics struct {
	InstanceID   string    `json:"instance_id"`
	Timestamp    time.Time `json:"timestamp"`
	CPUUsageNano int64     `json:"cpu_usage_nano"`
	MemoryUsed   int64     `json:"memory_used_bytes"`
	MemoryMax    int64     `json:"memory_max_bytes"`
	DiskUsed     int64     `json:"disk_used_bytes"`
	DiskMax      int64     `json:"disk_max_bytes"`
	NetworkRx    int64     `json:"network_rx_bytes"`
	NetworkTx    int64     `json:"network_tx_bytes"`
}

// Image represents a provider boot image.
type Image struct {
	ID           string    `json:"id"`
	Fingerprint  string    `json:"fingerprint"`
	Alias        string    `json:"alias"`
	Architecture string    `json:"architecture"`
	Type         string    `json:"type"` // container or virtual-machine
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
}

// Snapshot represents a point-in-time state capture of an instance.
type Snapshot struct {
	Name       string    `json:"name"`
	InstanceID string    `json:"instance_id"`
	Stateful   bool      `json:"stateful"`
	CreatedAt  time.Time `json:"created_at"`
}

// StoragePool represents a backend storage pool.
type StoragePool struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"` // dir, lvm, zfs, btrfs
	UsedBytes  int64  `json:"used_bytes"`
	TotalBytes int64  `json:"total_bytes"`
}

// Network represents a virtual network/bridge.
type Network struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // bridge, macvlan, physical
	CIDR      string `json:"cidr,omitempty"`
	DHCP      bool   `json:"dhcp"`
	NAT       bool   `json:"nat"`
	ManagedBy string `json:"managed_by"`
}
