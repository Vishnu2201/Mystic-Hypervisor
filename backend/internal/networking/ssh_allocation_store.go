package networking

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SSHAllocationStore defines the persistent storage interface for public SSH allocations.
type SSHAllocationStore interface {
	Save(allocations map[string]*SSHAllocation) error
	Load() (map[string]*SSHAllocation, error)
	FilePath() string
}

// FileSSHAllocationStore implements SSHAllocationStore using a local JSON file with atomic writes.
type FileSSHAllocationStore struct {
	mu       sync.Mutex
	filePath string
}

// NewFileSSHAllocationStore constructs a FileSSHAllocationStore at the given file path.
func NewFileSSHAllocationStore(filePath string) *FileSSHAllocationStore {
	if filePath == "" {
		if envPath := os.Getenv("MYSTIC_SSH_ALLOCATION_STORE_PATH"); envPath != "" {
			filePath = envPath
		} else {
			filePath = "/var/lib/mystic/ssh_allocations.json"
		}
	}
	return &FileSSHAllocationStore{filePath: filePath}
}

// FilePath returns the configured store file path.
func (fs *FileSSHAllocationStore) FilePath() string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.filePath
}

// Save atomically writes SSH allocations to disk.
func (fs *FileSSHAllocationStore) Save(allocations map[string]*SSHAllocation) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create SSH allocation store directory '%s': %w", dir, err)
	}

	data, err := json.MarshalIndent(allocations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SSH allocations to JSON: %w", err)
	}

	tmpFile := fs.filePath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary SSH allocation store file '%s': %w", tmpFile, err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write data to temporary SSH allocation store file '%s': %w", tmpFile, err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync temporary SSH allocation store file '%s': %w", tmpFile, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close temporary SSH allocation store file '%s': %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit SSH allocation store file from '%s' to '%s': %w", tmpFile, fs.filePath, err)
	}

	return nil
}

// Load reads persisted SSH allocations from disk.
func (fs *FileSSHAllocationStore) Load() (map[string]*SSHAllocation, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	allocations := make(map[string]*SSHAllocation)

	stat, err := os.Stat(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return allocations, nil
		}
		return nil, fmt.Errorf("failed to stat SSH allocation store file '%s': %w", fs.filePath, err)
	}

	if stat.Size() == 0 {
		return allocations, nil
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH allocation store file '%s': %w", fs.filePath, err)
	}

	if err := json.Unmarshal(data, &allocations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SSH allocation store JSON from '%s': %w", fs.filePath, err)
	}

	return allocations, nil
}
