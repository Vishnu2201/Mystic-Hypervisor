package networking

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
)

var (
	ErrExposureNotFound = errors.New("network exposure not found")
	ErrExposureConflict = errors.New("network exposure configuration conflict")
	ErrInvalidExposure  = errors.New("invalid network exposure configuration")
)

// ExposureManager manages first-class network exposures and integrates with AllocatorEngine.
type ExposureManager struct {
	mu        sync.RWMutex
	exposures map[string]*NetworkExposure
	allocator *AllocatorEngine
	store     ExposureStore
	driver    ProviderExposureDriver
}

// NewExposureManager constructs an ExposureManager with default FileExposureStore.
func NewExposureManager(store ExposureStore, driver ProviderExposureDriver) *ExposureManager {
	if store == nil {
		store = NewFileExposureStore("")
	}
	em := &ExposureManager{
		exposures: make(map[string]*NetworkExposure),
		allocator: NewAllocatorEngine(),
		store:     store,
		driver:    driver,
	}

	if loaded, err := store.Load(); err == nil && loaded != nil {
		em.exposures = loaded
		logging.GetLogger().Info("Loaded network exposures from store", "store_path", store.FilePath(), "count", len(em.exposures))
	} else if err != nil {
		logging.GetLogger().Error("Failed to load network exposure store", "store_path", store.FilePath(), "error", err)
	}

	return em
}

// StorePath returns the file path of the underlying exposure store.
func (em *ExposureManager) StorePath() string {
	if em.store != nil {
		return em.store.FilePath()
	}
	return "in-memory"
}

func (em *ExposureManager) saveStoreUnlocked() error {
	if em.store != nil {
		if err := em.store.Save(em.exposures); err != nil {
			logging.GetLogger().Error("Failed to persist network exposures store", "store_path", em.StorePath(), "error", err)
			return fmt.Errorf("failed to persist network exposures store: %w", err)
		}
	}
	return nil
}

// CreateNetworkExposure validates and persists a new NetworkExposure entity.
func (em *ExposureManager) CreateNetworkExposure(ctx context.Context, exp *NetworkExposure) (*NetworkExposure, error) {
	if exp == nil {
		return nil, fmt.Errorf("exposure configuration cannot be nil: %w", ErrInvalidExposure)
	}

	if exp.WorkloadID == "" {
		return nil, fmt.Errorf("workload_id is required: %w", ErrInvalidExposure)
	}

	if exp.PublicPort < 1 || exp.PublicPort > 65535 {
		return nil, fmt.Errorf("public_port %d must be between 1 and 65535: %w", exp.PublicPort, ErrInvalidExposure)
	}

	if exp.InternalPort < 1 || exp.InternalPort > 65535 {
		return nil, fmt.Errorf("internal_port %d must be between 1 and 65535: %w", exp.InternalPort, ErrInvalidExposure)
	}

	if exp.InternalIP != "" && net.ParseIP(exp.InternalIP) == nil {
		return nil, fmt.Errorf("invalid internal_ip address '%s': %w", exp.InternalIP, ErrInvalidExposure)
	}

	if exp.Protocol == "" {
		exp.Protocol = hosts.ProtocolTCP
	} else if exp.Protocol != hosts.ProtocolTCP && exp.Protocol != hosts.ProtocolUDP && exp.Protocol != hosts.ProtocolTCPUDP {
		return nil, fmt.Errorf("invalid protocol '%s': %w", exp.Protocol, ErrInvalidExposure)
	}

	em.mu.Lock()
	defer em.mu.Unlock()

	// Unique ID generation if omitted
	if exp.ID == "" {
		exp.ID = fmt.Sprintf("exp-%s-%d", exp.WorkloadID, exp.PublicPort)
	}

	if _, exists := em.exposures[exp.ID]; exists {
		return nil, fmt.Errorf("network exposure '%s' already exists: %w", exp.ID, ErrExposureConflict)
	}

	// Validate using AllocatorEngine
	valRes := em.validateExposureUnlocked(exp)
	if !valRes.IsValid || (valRes.Status != ConflictAvailable && valRes.Status != "") {
		msg := valRes.Message
		if len(valRes.Conflicts) > 0 {
			msg = valRes.Conflicts[0].Message
		}
		return nil, fmt.Errorf("port allocation validation failed: %s: %w", msg, ErrExposureConflict)
	}

	if exp.DesiredState == "" {
		exp.DesiredState = hosts.ExposureStateConfigured
	}
	exp.ActualState = hosts.ExposureStateConfigured
	exp.SyncStatus = instances.SyncInSync
	exp.CreatedAt = time.Now()
	exp.UpdatedAt = time.Now()

	em.exposures[exp.ID] = exp
	if err := em.saveStoreUnlocked(); err != nil {
		delete(em.exposures, exp.ID)
		return nil, err
	}

	return exp, nil
}

// GetNetworkExposure retrieves an exposure by ID.
func (em *ExposureManager) GetNetworkExposure(ctx context.Context, id string) (*NetworkExposure, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	exp, exists := em.exposures[id]
	if !exists {
		return nil, fmt.Errorf("exposure '%s' not found: %w", id, ErrExposureNotFound)
	}
	return exp, nil
}

// ListNetworkExposures lists all managed exposures.
func (em *ExposureManager) ListNetworkExposures(ctx context.Context) ([]NetworkExposure, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	res := make([]NetworkExposure, 0, len(em.exposures))
	for _, exp := range em.exposures {
		res = append(res, *exp)
	}
	return res, nil
}

// ListWorkloadExposures lists exposures associated with a specific workload.
func (em *ExposureManager) ListWorkloadExposures(ctx context.Context, workloadID string) ([]NetworkExposure, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	res := make([]NetworkExposure, 0)
	for _, exp := range em.exposures {
		if exp.WorkloadID == workloadID {
			res = append(res, *exp)
		}
	}
	return res, nil
}

// UpdateNetworkExposure updates an existing NetworkExposure.
func (em *ExposureManager) UpdateNetworkExposure(ctx context.Context, id string, updated *NetworkExposure) (*NetworkExposure, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	existing, exists := em.exposures[id]
	if !exists {
		return nil, fmt.Errorf("exposure '%s' not found: %w", id, ErrExposureNotFound)
	}

	if updated.PublicPort > 0 {
		existing.PublicPort = updated.PublicPort
	}
	if updated.InternalPort > 0 {
		existing.InternalPort = updated.InternalPort
	}
	if updated.InternalIP != "" {
		existing.InternalIP = updated.InternalIP
	}
	if updated.PublicIP != "" {
		existing.PublicIP = updated.PublicIP
	}
	if updated.Protocol != "" {
		existing.Protocol = updated.Protocol
	}
	if updated.ExposureMode != "" {
		existing.ExposureMode = updated.ExposureMode
	}
	if updated.Description != "" {
		existing.Description = updated.Description
	}

	valRes := em.validateExposureUnlocked(existing)
	if !valRes.IsValid || (valRes.Status != ConflictAvailable && valRes.Status != "") {
		msg := valRes.Message
		if len(valRes.Conflicts) > 0 {
			msg = valRes.Conflicts[0].Message
		}
		return nil, fmt.Errorf("updated exposure validation failed: %s: %w", msg, ErrExposureConflict)
	}

	existing.UpdatedAt = time.Now()
	if err := em.saveStoreUnlocked(); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeleteNetworkExposure deletes an exposure by ID.
func (em *ExposureManager) DeleteNetworkExposure(ctx context.Context, id string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	exp, exists := em.exposures[id]
	if !exists {
		return fmt.Errorf("exposure '%s' not found: %w", id, ErrExposureNotFound)
	}

	if em.driver != nil {
		_ = em.driver.DeleteExposure(ctx, exp)
	}

	delete(em.exposures, id)
	return em.saveStoreUnlocked()
}

// SetDriver sets or updates the ProviderExposureDriver implementation.
func (em *ExposureManager) SetDriver(driver ProviderExposureDriver) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.driver = driver
}

// ApplyExposure executes real provider exposure application (e.g. creating Incus proxy device).
func (em *ExposureManager) ApplyExposure(ctx context.Context, id string) (*NetworkExposure, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	exp, exists := em.exposures[id]
	if !exists {
		return nil, fmt.Errorf("exposure '%s' not found: %w", id, ErrExposureNotFound)
	}

	if em.driver == nil {
		return nil, fmt.Errorf("no provider exposure driver is attached")
	}

	exp.DesiredState = hosts.ExposureStateApplied
	exp.UpdatedAt = time.Now()

	err := em.driver.CreateExposure(ctx, exp)
	if err != nil {
		exp.ActualState = hosts.ExposureStateUnconfigured
		exp.SyncStatus = instances.SyncOutOfSync
		_ = em.saveStoreUnlocked()
		return nil, fmt.Errorf("provider failed to apply exposure: %w", err)
	}

	exp.ActualState = hosts.ExposureStateApplied
	exp.SyncStatus = instances.SyncInSync
	exp.LastSync = time.Now()

	if err := em.saveStoreUnlocked(); err != nil {
		return nil, err
	}

	return exp, nil
}

// ValidateNetworkExposure validates a network exposure proposal against AllocatorEngine.
func (em *ExposureManager) ValidateNetworkExposure(ctx context.Context, exp *NetworkExposure) (*ValidationResult, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	res := em.validateExposureUnlocked(exp)
	return &res, nil
}

func (em *ExposureManager) validateExposureUnlocked(exp *NetworkExposure) ValidationResult {
	req := PortAllocationRequest{
		WorkloadID:        exp.WorkloadID,
		HostID:            "host-main",
		GatewayID:         exp.GatewayID,
		Mode:              AllocationModeSingle,
		ExternalStartPort: exp.PublicPort,
		ExternalEndPort:   exp.PublicPort,
		InternalStartPort: exp.InternalPort,
		InternalEndPort:   exp.InternalPort,
		Protocol:          exp.Protocol,
		PublicIP:          exp.PublicIP,
		DestinationIP:     exp.InternalIP,
	}

	existingRules := make([]hosts.ForwardingRule, 0)
	for _, item := range em.exposures {
		if item.ID == exp.ID {
			continue // Skip self during update validation
		}
		existingRules = append(existingRules, item.ToForwardingRule())
	}

	return em.allocator.ValidateAllocation(req, nil, existingRules, nil, nil, nil)
}

// ReconcileExposure resolves exposure desired state against provider observed state.
func (em *ExposureManager) ReconcileExposure(ctx context.Context, id string) (*NetworkExposure, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	exp, exists := em.exposures[id]
	if !exists {
		return nil, fmt.Errorf("exposure '%s' not found: %w", id, ErrExposureNotFound)
	}

	exp.LastSync = time.Now()

	if em.driver != nil {
		status, err := em.driver.GetExposure(ctx, exp)
		if err == nil && status != nil {
			if status.Active {
				exp.ActualState = hosts.ExposureStateApplied
			} else {
				exp.ActualState = hosts.ExposureStateUnconfigured
			}

			if exp.DesiredState == exp.ActualState {
				exp.SyncStatus = instances.SyncInSync
			} else {
				exp.SyncStatus = instances.SyncOutOfSync
			}
		}
	} else {
		exp.SyncStatus = instances.SyncInSync
	}

	em.saveStoreUnlocked()
	return exp, nil
}

// ReconcileAllExposures reconciles all exposures.
func (em *ExposureManager) ReconcileAllExposures(ctx context.Context) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	for _, exp := range em.exposures {
		exp.LastSync = time.Now()
		if em.driver != nil {
			status, err := em.driver.GetExposure(ctx, exp)
			if err == nil && status != nil {
				if status.Active {
					exp.ActualState = hosts.ExposureStateApplied
				} else {
					exp.ActualState = hosts.ExposureStateUnconfigured
				}

				if exp.DesiredState == exp.ActualState {
					exp.SyncStatus = instances.SyncInSync
				} else {
					exp.SyncStatus = instances.SyncOutOfSync
				}
			}
		}
	}
	return em.saveStoreUnlocked()
}

// MigrateForwardingRules converts legacy ForwardingRules into first-class NetworkExposures idempotently.
func (em *ExposureManager) MigrateForwardingRules(workloadID, workloadName string, mode hosts.ExposureMode, rules []hosts.ForwardingRule) int {
	if len(rules) == 0 {
		return 0
	}

	em.mu.Lock()
	defer em.mu.Unlock()

	migratedCount := 0
	for _, rule := range rules {
		exp := ConvertForwardingRuleToExposure(rule, workloadID, workloadName, mode)
		if _, exists := em.exposures[exp.ID]; !exists {
			em.exposures[exp.ID] = exp
			migratedCount++
		}
	}

	if migratedCount > 0 {
		_ = em.saveStoreUnlocked()
	}

	return migratedCount
}
