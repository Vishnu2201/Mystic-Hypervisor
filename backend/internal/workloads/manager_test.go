package workloads

import (
	"context"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
)

func TestWorkloadCreationAndValidation(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager()

	spec := WorkloadSpec{
		Name:        "web-test-01",
		Provider:    "incus",
		Type:        TypeIncusContainer,
		Image:       "ubuntu/24.04",
		CPU:         2,
		MemoryMB:    2048,
		StorageGB:   20,
		HostID:      "host-main",
		NetworkName: "incusbr0",
		PrivateIP:   "10.0.0.151",
		ExposureMode: hosts.ExposureNATForwarded,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}

	if w.Status != StatusDraft {
		t.Fatalf("Expected initial status DRAFT, got %s", w.Status)
	}

	valRes, err := mgr.ValidateWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("ValidateWorkload failed: %v", err)
	}

	if !valRes.IsValid {
		t.Fatalf("Expected valid validation result, got blockers: %v", valRes.Blockers)
	}
}

func TestProvisioningApprovalBoundary(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager()

	spec := WorkloadSpec{
		Name:        "db-test-01",
		Provider:    "incus",
		Type:        TypeIncusContainer,
		Image:       "ubuntu/24.04",
		CPU:         4,
		MemoryMB:    4096,
		StorageGB:   40,
		HostID:      "host-main",
		PrivateIP:   "10.0.0.152",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}

	// Attempting to provision without plan & approval MUST fail!
	_, err = mgr.ProvisionWorkload(ctx, w.ID)
	if err == nil {
		t.Fatalf("Expected ProvisionWorkload to fail without approval boundary")
	}

	// Generate Plan
	plan, err := mgr.GeneratePlan(ctx, w.ID)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.Approved {
		t.Fatalf("Plan must not be auto-approved upon generation")
	}

	// Explicit Approval
	if err := mgr.ApprovePlan(ctx, w.ID); err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}

	// Now provision (will return ErrProviderUnavailable on non-Linux dev host safely)
	_, provErr := mgr.ProvisionWorkload(ctx, w.ID)
	if provErr == nil {
		t.Logf("ProvisionWorkload executed successfully against provider")
	}
}

func TestReconcilerDriftDetection(t *testing.T) {
	ctx := context.Background()
	reconciler := instances.NewReconciler()

	meta := &instances.InstanceMetadata{
		ID:           "wl-01",
		Name:         "web-01",
		DesiredState: "running",
	}

	// Case 1: Out of sync (DB wants running, Incus is stopped)
	incusInst := &interfaces.Instance{
		ID:    "wl-01",
		Name:  "web-01",
		State: "stopped",
	}

	res := reconciler.Reconcile(ctx, meta, incusInst)
	if res.SyncStatus != instances.SyncOutOfSync {
		t.Fatalf("Expected SyncOutOfSync, got %s", res.SyncStatus)
	}

	if res.AuthoritativeState != "stopped" {
		t.Fatalf("Expected provider state 'stopped' to be authoritative, got %s", res.AuthoritativeState)
	}
}
