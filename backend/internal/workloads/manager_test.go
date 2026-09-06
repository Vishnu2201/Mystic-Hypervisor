package workloads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	listErr   error
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

func (tp *TestProvider) Preflight(ctx context.Context) (*interfaces.ProviderPreflightResult, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "Preflight")

	existing := make([]interfaces.PreflightInstance, 0, len(tp.instances))
	for _, inst := range tp.instances {
		ownership := interfaces.OwnershipExternal
		if inst.Labels != nil && (inst.Labels["user.mystic.owned"] == "true" || inst.Labels["mystic.owned"] == "true") {
			ownership = interfaces.OwnershipMysticOwned
		}
		existing = append(existing, interfaces.PreflightInstance{
			Name:      inst.Name,
			Type:      string(inst.Type),
			State:     string(inst.State),
			Ownership: ownership,
			IPAddress: inst.IPAddress,
		})
	}

	return &interfaces.ProviderPreflightResult{
		Provider:          "incus",
		Availability:      interfaces.AvailabilityAvailable,
		HealthStatus:      interfaces.ProviderHealthStatus{Installed: true, Reachable: true, Operational: true, Capable: true},
		ExistingInstances: existing,
	}, nil
}

func (tp *TestProvider) ListInstances(ctx context.Context) ([]interfaces.Instance, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "ListInstances")
	if tp.listErr != nil {
		return nil, tp.listErr
	}
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

func (tp *TestProvider) AdoptInstance(ctx context.Context, name string, workloadID string) (*interfaces.Instance, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.calls = append(tp.calls, "AdoptInstance:"+name)
	inst, ok := tp.instances[name]
	if !ok {
		return nil, interfaces.ErrInstanceNotFound
	}
	if inst.Labels == nil {
		inst.Labels = make(map[string]string)
	}
	inst.Labels["user.mystic.owned"] = "true"
	inst.Labels["user.mystic.workload_id"] = workloadID
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

func (tp *TestProvider) RenameInstance(ctx context.Context, oldName, newName string) error {
	return nil
}
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
	hasMutatingCalls := func(calls []string) bool {
		for _, c := range calls {
			if strings.HasPrefix(c, "CreateInstance") || strings.HasPrefix(c, "StartInstance") {
				return true
			}
		}
		return false
	}
	if hasMutatingCalls(tp.GetCalls()) {
		t.Fatalf("Provider mutating calls recorded before approval: %v", tp.GetCalls())
	}

	// 2. Attempt provision before approval and verify it is rejected
	_, provErr := mgr.ProvisionWorkload(ctx, w.ID)
	if provErr == nil {
		t.Fatalf("Expected ProvisionWorkload to fail without approval boundary")
	}

	// 3. Confirm test provider recorded zero provisioning calls
	if hasMutatingCalls(tp.GetCalls()) {
		t.Fatalf("Provider mutating calls recorded on unapproved provision attempt: %v", tp.GetCalls())
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

	for _, call := range tp.GetCalls() {
		if strings.HasPrefix(call, "CreateInstance") || strings.HasPrefix(call, "StartInstance") {
			t.Fatalf("Provider mutating action executed despite missing approval! Calls: %v", tp.GetCalls())
		}
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

func TestWorkloadAdoptionSuccess(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	tp.instances["ext-nano-01"] = &interfaces.Instance{
		ID:        "ext-nano-01",
		Name:      "ext-nano-01",
		Type:      interfaces.InstanceTypeContainer,
		State:     interfaces.StateRunning,
		Provider:  "incus",
		Node:      "local",
		IPAddress: "10.170.92.70",
		Limits: interfaces.ResourceLimits{
			CPUCores:    2,
			MemoryBytes: 2048 * 1024 * 1024,
		},
		Labels: map[string]string{
			"image": "ubuntu/24.04",
		},
	}

	w, err := mgr.AdoptWorkload(ctx, "ext-nano-01")
	if err != nil {
		t.Fatalf("AdoptWorkload failed: %v", err)
	}

	if w.Name != "ext-nano-01" {
		t.Errorf("Expected workload name ext-nano-01, got %s", w.Name)
	}
	if w.Status != StatusRunning {
		t.Errorf("Expected workload status RUNNING, got %s", w.Status)
	}
	if w.SyncStatus != instances.SyncInSync {
		t.Errorf("Expected workload sync status IN_SYNC, got %s", w.SyncStatus)
	}
	if w.NetworkConfig.PrivateIPv4 != "10.170.92.70" {
		t.Errorf("Expected IP 10.170.92.70, got %s", w.NetworkConfig.PrivateIPv4)
	}

	calls := tp.GetCalls()
	for _, call := range calls {
		if strings.HasPrefix(call, "CreateInstance") || strings.HasPrefix(call, "StartInstance") ||
			strings.HasPrefix(call, "StopInstance") || strings.HasPrefix(call, "RestartInstance") {
			t.Errorf("Forbidden mutating lifecycle call during adoption: %s", call)
		}
	}
}

func TestWorkloadAdoptionIdempotencyAndConflict(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	tp.instances["ext-nano-02"] = &interfaces.Instance{
		ID:    "ext-nano-02",
		Name:  "ext-nano-02",
		Type:  interfaces.InstanceTypeContainer,
		State: interfaces.StateRunning,
	}

	w, err := mgr.AdoptWorkload(ctx, "ext-nano-02")
	if err != nil {
		t.Fatalf("Initial AdoptWorkload failed: %v", err)
	}
	if w == nil {
		t.Fatalf("Expected non-nil workload")
	}

	_, err = mgr.AdoptWorkload(ctx, "ext-nano-02")
	if err == nil || !errors.Is(err, ErrAlreadyManaged) {
		t.Fatalf("Expected ErrAlreadyManaged on double adoption attempt, got %v", err)
	}
}

func TestWorkloadAdoptionNonExistentInstance(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	_, err := mgr.AdoptWorkload(ctx, "ghost-instance")
	if err == nil || !errors.Is(err, ErrIncusInstanceNotFound) {
		t.Fatalf("Expected ErrIncusInstanceNotFound for missing instance adoption, got %v", err)
	}
}

func TestFileStoreSaveAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mystic-store-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storeFile := filepath.Join(tempDir, "workloads.json")
	store := NewFileStore(storeFile)

	workloadsMap := map[string]*Workload{
		"wl-01": {
			ID:                 "wl-01",
			Name:               "test-store-inst",
			Provider:           "incus",
			ProviderInstanceID: "test-store-inst",
			Type:               TypeIncusContainer,
			Status:             StatusRunning,
			DesiredState:       interfaces.StateRunning,
			ActualState:        interfaces.StateRunning,
			SyncStatus:         instances.SyncInSync,
		},
	}

	if err := store.Save(workloadsMap); err != nil {
		t.Fatalf("Store Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Store Load failed: %v", err)
	}

	if len(loaded) != 1 || loaded["wl-01"] == nil {
		t.Fatalf("Expected 1 workload loaded, got %+v", loaded)
	}

	if loaded["wl-01"].Name != "test-store-inst" {
		t.Errorf("Expected workload name test-store-inst, got %s", loaded["wl-01"].Name)
	}
}

func TestManagerRestartPersistenceAndReconciliation(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "mystic-restart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storeFile := filepath.Join(tempDir, "workloads.json")
	store1 := NewFileStore(storeFile)

	tp := NewTestProvider()
	tp.instances["ext-nano-03"] = &interfaces.Instance{
		ID:        "ext-nano-03",
		Name:      "ext-nano-03",
		Type:      interfaces.InstanceTypeContainer,
		State:     interfaces.StateRunning,
		Provider:  "incus",
		IPAddress: "10.170.92.93",
	}

	mgr1 := NewManagerWithProviderAndStore(tp, store1)

	adoptedW, err := mgr1.AdoptWorkload(ctx, "ext-nano-03")
	if err != nil {
		t.Fatalf("Initial adoption failed: %v", err)
	}

	store2 := NewFileStore(storeFile)
	mgr2 := NewManagerWithProviderAndStore(tp, store2)

	recoveredW, err := mgr2.GetWorkload(ctx, adoptedW.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve recovered workload after restart: %v", err)
	}

	if recoveredW.Name != "ext-nano-03" {
		t.Errorf("Expected workload name ext-nano-03, got %s", recoveredW.Name)
	}
	if recoveredW.Status != StatusRunning {
		t.Errorf("Expected status RUNNING after restart reconciliation, got %s", recoveredW.Status)
	}
	if recoveredW.SyncStatus != instances.SyncInSync {
		t.Errorf("Expected SyncInSync after restart reconciliation, got %s", recoveredW.SyncStatus)
	}

	pfRes, err := mgr2.GetProviderPreflight(ctx, "incus")
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	if len(pfRes.ExistingInstances) != 1 || pfRes.ExistingInstances[0].Ownership != interfaces.OwnershipMysticOwned {
		t.Errorf("Expected instance to be classified MYSTIC_OWNED after restart, got %+v", pfRes.ExistingInstances)
	}
}

func TestManagerOrphanHandlingWhenInstanceMissing(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "mystic-orphan-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storeFile := filepath.Join(tempDir, "workloads.json")
	store := NewFileStore(storeFile)
	tp := NewTestProvider()

	tp.instances["ext-nano-04"] = &interfaces.Instance{
		ID:        "ext-nano-04",
		Name:      "ext-nano-04",
		Type:      interfaces.InstanceTypeContainer,
		State:     interfaces.StateRunning,
		Provider:  "incus",
		IPAddress: "10.170.92.94",
	}

	mgr := NewManagerWithProviderAndStore(tp, store)
	adoptedW, err := mgr.AdoptWorkload(ctx, "ext-nano-04")
	if err != nil {
		t.Fatalf("AdoptWorkload failed: %v", err)
	}

	delete(tp.instances, "ext-nano-04")

	_ = mgr.ReconcileAll(ctx)

	reconciledW, err := mgr.GetWorkload(ctx, adoptedW.ID)
	if err != nil {
		t.Fatalf("GetWorkload failed after orphan event: %v", err)
	}

	if reconciledW.Status != StatusOrphaned {
		t.Errorf("Expected workload status ORPHANED for missing instance, got %s", reconciledW.Status)
	}
	if reconciledW.SyncStatus != instances.SyncProviderMissing {
		t.Errorf("Expected SyncProviderMissing, got %s", reconciledW.SyncStatus)
	}

	if len(tp.instances) != 0 {
		t.Errorf("Forbidden auto-recreation of missing instance! Provider instances: %+v", tp.instances)
	}
}

func TestManagerGetAdoptionPreview(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	tp.instances["ext-preview-01"] = &interfaces.Instance{
		ID:        "ext-preview-01",
		Name:      "ext-preview-01",
		Type:      interfaces.InstanceTypeContainer,
		State:     interfaces.StateRunning,
		Provider:  "incus",
		IPAddress: "10.170.92.95",
		Limits: interfaces.ResourceLimits{
			CPUCores:    4,
			MemoryBytes: 4096 * 1024 * 1024,
		},
	}

	prev, err := mgr.GetAdoptionPreview(ctx, "ext-preview-01")
	if err != nil {
		t.Fatalf("GetAdoptionPreview failed: %v", err)
	}

	if !prev.CanAdopt || prev.AlreadyManaged {
		t.Errorf("Expected CanAdopt true and AlreadyManaged false, got %+v", prev)
	}
	if prev.CPUCores != 4 || prev.MemoryBytes/(1024*1024) != 4096 {
		t.Errorf("Expected limits 4 cores / 4096MB, got %d cores / %dMB", prev.CPUCores, prev.MemoryBytes/(1024*1024))
	}

	_, err = mgr.AdoptWorkload(ctx, "ext-preview-01")
	if err != nil {
		t.Fatalf("AdoptWorkload failed: %v", err)
	}

	prevManaged, err := mgr.GetAdoptionPreview(ctx, "ext-preview-01")
	if err != nil {
		t.Fatalf("GetAdoptionPreview failed after adoption: %v", err)
	}

	if prevManaged.CanAdopt || !prevManaged.AlreadyManaged {
		t.Errorf("Expected CanAdopt false and AlreadyManaged true after adoption, got %+v", prevManaged)
	}
	if len(prevManaged.Blockers) == 0 {
		t.Errorf("Expected blocker message for already managed instance")
	}
}

func TestFileStoreMissingFileReturnsEmpty(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mystic-store-missing-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	nonExistentFile := filepath.Join(tempDir, "sub", "nonexistent.json")
	store := NewFileStore(nonExistentFile)

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Expected clean empty state for missing file, got error: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("Expected empty map for missing file, got %d items", len(loaded))
	}
}

func TestFileStoreMalformedFileReturnsError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mystic-store-malformed-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	malformedFile := filepath.Join(tempDir, "bad_workloads.json")
	if err := os.WriteFile(malformedFile, []byte("{ invalid json content ..."), 0600); err != nil {
		t.Fatalf("Failed to write malformed file: %v", err)
	}

	store := NewFileStore(malformedFile)
	_, err = store.Load()
	if err == nil {
		t.Fatalf("Expected explicit error for malformed store file, got nil")
	}

	tp := NewTestProvider()
	mgr := NewManagerWithProviderAndStore(tp, store)
	// Must not wipe the malformed file on disk with empty state
	data, _ := os.ReadFile(malformedFile)
	if string(data) == "{}" {
		t.Fatalf("Manager initialization wiped corrupted file on disk!")
	}
	_ = mgr
}

func TestFileStoreWriteFailureReturnsError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mystic-store-readonly-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Point store to a path inside a non-existent directory where creation will fail or directory is invalid
	invalidPath := filepath.Join(tempDir, "read_only_file.txt", "workloads.json")
	if err := os.WriteFile(filepath.Join(tempDir, "read_only_file.txt"), []byte("file-not-dir"), 0600); err != nil {
		t.Fatalf("Failed to create blocker file: %v", err)
	}

	store := NewFileStore(invalidPath)
	err = store.Save(map[string]*Workload{
		"wl-test": {ID: "wl-test", Name: "test"},
	})
	if err == nil {
		t.Fatalf("Expected explicit error when saving to unwriteable path, got nil")
	}
}

func TestManagerReconcileAllProviderErrorPreservesState(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "mystic-provider-err-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storeFile := filepath.Join(tempDir, "workloads.json")
	store := NewFileStore(storeFile)

	tp := NewTestProvider()
	tp.listErr = errors.New("provider socket connection refused")

	mgr := NewManagerWithProviderAndStore(tp, store)

	mgr.workloads["wl-existing"] = &Workload{
		ID:         "wl-existing",
		Name:       "existing-inst",
		Status:     StatusRunning,
		SyncStatus: instances.SyncInSync,
	}

	err = mgr.ReconcileAll(ctx)
	if err == nil {
		t.Fatalf("Expected ReconcileAll to return provider error when ListInstances fails")
	}

	w, getErr := mgr.GetWorkload(ctx, "wl-existing")
	if getErr != nil {
		t.Fatalf("GetWorkload failed: %v", getErr)
	}
	if w.Status != StatusRunning || w.SyncStatus != instances.SyncInSync {
		t.Errorf("Provider error during reconciliation MUST NOT mark workload as ORPHANED or wiped; got status=%s sync=%s", w.Status, w.SyncStatus)
	}
}

func TestCriticalAcceptanceAdoptionPersistenceRoundTrip(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "mystic-critical-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storeFile := filepath.Join(tempDir, "sub", "workloads.json")
	store1 := NewFileStore(storeFile)

	tp := NewTestProvider()
	tp.instances["ext-critical-01"] = &interfaces.Instance{
		ID:        "ext-critical-01",
		Name:      "ext-critical-01",
		Type:      interfaces.InstanceTypeContainer,
		State:     interfaces.StateRunning,
		Provider:  "incus",
		IPAddress: "10.170.92.99",
	}

	mgr1 := NewManagerWithProviderAndStore(tp, store1)
	adoptedW, err := mgr1.AdoptWorkload(ctx, "ext-critical-01")
	if err != nil {
		t.Fatalf("AdoptWorkload failed: %v", err)
	}

	// 1. Verify parent directory was created
	if _, err := os.Stat(filepath.Dir(storeFile)); os.IsNotExist(err) {
		t.Fatalf("FileStore Save failed to create parent directory!")
	}

	// 2. Verify workloads.json file exists
	if _, err := os.Stat(storeFile); os.IsNotExist(err) {
		t.Fatalf("FileStore Save failed to create workloads.json!")
	}

	// 3. Destroy mgr1 and create mgr2 with same store file path
	store2 := NewFileStore(storeFile)
	mgr2 := NewManagerWithProviderAndStore(tp, store2)

	// 4. Verify workload restored with same ID, Name, and ProviderInstanceID
	restoredW, err := mgr2.GetWorkload(ctx, adoptedW.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve restored workload after manager recreation: %v", err)
	}

	if restoredW.ID != adoptedW.ID {
		t.Errorf("Expected ID %s, got %s", adoptedW.ID, restoredW.ID)
	}
	if restoredW.Name != "ext-critical-01" {
		t.Errorf("Expected Name ext-critical-01, got %s", restoredW.Name)
	}
	if restoredW.ProviderInstanceID != "ext-critical-01" {
		t.Errorf("Expected ProviderInstanceID ext-critical-01, got %s", restoredW.ProviderInstanceID)
	}
}

func TestAdoptWorkloadAbortsIfPersistenceFails(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "mystic-abort-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file blocking directory creation to simulate write failure
	blocker := filepath.Join(tempDir, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0600); err != nil {
		t.Fatalf("Failed to create blocker file: %v", err)
	}
	unwriteableStorePath := filepath.Join(blocker, "workloads.json")

	store := NewFileStore(unwriteableStorePath)
	tp := NewTestProvider()
	tp.instances["ext-abort-01"] = &interfaces.Instance{
		ID:       "ext-abort-01",
		Name:     "ext-abort-01",
		Type:     interfaces.InstanceTypeContainer,
		State:    interfaces.StateRunning,
		Provider: "incus",
	}

	mgr := NewManagerWithProviderAndStore(tp, store)
	_, err = mgr.AdoptWorkload(ctx, "ext-abort-01")
	if err == nil {
		t.Fatalf("Expected AdoptWorkload to fail when persistence is unwriteable, got success!")
	}

	// Transactional Guarantee: Incus MUST NOT be mutated if disk write failed
	inst := tp.instances["ext-abort-01"]
	if inst.Labels != nil && (inst.Labels["user.mystic.owned"] == "true" || inst.Labels["mystic.owned"] == "true") {
		t.Fatalf("Transactional failure: provider instance was tagged despite store write failure!")
	}
}

func TestRecoveryOfUntrackedTaggedInstance(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "mystic-recovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storeFile := filepath.Join(tempDir, "workloads.json")
	store1 := NewFileStore(storeFile)
	tp := NewTestProvider()

	// Simulate VPS state: test-nano has tags on Incus (user.mystic.owned = true, user.mystic.workload_id = wl-12345), but DB record was lost
	tp.instances["test-nano"] = &interfaces.Instance{
		ID:        "test-nano",
		Name:      "test-nano",
		Type:      interfaces.InstanceTypeContainer,
		State:     interfaces.StateRunning,
		Provider:  "incus",
		IPAddress: "10.170.92.70",
		Labels: map[string]string{
			"user.mystic.owned":       "true",
			"user.mystic.workload_id": "wl-12345",
		},
	}

	mgr1 := NewManagerWithProviderAndStore(tp, store1)

	// 1. Verify Adoption Preview returns CanAdopt: true and warning for recovery
	prev, err := mgr1.GetAdoptionPreview(ctx, "test-nano")
	if err != nil {
		t.Fatalf("GetAdoptionPreview failed: %v", err)
	}
	if !prev.CanAdopt || prev.AlreadyManaged {
		t.Fatalf("Expected CanAdopt true and AlreadyManaged false for untracked tagged instance, got %+v", prev)
	}
	if len(prev.Warnings) == 0 {
		t.Errorf("Expected recovery warning message in adoption preview")
	}

	// 2. Perform explicit adoption
	recoveredW, err := mgr1.AdoptWorkload(ctx, "test-nano")
	if err != nil {
		t.Fatalf("AdoptWorkload failed to recover untracked tagged instance: %v", err)
	}
	if recoveredW.ID != "wl-12345" {
		t.Errorf("Expected workload ID wl-12345 preserved from provider label, got %s", recoveredW.ID)
	}

	// 3. Restart manager and verify persistence restored
	store2 := NewFileStore(storeFile)
	mgr2 := NewManagerWithProviderAndStore(tp, store2)

	restoredW, err := mgr2.GetWorkload(ctx, "wl-12345")
	if err != nil {
		t.Fatalf("Failed to retrieve recovered workload after manager restart: %v", err)
	}
	if restoredW.Name != "test-nano" || restoredW.Status != StatusRunning {
		t.Errorf("Expected restored workload test-nano / RUNNING, got %+v", restoredW)
	}
}

func TestAdoptionImports3GiBMemoryLimit(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	tp.instances["test-nano-3gib"] = &interfaces.Instance{
		ID:       "test-nano-3gib",
		Name:     "test-nano-3gib",
		Type:     interfaces.InstanceTypeContainer,
		State:    interfaces.StateRunning,
		Provider: "incus",
		Limits: interfaces.ResourceLimits{
			CPUCores:    1,
			MemoryBytes: 3 * 1024 * 1024 * 1024, // 3GiB = 3221225472 bytes
		},
	}

	mgr := NewManagerWithProvider(tp)
	w, err := mgr.AdoptWorkload(ctx, "test-nano-3gib")
	if err != nil {
		t.Fatalf("AdoptWorkload failed: %v", err)
	}

	if w.MemoryMB != 3072 {
		t.Errorf("Expected adopted workload MemoryMB == 3072 for 3GiB limit, got %d", w.MemoryMB)
	}
}

func TestAdoptionImportsVariousMemoryUnits(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		bytes      int64
		expectedMB int64
	}{
		{"512mib-inst", 512 * 1024 * 1024, 512},
		{"1gib-inst", 1 * 1024 * 1024 * 1024, 1024},
		{"4096mib-inst", 4096 * 1024 * 1024, 4096},
	}

	for _, tt := range tests {
		tp := NewTestProvider()
		tp.instances[tt.name] = &interfaces.Instance{
			ID:       tt.name,
			Name:     tt.name,
			Type:     interfaces.InstanceTypeContainer,
			State:    interfaces.StateRunning,
			Provider: "incus",
			Limits: interfaces.ResourceLimits{
				CPUCores:    2,
				MemoryBytes: tt.bytes,
			},
		}

		mgr := NewManagerWithProvider(tp)
		w, err := mgr.AdoptWorkload(ctx, tt.name)
		if err != nil {
			t.Fatalf("AdoptWorkload for %s failed: %v", tt.name, err)
		}
		if w.MemoryMB != tt.expectedMB {
			t.Errorf("For %s expected MemoryMB == %d, got %d", tt.name, tt.expectedMB, w.MemoryMB)
		}
	}
}

func TestAdoptionRegressionNoFallbackWhenValidMemoryPresent(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	tp.instances["valid-mem-inst"] = &interfaces.Instance{
		ID:       "valid-mem-inst",
		Name:     "valid-mem-inst",
		Type:     interfaces.InstanceTypeContainer,
		State:    interfaces.StateRunning,
		Provider: "incus",
		Limits: interfaces.ResourceLimits{
			CPUCores:    1,
			MemoryBytes: 3 * 1024 * 1024 * 1024, // 3GiB
		},
	}

	mgr := NewManagerWithProvider(tp)

	// Verify adoption preview does not display 512MB fallback
	prev, err := mgr.GetAdoptionPreview(ctx, "valid-mem-inst")
	if err != nil {
		t.Fatalf("GetAdoptionPreview failed: %v", err)
	}
	if prev.MemoryBytes != 3*1024*1024*1024 {
		t.Errorf("Adoption preview regression: expected 3GiB (%d bytes), got %d bytes", 3*1024*1024*1024, prev.MemoryBytes)
	}

	// Verify AdoptWorkload does not set 512MB fallback
	w, err := mgr.AdoptWorkload(ctx, "valid-mem-inst")
	if err != nil {
		t.Fatalf("AdoptWorkload failed: %v", err)
	}
	if w.MemoryMB == 512 {
		t.Fatalf("REGRESSION DETECTED: AdoptWorkload silently fell back to 512MB when valid 3GiB memory limit was present!")
	}
	if w.MemoryMB != 3072 {
		t.Errorf("Expected MemoryMB == 3072, got %d", w.MemoryMB)
	}
}

func TestReconciliationDetectsMemoryDrift(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	tp.instances["test-nano"] = &interfaces.Instance{
		ID:       "test-nano",
		Name:     "test-nano",
		Type:     interfaces.InstanceTypeContainer,
		State:    interfaces.StateRunning,
		Provider: "incus",
		Limits: interfaces.ResourceLimits{
			CPUCores:    1,
			MemoryBytes: 3 * 1024 * 1024 * 1024, // 3GiB = 3072MB
			DiskBytes:   10 * 1024 * 1024 * 1024,
		},
		IPAddress: "10.170.92.70",
	}

	mgr := NewManagerWithProvider(tp)
	w, err := mgr.AdoptWorkload(ctx, "test-nano")
	if err != nil {
		t.Fatalf("AdoptWorkload failed: %v", err)
	}

	if w.MemoryMB != 3072 {
		t.Fatalf("Expected initial adopted MemoryMB to be 3072, got %d", w.MemoryMB)
	}
	if w.SyncStatus != instances.SyncInSync {
		t.Fatalf("Expected initial SyncStatus to be in_sync, got %s", w.SyncStatus)
	}

	// Drift Test: Change provider memory configuration out-of-band to 2GiB
	tp.instances["test-nano"].Limits.MemoryBytes = 2 * 1024 * 1024 * 1024 // 2GiB

	// Trigger reconciliation
	if err := mgr.ReconcileAll(ctx); err != nil {
		t.Fatalf("ReconcileAll failed: %v", err)
	}

	reconciledW := mgr.workloads[w.ID]
	// 1. Desired configuration is preserved
	if reconciledW.MemoryMB != 3072 {
		t.Errorf("Desired configuration lost: expected MemoryMB == 3072, got %d", reconciledW.MemoryMB)
	}
	// 2. Sync status correctly identifies drift
	if reconciledW.SyncStatus != instances.SyncOutOfSync {
		t.Errorf("Expected SyncStatus == out_of_sync, got %s", reconciledW.SyncStatus)
	}
	// 3. Operational status flags DRIFTED
	if reconciledW.Status != StatusDrifted {
		t.Errorf("Expected Status == DRIFTED, got %s", reconciledW.Status)
	}
}

func TestBackgroundReconciliationWorkerLifecycleAndNonOverlapping(t *testing.T) {
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	// Start worker
	mgr.StartBackgroundReconciliation(20 * time.Millisecond)

	// Call Start again to verify idempotency (no overlapping goroutines)
	mgr.StartBackgroundReconciliation(20 * time.Millisecond)

	time.Sleep(60 * time.Millisecond)

	// Stop worker cleanly
	mgr.StopBackgroundReconciliation()

	// Calling Stop again should be safe
	mgr.StopBackgroundReconciliation()
}

func TestWorkloadProvisioningSSHPortAllocation(t *testing.T) {
	ctx := context.Background()
	tp := NewTestProvider()
	mgr := NewManagerWithProvider(tp)

	spec := WorkloadSpec{
		Name:         "vps-ssh-test-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "images:debian/13",
		CPU:          1,
		MemoryMB:     1024,
		StorageGB:    10,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.199",
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

	provisioned, err := mgr.ProvisionWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("ProvisionWorkload failed: %v", err)
	}

	if provisioned.SSHAccessInfo == nil {
		t.Fatalf("Expected SSHAccessInfo to be populated upon provisioning")
	}

	if provisioned.SSHAccessInfo.Port < 22100 || provisioned.SSHAccessInfo.Port > 22200 {
		t.Errorf("Expected allocated SSH port in range 22100-22200, got %d", provisioned.SSHAccessInfo.Port)
	}

	if provisioned.SSHAccessInfo.Host != "ssh.mysticservers.com" {
		t.Errorf("Expected SSH host ssh.mysticservers.com, got %s", provisioned.SSHAccessInfo.Host)
	}

	if provisioned.SSHAccessInfo.Status != networking.SSHStatusActive {
		t.Errorf("Expected SSH status ACTIVE, got %s", provisioned.SSHAccessInfo.Status)
	}

	expectedCmd := networking.GenerateSSHConnectionCommand(provisioned.SSHAccessInfo.Host, provisioned.SSHAccessInfo.Port, "root")
	if provisioned.SSHAccessInfo.ConnectionCommand != expectedCmd {
		t.Errorf("Expected connection command '%s', got '%s'", expectedCmd, provisioned.SSHAccessInfo.ConnectionCommand)
	}

	// Verify deletion releases SSH port
	if err := mgr.DeleteWorkload(ctx, w.ID); err != nil {
		t.Fatalf("DeleteWorkload failed: %v", err)
	}

	_, getErr := mgr.ExposureManager().SSHPortAllocator().GetAllocation(ctx, w.ID)
	if getErr == nil {
		t.Errorf("Expected SSH allocation to be released/not found after workload deletion")
	}
}

func TestStoreIsolationAndDefaults(t *testing.T) {
	ctx := context.Background()

	// 1. Verify NewManagerWithProvider uses in-memory mode for all child stores
	tp := NewTestProvider()
	memMgr := NewManagerWithProvider(tp)

	if memMgr.StorePath() != "in-memory" {
		t.Errorf("Expected WorkloadManager store path to be 'in-memory', got '%s'", memMgr.StorePath())
	}
	if memMgr.ExposureManager().StorePath() != "in-memory" {
		t.Errorf("Expected ExposureManager store path to be 'in-memory', got '%s'", memMgr.ExposureManager().StorePath())
	}
	if memMgr.ExposureManager().SSHPortAllocator().StorePath() != "in-memory" {
		t.Errorf("Expected SSHPortAllocator store path to be 'in-memory', got '%s'", memMgr.ExposureManager().SSHPortAllocator().StorePath())
	}
	if memMgr.ServiceManager().StorePath() != "in-memory" {
		t.Errorf("Expected ServiceManager store path to be 'in-memory', got '%s'", memMgr.ServiceManager().StorePath())
	}
	if memMgr.ConnectionManager().StorePath() != "in-memory" {
		t.Errorf("Expected ConnectionManager store path to be 'in-memory', got '%s'", memMgr.ConnectionManager().StorePath())
	}

	// Create and provision a workload in in-memory mode, verifying zero disk persistence errors
	spec := WorkloadSpec{
		Name:         "mem-vps-01",
		Provider:     "incus",
		Type:         TypeIncusContainer,
		Image:        "images:debian/13",
		CPU:          1,
		MemoryMB:     1024,
		StorageGB:    10,
		HostID:       "host-main",
		PrivateIP:    "10.0.0.188",
		ExposureMode: hosts.ExposurePrivateOnly,
	}

	w, err := memMgr.CreateWorkload(ctx, spec)
	if err != nil {
		t.Fatalf("CreateWorkload failed in in-memory mode: %v", err)
	}
	if _, err := memMgr.GeneratePlan(ctx, w.ID); err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	if err := memMgr.ApprovePlan(ctx, w.ID); err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}
	provW, err := memMgr.ProvisionWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("ProvisionWorkload in in-memory mode failed (store leaked to disk?): %v", err)
	}
	if provW.SSHAccessInfo == nil || provW.SSHAccessInfo.Port != 22100 {
		t.Errorf("Expected allocated SSH port 22100 in memory, got %+v", provW.SSHAccessInfo)
	}

	// 2. Verify NewManagerWithProviderAndStore with temp directory isolates all child stores to temp directory
	tp2 := NewTestProvider()
	tempDir := t.TempDir()
	wlStorePath := filepath.Join(tempDir, "workloads.json")
	fileMgr := NewManagerWithProviderAndStore(tp2, NewFileStore(wlStorePath))

	if fileMgr.StorePath() != wlStorePath {
		t.Errorf("Expected WorkloadManager store path '%s', got '%s'", wlStorePath, fileMgr.StorePath())
	}
	expPath := filepath.Join(tempDir, "network_exposures.json")
	if fileMgr.ExposureManager().StorePath() != expPath {
		t.Errorf("Expected ExposureManager store path '%s', got '%s'", expPath, fileMgr.ExposureManager().StorePath())
	}
	sshPath := filepath.Join(tempDir, "ssh_allocations.json")
	if fileMgr.ExposureManager().SSHPortAllocator().StorePath() != sshPath {
		t.Errorf("Expected SSHPortAllocator store path '%s', got '%s'", sshPath, fileMgr.ExposureManager().SSHPortAllocator().StorePath())
	}
	svcPath := filepath.Join(tempDir, "services.json")
	if fileMgr.ServiceManager().StorePath() != svcPath {
		t.Errorf("Expected ServiceManager store path '%s', got '%s'", svcPath, fileMgr.ServiceManager().StorePath())
	}
	connPath := filepath.Join(tempDir, "connection_profiles.json")
	if fileMgr.ConnectionManager().StorePath() != connPath {
		t.Errorf("Expected ConnectionManager store path '%s', got '%s'", connPath, fileMgr.ConnectionManager().StorePath())
	}

	spec2 := spec
	spec2.Name = "file-vps-01"
	spec2.PrivateIP = "10.0.0.189"

	// Provision workload in file-backed temp directory, ensuring persistence files are created in tempDir
	w2, err := fileMgr.CreateWorkload(ctx, spec2)
	if err != nil {
		t.Fatalf("CreateWorkload failed: %v", err)
	}
	if _, err := fileMgr.GeneratePlan(ctx, w2.ID); err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	if err := fileMgr.ApprovePlan(ctx, w2.ID); err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}
	if _, err := fileMgr.ProvisionWorkload(ctx, w2.ID); err != nil {
		t.Fatalf("ProvisionWorkload with temp store failed: %v", err)
	}

	if _, err := os.Stat(sshPath); os.IsNotExist(err) {
		t.Errorf("Expected SSH allocation file '%s' to exist after provisioning", sshPath)
	}

	// 3. Verify production NewManager retains /var/lib/mystic defaults
	prodMgr := NewManager()
	if filepath.Clean(prodMgr.StorePath()) != filepath.Clean("/var/lib/mystic/workloads.json") {
		t.Errorf("Expected production workload store path '/var/lib/mystic/workloads.json', got '%s'", prodMgr.StorePath())
	}
	if filepath.Clean(prodMgr.ExposureManager().StorePath()) != filepath.Clean("/var/lib/mystic/network_exposures.json") {
		t.Errorf("Expected production exposure store path '/var/lib/mystic/network_exposures.json', got '%s'", prodMgr.ExposureManager().StorePath())
	}
	if filepath.Clean(prodMgr.ExposureManager().SSHPortAllocator().StorePath()) != filepath.Clean("/var/lib/mystic/ssh_allocations.json") {
		t.Errorf("Expected production SSH store path '/var/lib/mystic/ssh_allocations.json', got '%s'", prodMgr.ExposureManager().SSHPortAllocator().StorePath())
	}
}
