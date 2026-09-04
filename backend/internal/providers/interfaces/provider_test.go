package interfaces

import (
	"context"
	"testing"
)

func TestCapabilitySet(t *testing.T) {
	caps := NewCapabilitySet(CapVM, CapContainer, CapSnapshots)

	if !caps.Has(CapVM) {
		t.Error("expected capability CapVM to be present")
	}
	if !caps.Has(CapContainer) {
		t.Error("expected capability CapContainer to be present")
	}
	if !caps.Has(CapSnapshots) {
		t.Error("expected capability CapSnapshots to be present")
	}
	if caps.Has(CapLiveMigration) {
		t.Error("expected capability CapLiveMigration to be absent")
	}

	list := caps.List()
	if len(list) != 3 {
		t.Errorf("expected 3 capabilities, got %d", len(list))
	}
}

type dummyProvider struct {
	name string
	caps CapabilitySet
}

func (d *dummyProvider) Name() string                                  { return d.name }
func (d *dummyProvider) Capabilities() CapabilitySet                  { return d.caps }
func (d *dummyProvider) Ping(ctx context.Context) error                { return nil }
func (d *dummyProvider) Close() error                                  { return nil }
func (d *dummyProvider) InstanceProvider() (InstanceProvider, bool)   { return nil, false }
func (d *dummyProvider) ImageProvider() (ImageProvider, bool)          { return nil, false }
func (d *dummyProvider) SnapshotProvider() (SnapshotProvider, bool)   { return nil, false }
func (d *dummyProvider) StorageProvider() (StorageProvider, bool)     { return nil, false }
func (d *dummyProvider) NetworkProvider() (NetworkProvider, bool)     { return nil, false }

func TestProviderRegistry(t *testing.T) {
	ResetRegistry()
	defer ResetRegistry()

	dp := &dummyProvider{name: "test-provider", caps: NewCapabilitySet(CapContainer)}

	if err := RegisterProvider("test-provider", dp); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	p, err := GetProvider("test-provider")
	if err != nil {
		t.Fatalf("GetProvider failed: %v", err)
	}
	if p.Name() != "test-provider" {
		t.Errorf("expected provider name 'test-provider', got %s", p.Name())
	}

	all := ListProviders()
	if len(all) != 1 {
		t.Errorf("expected 1 registered provider, got %d", len(all))
	}

	if err := RegisterProvider("test-provider", dp); err == nil {
		t.Error("expected duplicate registration error, got nil")
	}
}
