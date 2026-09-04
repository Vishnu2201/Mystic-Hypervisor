package instances

import (
	"context"
	"testing"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

func TestReconcilerProviderAuthoritative(t *testing.T) {
	rec := NewReconciler()
	ctx := context.Background()

	meta := &InstanceMetadata{
		ID:           "vm-101",
		Name:         "test-web-vm",
		Type:         interfaces.InstanceTypeVM,
		DesiredState: interfaces.StateRunning,
		CreatedAt:    time.Now(),
	}

	// Provider reports VM is STOPPED
	providerInst := &interfaces.Instance{
		ID:        "vm-101",
		Name:      "test-web-vm",
		Type:      interfaces.InstanceTypeVM,
		State:     interfaces.StateStopped,
		IPAddress: "",
	}

	result := rec.Reconcile(ctx, meta, providerInst)

	if result == nil {
		t.Fatal("expected non-nil reconciled instance")
	}

	// The provider's state (STOPPED) must be authoritative despite DB metadata desired state (RUNNING)
	if result.AuthoritativeState != interfaces.StateStopped {
		t.Errorf("expected AuthoritativeState StateStopped, got %s", result.AuthoritativeState)
	}

	if result.SyncStatus != SyncOutOfSync {
		t.Errorf("expected SyncStatus SyncOutOfSync, got %s", result.SyncStatus)
	}
}

func TestReconcilerMissingFromProvider(t *testing.T) {
	rec := NewReconciler()
	ctx := context.Background()

	meta := &InstanceMetadata{
		ID:   "ct-202",
		Name: "test-container",
		Type: interfaces.InstanceTypeContainer,
	}

	result := rec.Reconcile(ctx, meta, nil)

	if result.AuthoritativeState != interfaces.StateUnknown {
		t.Errorf("expected StateUnknown for missing provider instance, got %s", result.AuthoritativeState)
	}

	if result.SyncStatus != SyncProviderMissing {
		t.Errorf("expected SyncStatus SyncProviderMissing, got %s", result.SyncStatus)
	}
}
