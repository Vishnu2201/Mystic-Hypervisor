package workloads

import (
	"context"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// WorkloadType defines supported hypervisor workload categories.
type WorkloadType string

const (
	TypeIncusContainer WorkloadType = "INCUS_CONTAINER"
	TypeIncusVM        WorkloadType = "INCUS_VM"
	TypeKVMVM          WorkloadType = "KVM_VM"
	TypeLXCContainer   WorkloadType = "LXC_CONTAINER"
)

// WorkloadStatus represents the complete provisioning and operational lifecycle.
type WorkloadStatus string

const (
	StatusDraft        WorkloadStatus = "DRAFT"
	StatusValidated    WorkloadStatus = "VALIDATED"
	StatusPlanned      WorkloadStatus = "PLANNED"
	StatusApproved     WorkloadStatus = "APPROVED"
	StatusProvisioning WorkloadStatus = "PROVISIONING"
	StatusRunning      WorkloadStatus = "RUNNING"
	StatusStopped      WorkloadStatus = "STOPPED"
	StatusFailed       WorkloadStatus = "FAILED"
	StatusDrifted      WorkloadStatus = "DRIFTED"
	StatusUnknown      WorkloadStatus = "UNKNOWN"
)

// Workload represents a real guest virtual machine or container managed by Mystic.
type Workload struct {
	ID                 string                           `json:"id"`
	Name               string                           `json:"name"`
	HostID             string                           `json:"host_id"`
	Provider           string                           `json:"provider"` // e.g. "incus"
	ProviderInstanceID string                           `json:"provider_instance_id"`
	Type               WorkloadType                     `json:"type"`
	Status             WorkloadStatus                   `json:"status"`
	DesiredState       interfaces.InstanceState         `json:"desired_state"`
	ActualState        interfaces.InstanceState         `json:"actual_state"`
	SyncStatus         instances.SyncStatus             `json:"sync_status"`
	CPU                int                              `json:"cpu"`
	MemoryMB           int64                            `json:"memory_mb"`
	StorageGB          int64                            `json:"storage_gb"`
	Image              string                           `json:"image"`
	Project            string                           `json:"project"`
	Profile            string                           `json:"profile"`
	NetworkConfig      networking.WorkloadNetworkConfig `json:"network_config"`
	PortRequest        networking.PortAllocationRequest `json:"port_request,omitempty"`
	PlanHash           string                           `json:"plan_hash,omitempty"`
	CreatedAt          string                           `json:"created_at"`
	UpdatedAt          string                           `json:"updated_at"`
	LastProviderSync   string                           `json:"last_provider_sync"`
	ErrorDetails       string                           `json:"error_details,omitempty"`
}

// WorkloadSpec defines the payload for creating a workload intent.
type WorkloadSpec struct {
	Name          string                        `json:"name"`
	Provider      string                        `json:"provider"`
	Type          WorkloadType                  `json:"type"`
	Image         string                        `json:"image"`
	Project       string                        `json:"project"`
	Profile       string                        `json:"profile"`
	CPU           int                           `json:"cpu"`
	MemoryMB      int64                         `json:"memory_mb"`
	StorageGB     int64                         `json:"storage_gb"`
	HostID        string                        `json:"host_id"`
	NetworkName   string                        `json:"network_name"`
	PrivateIP     string                        `json:"private_ip"`
	ExposureMode  hosts.ExposureMode            `json:"exposure_mode"`
	GatewayID     string                        `json:"gateway_id,omitempty"`
	PortRequest   networking.PortAllocationRequest `json:"port_request"`
}

// ProvisioningPlan summarizes pre-flight validation and action sequence.
type ProvisioningPlan struct {
	WorkloadID       string                      `json:"workload_id"`
	WorkloadName     string                      `json:"workload_name"`
	Provider         string                      `json:"provider"`
	Type             WorkloadType                `json:"type"`
	Image            string                      `json:"image"`
	Resources        map[string]interface{}      `json:"resources"`
	Network          map[string]interface{}      `json:"network"`
	Exposure         map[string]interface{}      `json:"exposure"`
	Actions          []string                    `json:"actions"`
	Risks            []string                    `json:"risks"`
	IsValid          bool                        `json:"is_valid"`
	ValidationResult networking.ValidationResult `json:"validation_result"`
	PlanHash         string                      `json:"plan_hash"`
	Approved         bool                        `json:"approved"`
	CreatedAt        string                      `json:"created_at"`
}

// WorkloadService defines workload management boundaries.
type WorkloadService interface {
	CreateWorkload(ctx context.Context, spec WorkloadSpec) (*Workload, error)
	ValidateWorkload(ctx context.Context, id string) (*networking.ValidationResult, error)
	GeneratePlan(ctx context.Context, id string) (*ProvisioningPlan, error)
	ApprovePlan(ctx context.Context, id string) error
	ProvisionWorkload(ctx context.Context, id string) (*Workload, error)
	StartWorkload(ctx context.Context, id string) (*Workload, error)
	StopWorkload(ctx context.Context, id string, force bool) (*Workload, error)
	RestartWorkload(ctx context.Context, id string) (*Workload, error)
	DeleteWorkload(ctx context.Context, id string) error
	ReconcileWorkload(ctx context.Context, id string) (*Workload, error)
	GetWorkload(ctx context.Context, id string) (*Workload, error)
	ListWorkloads(ctx context.Context) ([]Workload, error)
}
