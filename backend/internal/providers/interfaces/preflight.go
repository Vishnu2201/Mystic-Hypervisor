package interfaces

import "context"

type ProviderAvailability string

const (
	AvailabilityAvailable   ProviderAvailability = "AVAILABLE"
	AvailabilityUnavailable ProviderAvailability = "UNAVAILABLE"
)

type InstanceOwnership string

const (
	OwnershipMysticOwned InstanceOwnership = "MYSTIC_OWNED"
	OwnershipExternal    InstanceOwnership = "EXTERNAL"
	OwnershipUnknown     InstanceOwnership = "UNKNOWN"
)

type ProviderHealthStatus struct {
	Installed   bool `json:"installed"`
	Reachable   bool `json:"reachable"`
	Operational bool `json:"operational"`
	Capable     bool `json:"capable"`
}

type PreflightInstance struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	State     string            `json:"state"`
	Ownership InstanceOwnership `json:"ownership"`
	IPAddress string            `json:"ip_address,omitempty"`
}

type DiscoveredNetwork struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Managed bool   `json:"managed"`
	IPv4    string `json:"ipv4,omitempty"`
	IPv6    string `json:"ipv6,omitempty"`
	State   string `json:"state,omitempty"`
}

type DiscoveredStoragePool struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Status     string `json:"status,omitempty"`
	UsedBytes  int64  `json:"used_bytes,omitempty"`
	TotalBytes int64  `json:"total_bytes,omitempty"`
}

type DiscoveredImage struct {
	Fingerprint  string `json:"fingerprint"`
	Alias        string `json:"alias"`
	Description  string `json:"description,omitempty"`
	OS           string `json:"os,omitempty"`
	Release      string `json:"release,omitempty"`
	Architecture string `json:"architecture"`
	SizeBytes    int64  `json:"size_bytes"`
}

type ProviderServerInfo struct {
	ServerVersion string `json:"server_version,omitempty"`
	OS            string `json:"os,omitempty"`
	Kernel        string `json:"kernel,omitempty"`
	Architecture  string `json:"architecture,omitempty"`
	KVMSupported  bool   `json:"kvm_supported"`
}

type ProviderPreflightResult struct {
	Provider          string                  `json:"provider"`
	Availability      ProviderAvailability    `json:"availability"`
	HealthStatus      ProviderHealthStatus    `json:"health_status"`
	ServerInfo        ProviderServerInfo      `json:"server_info"`
	Capabilities      []string                `json:"capabilities"`
	ExistingInstances []PreflightInstance     `json:"existing_instances"`
	Networks          []DiscoveredNetwork     `json:"networks"`
	StoragePools      []DiscoveredStoragePool `json:"storage_pools"`
	Images            []DiscoveredImage       `json:"images"`
	Warnings          []string                `json:"warnings"`
	Blockers          []string                `json:"blockers"`
}

// ProviderPreflightChecker is an optional interface implemented by providers to perform read-only preflight discovery.
type ProviderPreflightChecker interface {
	Preflight(ctx context.Context) (*ProviderPreflightResult, error)
}
