package connections

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
	"github.com/mystic-hypervisor/mystic/backend/internal/services"
)

var (
	ErrConnectionNotFound = errors.New("connection profile not found")
	ErrConnectionConflict = errors.New("connection profile configuration conflict")
	ErrInvalidConnection  = errors.New("invalid connection profile configuration")
)

// ServiceResolver is an interface to look up services when validating connection profiles.
type ServiceResolver interface {
	GetService(ctx context.Context, id string) (*services.Service, error)
}

// ExposureResolver is an interface to look up network exposures.
type ExposureResolver interface {
	GetNetworkExposure(ctx context.Context, id string) (*networking.NetworkExposure, error)
}

// ConnectionManager manages connection profiles and deterministic connection URL/CLI generation.
type ConnectionManager struct {
	mu          sync.RWMutex
	profiles    map[string]*ConnectionProfile
	store       ConnectionStore
	svcResolver ServiceResolver
	expResolver ExposureResolver
}

// NewConnectionManager constructs a ConnectionManager with default FileConnectionStore.
func NewConnectionManager(store ConnectionStore, svcResolver ServiceResolver, expResolver ExposureResolver) *ConnectionManager {
	if store == nil {
		store = NewFileConnectionStore("")
	}
	cm := &ConnectionManager{
		profiles:    make(map[string]*ConnectionProfile),
		store:       store,
		svcResolver: svcResolver,
		expResolver: expResolver,
	}

	if loaded, err := store.Load(); err == nil && loaded != nil {
		cm.profiles = loaded
		logging.GetLogger().Info("Loaded connection profiles from store", "store_path", store.FilePath(), "count", len(cm.profiles))
	} else if err != nil {
		logging.GetLogger().Error("Failed to load connection store", "store_path", store.FilePath(), "error", err)
	}

	return cm
}

// StorePath returns the file path of the underlying connection store.
func (cm *ConnectionManager) StorePath() string {
	if cm.store != nil {
		return cm.store.FilePath()
	}
	return "in-memory"
}

func (cm *ConnectionManager) saveStoreUnlocked() error {
	if cm.store != nil {
		if err := cm.store.Save(cm.profiles); err != nil {
			logging.GetLogger().Error("Failed to persist connection store", "store_path", cm.StorePath(), "error", err)
			return fmt.Errorf("failed to persist connection store: %w", err)
		}
	}
	return nil
}

// GenerateConnectionProfile deterministically generates a ConnectionProfile for a given Service and Exposure.
func (cm *ConnectionManager) GenerateConnectionProfile(svc *services.Service, exp *networking.NetworkExposure, targetUser string, credentialID string) (*ConnectionProfile, error) {
	if svc == nil {
		return nil, fmt.Errorf("service cannot be nil: %w", ErrInvalidConnection)
	}

	// Endpoint Resolution Rule
	host := ""
	port := 0

	if svc.ExposureID != "" && exp != nil && exp.PublicPort > 0 {
		host = exp.PublicIP
		if host == "" {
			host = svc.InternalIP
		}
		port = exp.PublicPort
	} else {
		host = svc.InternalIP
		port = svc.InternalPort
	}

	if svc.Type != services.ServiceTypeConsole {
		if strings.TrimSpace(host) == "" {
			return nil, fmt.Errorf("endpoint host cannot be empty for service '%s': %w", svc.ID, ErrInvalidConnection)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("endpoint port %d must be between 1 and 65535: %w", port, ErrInvalidConnection)
		}
	}

	if svc.Type == services.ServiceTypeSSH {
		if strings.TrimSpace(targetUser) == "" {
			return nil, fmt.Errorf("target_user is required for SSH connection profiles: %w", ErrInvalidConnection)
		}
	}

	connURL := ""
	cliCmd := ""
	label := fmt.Sprintf("%s (%s)", svc.Name, svc.Type)

	switch svc.Type {
	case services.ServiceTypeSSH:
		if port == 22 {
			connURL = fmt.Sprintf("ssh://%s@%s", targetUser, host)
			cliCmd = fmt.Sprintf("ssh %s@%s", targetUser, host)
		} else {
			connURL = fmt.Sprintf("ssh://%s@%s:%d", targetUser, host, port)
			cliCmd = fmt.Sprintf("ssh -p %d %s@%s", port, targetUser, host)
		}
	case services.ServiceTypeHTTP:
		if port == 80 {
			connURL = fmt.Sprintf("http://%s", host)
			cliCmd = fmt.Sprintf("curl -v http://%s", host)
		} else {
			connURL = fmt.Sprintf("http://%s:%d", host, port)
			cliCmd = fmt.Sprintf("curl -v http://%s:%d", host, port)
		}
	case services.ServiceTypeHTTPS:
		if port == 443 {
			connURL = fmt.Sprintf("https://%s", host)
			cliCmd = fmt.Sprintf("curl -v https://%s", host)
		} else {
			connURL = fmt.Sprintf("https://%s:%d", host, port)
			cliCmd = fmt.Sprintf("curl -v https://%s:%d", host, port)
		}
	case services.ServiceTypeTCP:
		connURL = fmt.Sprintf("tcp://%s:%d", host, port)
		cliCmd = fmt.Sprintf("nc -zv %s %d", host, port)
	case services.ServiceTypeUDP:
		connURL = fmt.Sprintf("udp://%s:%d", host, port)
		cliCmd = fmt.Sprintf("nc -zuv %s %d", host, port)
	case services.ServiceTypeConsole:
		connURL = ""
		targetName := svc.WorkloadName
		if targetName == "" {
			targetName = svc.WorkloadID
		}
		cliCmd = fmt.Sprintf("incus console %s", targetName)
	default:
		return nil, fmt.Errorf("unsupported service type '%s' for connection generation: %w", svc.Type, ErrInvalidConnection)
	}

	profile := &ConnectionProfile{
		ID:            fmt.Sprintf("conn-%s-%s", svc.ID, strings.ToLower(string(svc.Type))),
		ServiceID:     svc.ID,
		WorkloadID:    svc.WorkloadID,
		Label:         label,
		Protocol:      string(svc.Type),
		EndpointHost:  host,
		EndpointPort:  port,
		TargetUser:    targetUser,
		CredentialID:  credentialID,
		ConnectionURL: connURL,
		CLICommand:    cliCmd,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	return profile, nil
}

// CreateConnectionProfile validates and persists a new ConnectionProfile entity.
func (cm *ConnectionManager) CreateConnectionProfile(ctx context.Context, profile *ConnectionProfile) (*ConnectionProfile, error) {
	if profile == nil {
		return nil, fmt.Errorf("connection profile cannot be nil: %w", ErrInvalidConnection)
	}

	if profile.ServiceID == "" {
		return nil, fmt.Errorf("service_id is required: %w", ErrInvalidConnection)
	}

	if profile.WorkloadID == "" {
		return nil, fmt.Errorf("workload_id is required: %w", ErrInvalidConnection)
	}

	if strings.TrimSpace(profile.EndpointHost) == "" {
		return nil, fmt.Errorf("endpoint_host is required: %w", ErrInvalidConnection)
	}

	if profile.Protocol == string(services.ServiceTypeSSH) && strings.TrimSpace(profile.TargetUser) == "" {
		return nil, fmt.Errorf("target_user is required for SSH connection profiles: %w", ErrInvalidConnection)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if profile.ID == "" {
		profile.ID = fmt.Sprintf("conn-%s-%d", profile.ServiceID, time.Now().UnixNano())
	}

	if _, exists := cm.profiles[profile.ID]; exists {
		return nil, fmt.Errorf("connection profile '%s' already exists: %w", profile.ID, ErrConnectionConflict)
	}

	profile.CreatedAt = time.Now()
	profile.UpdatedAt = time.Now()

	cm.profiles[profile.ID] = profile
	if err := cm.saveStoreUnlocked(); err != nil {
		delete(cm.profiles, profile.ID)
		return nil, err
	}

	return profile, nil
}

// GetConnectionProfile retrieves a connection profile by ID.
func (cm *ConnectionManager) GetConnectionProfile(ctx context.Context, id string) (*ConnectionProfile, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	profile, exists := cm.profiles[id]
	if !exists {
		return nil, fmt.Errorf("connection profile '%s' not found: %w", id, ErrConnectionNotFound)
	}
	return profile, nil
}

// ListConnectionProfiles lists all managed connection profiles.
func (cm *ConnectionManager) ListConnectionProfiles(ctx context.Context) ([]ConnectionProfile, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	res := make([]ConnectionProfile, 0, len(cm.profiles))
	for _, p := range cm.profiles {
		res = append(res, *p)
	}
	return res, nil
}

// ListServiceConnections lists connection profiles for a specific service.
func (cm *ConnectionManager) ListServiceConnections(ctx context.Context, serviceID string) ([]ConnectionProfile, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	res := make([]ConnectionProfile, 0)
	for _, p := range cm.profiles {
		if p.ServiceID == serviceID {
			res = append(res, *p)
		}
	}
	return res, nil
}

// UpdateConnectionProfile updates an existing ConnectionProfile.
func (cm *ConnectionManager) UpdateConnectionProfile(ctx context.Context, id string, updated *ConnectionProfile) (*ConnectionProfile, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	existing, exists := cm.profiles[id]
	if !exists {
		return nil, fmt.Errorf("connection profile '%s' not found: %w", id, ErrConnectionNotFound)
	}

	if updated.Label != "" {
		existing.Label = updated.Label
	}
	if updated.TargetUser != "" {
		existing.TargetUser = updated.TargetUser
	}
	if updated.CredentialID != "" {
		existing.CredentialID = updated.CredentialID
	}
	if updated.ConnectionURL != "" {
		existing.ConnectionURL = updated.ConnectionURL
	}
	if updated.CLICommand != "" {
		existing.CLICommand = updated.CLICommand
	}

	existing.UpdatedAt = time.Now()
	if err := cm.saveStoreUnlocked(); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeleteConnectionProfile deletes a connection profile by ID.
func (cm *ConnectionManager) DeleteConnectionProfile(ctx context.Context, id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.profiles[id]; !exists {
		return fmt.Errorf("connection profile '%s' not found: %w", id, ErrConnectionNotFound)
	}

	delete(cm.profiles, id)
	return cm.saveStoreUnlocked()
}
