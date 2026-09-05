package networking

import (
	"context"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
)

// AllocationMode defines how ports are requested.
type AllocationMode string

const (
	AllocationModeSingle   AllocationMode = "SINGLE"
	AllocationModeRange    AllocationMode = "RANGE"
	AllocationModeExplicit AllocationMode = "EXPLICIT"
)

// GatewayManagement defines whether Mystic can manage the gateway device directly.
type GatewayManagement string

const (
	GatewayManagedByMystic   GatewayManagement = "MANAGED_BY_MYSTIC"
	GatewayExternallyManaged GatewayManagement = "EXTERNALLY_MANAGED"
	GatewayManagementUnknown GatewayManagement = "UNKNOWN"
)

// ConflictStatus defines detailed diagnostic conflict categorization.
type ConflictStatus string

const (
	ConflictAvailable                ConflictStatus = "AVAILABLE"
	ConflictAlreadyAllocatedByMystic ConflictStatus = "ALREADY_ALLOCATED_BY_MYSTIC"
	ConflictListeningOnHost          ConflictStatus = "LISTENING_ON_HOST"
	ConflictReservedManagement       ConflictStatus = "RESERVED_MANAGEMENT"
	ConflictOwnedByExternalSubsystem ConflictStatus = "OWNED_BY_EXTERNAL_SUBSYSTEM"
	ConflictUnknown                  ConflictStatus = "UNKNOWN"
	ConflictPoolUnconfigured         ConflictStatus = "ALLOCATION_POOL_UNCONFIGURED"
)

// Interface represents a physical or virtual host network interface.
type Interface struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	IPAddresses []string `json:"ip_addresses"`
	MAC         string   `json:"mac"`
	IsUp        bool     `json:"is_up"`
}

// WorkloadNetworkConfig represents the network exposure identity of a VM or container.
type WorkloadNetworkConfig struct {
	ID                string                 `json:"id"`
	WorkloadID        string                 `json:"workload_id"`
	WorkloadName      string                 `json:"workload_name"`
	HostID            string                 `json:"host_id"`
	NetworkID         string                 `json:"network_id"`
	NetworkName       string                 `json:"network_name"`
	Interface         string                 `json:"interface"`
	PrivateIPv4       string                 `json:"private_ipv4"`
	PrivateIPv6       string                 `json:"private_ipv6,omitempty"`
	PublicIPv4        string                 `json:"public_ipv4,omitempty"`
	PublicIPv6        string                 `json:"public_ipv6,omitempty"`
	ExposureMode      hosts.ExposureMode     `json:"exposure_mode"`
	GatewayID         string                 `json:"gateway_id,omitempty"`
	ForwardingRules   []hosts.ForwardingRule `json:"forwarding_rules"`
	AllocationState   hosts.ExposureState    `json:"allocation_state"`
	VerificationState string                 `json:"verification_state"`
}

// AllocationPool defines an administrator-configured external port range pool.
type AllocationPool struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	StartPort    int            `json:"start_port"`
	EndPort      int            `json:"end_port"`
	Protocol     hosts.Protocol `json:"protocol"`
	IsConfigured bool           `json:"is_configured"`
}

// GatewayConfig represents detailed configuration and management capabilities of a gateway.
type GatewayConfig struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Type                 hosts.GatewayType `json:"type"`
	PublicIPs            []string          `json:"public_ips"`
	PrivateIPs           []string          `json:"private_ips"`
	ManagementAddress    string            `json:"management_address,omitempty"`
	ManagementCapability GatewayManagement `json:"management_capability"`
	ManagedBy            string            `json:"managed_by"`
	Enabled              bool              `json:"enabled"`
}

// PortMapping represents a normalized 1:1 single or range port forwarding pair.
type PortMapping struct {
	ExternalPort  int            `json:"external_port"`
	InternalPort  int            `json:"internal_port"`
	Protocol      hosts.Protocol `json:"protocol"`
	PublicIP      string         `json:"public_ip"`
	DestinationIP string         `json:"destination_ip"`
}

// ConflictDetail provides granular feedback on a specific port collision.
type ConflictDetail struct {
	Port     int            `json:"port"`
	Protocol hosts.Protocol `json:"protocol"`
	Status   ConflictStatus `json:"status"`
	Owner    string         `json:"owner"`
	Message  string         `json:"message"`
}

// PortAllocationRequest describes an incoming port allocation query or creation attempt.
type PortAllocationRequest struct {
	WorkloadID        string                 `json:"workload_id"`
	HostID            string                 `json:"host_id"`
	GatewayID         string                 `json:"gateway_id,omitempty"`
	Mode              AllocationMode         `json:"mode"` // SINGLE, RANGE, EXPLICIT
	ExternalStartPort int                    `json:"external_start_port"`
	ExternalEndPort   int                    `json:"external_end_port"`
	InternalStartPort int                    `json:"internal_start_port"`
	InternalEndPort   int                    `json:"internal_end_port"`
	Protocol          hosts.Protocol         `json:"protocol"`
	PublicIP          string                 `json:"public_ip,omitempty"`
	DestinationIP     string                 `json:"destination_ip"`
	ExplicitRules     []hosts.ForwardingRule `json:"explicit_rules,omitempty"`
}

// ValidationResult summarizes the outcome of port validation and conflict checking.
type ValidationResult struct {
	IsValid            bool                `json:"is_valid"`
	Status             ConflictStatus      `json:"status"`
	Conflicts          []ConflictDetail    `json:"conflicts"`
	Warnings           []string            `json:"warnings"`
	Blockers           []string            `json:"blockers"`
	NormalizedMappings []PortMapping       `json:"normalized_mappings"`
	AllocationState    hosts.ExposureState `json:"allocation_state"`
	Message            string              `json:"message"`
}

// NetworkService defines host network interface queries.
type NetworkService interface {
	ListInterfaces(ctx context.Context) ([]Interface, error)
}
