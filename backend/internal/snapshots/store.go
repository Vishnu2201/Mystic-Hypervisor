package snapshots

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SnapshotStore defines the persistent storage interface for workload snapshots.
type SnapshotStore interface {
	Save(snapshots map[string]*Snapshot) error
	Load() (map[string]*Snapshot, error)
	FilePath() string
}

// FileSnapshotStore implements SnapshotStore using a local JSON file with atomic writes.
type FileSnapshotStore struct {
	mu       sync.Mutex
	filePath string
}

// NewFileSnapshotStore constructs a FileSnapshotStore at the given file path.
func NewFileSnapshotStore(filePath string) *FileSnapshotStore {
	if filePath == "" {
		if envPath := os.Getenv("MYSTIC_SNAPSHOT_STORE_PATH"); envPath != "" {
			filePath = envPath
		} else {
			filePath = "/var/lib/mystic/snapshots.json"
		}
	}
	return &FileSnapshotStore{filePath: filePath}
}

// FilePath returns the configured store file path.
func (fs *FileSnapshotStore) FilePath() string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.filePath
}

// Save atomically writes snapshots to disk.
func (fs *FileSnapshotStore) Save(snapshots map[string]*Snapshot) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot store directory '%s': %w", dir, err)
	}

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshots to JSON: %w", err)
	}

	tmpFile := fs.filePath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary snapshot store file '%s': %w", tmpFile, err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write data to temporary snapshot store file '%s': %w", tmpFile, err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync temporary snapshot store file '%s': %w", tmpFile, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close temporary snapshot store file '%s': %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit snapshot store file from '%s' to '%s': %w", tmpFile, fs.filePath, err)
	}

	return nil
}

// Load reads persisted snapshots from disk.
func (fs *FileSnapshotStore) Load() (map[string]*Snapshot, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	snapshots := make(map[string]*Snapshot)

	stat, err := os.Stat(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshots, nil
		}
		return nil, fmt.Errorf("failed to stat snapshot store file '%s': %w", fs.filePath, err)
	}

	if stat.Size() == 0 {
		return snapshots, nil
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot store file '%s': %w", fs.filePath, err)
	}

	if err := json.Unmarshal(data, &snapshots); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot store JSON from '%s': %w", fs.filePath, err)
	}

	return snapshots, nil
}
