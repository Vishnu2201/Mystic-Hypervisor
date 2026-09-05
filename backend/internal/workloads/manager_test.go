package workloads

import (
	"context"
	"sync"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// TestProvider is a deterministic test-only double for virtualization provider calls.
type TestProvider struct {
	mu        sync.Mutex
	calls     []string
	instances map[string]*interfaces.Instance
	createErr error
	startErr  error
	getErr    error
}

func NewTestProvider() *TestProvider {
	return &TestProvider{
		calls:     make([]string, 0),
		instances: make(map[string]*interfaces.Instance),
	}
}

func (tp *TestProvider) Name() string { return "test-provider" }
func (tp *TestProvider) Capabilities() interfaces.CapabilitySet {
	return interfaces.NewCapabilitySet(interfaces.CapContainer, interfaces.CapVM)
}
func (tp *TestProvider) Ping(ctx context.Context) error { return nil }
func (tp *TestProvider) Close() error                   { return nil }

func (tp *TestProvider) InstanceProvider() (interfaces.InstanceProvider, bool) { return tp, true }
func (tp *TestProvider) ImageProvider() (interfaces.ImageProvider, bool)       { return nil, false }
func (tp *TestProvider) SnapshotProvider() (interfaces.SnapshotProvider, bool) { return nil, false }
func (tp *TestProvider) StorageProvider() (interfaces.StorageProvider, bool)   { return nil, false }
func (tp *TestProvider) NetworkProvider() (interfaces.NetworkProvider, bool)   { return nil, false }

func (tp *TestProvider) ListInstances(ctx context.Context) ([]interfaces.Instance, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "ListInstances")
	res := make([]interfaces.Instance, 0, len(tp.instances))
	for _, inst := range tp.instances {
		res = append(res, *inst)
	}
	return res, nil
}

func (tp *TestProvider) GetInstance(ctx context.Context, idOrName string) (*interfaces.Instance, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "GetInstance:"+idOrName)
	if tp.getErr != nil {
		return nil, tp.getErr
	}
	inst, ok := tp.instances[idOrName]
	if !ok {
		return nil, interfaces.ErrInstanceNotFound
	}
	return inst, nil
}

func (tp *TestProvider) CreateInstance(ctx context.Context, inst *interfaces.Instance) (*interfaces.Instance, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "CreateInstance:"+inst.Name)
	if tp.createErr != nil {
		return nil, tp.createErr
	}
	created := *inst
	created.ID = inst.Name
	created.State = interfaces.StateStopped
	created.IPAddress = "10.0.0.152"
	tp.instances[inst.Name] = &created
	return &created, nil
}

func (tp *TestProvider) StartInstance(ctx context.Context, idOrName string) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "StartInstance:"+idOrName)
	if tp.startErr != nil {
		return tp.startErr
	}
	if inst, ok := tp.instances[idOrName]; ok {
		inst.State = interfaces.StateRunning
	}
	return nil
}

func (tp *TestProvider) StopInstance(ctx context.Context, idOrName string, force bool) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "StopInstance:"+idOrName)
	if inst, ok := tp.instances[idOrName]; ok {
		inst.State = interfaces.StateStopped
	}
	return nil
}

func (tp *TestProvider) RestartInstance(ctx context.Context, idOrName string) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "RestartInstance:"+idOrName)
	if inst, ok := tp.instances[idOrName]; ok {
		inst.State = interfaces.StateRunning
	}
	return nil
}

func (tp *TestProvider) DeleteInstance(ctx context.Context, idOrName string) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "DeleteInstance:"+idOrName)
	delete(tp.instances, idOrName)
	return nil
}

func (tp *TestProvider) RenameInstance(ctx context.Context, oldName, newName string) error { return nil }
func (tp *TestProvider) ResizeInstance(ctx context.Context, idOrName string, limits interfaces.ResourceLimits) error {
	return nil
}
func (tp *TestProvider) GetInstanceMetrics(ctx context.Context, idOrName string) (*interfaces.InstanceMetrics, error) {
	return nil, nil
}

func (tp *TestProvider) GetCalls() []string {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	copied := make([]string, len(tp.calls))
	copy(copied, tp.calls)
	return copied
}

func TestWorkloadCreationAndValidation(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "web-test-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          2,
		MemoryMB:     2048,
		StorageGB:    20,
		HostID:       "host-main",
		NetworkName:  "incusbr0",
		PrivateIP:    "10.0.0.151",
		ExposureMode: hosts.ExposureNATForwarded,
		PortRequest: networking.PortAllocationRequest{
			Mode:              networking.AllocationModeSingle,
			ExternalStartPort: 20022,
			ExternalEndPort:   20022,
			InternalStartPort: 22,
			InternalEndPort:   22,
			Protocol:          hosts.ProtocolTCP,
		},
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
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "db-test-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          4,
		MemoryMB:     4096,
		StorageGB:    40,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.152",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}

	valRes, err := mgr.ValidateWorkload(ctx, w.ID)
	if err != nil || !valRes.IsValid {
		t.Fatalf("ValidateWorkload failed: %v, blockers: %v", err, valRes.Blockers)
	}

	// Generate Plan
	plan, err := mgr.GeneratePlan(ctx, w.ID)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.Approved {
		t.Fatalf("Plan must not be auto-approved upon generation")
	}

	// 1. Confirm provisioning has NOT occurred before approval
	callsBefore := tp.GetCalls()
	if len(callsBefore) != 0 {
		t.Fatalf("Provider calls recorded before approval: %v", callsBefore)
	}

	// 2. Attempt provision before approval and verify it is rejected
	_, provErr := mgr.ProvisionWorkload(ctx, w.ID)
	if provErr == nil {
		t.Fatalf("Expected ProvisionWorkload to fail without approval boundary")
	}

	// 3. Confirm test provider recorded zero provisioning calls
	callsAfterUnapproved := tp.GetCalls()
	if len(callsAfterUnapproved) != 0 {
		t.Fatalf("Provider calls recorded on unapproved provision attempt: %v", callsAfterUnapproved)
	}

	// 4. Approve the plan
	if err := mgr.ApprovePlan(ctx, w.ID); err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}

	// 5. Provision the workload
	provisionedW, provErr := mgr.ProvisionWorkload(ctx, w.ID)
	if provErr != nil {
		t.Fatalf("ProvisionWorkload failed after explicit approval: %v", provErr)
	}

	// 6. Confirm provider creation/provisioning calls occurred
	callsAfterApproved := tp.GetCalls()
	if len(callsAfterApproved) == 0 {
		t.Fatalf("Expected provider calls after approval, got 0")
	}

	hasCreate := false
	for _, call := range callsAfterApproved {
		if call == "CreateInstance:db-test-01" {
			hasCreate = true
			break
		}
	}
	if !hasCreate {
		t.Fatalf("Expected CreateInstance call in provider logs, got: %v", callsAfterApproved)
	}

	// 7. Confirm workload transitions to expected RUNNING state
	if provisionedW.Status != StatusRunning {
		t.Fatalf("Expected status RUNNING after provision, got %s", provisionedW.Status)
	}
}

func TestProviderNeverCalledWithoutApproval(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "unapproved-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "ubuntu/24.04",
		CPU:          1,
		MemoryMB:     1024,
		StorageGB:    10,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.153",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := mgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}

	// Attempt provisioning without even generating or approving plan
	_, err = mgr.ProvisionWorkload(ctx, w.ID)
	if err == nil {
		t.Fatalf("Expected ProvisionWorkload to return error for unapproved workload")
	}

	if len(tp.GetCalls()) != 0 {
		t.Fatalf("Provider was called despite missing approval! Calls: %v", tp.GetCalls())
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

	// Case: Out of sync (DB wants running, Incus is stopped)
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
