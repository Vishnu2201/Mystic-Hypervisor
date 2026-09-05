package workloads

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/incus"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// Manager implements WorkloadService and manages real workload lifecycle.
type Manager struct {
	mu          sync.RWMutex
	workloads   map[string]*Workload
	plans       map[string]*ProvisioningPlan
	allocator   *networking.AllocatorEngine
	incusDriver interfaces.Provider
	reconciler  *instances.Reconciler
	guard       *ExecutionGuard
	store       Store
}

// StorePath returns the filesystem path of the workload store if using FileStore.
func (m *Manager) StorePath() string {
	if fs, ok := m.store.(*FileStore); ok && fs != nil {
		return fs.FilePath()
	}
	return "in-memory"
}

// NewManager constructs a WorkloadManager with default Incus provider and default file store.
func NewManager() *Manager {
	return NewManagerWithProviderAndStore(incus.NewIncusProvider("/var/lib/incus/unix.socket"), NewFileStore(""))
}

// NewManagerWithProvider constructs a WorkloadManager with a specific virtualization provider.
func NewManagerWithProvider(provider interfaces.Provider) *Manager {
	return NewManagerWithProviderAndStore(provider, nil)
}

// NewManagerWithProviderAndStore constructs a WorkloadManager with provider and persistent store.
func NewManagerWithProviderAndStore(provider interfaces.Provider, store Store) *Manager {
	m := &Manager{
		workloads:   make(map[string]*Workload),
		plans:       make(map[string]*ProvisioningPlan),
		allocator:   networking.NewAllocatorEngine(),
		incusDriver: provider,
		reconciler:  instances.NewReconciler(),
		guard:       NewExecutionGuard(15 * time.Second),
		store:       store,
	}

	if m.store != nil {
		loaded, err := m.store.Load()
		if err != nil {
			logging.GetLogger().Error("Failed to load workload store on manager startup",
				"store_path", m.StorePath(),
				"error", err,
			)
			// Crucial safety boundary: DO NOT run ReconcileAll or overwrite store if loading failed!
			return m
		}
		if loaded != nil {
			m.workloads = loaded
			logging.GetLogger().Info("Loaded persisted workloads from store",
				"store_path", m.StorePath(),
				"count", len(m.workloads),
			)
		}
	}

	if err := m.ReconcileAll(context.Background()); err != nil {
		logging.GetLogger().Warn("Startup reconciliation encountered provider warning/error", "error", err)
	}
	return m
}

func (m *Manager) saveStoreUnlocked() error {
	if m.store != nil {
		if err := m.store.Save(m.workloads); err != nil {
			logging.GetLogger().Error("Failed to persist workload store",
				"store_path", m.StorePath(),
				"error", err,
			)
			return fmt.Errorf("failed to persist workload store: %w", err)
		}
	}
	return nil
}

func (m *Manager) CreateWorkload(ctx context.Context, spec WorkloadSpec) (*Workload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("workload name cannot be empty")
	}

	// Check name collision in stored workloads
	for _, w := range m.workloads {
		if w.Name == spec.Name {
			return nil, fmt.Errorf("workload with name '%s' already exists", spec.Name)
		}
	}

	id := fmt.Sprintf("wl-%d", time.Now().UnixNano())
	now := time.Now().Format(time.RFC3339)

	providerName := spec.Provider
	if providerName == "" {
		providerName = "incus"
	}
	wType := spec.Type
	if wType == "" {
		wType = TypeIncusContainer
	}

	w := &Workload{
		ID:                 id,
		Name:               spec.Name,
		HostID:             spec.HostID,
		Provider:           providerName,
		ProviderInstanceID: spec.Name,
		Type:               wType,
		Status:             StatusDraft,
		DesiredState:       interfaces.StateRunning,
		ActualState:        interfaces.StateUnknown,
		SyncStatus:         instances.SyncProviderMissing,
		CPU:                spec.CPU,
		MemoryMB:           spec.MemoryMB,
		StorageGB:          spec.StorageGB,
		Image:              spec.Image,
		Project:            spec.Project,
		Profile:            spec.Profile,
		NetworkConfig: networking.WorkloadNetworkConfig{
			ID:           id,
			WorkloadID:   id,
			WorkloadName: spec.Name,
			HostID:       spec.HostID,
			NetworkName:  spec.NetworkName,
			PrivateIPv4:  spec.PrivateIP,
			ExposureMode: spec.ExposureMode,
			GatewayID:    spec.GatewayID,
		},
		PortRequest:      spec.PortRequest,
		CreatedAt:        now,
		UpdatedAt:        now,
		LastProviderSync: "never",
	}

	w.PlanHash = ComputePlanHash(w)
	m.workloads[id] = w
	if err := m.saveStoreUnlocked(); err != nil {
		delete(m.workloads, id)
		return nil, fmt.Errorf("failed to persist created workload: %w", err)
	}
	return w, nil
}

func (m *Manager) AdoptWorkload(ctx context.Context, name string) (*Workload, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("instance name for adoption cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already managed in m.workloads
	for _, existing := range m.workloads {
		if existing.Name == name || existing.ProviderInstanceID == name {
			return nil, fmt.Errorf("instance '%s' is already managed: %w", name, ErrAlreadyManaged)
		}
	}

	instProvider, ok := m.incusDriver.InstanceProvider()
	if !ok {
		return nil, interfaces.ErrProviderUnavailable
	}

	inst, err := instProvider.GetInstance(ctx, name)
	if err != nil {
		if errors.Is(err, interfaces.ErrInstanceNotFound) {
			return nil, fmt.Errorf("instance '%s' not found on provider: %w", name, ErrIncusInstanceNotFound)
		}
		return nil, fmt.Errorf("failed to query provider for instance '%s': %w", name, err)
	}

	// Check if instance is already tagged as mystic-owned
	if inst.Labels != nil && (inst.Labels["user.mystic.owned"] == "true" || inst.Labels["mystic.owned"] == "true") {
		return nil, fmt.Errorf("instance '%s' is already tagged as managed: %w", name, ErrAlreadyManaged)
	}

	id := fmt.Sprintf("wl-%d", time.Now().UnixNano())
	now := time.Now().Format(time.RFC3339)

	// Apply adoption metadata to real instance via provider (non-destructive)
	if adopter, ok := m.incusDriver.(interfaces.InstanceAdopter); ok {
		if _, err := adopter.AdoptInstance(ctx, name, id); err != nil {
			return nil, fmt.Errorf("failed to apply adoption metadata to provider instance: %w", err)
		}
	}

	wStatus := StatusStopped
	if inst.State == interfaces.StateRunning {
		wStatus = StatusRunning
	} else if inst.State == interfaces.StateError {
		wStatus = StatusFailed
	}

	wType := TypeIncusContainer
	if inst.Type == interfaces.InstanceTypeVM {
		wType = TypeIncusVM
	}

	cpuCores := inst.Limits.CPUCores
	if cpuCores <= 0 {
		cpuCores = 1
	}

	memMB := inst.Limits.MemoryBytes / (1024 * 1024)
	if memMB <= 0 {
		memMB = 512
	}

	imgName := "adopted-external"
	if inst.Labels != nil && inst.Labels["image"] != "" {
		imgName = inst.Labels["image"]
	}

	w := &Workload{
		ID:                 id,
		Name:               inst.Name,
		HostID:             "host-local",
		Provider:           m.incusDriver.Name(),
		ProviderInstanceID: inst.Name,
		Type:               wType,
		Status:             wStatus,
		DesiredState:       inst.State,
		ActualState:        inst.State,
		SyncStatus:         instances.SyncInSync,
		CPU:                cpuCores,
		MemoryMB:           memMB,
		StorageGB:          10,
		Image:              imgName,
		Project:            "default",
		Profile:            "default",
		NetworkConfig: networking.WorkloadNetworkConfig{
			ID:           id,
			WorkloadID:   id,
			WorkloadName: inst.Name,
			HostID:       "host-local",
			NetworkName:  "incusbr0",
			PrivateIPv4:  inst.IPAddress,
			ExposureMode: hosts.ExposurePrivateOnly,
		},
		CreatedAt:        now,
		UpdatedAt:        now,
		LastProviderSync: now,
	}

	w.PlanHash = ComputePlanHash(w)
	m.workloads[id] = w

	if err := m.saveStoreUnlocked(); err != nil {
		logging.GetLogger().Error("Workload adoption succeeded in Incus but persistence write failed", "id", id, "name", inst.Name, "error", err)
		return w, fmt.Errorf("instance adopted in provider but failed to persist workload store: %w", err)
	}
	m.guard.LogAudit(id, "ADOPT", m.incusDriver.Name(), inst.Name, w.PlanHash, string(w.Status), ResultSuccess, nil)

	return w, nil
}

func (m *Manager) GetAdoptionPreview(ctx context.Context, name string) (*AdoptionPreview, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("instance name cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	instProvider, ok := m.incusDriver.InstanceProvider()
	if !ok {
		return nil, interfaces.ErrProviderUnavailable
	}

	inst, err := instProvider.GetInstance(ctx, name)
	if err != nil {
		if errors.Is(err, interfaces.ErrInstanceNotFound) {
			return nil, fmt.Errorf("instance '%s' not found on provider: %w", name, ErrIncusInstanceNotFound)
		}
		return nil, fmt.Errorf("failed to query provider for instance '%s': %w", name, err)
	}

	alreadyManaged := false
	for _, existing := range m.workloads {
		if existing.Name == name || existing.ProviderInstanceID == name {
			alreadyManaged = true
			break
		}
	}

	if inst.Labels != nil && (inst.Labels["user.mystic.owned"] == "true" || inst.Labels["mystic.owned"] == "true") {
		alreadyManaged = true
	}

	blockers := make([]string, 0)
	warnings := make([]string, 0)

	if alreadyManaged {
		blockers = append(blockers, fmt.Sprintf("Instance '%s' is already managed by Mystic Hypervisor.", name))
	}

	cpuCores := inst.Limits.CPUCores
	if cpuCores <= 0 {
		cpuCores = 1
	}

	memBytes := inst.Limits.MemoryBytes
	if memBytes <= 0 {
		memBytes = 512 * 1024 * 1024
	}

	imgName := "adopted-external"
	if inst.Labels != nil && inst.Labels["image"] != "" {
		imgName = inst.Labels["image"]
	}

	ownership := interfaces.OwnershipExternal
	if alreadyManaged {
		ownership = interfaces.OwnershipMysticOwned
	}

	return &AdoptionPreview{
		InstanceName:   inst.Name,
		Provider:       m.incusDriver.Name(),
		Type:           inst.Type,
		State:          inst.State,
		IPAddress:      inst.IPAddress,
		CPUCores:       cpuCores,
		MemoryBytes:    memBytes,
		StorageGB:      10,
		Image:          imgName,
		Network:        "incusbr0",
		Ownership:      ownership,
		AlreadyManaged: alreadyManaged,
		CanAdopt:       !alreadyManaged && len(blockers) == 0,
		Blockers:       blockers,
		Warnings:       warnings,
	}, nil
}

func (m *Manager) ReconcileAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instProvider, ok := m.incusDriver.InstanceProvider()
	if !ok {
		return nil
	}

	liveInsts, err := instProvider.ListInstances(ctx)
	if err != nil {
		logging.GetLogger().Warn("ReconcileAll skipped provider state sync: ListInstances returned error", "error", err)
		return fmt.Errorf("reconciliation failed to list provider instances: %w", err)
	}

	liveMap := make(map[string]*interfaces.Instance)
	for i := range liveInsts {
		liveMap[liveInsts[i].Name] = &liveInsts[i]
	}

	for _, w := range m.workloads {
		if w.Status == StatusDraft {
			continue
		}

		targetName := w.ProviderInstanceID
		if targetName == "" {
			targetName = w.Name
		}

		liveInst, exists := liveMap[targetName]
		if exists {
			w.ActualState = liveInst.State
			if liveInst.IPAddress != "" {
				w.NetworkConfig.PrivateIPv4 = liveInst.IPAddress
			}
			if liveInst.State == interfaces.StateRunning {
				w.Status = StatusRunning
			} else if liveInst.State == interfaces.StateStopped {
				w.Status = StatusStopped
			}
			w.SyncStatus = instances.SyncInSync
			w.LastProviderSync = time.Now().Format(time.RFC3339)
		} else {
			w.ActualState = interfaces.StateUnknown
			w.Status = StatusOrphaned
			w.SyncStatus = instances.SyncProviderMissing
			w.LastProviderSync = time.Now().Format(time.RFC3339)
		}
	}

	if err := m.saveStoreUnlocked(); err != nil {
		logging.GetLogger().Warn("ReconcileAll completed but store save returned error", "error", err)
	}
	return nil
}

func (m *Manager) ValidateWorkload(ctx context.Context, id string) (*networking.ValidationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.validateWorkloadUnlocked(ctx, id)
}

func (m *Manager) validateWorkloadUnlocked(ctx context.Context, id string) (*networking.ValidationResult, error) {
	w, exists := m.workloads[id]
	if !exists {
		return nil, fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}

	// If no explicit port allocation is requested and exposure is PrivateOnly or Unconfigured, validation passes without port allocation.
	if w.PortRequest.Protocol == "" && w.PortRequest.Mode == "" &&
		(w.NetworkConfig.ExposureMode == hosts.ExposurePrivateOnly || w.NetworkConfig.ExposureMode == hosts.ExposureUnconfigured) {
		return &networking.ValidationResult{
			IsValid:         true,
			Status:          networking.ConflictAvailable,
			Conflicts:       []networking.ConflictDetail{},
			Warnings:        []string{},
			Blockers:        []string{},
			AllocationState: hosts.ExposureStateConfigured,
			Message:         "Workload configuration valid. Private/Unconfigured exposure requires no port allocation.",
		}, nil
	}

	// Build PortAllocationRequest from workload PortRequest with context overrides
	req := w.PortRequest
	req.WorkloadID = w.ID
	req.HostID = w.HostID
	if req.GatewayID == "" {
		req.GatewayID = w.NetworkConfig.GatewayID
	}
	if req.DestinationIP == "" {
		req.DestinationIP = w.NetworkConfig.PrivateIPv4
	}

	knownWorkloads := make([]networking.WorkloadNetworkConfig, 0, len(m.workloads))
	for _, item := range m.workloads {
		knownWorkloads = append(knownWorkloads, item.NetworkConfig)
	}

	res := m.allocator.ValidateAllocation(req, nil, nil, knownWorkloads, nil, nil)
	return &res, nil
}

func (m *Manager) GeneratePlan(ctx context.Context, id string) (*ProvisioningPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, exists := m.workloads[id]
	if !exists {
		return nil, fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}

	valRes, err := m.validateWorkloadUnlocked(ctx, id)
	if err != nil {
		return nil, err
	}

	planHash := ComputePlanHash(w)
	w.PlanHash = planHash

	plan := &ProvisioningPlan{
		WorkloadID:   w.ID,
		WorkloadName: w.Name,
		Provider:     w.Provider,
		Type:         w.Type,
		Image:        w.Image,
		PlanHash:     planHash,
		Resources: map[string]interface{}{
			"cpu":        w.CPU,
			"memory_mb":  w.MemoryMB,
			"storage_gb": w.StorageGB,
		},
		Network: map[string]interface{}{
			"network_name": w.NetworkConfig.NetworkName,
			"private_ip":   w.NetworkConfig.PrivateIPv4,
		},
		Exposure: map[string]interface{}{
			"mode":       w.NetworkConfig.ExposureMode,
			"gateway_id": w.NetworkConfig.GatewayID,
		},
		Actions: []string{
			fmt.Sprintf("Validate %s provider availability", w.Provider),
			fmt.Sprintf("Create %s instance '%s' from image '%s'", w.Type, w.Name, w.Image),
			fmt.Sprintf("Configure limits (CPU: %d, RAM: %dMB)", w.CPU, w.MemoryMB),
			fmt.Sprintf("Apply network configuration (%s mode)", w.NetworkConfig.ExposureMode),
			"Start instance and verify live runtime state",
		},
		Risks:            []string{},
		IsValid:          valRes.IsValid,
		ValidationResult: *valRes,
		Approved:         false,
		CreatedAt:        time.Now().Format(time.RFC3339),
	}

	if w.NetworkConfig.ExposureMode == hosts.ExposureExternalGateway {
		plan.Risks = append(plan.Risks, "Mystic cannot configure external gateways automatically. External forwarding must be configured on upstream router/proxy.")
	}

	m.plans[id] = plan
	w.Status = StatusPlanned
	w.UpdatedAt = time.Now().Format(time.RFC3339)
	return plan, nil
}

func (m *Manager) ApprovePlan(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, exists := m.plans[id]
	if !exists {
		return fmt.Errorf("no provisioning plan generated for workload '%s': %w", id, ErrPlanNotApproved)
	}

	if !plan.IsValid {
		return fmt.Errorf("cannot approve invalid provisioning plan: %s", plan.ValidationResult.Message)
	}

	w, ok := m.workloads[id]
	if !ok {
		return fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}

	// Verify plan immutability: specification must match plan hash
	currentHash := ComputePlanHash(w)
	if currentHash != plan.PlanHash {
		w.Status = StatusDraft
		plan.Approved = false
		return fmt.Errorf("workload specification modified after plan generation: %w", ErrPlanInvalidated)
	}

	plan.Approved = true
	w.Status = StatusApproved
	w.UpdatedAt = time.Now().Format(time.RFC3339)
	m.saveStoreUnlocked()
	return nil
}

func (m *Manager) ProvisionWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	plan, planExists := m.plans[id]

	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}
	if !planExists || !plan.Approved {
		m.mu.Unlock()
		return nil, fmt.Errorf("workload '%s' has not been approved for provisioning: %w", id, ErrPlanNotApproved)
	}

	// Verify Plan Immutability
	currentHash := ComputePlanHash(w)
	if currentHash != plan.PlanHash {
		w.Status = StatusDraft
		plan.Approved = false
		m.mu.Unlock()
		return nil, fmt.Errorf("workload spec changed after approval: %w", ErrPlanInvalidated)
	}

	// Idempotency check: if already completed, return idempotent success
	opKey := m.guard.ComputeOperationKey(w.ID, "PROVISION", plan.PlanHash)
	if rec, found := m.guard.GetOperationRecord(opKey); found && rec.Result == ResultSuccess && w.Status == StatusRunning {
		m.mu.Unlock()
		return w, nil
	}

	w.Status = StatusProvisioning
	w.UpdatedAt = time.Now().Format(time.RFC3339)
	provider := m.incusDriver
	m.mu.Unlock()

	// Provider Capability Check
	requiredCap := interfaces.CapContainer
	if w.Type == TypeIncusVM || w.Type == TypeKVMVM {
		requiredCap = interfaces.CapVM
	}
	if err := m.guard.VerifyCapability(provider, requiredCap); err != nil {
		m.mu.Lock()
		w.Status = StatusFailed
		w.ErrorDetails = err.Error()
		w.UpdatedAt = time.Now().Format(time.RFC3339)
		m.mu.Unlock()
		m.guard.LogAudit(w.ID, "PROVISION", provider.Name(), w.Name, plan.PlanHash, "PROVISIONING", ResultFailed, err)
		return w, err
	}

	// Bounded Provider Execution Timeout Context
	execCtx, cancel := context.WithTimeout(ctx, m.guard.timeout)
	defer cancel()

	instProvider, ok := provider.InstanceProvider()
	if !ok {
		m.mu.Lock()
		w.Status = StatusFailed
		w.ErrorDetails = "Provider driver unattached"
		m.mu.Unlock()
		m.guard.LogAudit(w.ID, "PROVISION", provider.Name(), w.Name, plan.PlanHash, "PROVISIONING", ResultFailed, interfaces.ErrProviderUnavailable)
		return w, interfaces.ErrProviderUnavailable
	}

	// Idempotency / Existing Resource Check on Provider
	existingInst, getErr := instProvider.GetInstance(execCtx, w.Name)
	if getErr == nil && existingInst != nil {
		ownership := CheckOwnership(existingInst, w.ID)
		if ownership == OwnershipExternal {
			m.mu.Lock()
			w.Status = StatusFailed
			w.ErrorDetails = "Ownership conflict: external resource with matching name exists"
			m.mu.Unlock()
			m.guard.LogAudit(w.ID, "PROVISION", provider.Name(), w.Name, plan.PlanHash, "PROVISIONING", ResultFailed, ErrOwnershipConflict)
			return w, fmt.Errorf("%w: resource '%s' is not owned by Mystic", ErrOwnershipConflict, w.Name)
		}

		if ownership == OwnershipMystic {
			// Resource owned by Mystic -> check if specs match
			if existingInst.Limits.CPUCores == w.CPU {
				m.mu.Lock()
				w.ActualState = existingInst.State
				w.Status = StatusRunning
				w.LastProviderSync = time.Now().Format(time.RFC3339)
				m.mu.Unlock()
				m.guard.RecordOperation(opKey, w.ID, "PROVISION", plan.PlanHash, ResultSuccess)
				m.guard.LogAudit(w.ID, "PROVISION", provider.Name(), w.Name, plan.PlanHash, "RUNNING", ResultSuccess, nil)
				return w, nil
			} else {
				m.mu.Lock()
				w.Status = StatusDrifted
				w.ErrorDetails = "Existing resource configuration differs from requested specification"
				m.mu.Unlock()
				m.guard.LogAudit(w.ID, "PROVISION", provider.Name(), w.Name, plan.PlanHash, "DRIFTED", ResultFailed, ErrWorkloadConfigConflict)
				return w, ErrWorkloadConfigConflict
			}
		}
	}

	// Resource does not exist -> Create Instance with Mystic Ownership Metadata
	inst := &interfaces.Instance{
		Name: w.Name,
		Type: interfaces.InstanceTypeContainer,
		Limits: interfaces.ResourceLimits{
			CPUCores:    w.CPU,
			MemoryBytes: w.MemoryMB * 1024 * 1024,
		},
		Labels: map[string]string{
			"image":                   w.Image,
			"user.mystic.owned":       "true",
			"user.mystic.workload_id": w.ID,
			"user.mystic.host_id":     w.HostID,
			"mystic.owned":            "true",
			"mystic.workload_id":      w.ID,
		},
	}
	if w.Type == TypeIncusVM || w.Type == TypeKVMVM {
		inst.Type = interfaces.InstanceTypeVM
	}

	createdInst, createErr := instProvider.CreateInstance(execCtx, inst)
	if createErr != nil {
		// Result Classification: UNKNOWN on context timeout, FAILED on provider error
		if errors.Is(createErr, context.DeadlineExceeded) || errors.Is(createErr, context.Canceled) {
			m.mu.Lock()
			w.Status = StatusUnknown
			w.ErrorDetails = fmt.Sprintf("Provisioning execution timed out: %v", createErr)
			m.mu.Unlock()
			m.guard.RecordOperation(opKey, w.ID, "PROVISION", plan.PlanHash, ResultUnknown)
			m.guard.LogAudit(w.ID, "PROVISION", provider.Name(), w.Name, plan.PlanHash, "UNKNOWN", ResultUnknown, createErr)
			return w, fmt.Errorf("provider provisioning timed out: state unknown")
		}

		m.mu.Lock()
		w.Status = StatusFailed
		w.ErrorDetails = fmt.Sprintf("Provisioning execution failed: %v", createErr)
		w.UpdatedAt = time.Now().Format(time.RFC3339)
		m.mu.Unlock()
		m.guard.RecordOperation(opKey, w.ID, "PROVISION", plan.PlanHash, ResultFailed)
		m.guard.LogAudit(w.ID, "PROVISION", provider.Name(), w.Name, plan.PlanHash, "FAILED", ResultFailed, createErr)
		return w, fmt.Errorf("provider provisioning failed: %w", createErr)
	}

	// Start instance after creation
	_ = instProvider.StartInstance(execCtx, createdInst.Name)
	liveInst, _ := instProvider.GetInstance(execCtx, createdInst.Name)

	m.mu.Lock()
	if liveInst != nil {
		w.ActualState = liveInst.State
		if liveInst.IPAddress != "" {
			w.NetworkConfig.PrivateIPv4 = liveInst.IPAddress
		}
	} else {
		w.ActualState = interfaces.StateRunning
	}
	w.Status = StatusRunning
	w.LastProviderSync = time.Now().Format(time.RFC3339)
	w.UpdatedAt = time.Now().Format(time.RFC3339)
	m.saveStoreUnlocked()
	m.mu.Unlock()

	m.guard.RecordOperation(opKey, w.ID, "PROVISION", plan.PlanHash, ResultSuccess)
	m.guard.LogAudit(w.ID, "PROVISION", provider.Name(), createdInst.Name, plan.PlanHash, "RUNNING", ResultSuccess, nil)
	return w, nil
}

func (m *Manager) StartWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}

	// State-aware lifecycle validation: Idempotent success if already running
	if w.Status == StatusRunning {
		m.mu.Unlock()
		return w, nil
	}
	if w.Status == StatusProvisioning {
		m.mu.Unlock()
		return w, fmt.Errorf("%w: cannot start workload while provisioning is in progress", ErrIllegalStateTransition)
	}
	if w.Status == StatusUnknown {
		m.mu.Unlock()
		return w, fmt.Errorf("%w: workload state is UNKNOWN; run reconcile before starting", ErrIllegalStateTransition)
	}

	provider := m.incusDriver
	w.DesiredState = interfaces.StateRunning
	m.mu.Unlock()

	execCtx, cancel := context.WithTimeout(ctx, m.guard.timeout)
	defer cancel()

	if instProvider, ok := provider.InstanceProvider(); ok {
		if err := instProvider.StartInstance(execCtx, w.Name); err != nil {
			m.guard.LogAudit(w.ID, "START", provider.Name(), w.Name, w.PlanHash, "FAILED", ResultFailed, err)
			return w, fmt.Errorf("failed to start workload: %w", err)
		}
	}

	m.guard.LogAudit(w.ID, "START", provider.Name(), w.Name, w.PlanHash, "RUNNING", ResultSuccess, nil)
	return m.ReconcileWorkload(ctx, id)
}

func (m *Manager) StopWorkload(ctx context.Context, id string, force bool) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}

	// State-aware lifecycle validation: Idempotent success if already stopped
	if w.Status == StatusStopped {
		m.mu.Unlock()
		return w, nil
	}
	if w.Status == StatusProvisioning {
		m.mu.Unlock()
		return w, fmt.Errorf("%w: cannot stop workload while provisioning is incomplete", ErrIllegalStateTransition)
	}

	provider := m.incusDriver
	w.DesiredState = interfaces.StateStopped
	m.mu.Unlock()

	execCtx, cancel := context.WithTimeout(ctx, m.guard.timeout)
	defer cancel()

	if instProvider, ok := provider.InstanceProvider(); ok {
		if err := instProvider.StopInstance(execCtx, w.Name, force); err != nil {
			m.guard.LogAudit(w.ID, "STOP", provider.Name(), w.Name, w.PlanHash, "FAILED", ResultFailed, err)
			return w, fmt.Errorf("failed to stop workload: %w", err)
		}
	}

	m.guard.LogAudit(w.ID, "STOP", provider.Name(), w.Name, w.PlanHash, "STOPPED", ResultSuccess, nil)
	return m.ReconcileWorkload(ctx, id)
}

func (m *Manager) RestartWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}

	if w.Status == StatusProvisioning || w.Status == StatusUnknown {
		m.mu.Unlock()
		return w, fmt.Errorf("%w: cannot restart workload in state '%s'", ErrIllegalStateTransition, w.Status)
	}

	provider := m.incusDriver
	m.mu.Unlock()

	execCtx, cancel := context.WithTimeout(ctx, m.guard.timeout)
	defer cancel()

	if instProvider, ok := provider.InstanceProvider(); ok {
		if err := instProvider.RestartInstance(execCtx, w.Name); err != nil {
			m.guard.LogAudit(w.ID, "RESTART", provider.Name(), w.Name, w.PlanHash, "FAILED", ResultFailed, err)
			return w, fmt.Errorf("failed to restart workload: %w", err)
		}
	}

	m.guard.LogAudit(w.ID, "RESTART", provider.Name(), w.Name, w.PlanHash, "RUNNING", ResultSuccess, nil)
	return m.ReconcileWorkload(ctx, id)
}

func (m *Manager) DeleteWorkload(ctx context.Context, id string) error {
	m.mu.Lock()
	w, exists := m.workloads[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}
	provider := m.incusDriver
	m.mu.Unlock()

	execCtx, cancel := context.WithTimeout(ctx, m.guard.timeout)
	defer cancel()

	if instProvider, ok := provider.InstanceProvider(); ok {
		// Delete Safety: Inspect provider resource & verify Mystic ownership
		existingInst, err := instProvider.GetInstance(execCtx, w.Name)
		if err == nil && existingInst != nil {
			ownership := CheckOwnership(existingInst, w.ID)
			if ownership == OwnershipExternal {
				m.guard.LogAudit(w.ID, "DELETE", provider.Name(), w.Name, w.PlanHash, "FAILED", ResultFailed, ErrOwnershipConflict)
				return fmt.Errorf("%w: refusal to delete external unowned resource '%s'", ErrOwnershipConflict, w.Name)
			}
		}

		if err := instProvider.DeleteInstance(execCtx, w.Name); err != nil && !errors.Is(err, interfaces.ErrInstanceNotFound) {
			m.guard.LogAudit(w.ID, "DELETE", provider.Name(), w.Name, w.PlanHash, "FAILED", ResultFailed, err)
			return fmt.Errorf("failed to delete workload from provider: %w", err)
		}
	}

	m.mu.Lock()
	delete(m.workloads, id)
	delete(m.plans, id)
	m.saveStoreUnlocked()
	m.mu.Unlock()

	m.guard.LogAudit(w.ID, "DELETE", provider.Name(), w.Name, w.PlanHash, "DELETED", ResultSuccess, nil)
	return nil
}

func (m *Manager) ReconcileWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}
	provider := m.incusDriver
	m.mu.Unlock()

	execCtx, cancel := context.WithTimeout(ctx, m.guard.timeout)
	defer cancel()

	var liveInst *interfaces.Instance
	if instProvider, ok := provider.InstanceProvider(); ok {
		liveInst, _ = instProvider.GetInstance(execCtx, w.Name)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Provisioning Recovery for UNKNOWN / Ambiguous Status
	if w.Status == StatusUnknown || w.Status == StatusProvisioning {
		if liveInst != nil {
			ownership := CheckOwnership(liveInst, w.ID)
			if ownership == OwnershipMystic {
				w.ActualState = liveInst.State
				if liveInst.IPAddress != "" {
					w.NetworkConfig.PrivateIPv4 = liveInst.IPAddress
				}
				if liveInst.State == interfaces.StateRunning {
					w.Status = StatusRunning
				} else {
					w.Status = StatusStopped
				}
			} else if ownership == OwnershipExternal {
				w.Status = StatusFailed
				w.ErrorDetails = "Ownership conflict: resource belongs to external subsystem"
			}
		} else {
			w.Status = StatusFailed
			w.ErrorDetails = "Provider confirmed resource does not exist after ambiguous operation"
		}
		w.LastProviderSync = time.Now().Format(time.RFC3339)
		m.saveStoreUnlocked()
		return w, nil
	}

	meta := &instances.InstanceMetadata{
		ID:           w.ID,
		Name:         w.Name,
		Type:         interfaces.InstanceTypeContainer,
		DesiredState: w.DesiredState,
	}

	reconciled := m.reconciler.Reconcile(ctx, meta, liveInst)
	w.ActualState = reconciled.AuthoritativeState
	w.SyncStatus = reconciled.SyncStatus
	w.LastProviderSync = time.Now().Format(time.RFC3339)

	if reconciled.SyncStatus == instances.SyncOutOfSync {
		w.Status = StatusDrifted
	} else if reconciled.SyncStatus == instances.SyncProviderMissing {
		w.Status = StatusOrphaned
	} else if reconciled.AuthoritativeState == interfaces.StateRunning {
		w.Status = StatusRunning
	} else if reconciled.AuthoritativeState == interfaces.StateStopped {
		w.Status = StatusStopped
	}

	m.saveStoreUnlocked()
	return w, nil
}

func (m *Manager) GetWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w, exists := m.workloads[id]
	if !exists {
		return nil, fmt.Errorf("workload '%s' not found: %w", id, ErrWorkloadNotFound)
	}
	return w, nil
}

func (m *Manager) ListWorkloads(ctx context.Context) ([]Workload, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Workload, 0, len(m.workloads))
	for _, w := range m.workloads {
		result = append(result, *w)
	}
	return result, nil
}

func (m *Manager) GetProviderPreflight(ctx context.Context, providerName string) (*interfaces.ProviderPreflightResult, error) {
	p := m.incusDriver
	if p == nil || (providerName != "" && p.Name() != providerName) {
		if registered, err := interfaces.GetProvider(providerName); err == nil {
			p = registered
		}
	}

	var res *interfaces.ProviderPreflightResult
	if checker, ok := p.(interfaces.ProviderPreflightChecker); ok {
		var pfErr error
		res, pfErr = checker.Preflight(ctx)
		if pfErr != nil {
			return nil, pfErr
		}
	} else if pingErr := p.Ping(ctx); pingErr != nil {
		return &interfaces.ProviderPreflightResult{
			Provider:     providerName,
			Availability: interfaces.AvailabilityUnavailable,
			HealthStatus: interfaces.ProviderHealthStatus{
				Installed:   false,
				Reachable:   false,
				Operational: false,
				Capable:     false,
			},
			Capabilities: p.Capabilities().Slice(),
			Blockers:     []string{fmt.Sprintf("Provider '%s' unavailable: %v", providerName, pingErr)},
		}, nil
	} else {
		res = &interfaces.ProviderPreflightResult{
			Provider:     providerName,
			Availability: interfaces.AvailabilityAvailable,
			HealthStatus: interfaces.ProviderHealthStatus{
				Installed:   true,
				Reachable:   true,
				Operational: true,
				Capable:     true,
			},
			Capabilities: p.Capabilities().Slice(),
		}
	}

	if res != nil && len(res.ExistingInstances) > 0 {
		m.mu.RLock()
		defer m.mu.RUnlock()

		for i := range res.ExistingInstances {
			inst := &res.ExistingInstances[i]
			hasWorkload := false
			for _, w := range m.workloads {
				if w.Name == inst.Name || w.ProviderInstanceID == inst.Name {
					hasWorkload = true
					break
				}
			}

			if inst.Ownership == interfaces.OwnershipMysticOwned {
				if !hasWorkload {
					inst.Ownership = interfaces.OwnershipUnknown
					res.Warnings = append(res.Warnings, fmt.Sprintf("Instance '%s' has Mystic ownership metadata but is missing from Mystic persistent database.", inst.Name))
				}
			} else if hasWorkload {
				inst.Ownership = interfaces.OwnershipMysticOwned
			}
		}
	}

	return res, nil
}
