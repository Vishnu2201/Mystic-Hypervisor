package snapshots

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

type mockSnapshotProvider struct {
	snapshots map[string]*interfaces.Snapshot
}

func newMockSnapshotProvider() *mockSnapshotProvider {
	return &mockSnapshotProvider{snapshots: make(map[string]*interfaces.Snapshot)}
}

func (m *mockSnapshotProvider) ListSnapshots(ctx context.Context, instanceID string) ([]interfaces.Snapshot, error) {
	result := make([]interfaces.Snapshot, 0)
	for _, s := range m.snapshots {
		if s.InstanceID == instanceID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *mockSnapshotProvider) CreateSnapshot(ctx context.Context, instanceID, snapshotName string, stateful bool) (*interfaces.Snapshot, error) {
	key := instanceID + "/" + snapshotName
	if _, exists := m.snapshots[key]; exists {
		return nil, interfaces.ErrInstanceExists
	}
	s := &interfaces.Snapshot{
		Name:       snapshotName,
		InstanceID: instanceID,
		Stateful:   stateful,
		CreatedAt:  time.Now(),
	}
	m.snapshots[key] = s
	return s, nil
}

func (m *mockSnapshotProvider) RestoreSnapshot(ctx context.Context, instanceID, snapshotName string) error {
	key := instanceID + "/" + snapshotName
	if _, exists := m.snapshots[key]; !exists {
		return interfaces.ErrInstanceNotFound
	}
	return nil
}

func (m *mockSnapshotProvider) DeleteSnapshot(ctx context.Context, instanceID, snapshotName string) error {
	key := instanceID + "/" + snapshotName
	if _, exists := m.snapshots[key]; !exists {
		return interfaces.ErrInstanceNotFound
	}
	delete(m.snapshots, key)
	return nil
}

type dummyProvider struct {
	snapProv interfaces.SnapshotProvider
}

func (d *dummyProvider) Name() string                                          { return "dummy" }
func (d *dummyProvider) Capabilities() interfaces.CapabilitySet                { return interfaces.NewCapabilitySet() }
func (d *dummyProvider) Ping(ctx context.Context) error                        { return nil }
func (d *dummyProvider) Close() error                                          { return nil }
func (d *dummyProvider) InstanceProvider() (interfaces.InstanceProvider, bool) { return nil, false }
func (d *dummyProvider) ImageProvider() (interfaces.ImageProvider, bool)       { return nil, false }
func (d *dummyProvider) SnapshotProvider() (interfaces.SnapshotProvider, bool) {
	return d.snapProv, true
}
func (d *dummyProvider) StorageProvider() (interfaces.StorageProvider, bool) { return nil, false }
func (d *dummyProvider) NetworkProvider() (interfaces.NetworkProvider, bool) { return nil, false }

type dummyResolver struct{}

func (r *dummyResolver) ResolveWorkloadInstance(ctx context.Context, workloadID string) (string, error) {
	if workloadID == "wl-nonexistent" {
		return "", ErrWorkloadNotFound
	}
	return "test-instance", nil
}

func TestValidateSnapshotName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "snap-01", false},
		{"valid dots and hyphens", "snap.v1.0-alpha", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"option injection hyphen", "-option-flag", true},
		{"invalid symbols", "snap$shot!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSnapshotName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSnapshotName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestFileSnapshotStore_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "snapshots.json")

	store := NewFileSnapshotStore(storeFile)

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load empty store failed: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 snapshots, got %d", len(loaded))
	}

	snaps := map[string]*Snapshot{
		"wl-01/snap-1": {
			ID:           "wl-01/snap-1",
			Name:         "snap-1",
			WorkloadID:   "wl-01",
			WorkloadName: "test-instance",
			Stateful:     false,
			CreatedAt:    time.Now(),
			Status:       "ACTIVE",
		},
	}

	if err := store.Save(snaps); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	store2 := NewFileSnapshotStore(storeFile)
	reloaded, err := store2.Load()
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if len(reloaded) != 1 || reloaded["wl-01/snap-1"] == nil {
		t.Fatalf("Reloaded map mismatch: %v", reloaded)
	}
	if reloaded["wl-01/snap-1"].Name != "snap-1" {
		t.Errorf("expected snapshot name 'snap-1', got '%s'", reloaded["wl-01/snap-1"].Name)
	}
}

func TestSnapshotManager_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "snapshots.json")

	mockProv := newMockSnapshotProvider()
	provider := &dummyProvider{snapProv: mockProv}
	store := NewFileSnapshotStore(storeFile)
	resolver := &dummyResolver{}

	mgr := NewManager(store, provider, resolver)
	ctx := context.Background()

	// 1. Create Snapshot
	snap, err := mgr.CreateSnapshot(ctx, "wl-01", "snap-test-01", false, "Test description")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snap.Name != "snap-test-01" {
		t.Errorf("expected snap name 'snap-test-01', got '%s'", snap.Name)
	}

	// 2. Duplicate Creation Rejection
	_, dupErr := mgr.CreateSnapshot(ctx, "wl-01", "snap-test-01", false, "")
	if dupErr == nil {
		t.Fatalf("expected duplicate creation to fail, but it succeeded")
	}

	// 3. Option Injection Rejection
	_, optErr := mgr.CreateSnapshot(ctx, "wl-01", "--option-flag", false, "")
	if optErr == nil {
		t.Fatalf("expected option flag snapshot name to be rejected, but it succeeded")
	}

	// 4. List Snapshots
	list, err := mgr.ListSnapshots(ctx, "wl-01")
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 snapshot in list, got %d", len(list))
	}

	// 5. Get Snapshot
	fetched, err := mgr.GetSnapshot(ctx, "wl-01", "snap-test-01")
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if fetched.Name != "snap-test-01" {
		t.Errorf("expected 'snap-test-01', got '%s'", fetched.Name)
	}

	// 6. Restore Snapshot
	restored, err := mgr.RestoreSnapshot(ctx, "wl-01", "snap-test-01")
	if err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	if restored.Status != "ACTIVE" {
		t.Errorf("expected restored status ACTIVE, got '%s'", restored.Status)
	}

	// 7. Delete Snapshot
	if err := mgr.DeleteSnapshot(ctx, "wl-01", "snap-test-01"); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	// 8. Verify Deletion
	_, getDeletedErr := mgr.GetSnapshot(ctx, "wl-01", "snap-test-01")
	if getDeletedErr == nil {
		t.Fatalf("expected GetSnapshot to fail after deletion")
	}
}

func TestFileSnapshotStore_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "corrupt.json")

	_ = os.WriteFile(storeFile, []byte("{invalid-json-content"), 0644)

	store := NewFileSnapshotStore(storeFile)
	_, err := store.Load()
	if err == nil {
		t.Fatalf("expected error loading corrupted JSON store file, got nil")
	}
}
