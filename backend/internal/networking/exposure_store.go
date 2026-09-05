package networking

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ExposureStore defines the persistent storage interface for network exposures.
type ExposureStore interface {
	Save(exposures map[string]*NetworkExposure) error
	Load() (map[string]*NetworkExposure, error)
	FilePath() string
}

// FileExposureStore implements ExposureStore using a local JSON file with atomic writes.
type FileExposureStore struct {
	mu       sync.Mutex
	filePath string
}

// NewFileExposureStore constructs a FileExposureStore at the given file path.
func NewFileExposureStore(filePath string) *FileExposureStore {
	if filePath == "" {
		if envPath := os.Getenv("MYSTIC_EXPOSURE_STORE_PATH"); envPath != "" {
			filePath = envPath
		} else {
			filePath = "/var/lib/mystic/network_exposures.json"
		}
	}
	return &FileExposureStore{filePath: filePath}
}

// FilePath returns the configured store file path.
func (fs *FileExposureStore) FilePath() string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.filePath
}

// Save atomically writes network exposures to disk.
func (fs *FileExposureStore) Save(exposures map[string]*NetworkExposure) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create exposure store directory '%s': %w", dir, err)
	}

	data, err := json.MarshalIndent(exposures, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal exposures to JSON: %w", err)
	}

	tmpFile := fs.filePath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary exposure store file '%s': %w", tmpFile, err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write data to temporary exposure store file '%s': %w", tmpFile, err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync temporary exposure store file '%s': %w", tmpFile, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close temporary exposure store file '%s': %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit exposure store file from '%s' to '%s': %w", tmpFile, fs.filePath, err)
	}

	return nil
}

// Load reads persisted network exposures from disk.
func (fs *FileExposureStore) Load() (map[string]*NetworkExposure, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	exposures := make(map[string]*NetworkExposure)

	stat, err := os.Stat(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return exposures, nil
		}
		return nil, fmt.Errorf("failed to stat exposure store file '%s': %w", fs.filePath, err)
	}

	if stat.Size() == 0 {
		return exposures, nil
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read exposure store file '%s': %w", fs.filePath, err)
	}

	if err := json.Unmarshal(data, &exposures); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exposure store JSON from '%s': %w", fs.filePath, err)
	}

	return exposures, nil
}
