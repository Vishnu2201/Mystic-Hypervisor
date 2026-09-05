package connections

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ConnectionStore defines the persistent storage interface for connection profiles.
type ConnectionStore interface {
	Save(profiles map[string]*ConnectionProfile) error
	Load() (map[string]*ConnectionProfile, error)
	FilePath() string
}

// FileConnectionStore implements ConnectionStore using a local JSON file with atomic writes.
type FileConnectionStore struct {
	mu       sync.Mutex
	filePath string
}

// NewFileConnectionStore constructs a FileConnectionStore at the given file path.
func NewFileConnectionStore(filePath string) *FileConnectionStore {
	if filePath == "" {
		if envPath := os.Getenv("MYSTIC_CONNECTION_STORE_PATH"); envPath != "" {
			filePath = envPath
		} else {
			filePath = "/var/lib/mystic/connection_profiles.json"
		}
	}
	return &FileConnectionStore{filePath: filePath}
}

// FilePath returns the configured store file path.
func (fs *FileConnectionStore) FilePath() string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.filePath
}

// Save atomically writes connection profiles to disk.
func (fs *FileConnectionStore) Save(profiles map[string]*ConnectionProfile) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create connection store directory '%s': %w", dir, err)
	}

	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal connection profiles to JSON: %w", err)
	}

	tmpFile := fs.filePath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary connection store file '%s': %w", tmpFile, err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write data to temporary connection store file '%s': %w", tmpFile, err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync temporary connection store file '%s': %w", tmpFile, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close temporary connection store file '%s': %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit connection store file from '%s' to '%s': %w", tmpFile, fs.filePath, err)
	}

	return nil
}

// Load reads persisted connection profiles from disk.
func (fs *FileConnectionStore) Load() (map[string]*ConnectionProfile, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	profiles := make(map[string]*ConnectionProfile)

	stat, err := os.Stat(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return profiles, nil
		}
		return nil, fmt.Errorf("failed to stat connection store file '%s': %w", fs.filePath, err)
	}

	if stat.Size() == 0 {
		return profiles, nil
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read connection store file '%s': %w", fs.filePath, err)
	}

	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal connection store JSON from '%s': %w", fs.filePath, err)
	}

	return profiles, nil
}
