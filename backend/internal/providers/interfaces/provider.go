package interfaces

import (
	"context"
	"fmt"
	"sync"
)

// Provider is the primary interface implemented by virtualization drivers.
type Provider interface {
	Name() string
	Capabilities() CapabilitySet
	Ping(ctx context.Context) error
	Close() error

	InstanceProvider() (InstanceProvider, bool)
	ImageProvider() (ImageProvider, bool)
	SnapshotProvider() (SnapshotProvider, bool)
	StorageProvider() (StorageProvider, bool)
	NetworkProvider() (NetworkProvider, bool)
}

// InstanceProvider abstracts VM and container lifecycle management.
type InstanceProvider interface {
	ListInstances(ctx context.Context) ([]Instance, error)
	GetInstance(ctx context.Context, idOrName string) (*Instance, error)
	CreateInstance(ctx context.Context, inst *Instance) (*Instance, error)
	StartInstance(ctx context.Context, idOrName string) error
	StopInstance(ctx context.Context, idOrName string, force bool) error
	RestartInstance(ctx context.Context, idOrName string) error
	DeleteInstance(ctx context.Context, idOrName string) error
	RenameInstance(ctx context.Context, oldName, newName string) error
	ResizeInstance(ctx context.Context, idOrName string, limits ResourceLimits) error
	GetInstanceMetrics(ctx context.Context, idOrName string) (*InstanceMetrics, error)
}

// InstanceAdopter is an optional interface implemented by virtualization providers that support tagging and adopting external instances.
type InstanceAdopter interface {
	AdoptInstance(ctx context.Context, name string, workloadID string) (*Instance, error)
}

// ImageProvider abstracts OS image handling.
type ImageProvider interface {
	ListImages(ctx context.Context) ([]Image, error)
	GetImage(ctx context.Context, fingerprintOrAlias string) (*Image, error)
	DownloadImage(ctx context.Context, server, alias string) (*Image, error)
	DeleteImage(ctx context.Context, fingerprint string) error
}

// SnapshotProvider abstracts snapshot operations.
type SnapshotProvider interface {
	ListSnapshots(ctx context.Context, instanceID string) ([]Snapshot, error)
	CreateSnapshot(ctx context.Context, instanceID, snapshotName string, stateful bool) (*Snapshot, error)
	RestoreSnapshot(ctx context.Context, instanceID, snapshotName string) error
	DeleteSnapshot(ctx context.Context, instanceID, snapshotName string) error
}

// StorageProvider abstracts storage pool management.
type StorageProvider interface {
	ListStoragePools(ctx context.Context) ([]StoragePool, error)
	GetStoragePool(ctx context.Context, name string) (*StoragePool, error)
}

// NetworkProvider abstracts virtual networks and bridges.
type NetworkProvider interface {
	ListNetworks(ctx context.Context) ([]Network, error)
	GetNetwork(ctx context.Context, name string) (*Network, error)
}

// Global Provider Registry
var (
	registryMutex sync.RWMutex
	providers     = make(map[string]Provider)
)

// RegisterProvider registers a hypervisor provider driver.
func RegisterProvider(name string, p Provider) error {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if p == nil {
		return fmt.Errorf("provider implementation cannot be nil")
	}
	if _, exists := providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}
	providers[name] = p
	return nil
}

// GetProvider retrieves a registered provider by name.
func GetProvider(name string) (Provider, error) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	p, exists := providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %q not found in registry", name)
	}
	return p, nil
}

// ListProviders returns a map of all registered provider names and their capabilities.
func ListProviders() map[string]CapabilitySet {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	result := make(map[string]CapabilitySet, len(providers))
	for name, p := range providers {
		result[name] = p.Capabilities()
	}
	return result
}

// ResetRegistry clears registered providers (used primarily for unit testing).
func ResetRegistry() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	providers = make(map[string]Provider)
}
