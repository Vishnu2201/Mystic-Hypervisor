package snapshots

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

var (
	ErrInvalidSnapshotName = errors.New("invalid snapshot name")
	ErrSnapshotNotFound    = errors.New("snapshot not found")
	ErrSnapshotExists      = errors.New("snapshot with given name already exists")
	ErrWorkloadNotFound    = errors.New("workload not found")
)

var validSnapshotNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)

// SnapshotWorkloadResolver resolves target instance names and existence for a workload ID.
type SnapshotWorkloadResolver interface {
	ResolveWorkloadInstance(ctx context.Context, workloadID string) (instanceName string, err error)
}

// Manager orchestrates snapshot lifecycle, persistence, and provider interaction.
type Manager struct {
	mu          sync.RWMutex
	store       SnapshotStore
	provider    interfaces.Provider
	resolver    SnapshotWorkloadResolver
	snapshotMap map[string]*Snapshot // Key: snapshot ID ("<workloadID>/<snapshotName>")
}

// NewManager creates a SnapshotManager instance.
func NewManager(store SnapshotStore, provider interfaces.Provider, resolver SnapshotWorkloadResolver) *Manager {
	m := &Manager{
		store:       store,
		provider:    provider,
		resolver:    resolver,
		snapshotMap: make(map[string]*Snapshot),
	}

	if store != nil {
		if loaded, err := m.store.Load(); err == nil && loaded != nil {
			m.snapshotMap = loaded
			logging.GetLogger().Info("Loaded persisted snapshots from store",
				"store_path", m.store.FilePath(),
				"count", len(loaded),
			)
		}
	}

	return m
}

// StorePath returns the configured filesystem path of the snapshot store.
func (m *Manager) StorePath() string {
	if m.store != nil {
		return m.store.FilePath()
	}
	return ""
}

// SetProvider attaches or updates the virtualization provider.
func (m *Manager) SetProvider(provider interfaces.Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.provider = provider
}

// SetResolver attaches or updates the workload resolver.
func (m *Manager) SetResolver(resolver SnapshotWorkloadResolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resolver = resolver
}

// ValidateSnapshotName checks that the snapshot name is non-empty, safe against option injection, and within length limits.
func ValidateSnapshotName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: snapshot name cannot be empty", ErrInvalidSnapshotName)
	}
	if strings.HasPrefix(trimmed, "-") {
		return fmt.Errorf("%w: snapshot name '%s' cannot start with a hyphen (option injection rejected)", ErrInvalidSnapshotName, trimmed)
	}
	if len(trimmed) > 64 {
		return fmt.Errorf("%w: snapshot name length exceeds 64 characters", ErrInvalidSnapshotName)
	}
	if !validSnapshotNameRegex.MatchString(trimmed) {
		return fmt.Errorf("%w: snapshot name '%s' contains invalid characters", ErrInvalidSnapshotName, trimmed)
	}
	return nil
}

func MakeSnapshotID(workloadID, snapshotName string) string {
	return fmt.Sprintf("%s/%s", workloadID, snapshotName)
}

func (m *Manager) getSnapshotProvider() (interfaces.SnapshotProvider, error) {
	if m.provider == nil {
		return nil, interfaces.ErrProviderUnavailable
	}
	snapProv, ok := m.provider.SnapshotProvider()
	if !ok || snapProv == nil {
		return nil, interfaces.ErrUnsupportedOperation
	}
	return snapProv, nil
}

func (m *Manager) resolveInstanceName(ctx context.Context, workloadID string) (string, error) {
	workloadID = strings.TrimSpace(workloadID)
	if workloadID == "" {
		return "", fmt.Errorf("%w: workload ID is required", ErrWorkloadNotFound)
	}

	if m.resolver != nil {
		instName, err := m.resolver.ResolveWorkloadInstance(ctx, workloadID)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrWorkloadNotFound, err)
		}
		if instName != "" {
			return instName, nil
		}
	}

	// Fallback to treating workloadID as the instance name if no resolver is attached
	return workloadID, nil
}

// ListSnapshots returns all snapshots for a given workload.
func (m *Manager) ListSnapshots(ctx context.Context, workloadID string) ([]*Snapshot, error) {
	instName, err := m.resolveInstanceName(ctx, workloadID)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Snapshot, 0)
	for _, snap := range m.snapshotMap {
		if snap.WorkloadID == workloadID || snap.WorkloadName == instName {
			cp := *snap
			result = append(result, &cp)
		}
	}

	// Also attempt sync with provider if available
	snapProv, provErr := m.getSnapshotProvider()
	if provErr == nil {
		provSnaps, err := snapProv.ListSnapshots(ctx, instName)
		if err == nil {
			// Merge provider snapshots if any missing
			existingNames := make(map[string]bool)
			for _, s := range result {
				existingNames[s.Name] = true
			}
			for _, ps := range provSnaps {
				if !existingNames[ps.Name] {
					newSnap := &Snapshot{
						ID:           MakeSnapshotID(workloadID, ps.Name),
						Name:         ps.Name,
						WorkloadID:   workloadID,
						WorkloadName: instName,
						Stateful:     ps.Stateful,
						CreatedAt:    ps.CreatedAt,
						Status:       "ACTIVE",
					}
					result = append(result, newSnap)
				}
			}
		}
	}

	return result, nil
}

// GetSnapshot retrieves a specific snapshot by workload ID and snapshot name.
func (m *Manager) GetSnapshot(ctx context.Context, workloadID, snapshotName string) (*Snapshot, error) {
	if err := ValidateSnapshotName(snapshotName); err != nil {
		return nil, err
	}

	snapID := MakeSnapshotID(workloadID, snapshotName)

	m.mu.RLock()
	snap, exists := m.snapshotMap[snapID]
	m.mu.RUnlock()

	if exists {
		cp := *snap
		return &cp, nil
	}

	// Attempt provider fallback lookup
	instName, err := m.resolveInstanceName(ctx, workloadID)
	if err != nil {
		return nil, err
	}

	snapProv, err := m.getSnapshotProvider()
	if err != nil {
		return nil, ErrSnapshotNotFound
	}

	provSnaps, err := snapProv.ListSnapshots(ctx, instName)
	if err != nil {
		return nil, ErrSnapshotNotFound
	}

	for _, ps := range provSnaps {
		if ps.Name == snapshotName {
			found := &Snapshot{
				ID:           snapID,
				Name:         ps.Name,
				WorkloadID:   workloadID,
				WorkloadName: instName,
				Stateful:     ps.Stateful,
				CreatedAt:    ps.CreatedAt,
				Status:       "ACTIVE",
			}
			return found, nil
		}
	}

	return nil, ErrSnapshotNotFound
}

// CreateSnapshot creates a new snapshot for a workload.
func (m *Manager) CreateSnapshot(ctx context.Context, workloadID, snapshotName string, stateful bool, description string) (*Snapshot, error) {
	if err := ValidateSnapshotName(snapshotName); err != nil {
		return nil, err
	}

	instName, err := m.resolveInstanceName(ctx, workloadID)
	if err != nil {
		return nil, err
	}

	snapID := MakeSnapshotID(workloadID, snapshotName)

	m.mu.Lock()
	if _, exists := m.snapshotMap[snapID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w: snapshot '%s' already exists for workload '%s'", ErrSnapshotExists, snapshotName, workloadID)
	}
	m.mu.Unlock()

	snapProv, err := m.getSnapshotProvider()
	if err != nil {
		return nil, fmt.Errorf("provider snapshot capability unavailable: %w", err)
	}

	// Execute snapshot creation on provider FIRST
	createdProvSnap, err := snapProv.CreateSnapshot(ctx, instName, snapshotName, stateful)
	if err != nil {
		return nil, fmt.Errorf("provider failed to create snapshot '%s' for instance '%s': %w", snapshotName, instName, err)
	}

	createdAt := time.Now()
	if createdProvSnap != nil && !createdProvSnap.CreatedAt.IsZero() {
		createdAt = createdProvSnap.CreatedAt
	}

	snap := &Snapshot{
		ID:           snapID,
		Name:         snapshotName,
		WorkloadID:   workloadID,
		WorkloadName: instName,
		Stateful:     stateful,
		CreatedAt:    createdAt,
		Status:       "ACTIVE",
		Description:  description,
	}

	m.mu.Lock()
	m.snapshotMap[snapID] = snap
	var saveErr error
	if m.store != nil {
		saveErr = m.store.Save(m.snapshotMap)
	}
	m.mu.Unlock()

	if saveErr != nil {
		logging.GetLogger().Error("Failed to persist snapshot store after provider creation",
			"snapshot_id", snapID,
			"error", saveErr,
		)
	}

	logging.GetLogger().Info("AUDIT_EVENT",
		"action", "snapshot.create",
		"target", snapID,
		"workload", instName,
		"result", "SUCCESS",
	)

	cp := *snap
	return &cp, nil
}

// RestoreSnapshot restores an instance to a target snapshot state.
func (m *Manager) RestoreSnapshot(ctx context.Context, workloadID, snapshotName string) (*Snapshot, error) {
	if err := ValidateSnapshotName(snapshotName); err != nil {
		return nil, err
	}

	instName, err := m.resolveInstanceName(ctx, workloadID)
	if err != nil {
		return nil, err
	}

	snap, err := m.GetSnapshot(ctx, workloadID, snapshotName)
	if err != nil {
		return nil, err
	}

	snapProv, err := m.getSnapshotProvider()
	if err != nil {
		return nil, fmt.Errorf("provider snapshot capability unavailable: %w", err)
	}

	if err := snapProv.RestoreSnapshot(ctx, instName, snapshotName); err != nil {
		return nil, fmt.Errorf("provider failed to restore snapshot '%s' for instance '%s': %w", snapshotName, instName, err)
	}

	m.mu.Lock()
	if existing, ok := m.snapshotMap[snap.ID]; ok {
		existing.Status = "ACTIVE"
		if m.store != nil {
			_ = m.store.Save(m.snapshotMap)
		}
	}
	m.mu.Unlock()

	logging.GetLogger().Info("AUDIT_EVENT",
		"action", "snapshot.restore",
		"target", snap.ID,
		"workload", instName,
		"result", "SUCCESS",
	)

	return snap, nil
}

// DeleteSnapshot deletes a snapshot from the provider and persistent store.
func (m *Manager) DeleteSnapshot(ctx context.Context, workloadID, snapshotName string) error {
	if err := ValidateSnapshotName(snapshotName); err != nil {
		return err
	}

	instName, err := m.resolveInstanceName(ctx, workloadID)
	if err != nil {
		return err
	}

	snapID := MakeSnapshotID(workloadID, snapshotName)

	snapProv, err := m.getSnapshotProvider()
	if err != nil {
		return fmt.Errorf("provider snapshot capability unavailable: %w", err)
	}

	// Delete from provider FIRST
	if err := snapProv.DeleteSnapshot(ctx, instName, snapshotName); err != nil {
		return fmt.Errorf("provider failed to delete snapshot '%s' for instance '%s': %w", snapshotName, instName, err)
	}

	m.mu.Lock()
	delete(m.snapshotMap, snapID)
	var saveErr error
	if m.store != nil {
		saveErr = m.store.Save(m.snapshotMap)
	}
	m.mu.Unlock()

	if saveErr != nil {
		logging.GetLogger().Error("Failed to update snapshot store after deletion",
			"snapshot_id", snapID,
			"error", saveErr,
		)
	}

	logging.GetLogger().Info("AUDIT_EVENT",
		"action", "snapshot.delete",
		"target", snapID,
		"workload", instName,
		"result", "SUCCESS",
	)

	return nil
}
