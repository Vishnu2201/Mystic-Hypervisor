package incus

import (
	"context"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// IncusProvider implements the Provider interface for Incus hypervisors.
type IncusProvider struct {
	socketPath string
	caps       interfaces.CapabilitySet
}

// NewIncusProvider creates an Incus provider stub.
func NewIncusProvider(socketPath string) *IncusProvider {
	if socketPath == "" {
		socketPath = "/var/lib/incus/unix.socket"
	}
	return &IncusProvider{
		socketPath: socketPath,
		caps: interfaces.NewCapabilitySet(
			interfaces.CapVM,
			interfaces.CapContainer,
			interfaces.CapSnapshots,
			interfaces.CapStoragePools,
			interfaces.CapConsoleStream,
			interfaces.CapExec,
			interfaces.CapResize,
			interfaces.CapCloudInit,
		),
	}
}

func (p *IncusProvider) Name() string {
	return "incus"
}

func (p *IncusProvider) Capabilities() interfaces.CapabilitySet {
	return p.caps
}

func (p *IncusProvider) Ping(ctx context.Context) error {
	// Milestone 1 stub: Return provider unavailable until Incus driver is activated in Milestone 3
	return interfaces.ErrProviderUnavailable
}

func (p *IncusProvider) Close() error {
	return nil
}

func (p *IncusProvider) InstanceProvider() (interfaces.InstanceProvider, bool) {
	return nil, false
}

func (p *IncusProvider) ImageProvider() (interfaces.ImageProvider, bool) {
	return nil, false
}

func (p *IncusProvider) SnapshotProvider() (interfaces.SnapshotProvider, bool) {
	return nil, false
}

func (p *IncusProvider) StorageProvider() (interfaces.StorageProvider, bool) {
	return nil, false
}

func (p *IncusProvider) NetworkProvider() (interfaces.NetworkProvider, bool) {
	return nil, false
}
