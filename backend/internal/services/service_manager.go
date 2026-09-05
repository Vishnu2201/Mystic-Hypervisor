package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
)

var (
	ErrServiceNotFound = errors.New("service not found")
	ErrServiceConflict = errors.New("service configuration conflict")
	ErrInvalidService  = errors.New("invalid service configuration")
)

// ExposureResolver is an interface to look up exposures when validating service exposure references.
type ExposureResolver interface {
	GetNetworkExposure(ctx context.Context, id string) (*networking.NetworkExposure, error)
}

// ServiceManager manages application service endpoints and their persistence.
type ServiceManager struct {
	mu          sync.RWMutex
	services    map[string]*Service
	store       ServiceStore
	expResolver ExposureResolver
}

// NewServiceManager constructs a ServiceManager with default FileServiceStore.
func NewServiceManager(store ServiceStore, expResolver ExposureResolver) *ServiceManager {
	if store == nil {
		store = NewFileServiceStore("")
	}
	sm := &ServiceManager{
		services:    make(map[string]*Service),
		store:       store,
		expResolver: expResolver,
	}

	if loaded, err := store.Load(); err == nil && loaded != nil {
		sm.services = loaded
		logging.GetLogger().Info("Loaded services from store", "store_path", store.FilePath(), "count", len(sm.services))
	} else if err != nil {
		logging.GetLogger().Error("Failed to load service store", "store_path", store.FilePath(), "error", err)
	}

	return sm
}

// StorePath returns the file path of the underlying service store.
func (sm *ServiceManager) StorePath() string {
	if sm.store != nil {
		return sm.store.FilePath()
	}
	return "in-memory"
}

func (sm *ServiceManager) saveStoreUnlocked() error {
	if sm.store != nil {
		if err := sm.store.Save(sm.services); err != nil {
			logging.GetLogger().Error("Failed to persist service store", "store_path", sm.StorePath(), "error", err)
			return fmt.Errorf("failed to persist service store: %w", err)
		}
	}
	return nil
}

// CreateService validates and persists a new Service entity.
func (sm *ServiceManager) CreateService(ctx context.Context, svc *Service) (*Service, error) {
	if svc == nil {
		return nil, fmt.Errorf("service configuration cannot be nil: %w", ErrInvalidService)
	}

	if err := sm.validateServiceConfig(ctx, svc); err != nil {
		return nil, err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate ID if omitted
	if svc.ID == "" {
		svc.ID = fmt.Sprintf("svc-%s-%s-%d", svc.WorkloadID, strings.ToLower(string(svc.Type)), svc.InternalPort)
	}

	if _, exists := sm.services[svc.ID]; exists {
		return nil, fmt.Errorf("service '%s' already exists: %w", svc.ID, ErrServiceConflict)
	}

	if svc.DesiredState == "" {
		svc.DesiredState = "active"
	}
	svc.ActualState = "active"
	svc.SyncStatus = instances.SyncInSync
	svc.CreatedAt = time.Now()
	svc.UpdatedAt = time.Now()

	sm.services[svc.ID] = svc
	if err := sm.saveStoreUnlocked(); err != nil {
		delete(sm.services, svc.ID)
		return nil, err
	}

	return svc, nil
}

// GetService retrieves a service by ID.
func (sm *ServiceManager) GetService(ctx context.Context, id string) (*Service, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	svc, exists := sm.services[id]
	if !exists {
		return nil, fmt.Errorf("service '%s' not found: %w", id, ErrServiceNotFound)
	}
	return svc, nil
}

// ListServices lists all managed services.
func (sm *ServiceManager) ListServices(ctx context.Context) ([]Service, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	res := make([]Service, 0, len(sm.services))
	for _, svc := range sm.services {
		res = append(res, *svc)
	}
	return res, nil
}

// ListWorkloadServices lists services associated with a specific workload.
func (sm *ServiceManager) ListWorkloadServices(ctx context.Context, workloadID string) ([]Service, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	res := make([]Service, 0)
	for _, svc := range sm.services {
		if svc.WorkloadID == workloadID {
			res = append(res, *svc)
		}
	}
	return res, nil
}

// UpdateService updates an existing Service configuration.
func (sm *ServiceManager) UpdateService(ctx context.Context, id string, updated *Service) (*Service, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	existing, exists := sm.services[id]
	if !exists {
		return nil, fmt.Errorf("service '%s' not found: %w", id, ErrServiceNotFound)
	}

	if updated.Name != "" {
		existing.Name = updated.Name
	}
	if updated.Type != "" {
		existing.Type = updated.Type
	}
	if updated.InternalIP != "" {
		existing.InternalIP = updated.InternalIP
	}
	if updated.InternalPort > 0 {
		existing.InternalPort = updated.InternalPort
	}
	if updated.Protocol != "" {
		existing.Protocol = updated.Protocol
	}
	if updated.ExposureID != "" {
		existing.ExposureID = updated.ExposureID
	}
	if updated.Description != "" {
		existing.Description = updated.Description
	}
	existing.IsPublic = updated.IsPublic

	if err := sm.validateServiceConfig(ctx, existing); err != nil {
		return nil, err
	}

	existing.UpdatedAt = time.Now()
	if err := sm.saveStoreUnlocked(); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeleteService deletes a service by ID.
func (sm *ServiceManager) DeleteService(ctx context.Context, id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.services[id]; !exists {
		return fmt.Errorf("service '%s' not found: %w", id, ErrServiceNotFound)
	}

	delete(sm.services, id)
	return sm.saveStoreUnlocked()
}

func (sm *ServiceManager) validateServiceConfig(ctx context.Context, svc *Service) error {
	if strings.TrimSpace(svc.WorkloadID) == "" {
		return fmt.Errorf("workload_id is required: %w", ErrInvalidService)
	}

	if strings.TrimSpace(svc.Name) == "" {
		return fmt.Errorf("service name is required: %w", ErrInvalidService)
	}

	switch svc.Type {
	case ServiceTypeSSH, ServiceTypeHTTP, ServiceTypeHTTPS, ServiceTypeTCP, ServiceTypeUDP, ServiceTypeConsole:
		// valid
	default:
		return fmt.Errorf("unsupported service type '%s': %w", svc.Type, ErrInvalidService)
	}

	if svc.Type != ServiceTypeConsole {
		if svc.InternalPort < 1 || svc.InternalPort > 65535 {
			return fmt.Errorf("internal_port %d must be between 1 and 65535: %w", svc.InternalPort, ErrInvalidService)
		}
	}

	if svc.InternalIP != "" && net.ParseIP(svc.InternalIP) == nil {
		return fmt.Errorf("invalid internal_ip address '%s': %w", svc.InternalIP, ErrInvalidService)
	}

	// Protocol defaults & compatibility
	switch svc.Type {
	case ServiceTypeSSH, ServiceTypeHTTP, ServiceTypeHTTPS, ServiceTypeTCP:
		if svc.Protocol == "" {
			svc.Protocol = hosts.ProtocolTCP
		} else if svc.Protocol != hosts.ProtocolTCP && svc.Protocol != hosts.ProtocolTCPUDP {
			return fmt.Errorf("service type %s requires TCP protocol, got '%s': %w", svc.Type, svc.Protocol, ErrInvalidService)
		}
	case ServiceTypeUDP:
		if svc.Protocol == "" {
			svc.Protocol = hosts.ProtocolUDP
		} else if svc.Protocol != hosts.ProtocolUDP && svc.Protocol != hosts.ProtocolTCPUDP {
			return fmt.Errorf("service type UDP requires UDP protocol, got '%s': %w", svc.Protocol, ErrInvalidService)
		}
	case ServiceTypeConsole:
		if svc.Protocol == "" {
			svc.Protocol = hosts.ProtocolTCP
		}
	}

	// Exposure reference validation
	if svc.ExposureID != "" && sm.expResolver != nil {
		exp, err := sm.expResolver.GetNetworkExposure(ctx, svc.ExposureID)
		if err != nil {
			return fmt.Errorf("referenced exposure '%s' does not exist: %w", svc.ExposureID, ErrInvalidService)
		}
		if exp.WorkloadID != svc.WorkloadID {
			return fmt.Errorf("referenced exposure '%s' belongs to workload '%s', but service belongs to workload '%s': %w", svc.ExposureID, exp.WorkloadID, svc.WorkloadID, ErrInvalidService)
		}
		if svc.Type != ServiceTypeConsole && exp.InternalPort > 0 && exp.InternalPort != svc.InternalPort {
			return fmt.Errorf("exposed internal port (%d) does not match service internal port (%d): %w", exp.InternalPort, svc.InternalPort, ErrInvalidService)
		}
		svc.IsPublic = true
	}

	return nil
}
