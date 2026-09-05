package instances

import (
	"context"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// SyncStatus represents the alignment between Mystic DB metadata and provider runtime state.
type SyncStatus string

const (
	SyncInSync          SyncStatus = "in_sync"
	SyncOutOfSync       SyncStatus = "out_of_sync"
	SyncProviderMissing SyncStatus = "provider_missing"
	SyncOrphaned        SyncStatus = "orphaned_in_provider"
)

// InstanceMetadata stores user configurations and tags in Mystic DB.
type InstanceMetadata struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Type         interfaces.InstanceType  `json:"type"`
	DesiredState interfaces.InstanceState `json:"desired_state"`
	CPUCores     int                      `json:"cpu_cores,omitempty"`
	MemoryBytes  int64                    `json:"memory_bytes,omitempty"`
	DiskBytes    int64                    `json:"disk_bytes,omitempty"`
	OwnerID      string                   `json:"owner_id"`
	Node         string                   `json:"node"`
	Tags         []string                 `json:"tags,omitempty"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}

// ReconciledInstance represents the unified view presented to API and UI.
// The live state is strictly derived from the virtualization provider.
type ReconciledInstance struct {
	Metadata           InstanceMetadata            `json:"metadata"`
	AuthoritativeState interfaces.InstanceState    `json:"authoritative_state"`
	SyncStatus         SyncStatus                  `json:"sync_status"`
	LiveIPAddress      string                      `json:"live_ip_address,omitempty"`
	LiveMetrics        *interfaces.InstanceMetrics `json:"live_metrics,omitempty"`
}

// Reconciler handles state resolution between database metadata and real hypervisors.
type Reconciler struct{}

// NewReconciler creates a state reconciler.
func NewReconciler() *Reconciler {
	return &Reconciler{}
}

// Reconcile resolves metadata against provider runtime state.
// Provider state ALWAYS overrides metadata for runtime state display.
func (r *Reconciler) Reconcile(
	ctx context.Context,
	meta *InstanceMetadata,
	providerInst *interfaces.Instance,
) *ReconciledInstance {
	if meta == nil && providerInst == nil {
		return nil
	}

	// Case 1: Orphaned provider instance (exists in provider but not in DB)
	if meta == nil && providerInst != nil {
		return &ReconciledInstance{
			Metadata: InstanceMetadata{
				ID:        providerInst.ID,
				Name:      providerInst.Name,
				Type:      providerInst.Type,
				Node:      providerInst.Node,
				CreatedAt: providerInst.CreatedAt,
			},
			AuthoritativeState: providerInst.State,
			SyncStatus:         SyncOrphaned,
			LiveIPAddress:      providerInst.IPAddress,
		}
	}

	// Case 2: Missing from provider (exists in DB metadata but provider cannot find it)
	if meta != nil && providerInst == nil {
		return &ReconciledInstance{
			Metadata:           *meta,
			AuthoritativeState: interfaces.StateUnknown,
			SyncStatus:         SyncProviderMissing,
		}
	}

	// Case 3: Both exist — resolve sync status
	status := SyncInSync
	if meta.DesiredState != "" && meta.DesiredState != providerInst.State {
		status = SyncOutOfSync
	}

	metaMemMB := meta.MemoryBytes / (1024 * 1024)
	provMemMB := providerInst.Limits.MemoryBytes / (1024 * 1024)
	if metaMemMB > 0 && provMemMB > 0 && metaMemMB != provMemMB {
		status = SyncOutOfSync
	}

	metaDiskGB := meta.DiskBytes / (1024 * 1024 * 1024)
	provDiskGB := providerInst.Limits.DiskBytes / (1024 * 1024 * 1024)
	if metaDiskGB > 0 && provDiskGB > 0 && metaDiskGB != provDiskGB {
		status = SyncOutOfSync
	}

	if meta.CPUCores > 0 && providerInst.Limits.CPUCores > 0 && meta.CPUCores != providerInst.Limits.CPUCores {
		status = SyncOutOfSync
	}

	return &ReconciledInstance{
		Metadata:           *meta,
		AuthoritativeState: providerInst.State, // Provider is authoritative!
		SyncStatus:         status,
		LiveIPAddress:      providerInst.IPAddress,
	}
}
