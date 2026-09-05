package networking

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

var (
	ErrNoSSHPortsAvailable   = errors.New("No public SSH ports are currently available.")
	ErrSSHAllocationNotFound = errors.New("SSH allocation not found")
	ErrSSHAllocationConflict = errors.New("SSH allocation conflict")
)

// SSHPortAllocator manages thread-safe, persistent public TCP SSH port allocation for VPS workloads.
type SSHPortAllocator struct {
	mu          sync.RWMutex
	store       SSHAllocationStore
	allocations map[string]*SSHAllocation // Key: WorkloadID
	portMap     map[int]*SSHAllocation    // Key: PublicPort
	exposureMgr *ExposureManager
}

// NewSSHPortAllocator constructs an SSHPortAllocator.
func NewSSHPortAllocator(store SSHAllocationStore, exposureMgr *ExposureManager) *SSHPortAllocator {
	if store == nil {
		store = NewFileSSHAllocationStore("")
	}

	alloc := &SSHPortAllocator{
		store:       store,
		allocations: make(map[string]*SSHAllocation),
		portMap:     make(map[int]*SSHAllocation),
		exposureMgr: exposureMgr,
	}

	if loaded, err := store.Load(); err == nil && loaded != nil {
		for _, a := range loaded {
			alloc.allocations[a.WorkloadID] = a
			if a.Status != SSHStatusReleased && a.PublicPort >= SSHPortMin && a.PublicPort <= SSHPortMax {
				alloc.portMap[a.PublicPort] = a
			}
		}
		logging.GetLogger().Info("Loaded SSH port allocations from store",
			"store_path", store.FilePath(),
			"count", len(alloc.allocations),
		)
	}

	return alloc
}

// StorePath returns the file path of the underlying allocation store.
func (a *SSHPortAllocator) StorePath() string {
	if a.store != nil {
		return a.store.FilePath()
	}
	return "in-memory"
}

func (a *SSHPortAllocator) saveStoreUnlocked() error {
	if a.store != nil {
		if err := a.store.Save(a.allocations); err != nil {
			logging.GetLogger().Error("Failed to persist SSH allocation store",
				"store_path", a.StorePath(),
				"error", err,
			)
			return fmt.Errorf("failed to persist SSH allocation store: %w", err)
		}
	}
	return nil
}

// RegisterExternalPortReservation registers an existing infrastructure port (e.g. Jesko on 22100) as occupied.
func (a *SSHPortAllocator) RegisterExternalPortReservation(workloadID, instanceName string, port int, privateIP string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if port < SSHPortMin || port > SSHPortMax {
		return
	}

	now := time.Now()
	id := fmt.Sprintf("ssh-alloc-%d", port)
	alloc := &SSHAllocation{
		ID:               id,
		PublicPort:       port,
		Protocol:         "TCP",
		Purpose:          "SSH",
		WorkloadID:       workloadID,
		InstanceID:       instanceName,
		InstanceName:     instanceName,
		IncusHostID:      "host-local",
		IncusHostAddress: GetIncusHostAddress(),
		PublicSSHHost:    GetPublicSSHHost(),
		PrivateIP:        privateIP,
		DestinationPort:  22,
		Status:           SSHStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	a.allocations[workloadID] = alloc
	a.portMap[port] = alloc
	_ = a.saveStoreUnlocked()

	logging.GetLogger().Info("Registered external/reconciled SSH port reservation",
		"workload_id", workloadID,
		"instance_name", instanceName,
		"public_port", port,
		"private_ip", privateIP,
	)
}

// AllocatePort reserves the lowest available public TCP SSH port in range 22100-22200 for a workload.
func (a *SSHPortAllocator) AllocatePort(ctx context.Context, workloadID, instanceName string) (*SSHAllocation, error) {
	workloadID = strings.TrimSpace(workloadID)
	instanceName = strings.TrimSpace(instanceName)
	if workloadID == "" {
		return nil, fmt.Errorf("workload ID is required for SSH allocation")
	}
	if instanceName == "" {
		instanceName = workloadID
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if already allocated for this workload
	if existing, exists := a.allocations[workloadID]; exists && existing.Status != SSHStatusReleased {
		return existing, nil
	}

	// Find lowest available port from 22100 to 22200 inclusive
	allocatedPort := 0
	for port := SSHPortMin; port <= SSHPortMax; port++ {
		if _, occupied := a.portMap[port]; !occupied {
			// Also check ExposureManager rules if attached
			if a.exposureMgr != nil {
				if a.exposureMgr.IsPublicPortInUse(port) {
					continue
				}
			}
			allocatedPort = port
			break
		}
	}

	if allocatedPort == 0 {
		return nil, ErrNoSSHPortsAvailable
	}

	now := time.Now()
	id := fmt.Sprintf("ssh-alloc-%d", allocatedPort)
	alloc := &SSHAllocation{
		ID:               id,
		PublicPort:       allocatedPort,
		Protocol:         "TCP",
		Purpose:          "SSH",
		WorkloadID:       workloadID,
		InstanceID:       instanceName,
		InstanceName:     instanceName,
		IncusHostID:      "host-local",
		IncusHostAddress: GetIncusHostAddress(),
		PublicSSHHost:    GetPublicSSHHost(),
		DestinationPort:  22,
		Status:           SSHStatusAllocated,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	a.allocations[workloadID] = alloc
	a.portMap[allocatedPort] = alloc

	if err := a.saveStoreUnlocked(); err != nil {
		delete(a.allocations, workloadID)
		delete(a.portMap, allocatedPort)
		return nil, err
	}

	logging.GetLogger().Info("Allocated public TCP SSH port for workload",
		"workload_id", workloadID,
		"instance_name", instanceName,
		"public_port", allocatedPort,
	)

	return alloc, nil
}

// GetAllocation returns the SSH allocation for a given workload ID.
func (a *SSHPortAllocator) GetAllocation(ctx context.Context, workloadID string) (*SSHAllocation, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	alloc, exists := a.allocations[workloadID]
	if !exists || alloc.Status == SSHStatusReleased {
		return nil, ErrSSHAllocationNotFound
	}

	cp := *alloc
	return &cp, nil
}

// GetAllocationByPort returns the SSH allocation for a specific public port.
func (a *SSHPortAllocator) GetAllocationByPort(ctx context.Context, port int) (*SSHAllocation, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	alloc, exists := a.portMap[port]
	if !exists || alloc.Status == SSHStatusReleased {
		return nil, ErrSSHAllocationNotFound
	}

	cp := *alloc
	return &cp, nil
}

// ListAllocations returns all active SSH allocations.
func (a *SSHPortAllocator) ListAllocations(ctx context.Context) ([]*SSHAllocation, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]*SSHAllocation, 0, len(a.allocations))
	for _, alloc := range a.allocations {
		if alloc.Status != SSHStatusReleased {
			cp := *alloc
			result = append(result, &cp)
		}
	}

	return result, nil
}

// UpdateAllocationStatus updates status and private IP for an active allocation.
func (a *SSHPortAllocator) UpdateAllocationStatus(ctx context.Context, workloadID string, status SSHAllocationStatus, privateIP string) (*SSHAllocation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	alloc, exists := a.allocations[workloadID]
	if !exists {
		return nil, ErrSSHAllocationNotFound
	}

	alloc.Status = status
	if privateIP != "" {
		alloc.PrivateIP = privateIP
	}
	alloc.UpdatedAt = time.Now()

	if err := a.saveStoreUnlocked(); err != nil {
		return nil, err
	}

	cp := *alloc
	return &cp, nil
}

// ReleasePort releases a reserved public SSH port so it can be safely reused.
func (a *SSHPortAllocator) ReleasePort(ctx context.Context, workloadID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	alloc, exists := a.allocations[workloadID]
	if !exists {
		return nil
	}

	now := time.Now()
	alloc.Status = SSHStatusReleased
	alloc.ReleasedAt = &now
	alloc.UpdatedAt = now

	delete(a.portMap, alloc.PublicPort)

	if err := a.saveStoreUnlocked(); err != nil {
		logging.GetLogger().Error("Failed to persist store after SSH port release",
			"workload_id", workloadID,
			"public_port", alloc.PublicPort,
			"error", err,
		)
	}

	logging.GetLogger().Info("Released public TCP SSH port",
		"workload_id", workloadID,
		"public_port", alloc.PublicPort,
	)

	return nil
}

// ReconcileJeskoAndExistingInfrastructure scans existing exposures or raw provider details
// to detect jesko (or other instances) on port 22100 / SSH proxy ports, marking them occupied.
func (a *SSHPortAllocator) ReconcileJeskoAndExistingInfrastructure(ctx context.Context, exposures map[string]*NetworkExposure) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, exp := range exposures {
		if exp.PublicPort >= SSHPortMin && exp.PublicPort <= SSHPortMax {
			workloadID := exp.WorkloadID
			if workloadID == "" {
				workloadID = exp.WorkloadName
			}
			if workloadID == "" {
				workloadID = fmt.Sprintf("workload-port-%d", exp.PublicPort)
			}

			// Register allocation if missing
			if _, exists := a.portMap[exp.PublicPort]; !exists {
				now := time.Now()
				alloc := &SSHAllocation{
					ID:               fmt.Sprintf("ssh-alloc-%d", exp.PublicPort),
					PublicPort:       exp.PublicPort,
					Protocol:         "TCP",
					Purpose:          "SSH",
					WorkloadID:       workloadID,
					InstanceID:       exp.WorkloadName,
					InstanceName:     exp.WorkloadName,
					IncusHostID:      "host-local",
					IncusHostAddress: GetIncusHostAddress(),
					PublicSSHHost:    GetPublicSSHHost(),
					PrivateIP:        exp.InternalIP,
					DestinationPort:  exp.InternalPort,
					Status:           SSHStatusActive,
					CreatedAt:        now,
					UpdatedAt:        now,
				}
				a.allocations[workloadID] = alloc
				a.portMap[exp.PublicPort] = alloc
				logging.GetLogger().Info("Discovered & reconciled existing SSH proxy allocation",
					"public_port", exp.PublicPort,
					"workload", workloadID,
					"private_ip", exp.InternalIP,
				)
			}
		}
	}
	_ = a.saveStoreUnlocked()
}

// DiscoveredSSHProxyInfo holds parsed details of an Incus SSH proxy device.
type DiscoveredSSHProxyInfo struct {
	DeviceName      string
	PublicPort      int
	PublicIP        string
	PrivateIP       string
	DestinationPort int
	Protocol        string
}

// ParseSSHProxyDevice parses a raw Incus device property map to determine if it represents a public SSH proxy mapping.
func ParseSSHProxyDevice(deviceName string, dev map[string]string) (*DiscoveredSSHProxyInfo, bool) {
	if dev == nil {
		return nil, false
	}
	devType := strings.ToLower(dev["type"])
	if devType != "" && devType != "proxy" {
		return nil, false
	}

	listenStr := strings.TrimSpace(dev["listen"])
	connectStr := strings.TrimSpace(dev["connect"])

	if listenStr == "" || connectStr == "" {
		return nil, false
	}

	// Parse listen string: e.g., "tcp:0.0.0.0:22100", "tcp:100.67.231.100:22100", "tcp::22100"
	listenParts := strings.Split(listenStr, ":")
	if len(listenParts) < 2 {
		return nil, false
	}

	protocol := strings.ToUpper(listenParts[0])
	if protocol != "TCP" {
		return nil, false
	}

	portStr := listenParts[len(listenParts)-1]
	publicPort, err := strconv.Atoi(portStr)
	if err != nil || publicPort < SSHPortMin || publicPort > SSHPortMax {
		return nil, false
	}

	publicIP := "0.0.0.0"
	if len(listenParts) == 3 && listenParts[1] != "" {
		publicIP = listenParts[1]
	}

	// Parse connect string: e.g., "tcp:10.170.92.243:22"
	connectParts := strings.Split(connectStr, ":")
	if len(connectParts) < 2 {
		return nil, false
	}

	destPortStr := connectParts[len(connectParts)-1]
	destPort, err := strconv.Atoi(destPortStr)
	if err != nil || destPort != 22 {
		return nil, false
	}

	privateIP := ""
	if len(connectParts) == 3 {
		privateIP = connectParts[1]
	} else if len(connectParts) == 2 {
		privateIP = connectParts[0]
	}

	return &DiscoveredSSHProxyInfo{
		DeviceName:      deviceName,
		PublicPort:      publicPort,
		PublicIP:        publicIP,
		PrivateIP:       privateIP,
		DestinationPort: destPort,
		Protocol:        protocol,
	}, true
}

// ReconcileFromProviderInstances inspects real provider instances and their devices (including Jesko on 22100),
// automatically reserving matching SSH proxy ports so they are never reallocated.
func (a *SSHPortAllocator) ReconcileFromProviderInstances(ctx context.Context, instances []interfaces.Instance) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, inst := range instances {
		if inst.Devices == nil {
			continue
		}

		workloadID := inst.ID
		if workloadID == "" {
			workloadID = inst.Name
		}

		for devName, devProps := range inst.Devices {
			info, ok := ParseSSHProxyDevice(devName, devProps)
			if !ok {
				continue
			}

			// Register allocation if not already in portMap
			if _, exists := a.portMap[info.PublicPort]; !exists {
				now := time.Now()
				alloc := &SSHAllocation{
					ID:               fmt.Sprintf("ssh-alloc-%d", info.PublicPort),
					PublicPort:       info.PublicPort,
					Protocol:         info.Protocol,
					Purpose:          "SSH",
					WorkloadID:       workloadID,
					InstanceID:       inst.Name,
					InstanceName:     inst.Name,
					IncusHostID:      "host-local",
					IncusHostAddress: GetIncusHostAddress(),
					PublicSSHHost:    GetPublicSSHHost(),
					PrivateIP:        info.PrivateIP,
					DestinationPort:  info.DestinationPort,
					Status:           SSHStatusActive,
					CreatedAt:        now,
					UpdatedAt:        now,
				}
				a.allocations[workloadID] = alloc
				a.portMap[info.PublicPort] = alloc
				logging.GetLogger().Info("Discovered & reconciled existing provider SSH proxy device",
					"instance", inst.Name,
					"device", devName,
					"public_port", info.PublicPort,
					"private_ip", info.PrivateIP,
				)
			}
		}
	}
	_ = a.saveStoreUnlocked()
}

// Helper to check if a port is in use by exposure manager
func (em *ExposureManager) IsPublicPortInUse(port int) bool {
	if em == nil {
		return false
	}
	em.mu.RLock()
	defer em.mu.RUnlock()

	for _, exp := range em.exposures {
		if exp.PublicPort == port {
			return true
		}
	}
	return false
}
