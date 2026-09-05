package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ServiceStore defines the persistent storage interface for services.
type ServiceStore interface {
	Save(svcs map[string]*Service) error
	Load() (map[string]*Service, error)
	FilePath() string
}

// FileServiceStore implements ServiceStore using a local JSON file with atomic writes.
type FileServiceStore struct {
	mu       sync.Mutex
	filePath string
}

// NewFileServiceStore constructs a FileServiceStore at the given file path.
func NewFileServiceStore(filePath string) *FileServiceStore {
	if filePath == "" {
		if envPath := os.Getenv("MYSTIC_SERVICE_STORE_PATH"); envPath != "" {
			filePath = envPath
		} else {
			filePath = "/var/lib/mystic/services.json"
		}
	}
	return &FileServiceStore{filePath: filePath}
}

// FilePath returns the configured store file path.
func (fs *FileServiceStore) FilePath() string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.filePath
}

// Save atomically writes services to disk.
func (fs *FileServiceStore) Save(svcs map[string]*Service) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create service store directory '%s': %w", dir, err)
	}

	data, err := json.MarshalIndent(svcs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal services to JSON: %w", err)
	}

	tmpFile := fs.filePath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary service store file '%s': %w", tmpFile, err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write data to temporary service store file '%s': %w", tmpFile, err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync temporary service store file '%s': %w", tmpFile, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close temporary service store file '%s': %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit service store file from '%s' to '%s': %w", tmpFile, fs.filePath, err)
	}

	return nil
}

// Load reads persisted services from disk.
func (fs *FileServiceStore) Load() (map[string]*Service, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	svcs := make(map[string]*Service)

	stat, err := os.Stat(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return svcs, nil
		}
		return nil, fmt.Errorf("failed to stat service store file '%s': %w", fs.filePath, err)
	}

	if stat.Size() == 0 {
		return svcs, nil
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service store file '%s': %w", fs.filePath, err)
	}

	if err := json.Unmarshal(data, &svcs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service store JSON from '%s': %w", fs.filePath, err)
	}

	return svcs, nil
}
