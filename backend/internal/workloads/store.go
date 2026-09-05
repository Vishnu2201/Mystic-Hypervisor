package workloads

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store defines the persistent storage interface for managed workloads.
type Store interface {
	Save(workloads map[string]*Workload) error
	Load() (map[string]*Workload, error)
}

// FileStore implements Store using a local JSON file with atomic writes.
type FileStore struct {
	mu       sync.Mutex
	filePath string
}

// NewFileStore constructs a FileStore at the given file path.
func NewFileStore(filePath string) *FileStore {
	if filePath == "" {
		filePath = "/var/lib/mystic/workloads.json"
	}
	return &FileStore{filePath: filePath}
}

// Save atomically writes workloads to disk.
func (fs *FileStore) Save(workloads map[string]*Workload) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create workload store directory: %w", err)
	}

	data, err := json.MarshalIndent(workloads, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workloads: %w", err)
	}

	tmpFile := fs.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary workload store file: %w", err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit workload store file: %w", err)
	}

	return nil
}

// Load reads persisted workloads from disk.
func (fs *FileStore) Load() (map[string]*Workload, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	workloads := make(map[string]*Workload)

	if _, err := os.Stat(fs.filePath); os.IsNotExist(err) {
		return workloads, nil
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workload store file: %w", err)
	}

	if len(data) == 0 {
		return workloads, nil
	}

	if err := json.Unmarshal(data, &workloads); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workload store JSON: %w", err)
	}

	return workloads, nil
}
