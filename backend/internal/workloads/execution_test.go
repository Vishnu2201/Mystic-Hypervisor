package workloads

import (
	"context"
	"errors"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

func TestExecutionGuardSuccessfulProvision(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "web-exec-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          2,
		MemoryMB:     2048,
		StorageGB:    20,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.160",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}

	_, err = mgr.GeneratePlan(ctx, w.ID)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if err := mgr.ApprovePlan(ctx, w.ID); err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}

	provW, err := mgr.ProvisionWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("ProvisionWorkload failed: %v", err)
	}

	if provW.Status != StatusRunning {
		t.Fatalf("Expected status RUNNING, got %s", provW.Status)
	}
}

func TestExecutionGuardIdempotentProvision(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "web-exec-idem",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          2,
		MemoryMB:     2048,
		StorageGB:    20,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.161",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}
	_, _ = mgr.GeneratePlan(ctx, w.ID)
	_ = mgr.ApprovePlan(ctx, w.ID)

	_, err = mgr.ProvisionWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("First provision failed: %v", err)
	}

	callsFirst := len(tp.GetCalls())

	// Second provision attempt (idempotent)
	provW2, err := mgr.ProvisionWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("Second provision attempt failed: %v", err)
	}
	if provW2.Status != StatusRunning {
		t.Fatalf("Expected status RUNNING, got %s", provW2.Status)
	}

	callsSecond := len(tp.GetCalls())
	if callsSecond != callsFirst {
		t.Fatalf("Expected zero additional provider calls on idempotent provision, first=%d, second=%d", callsFirst, callsSecond)
	}
}

func TestExecutionGuardOwnershipConflictOnProvision(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	// Pre-seed provider with an EXTERNAL (unowned) resource of matching name
	extInst := &interfaces.Instance{
		ID:     "external-web-01",
		Name:   "external-web-01",
		State:  interfaces.StateRunning,
		Labels: map[string]string{"image": "ubuntu/20.04"}, // missing user.mystic.owned label
	}
	_, _ = tp.CreateInstance(ctx, extInst)

	spec := WorkloadSpec{
		Name:         "external-web-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          2,
		MemoryMB:     2048,
		StorageGB:    20,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.162",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}
	_, _ = mgr.GeneratePlan(ctx, w.ID)
	_ = mgr.ApprovePlan(ctx, w.ID)

	_, err = mgr.ProvisionWorkload(ctx, w.ID)
	if err == nil {
		t.Fatalf("Expected ProvisionWorkload to fail due to ownership conflict")
	}
	if !errors.Is(err, ErrOwnershipConflict) {
		t.Fatalf("Expected ErrOwnershipConflict, got %v", err)
	}
}

func TestExecutionGuardOwnershipConflictOnDelete(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	// Pre-seed provider with an EXTERNAL resource
	extInst := &interfaces.Instance{
		ID:     "external-del-01",
		Name:   "external-del-01",
		State:  interfaces.StateRunning,
		Labels: map[string]string{}, // unowned
	}
	_, _ = tp.CreateInstance(ctx, extInst)

	spec := WorkloadSpec{
		Name:         "external-del-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          2,
		MemoryMB:     2048,
		StorageGB:    20,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.163",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}

	err = mgr.DeleteWorkload(ctx, w.ID)
	if err == nil {
		t.Fatalf("Expected DeleteWorkload to fail when targeting unowned external resource")
	}
	if !errors.Is(err, ErrOwnershipConflict) {
		t.Fatalf("Expected ErrOwnershipConflict, got %v", err)
	}

	// Verify external resource was NOT deleted from provider
	inst, getErr := tp.GetInstance(ctx, "external-del-01")
	if getErr != nil || inst == nil {
		t.Fatalf("External resource was accidentally deleted!")
	}
}

func TestExecutionGuardPlanImmutability(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "immut-web-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          2,
		MemoryMB:     2048,
		StorageGB:    20,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.164",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}

	_, err = mgr.GeneratePlan(ctx, w.ID)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if err := mgr.ApprovePlan(ctx, w.ID); err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}

	// Mutate workload specification after plan approval!
	w.CPU = 8
	w.MemoryMB = 8192

	_, provErr := mgr.ProvisionWorkload(ctx, w.ID)
	if provErr == nil {
		t.Fatalf("Expected ProvisionWorkload to fail due to plan immutability violation")
	}
	if !errors.Is(provErr, ErrPlanInvalidated) {
		t.Fatalf("Expected ErrPlanInvalidated, got %v", provErr)
	}
}

func TestExecutionGuardIdempotentStartStop(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "startstop-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          2,
		MemoryMB:     2048,
		StorageGB:    20,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.165",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, _ := mgr.CreateWorkload(ctx, spec)
	_, _ = mgr.GeneratePlan(ctx, w.ID)
	_ = mgr.ApprovePlan(ctx, w.ID)
	_, _ = mgr.ProvisionWorkload(ctx, w.ID)

	// Workload is RUNNING. Calling StartWorkload should return idempotent success without error.
	wStarted, err := mgr.StartWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("StartWorkload on running workload failed: %v", err)
	}
	if wStarted.Status != StatusRunning {
		t.Fatalf("Expected status RUNNING, got %s", wStarted.Status)
	}

	// Stop Workload
	wStopped, err := mgr.StopWorkload(ctx, w.ID, false)
	if err != nil {
		t.Fatalf("StopWorkload failed: %v", err)
	}
	if wStopped.Status != StatusStopped {
		t.Fatalf("Expected status STOPPED, got %s", wStopped.Status)
	}

	// Calling StopWorkload again should return idempotent success
	wStopped2, err := mgr.StopWorkload(ctx, w.ID, false)
	if err != nil {
		t.Fatalf("StopWorkload on stopped workload failed: %v", err)
	}
	if wStopped2.Status != StatusStopped {
		t.Fatalf("Expected status STOPPED, got %s", wStopped2.Status)
	}
}

func TestExecutionGuardIllegalStateTransition(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "illegal-transition-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          2,
		MemoryMB:     2048,
		StorageGB:    20,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.166",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, _ := mgr.CreateWorkload(ctx, spec)
	w.Status = StatusProvisioning

	_, err := mgr.StartWorkload(ctx, w.ID)
	if err == nil {
		t.Fatalf("Expected StartWorkload to reject operation during PROVISIONING state")
	}
	if !errors.Is(err, ErrIllegalStateTransition) {
		t.Fatalf("Expected ErrIllegalStateTransition, got %v", err)
	}
}
