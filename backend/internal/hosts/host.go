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
	ExposurePrivateOnly     ExposureMode = "PRIVATE_ONLY"
	ExposureNATForwarded    ExposureMode = "NAT_FORWARDED"
	ExposureDirectPublic    ExposureMode = "DIRECT_PUBLIC"
	ExposureExternalGateway ExposureMode = "EXTERNAL_GATEWAY"
	ExposureUnconfigured    ExposureMode = "UNCONFIGURED"
)

// GatewayType defines the category of upstream router or proxy gateway.
type GatewayType string

const (
	GatewayUpstreamProvider GatewayType = "UPSTREAM_PROVIDER"
	GatewayExternalRouter   GatewayType = "EXTERNAL_ROUTER"
	GatewayExternalVPS      GatewayType = "EXTERNAL_VPS"
	GatewayMysticHost       GatewayType = "MYSTIC_HOST"
	GatewayDedicatedGateway GatewayType = "DEDICATED_GATEWAY"
	GatewayCloudGateway     GatewayType = "CLOUD_GATEWAY"
	GatewayUnknown          GatewayType = "UNKNOWN"
)

// ExposureState indicates the operational state of exposure configuration.
type ExposureState string

const (
	ExposureStateUnconfigured ExposureState = "UNCONFIGURED"
	ExposureStateConfigured   ExposureState = "CONFIGURED"
	ExposureStateRequested    ExposureState = "REQUESTED"
	ExposureStateApplied      ExposureState = "APPLIED"
	ExposureStateVerified     ExposureState = "VERIFIED"
	ExposureStateFailed       ExposureState = "FAILED"
)

// Protocol defines network protocol for port forwarding.
type Protocol string

const (
	ProtocolTCP    Protocol = "TCP"
	ProtocolUDP    Protocol = "UDP"
	ProtocolTCPUDP Protocol = "TCP_UDP"
)

// Gateway represents an upstream router, firewall, or proxy device.
type Gateway struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Type              GatewayType `json:"type"`
	PublicIPs         []string    `json:"public_ips"`
	PrivateIPs        []string    `json:"private_ips"`
	ManagementAddress string      `json:"management_address,omitempty"`
	ManagedBy         string      `json:"managed_by"`
	Enabled           bool        `json:"enabled"`
}

// ForwardingRule represents a conceptual NAT/port forwarding mapping.
type ForwardingRule struct {
	ID                string        `json:"id"`
	GatewayID         string        `json:"gateway_id,omitempty"`
	PublicIP          string        `json:"public_ip"`
	PublicPort        int           `json:"public_port"`
	Protocol          Protocol      `json:"protocol"`
	DestinationHostID string        `json:"destination_host_id"`
	DestinationIP     string        `json:"destination_ip"`
	DestinationPort   int           `json:"destination_port"`
	WorkloadID        string        `json:"workload_id,omitempty"`
	State             ExposureState `json:"state"`
	Owner             ResourceOwner `json:"owner"`
	Description       string        `json:"description,omitempty"`
	UpstreamIP        string        `json:"upstream_ip,omitempty"`
	ExternalPort      int           `json:"external_port,omitempty"`
	InternalIP        string        `json:"internal_ip,omitempty"`
	InternalPort      int           `json:"internal_port,omitempty"`
}

// NetworkExposureConfig defines user/administrator network exposure preferences.
type NetworkExposureConfig struct {
	ExposureMode    ExposureMode     `json:"exposure_mode"`
	GatewayID       string           `json:"gateway_id,omitempty"`
	GatewayPublicIP string           `json:"gateway_public_ip,omitempty"`
	ForwardingRules []ForwardingRule `json:"forwarding_rules"`
}

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
	ExposureConfig           NetworkExposureConfig    `json:"exposure_config"`
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
