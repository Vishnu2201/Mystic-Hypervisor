package workloads

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
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
}

// NewManager constructs a WorkloadManager.
func NewManager() *Manager {
	m := &Manager{
		workloads:   make(map[string]*Workload),
		plans:       make(map[string]*ProvisioningPlan),
		allocator:   networking.NewAllocatorEngine(),
		incusDriver: incus.NewIncusProvider("/var/lib/incus/unix.socket"),
		reconciler:  instances.NewReconciler(),
	}
	return m
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
		CreatedAt:        now,
		UpdatedAt:        now,
		LastProviderSync: "never",
	}

	m.workloads[id] = w
	return w, nil
}

func (m *Manager) ValidateWorkload(ctx context.Context, id string) (*networking.ValidationResult, error) {
	m.mu.RLock()
	w, exists := m.workloads[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workload '%s' not found", id)
	}

	// Build PortAllocationRequest if exposure calls for it
	req := networking.PortAllocationRequest{
		WorkloadID:    w.ID,
		HostID:        w.HostID,
		GatewayID:     w.NetworkConfig.GatewayID,
		DestinationIP: w.NetworkConfig.PrivateIPv4,
	}

	knownWorkloads := make([]networking.WorkloadNetworkConfig, 0, len(m.workloads))
	m.mu.RLock()
	for _, item := range m.workloads {
		knownWorkloads = append(knownWorkloads, item.NetworkConfig)
	}
	m.mu.RUnlock()

	res := m.allocator.ValidateAllocation(req, nil, nil, knownWorkloads, nil, nil)
	return &res, nil
}

func (m *Manager) GeneratePlan(ctx context.Context, id string) (*ProvisioningPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, exists := m.workloads[id]
	if !exists {
		return nil, fmt.Errorf("workload '%s' not found", id)
	}

	valRes, err := m.ValidateWorkload(ctx, id)
	if err != nil {
		return nil, err
	}

	plan := &ProvisioningPlan{
		WorkloadID:   w.ID,
		WorkloadName: w.Name,
		Provider:     w.Provider,
		Type:         w.Type,
		Image:        w.Image,
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
		return fmt.Errorf("no provisioning plan generated for workload '%s'", id)
	}

	if !plan.IsValid {
		return fmt.Errorf("cannot approve invalid provisioning plan: %s", plan.ValidationResult.Message)
	}

	plan.Approved = true
	if w, ok := m.workloads[id]; ok {
		w.Status = StatusApproved
		w.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	return nil
}

func (m *Manager) ProvisionWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	plan, planExists := m.plans[id]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("workload '%s' not found", id)
	}
	if !planExists || !plan.Approved {
		return nil, fmt.Errorf("workload '%s' has not been approved for provisioning", id)
	}

	m.mu.Lock()
	w.Status = StatusProvisioning
	w.UpdatedAt = time.Now().Format(time.RFC3339)
	m.mu.Unlock()

	// Check Incus provider availability
	if instProvider, ok := m.incusDriver.InstanceProvider(); ok {
		inst := &interfaces.Instance{
			Name: w.Name,
			Type: interfaces.InstanceTypeContainer,
			Limits: interfaces.ResourceLimits{
				CPUCores:    w.CPU,
				MemoryBytes: w.MemoryMB * 1024 * 1024,
			},
			Labels: map[string]string{
				"image": w.Image,
			},
		}

		createdInst, err := instProvider.CreateInstance(ctx, inst)
		if err != nil {
			m.mu.Lock()
			w.Status = StatusFailed
			w.ErrorDetails = fmt.Sprintf("Provisioning execution failed: %v", err)
			w.UpdatedAt = time.Now().Format(time.RFC3339)
			m.mu.Unlock()
			return w, fmt.Errorf("provider provisioning failed: %w", err)
		}

		// Start instance
		_ = instProvider.StartInstance(ctx, createdInst.Name)

		// Query real state
		liveInst, getErr := instProvider.GetInstance(ctx, createdInst.Name)
		m.mu.Lock()
		if getErr == nil && liveInst != nil {
			w.ActualState = liveInst.State
			w.NetworkConfig.PrivateIPv4 = liveInst.IPAddress
			w.Status = StatusRunning
		} else {
			w.ActualState = interfaces.StateRunning
			w.Status = StatusRunning
		}
		w.LastProviderSync = time.Now().Format(time.RFC3339)
		w.UpdatedAt = time.Now().Format(time.RFC3339)
		m.mu.Unlock()

		return w, nil
	}

	// Incus provider unavailable (e.g. non-Linux dev host or missing socket)
	m.mu.Lock()
	w.Status = StatusFailed
	w.ErrorDetails = "Virtualization provider 'incus' is unavailable on this host."
	w.UpdatedAt = time.Now().Format(time.RFC3339)
	m.mu.Unlock()
	return w, interfaces.ErrProviderUnavailable
}

func (m *Manager) StartWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("workload '%s' not found", id)
	}

	if instProvider, ok := m.incusDriver.InstanceProvider(); ok {
		if err := instProvider.StartInstance(ctx, w.Name); err != nil {
			return w, fmt.Errorf("failed to start workload: %w", err)
		}
	}

	return m.ReconcileWorkload(ctx, id)
}

func (m *Manager) StopWorkload(ctx context.Context, id string, force bool) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("workload '%s' not found", id)
	}

	if instProvider, ok := m.incusDriver.InstanceProvider(); ok {
		if err := instProvider.StopInstance(ctx, w.Name, force); err != nil {
			return w, fmt.Errorf("failed to stop workload: %w", err)
		}
	}

	return m.ReconcileWorkload(ctx, id)
}

func (m *Manager) RestartWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.Lock()
	w, exists := m.workloads[id]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("workload '%s' not found", id)
	}

	if instProvider, ok := m.incusDriver.InstanceProvider(); ok {
		if err := instProvider.RestartInstance(ctx, w.Name); err != nil {
			return w, fmt.Errorf("failed to restart workload: %w", err)
		}
	}

	return m.ReconcileWorkload(ctx, id)
}

func (m *Manager) DeleteWorkload(ctx context.Context, id string) error {
	m.mu.Lock()
	w, exists := m.workloads[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("workload '%s' not found", id)
	}

	if instProvider, ok := m.incusDriver.InstanceProvider(); ok {
		_ = instProvider.DeleteInstance(ctx, w.Name)
	}

	m.mu.Lock()
	delete(m.workloads, id)
	delete(m.plans, id)
	m.mu.Unlock()
	return nil
}

func (m *Manager) ReconcileWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, exists := m.workloads[id]
	if !exists {
		return nil, fmt.Errorf("workload '%s' not found", id)
	}

	var liveInst *interfaces.Instance
	if instProvider, ok := m.incusDriver.InstanceProvider(); ok {
		liveInst, _ = instProvider.GetInstance(ctx, w.Name)
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
	} else if reconciled.AuthoritativeState == interfaces.StateRunning {
		w.Status = StatusRunning
	} else if reconciled.AuthoritativeState == interfaces.StateStopped {
		w.Status = StatusStopped
	}

	return w, nil
}

func (m *Manager) GetWorkload(ctx context.Context, id string) (*Workload, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w, exists := m.workloads[id]
	if !exists {
		return nil, fmt.Errorf("workload '%s' not found", id)
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
