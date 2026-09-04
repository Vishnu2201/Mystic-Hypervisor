package kvm

import (
	"context"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// KVMProvider implements the Provider interface for direct KVM/QEMU hypervisor management.
type KVMProvider struct {
	binaryPath string
	caps       interfaces.CapabilitySet
}

// NewKVMProvider creates a KVM provider stub.
func NewKVMProvider(binaryPath string) *KVMProvider {
	if binaryPath == "" {
		binaryPath = "/usr/bin/qemu-system-x86_64"
	}
	return &KVMProvider{
		binaryPath: binaryPath,
		caps: interfaces.NewCapabilitySet(
			interfaces.CapVM,
			interfaces.CapSnapshots,
			interfaces.CapConsoleStream,
			interfaces.CapCloudInit,
		),
	}
}

func (p *KVMProvider) Name() string {
	return "kvm"
}

func (p *KVMProvider) Capabilities() interfaces.CapabilitySet {
	return p.caps
}

func (p *KVMProvider) Ping(ctx context.Context) error {
	return interfaces.ErrProviderUnavailable
}

func (p *KVMProvider) Close() error {
	return nil
}

func (p *KVMProvider) InstanceProvider() (interfaces.InstanceProvider, bool) {
	return nil, false
}

func (p *KVMProvider) ImageProvider() (interfaces.ImageProvider, bool) {
	return nil, false
}

func (p *KVMProvider) SnapshotProvider() (interfaces.SnapshotProvider, bool) {
	return nil, false
}

func (p *KVMProvider) StorageProvider() (interfaces.StorageProvider, bool) {
	return nil, false
}

func (p *KVMProvider) NetworkProvider() (interfaces.NetworkProvider, bool) {
	return nil, false
}
