package workloads

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		if envPath := os.Getenv("MYSTIC_WORKLOAD_STORE_PATH"); envPath != "" {
			filePath = envPath
		} else {
			filePath = "/var/lib/mystic/workloads.json"
		}
	}
	return &FileStore{filePath: filePath}
}

// FilePath returns the configured store file path.
func (fs *FileStore) FilePath() string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.filePath
}

// Save atomically writes workloads to disk.
func (fs *FileStore) Save(workloads map[string]*Workload) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create workload store directory '%s': %w", dir, err)
	}

	data, err := json.MarshalIndent(workloads, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workloads to JSON: %w", err)
	}

	tmpFile := fs.filePath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary workload store file '%s': %w", tmpFile, err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write data to temporary workload store file '%s': %w", tmpFile, err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync temporary workload store file '%s': %w", tmpFile, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close temporary workload store file '%s': %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit workload store file from '%s' to '%s': %w", tmpFile, fs.filePath, err)
	}

	return nil
}

// Load reads persisted workloads from disk.
func (fs *FileStore) Load() (map[string]*Workload, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	workloads := make(map[string]*Workload)

	stat, err := os.Stat(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return workloads, nil
		}
		return nil, fmt.Errorf("failed to stat workload store file '%s': %w", fs.filePath, err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("configured workload store path '%s' is a directory, expected a JSON file", fs.filePath)
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workload store file '%s': %w", fs.filePath, err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return workloads, nil
	}

	if err := json.Unmarshal(data, &workloads); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workload store JSON from '%s': %w", fs.filePath, err)
	}

	if workloads == nil {
		workloads = make(map[string]*Workload)
	}

	return workloads, nil
}
