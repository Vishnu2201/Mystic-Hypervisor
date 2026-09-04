package lxc

import (
	"context"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// LXCProvider implements the Provider interface for lightweight LXC container workloads.
type LXCProvider struct {
	socketPath string
	caps       interfaces.CapabilitySet
}

// NewLXCProvider creates an LXC provider stub.
func NewLXCProvider(socketPath string) *LXCProvider {
	if socketPath == "" {
		socketPath = "/var/lib/lxc/unix.socket"
	}
	return &LXCProvider{
		socketPath: socketPath,
		caps: interfaces.NewCapabilitySet(
			interfaces.CapContainer,
			interfaces.CapSnapshots,
			interfaces.CapExec,
			interfaces.CapResize,
		),
	}
}

func (p *LXCProvider) Name() string {
	return "lxc"
}

func (p *LXCProvider) Capabilities() interfaces.CapabilitySet {
	return p.caps
}

func (p *LXCProvider) Ping(ctx context.Context) error {
	return interfaces.ErrProviderUnavailable
}

func (p *LXCProvider) Close() error {
	return nil
}

func (p *LXCProvider) InstanceProvider() (interfaces.InstanceProvider, bool) {
	return nil, false
}

func (p *LXCProvider) ImageProvider() (interfaces.ImageProvider, bool) {
	return nil, false
}

func (p *LXCProvider) SnapshotProvider() (interfaces.SnapshotProvider, bool) {
	return nil, false
}

func (p *LXCProvider) StorageProvider() (interfaces.StorageProvider, bool) {
	return nil, false
}

func (p *LXCProvider) NetworkProvider() (interfaces.NetworkProvider, bool) {
	return nil, false
}
