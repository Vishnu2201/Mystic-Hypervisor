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

// ResourceOwner defines the explicit ownership model for pre-existing host assets.
type ResourceOwner string

const (
	OwnerSystem      ResourceOwner = "SYSTEM"
	OwnerMystic      ResourceOwner = "MYSTIC"
	OwnerIncus       ResourceOwner = "INCUS"
	OwnerLXC         ResourceOwner = "LXC"
	OwnerKVM         ResourceOwner = "KVM"
	OwnerDocker      ResourceOwner = "DOCKER"
	OwnerPterodactyl ResourceOwner = "PTERODACTYL"
	OwnerLibvirt     ResourceOwner = "LIBVIRT"
	OwnerUnknown     ResourceOwner = "UNKNOWN"
)

// ExposureMode represents user-configured network exposure topology.
type ExposureMode string

const (
	ExposurePrivateOnly  ExposureMode = "PRIVATE_ONLY"
	ExposureNATForwarded ExposureMode = "NAT_FORWARDED"
	ExposureDirectPublic ExposureMode = "DIRECT_PUBLIC"
	ExposureUnconfigured ExposureMode = "UNCONFIGURED"
)

// PublicIPAssignmentStatus indicates whether a globally routable public IP is assigned to a local interface.
type PublicIPAssignmentStatus string

const (
	PublicIPDirect      PublicIPAssignmentStatus = "DIRECT"
	PublicIPNotAssigned PublicIPAssignmentStatus = "NOT_ASSIGNED"
	PublicIPUnknown     PublicIPAssignmentStatus = "UNKNOWN"
)

// NATStatus indicates observed upstream NAT network topology.
type NATStatus string

const (
	NATNotDetected NATStatus = "NOT_DETECTED"
	NATLikely      NATStatus = "LIKELY"
	NATUnknown     NATStatus = "UNKNOWN"
)

// ForwardingRule represents a conceptual NAT/port forwarding mapping.
type ForwardingRule struct {
	UpstreamIP   string        `json:"upstream_ip"`
	ExternalPort int           `json:"external_port"`
	InternalIP   string        `json:"internal_ip"`
	InternalPort int           `json:"internal_port"`
	Protocol     string        `json:"protocol"` // TCP / UDP
	Owner        ResourceOwner `json:"owner"`
}

// HostInfo represents metadata and deep inspection telemetry of a hypervisor node.
type HostInfo struct {
	ID                       string                   `json:"id"`
	Hostname                 string                   `json:"hostname"`
	MachineIDHash            string                   `json:"machine_id_hash"`
	PrivateIP                string                   `json:"private_ip"`
	HostPublicIP             string                   `json:"host_public_ip"`
	UpstreamPublicIP         string                   `json:"upstream_public_ip,omitempty"`
	PublicIPAssignmentStatus PublicIPAssignmentStatus `json:"public_ip_assignment_status"`
	NATStatus                NATStatus                `json:"nat_status"`
	DetectedTopology         string                   `json:"detected_topology"`
	ConfiguredExposureMode   ExposureMode             `json:"configured_exposure_mode"`
	OS                       string                   `json:"os"`
	Kernel                   string                   `json:"kernel"`
	Architecture             string                   `json:"architecture"`
	BootMode                 string                   `json:"boot_mode"`
	Uptime                   string                   `json:"uptime"`
	CgroupVersion            string                   `json:"cgroup_version"`
	VirtEnvironment          string                   `json:"virt_environment"`
	CPUCores                 int                      `json:"cpu_cores"`
	CPUModel                 string                   `json:"cpu_model"`
	CPUTopology              string                   `json:"cpu_topology"`
	TotalRAMBytes            int64                    `json:"total_ram_bytes"`
	SwapRAMBytes             int64                    `json:"swap_ram_bytes"`
	RootDiskFree             string                   `json:"root_disk_free"`
	RootDiskTotal            string                   `json:"root_disk_total"`
	FSType                   string                   `json:"fs_type"`
	InodeUsage               string                   `json:"inode_usage"`
	KVMStatus                string                   `json:"kvm_status"`
	KVMReason                string                   `json:"kvm_reason"`
	Status                   HostStatus               `json:"status"`
	DefaultOwner             ResourceOwner            `json:"default_owner"`
	UpdatedAt                time.Time                `json:"updated_at"`
}

// HostInspector defines boundaries for host system inspection.
type HostInspector interface {
	InspectHost(ctx context.Context) (*HostInfo, error)
}
