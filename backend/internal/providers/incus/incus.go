package incus

import "bytes"
import "context"
import "encoding/json"
import "fmt"
import "os/exec"
import "sort"
import "strconv"
import "strings"
import "time"

import "github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"

// IncusProvider implements the Provider interface for Incus hypervisors.
type IncusProvider struct {
	socketPath string
	caps       interfaces.CapabilitySet
	execRunner func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// NewIncusProvider creates an Incus provider instance.
func NewIncusProvider(socketPath string) *IncusProvider {
	if socketPath == "" {
		socketPath = "/var/lib/incus/unix.socket"
	}
	return &IncusProvider{
		socketPath: socketPath,
		caps: interfaces.NewCapabilitySet(
			interfaces.CapVM,
			interfaces.CapContainer,
			interfaces.CapSnapshots,
			interfaces.CapStoragePools,
			interfaces.CapConsoleStream,
			interfaces.CapExec,
			interfaces.CapResize,
			interfaces.CapCloudInit,
		),
	}
}

// SetExecRunner allows setting a custom execution runner (primarily for unit testing offline).
func (p *IncusProvider) SetExecRunner(runner func(ctx context.Context, name string, args ...string) ([]byte, error)) {
	p.execRunner = runner
}

func (p *IncusProvider) runCmd(ctx context.Context, name string, args ...string) ([]byte, error) {
	if p.execRunner != nil {
		return p.execRunner(ctx, name, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(errOut.String()))
	}
	return out.Bytes(), nil
}

func (p *IncusProvider) Name() string {
	return "incus"
}

func (p *IncusProvider) Capabilities() interfaces.CapabilitySet {
	return p.caps
}

func (p *IncusProvider) Ping(ctx context.Context) error {
	_, err := p.runCmd(ctx, "incus", "version")
	if err != nil {
		return interfaces.ErrProviderUnavailable
	}
	return nil
}

func (p *IncusProvider) Close() error {
	return nil
}

func (p *IncusProvider) InstanceProvider() (interfaces.InstanceProvider, bool) {
	return p, true
}

func (p *IncusProvider) ImageProvider() (interfaces.ImageProvider, bool) {
	return p, true
}

func (p *IncusProvider) SnapshotProvider() (interfaces.SnapshotProvider, bool) {
	return p, true
}

func (p *IncusProvider) StorageProvider() (interfaces.StorageProvider, bool) {
	return p, true
}

func (p *IncusProvider) NetworkProvider() (interfaces.NetworkProvider, bool) {
	return p, true
}

// Raw JSON structures returned by Incus CLI queries
type incusRawServerEnvironment struct {
	Addresses     []string `json:"addresses"`
	Architecture  string   `json:"architecture"`
	Architectures []string `json:"architectures"`
	Driver        string   `json:"driver"`
	KernelVersion string   `json:"kernel_version"`
	OSName        string   `json:"os_name"`
	OSVersion     string   `json:"os_version"`
	ServerVersion string   `json:"server_version"`
	Storage       string   `json:"storage"`
	StorageDriver string   `json:"storage_driver"`
}

type incusRawServerInfo struct {
	Environment incusRawServerEnvironment `json:"environment"`
	Metadata    struct {
		Environment incusRawServerEnvironment `json:"environment"`
	} `json:"metadata"`
}

type incusDevice struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Network string `json:"network"`
	Parent  string `json:"parent"`
	Size    string `json:"size"`
}

type incusRawInstance struct {
	Name            string                 `json:"name"`
	Status          string                 `json:"status"`
	Type            string                 `json:"type"`
	Devices         map[string]incusDevice `json:"devices"`
	ExpandedDevices map[string]incusDevice `json:"expanded_devices"`
	State           struct {
		Status     string `json:"status"`
		StatusCode int    `json:"status_code"`
		Network    map[string]struct {
			Addresses []struct {
				Family  string `json:"family"`
				Address string `json:"address"`
				Scope   string `json:"scope"`
			} `json:"addresses"`
		} `json:"network"`
	} `json:"state"`
	Config  map[string]string `json:"config"`
	Created string            `json:"created_at"`
}

type incusRawNetworkDetail struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Managed bool              `json:"managed"`
	Status  string            `json:"status"`
	Config  map[string]string `json:"config"`
}

type incusRawStorageDetail struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Status string `json:"status"`
}

type incusRawImageDetail struct {
	Fingerprint  string `json:"fingerprint"`
	Architecture string `json:"architecture"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	CreatedAt    string `json:"created_at"`
	Aliases      []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"aliases"`
	Properties map[string]string `json:"properties"`
}

// Preflight performs read-only provider discovery and health status evaluation.
func (p *IncusProvider) Preflight(ctx context.Context) (*interfaces.ProviderPreflightResult, error) {
	res := &interfaces.ProviderPreflightResult{
		Provider:     "incus",
		Availability: interfaces.AvailabilityUnavailable,
		HealthStatus: interfaces.ProviderHealthStatus{
			Installed:   false,
			Reachable:   false,
			Operational: false,
			Capable:     false,
		},
		Capabilities:      p.caps.Slice(),
		ExistingInstances: []interfaces.PreflightInstance{},
		Networks:          []interfaces.DiscoveredNetwork{},
		StoragePools:      []interfaces.DiscoveredStoragePool{},
		Images:            []interfaces.DiscoveredImage{},
		Warnings:          []string{},
		Blockers:          []string{},
	}

	// 1. Installed Check (look in PATH or runner override)
	if p.execRunner == nil {
		if _, lookErr := exec.LookPath("incus"); lookErr != nil {
			res.Blockers = append(res.Blockers, "Incus CLI binary 'incus' not found in system PATH")
			return res, nil
		}
	}
	res.HealthStatus.Installed = true

	// 2. Reachable Check (incus query /1.0 API endpoint)
	infoBytes, err := p.runCmd(ctx, "incus", "query", "/1.0")
	if err != nil {
		res.Blockers = append(res.Blockers, fmt.Sprintf("Incus daemon is unreachable or socket error: %v", err))
		return res, nil
	}
	res.HealthStatus.Reachable = true

	var rawInfo incusRawServerInfo
	if err := json.Unmarshal(infoBytes, &rawInfo); err == nil {
		env := rawInfo.Environment
		if env.ServerVersion == "" && env.OSName == "" {
			env = rawInfo.Metadata.Environment
		}
		osStr := strings.TrimSpace(fmt.Sprintf("%s %s", env.OSName, env.OSVersion))
		archStr := env.Architecture
		if archStr == "" && len(env.Architectures) > 0 {
			archStr = env.Architectures[0]
		}
		res.ServerInfo = interfaces.ProviderServerInfo{
			ServerVersion: env.ServerVersion,
			OS:            osStr,
			Kernel:        env.KernelVersion,
			Architecture:  archStr,
			KVMSupported:  env.Driver == "qemu" || env.Driver == "kvm",
		}
	}

	// 3. Existing Instances Discovery
	listBytes, err := p.runCmd(ctx, "incus", "list", "--format", "json")
	if err == nil {
		var rawInsts []incusRawInstance
		if json.Unmarshal(listBytes, &rawInsts) == nil {
			for _, item := range rawInsts {
				ownership := interfaces.OwnershipExternal
				if item.Config["user.mystic.owned"] == "true" || item.Config["mystic.owned"] == "true" {
					ownership = interfaces.OwnershipMysticOwned
				}
				res.ExistingInstances = append(res.ExistingInstances, interfaces.PreflightInstance{
					Name:      item.Name,
					Type:      item.Type,
					State:     item.Status,
					Ownership: ownership,
					IPAddress: extractPrimaryIP(item),
				})
			}
		}
	}

	// 4. Networks Discovery
	netBytes, err := p.runCmd(ctx, "incus", "network", "list", "--format", "json")
	if err == nil {
		var rawNets []incusRawNetworkDetail
		if json.Unmarshal(netBytes, &rawNets) == nil {
			for _, netItem := range rawNets {
				res.Networks = append(res.Networks, interfaces.DiscoveredNetwork{
					Name:    netItem.Name,
					Type:    netItem.Type,
					Managed: netItem.Managed,
					IPv4:    netItem.Config["ipv4.address"],
					IPv6:    netItem.Config["ipv6.address"],
					State:   netItem.Status,
				})
			}
		}
	}

	// 5. Storage Pools Discovery
	storeBytes, err := p.runCmd(ctx, "incus", "storage", "list", "--format", "json")
	if err == nil {
		var rawStore []incusRawStorageDetail
		if json.Unmarshal(storeBytes, &rawStore) == nil {
			for _, storeItem := range rawStore {
				res.StoragePools = append(res.StoragePools, interfaces.DiscoveredStoragePool{
					Name:   storeItem.Name,
					Driver: storeItem.Driver,
					Status: storeItem.Status,
				})
			}
		}
	}

	// 6. Images Discovery
	imgBytes, err := p.runCmd(ctx, "incus", "image", "list", "--format", "json")
	if err == nil {
		var rawImgs []incusRawImageDetail
		if json.Unmarshal(imgBytes, &rawImgs) == nil {
			for _, imgItem := range rawImgs {
				aliasName := "none"
				if len(imgItem.Aliases) > 0 {
					aliasName = imgItem.Aliases[0].Name
				}
				res.Images = append(res.Images, interfaces.DiscoveredImage{
					Fingerprint:  imgItem.Fingerprint,
					Alias:        aliasName,
					Description:  imgItem.Properties["description"],
					OS:           imgItem.Properties["os"],
					Release:      imgItem.Properties["release"],
					Architecture: imgItem.Architecture,
					SizeBytes:    imgItem.Size,
				})
			}
		}
	}

	res.Availability = interfaces.AvailabilityAvailable
	res.HealthStatus.Operational = true
	res.HealthStatus.Capable = true

	if len(res.ExistingInstances) > 0 {
		extCount := 0
		for _, inst := range res.ExistingInstances {
			if inst.Ownership == interfaces.OwnershipExternal {
				extCount++
			}
		}
		if extCount > 0 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%d existing external provider resource(s) detected. Mystic will preserve and not automatically adopt them.", extCount))
		}
	}

	return res, nil
}

// --- InstanceProvider Implementation ---

func (p *IncusProvider) ListInstances(ctx context.Context) ([]interfaces.Instance, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}

	outBytes, err := p.runCmd(ctx, "incus", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to query incus instances: %w", err)
	}

	var raw []incusRawInstance
	if err := json.Unmarshal(outBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse incus instances json: %w", err)
	}

	instancesList := make([]interfaces.Instance, 0, len(raw))
	for _, item := range raw {
		instType := interfaces.InstanceTypeContainer
		if item.Type == "virtual-machine" {
			instType = interfaces.InstanceTypeVM
		}

		state := parseIncusState(item.Status)
		ip := extractPrimaryIP(item)

		cores, err := parseIncusCPULimit(item.Config["limits.cpu"])
		if err != nil && item.Config["limits.cpu"] != "" {
			return nil, fmt.Errorf("failed to parse instance '%s' cpu limit '%s': %w", item.Name, item.Config["limits.cpu"], err)
		}

		var memBytes int64
		if memStr, ok := item.Config["limits.memory"]; ok && strings.TrimSpace(memStr) != "" {
			var parseErr error
			memBytes, parseErr = parseIncusByteSize(memStr)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse instance '%s' memory limit '%s': %w", item.Name, memStr, parseErr)
			}
		}

		diskBytes := extractStorageBytes(item)

		createdTime, _ := time.Parse(time.RFC3339, item.Created)

		labels := make(map[string]string)
		for k, v := range item.Config {
			labels[k] = v
		}

		instancesList = append(instancesList, interfaces.Instance{
			ID:        item.Name,
			Name:      item.Name,
			Type:      instType,
			State:     state,
			Provider:  "incus",
			Node:      "local",
			IPAddress: ip,
			Limits: interfaces.ResourceLimits{
				CPUCores:    cores,
				MemoryBytes: memBytes,
				DiskBytes:   diskBytes,
			},
			Labels:    labels,
			CreatedAt: createdTime,
			UpdatedAt: time.Now(),
		})
	}

	return instancesList, nil
}

func (p *IncusProvider) GetInstance(ctx context.Context, idOrName string) (*interfaces.Instance, error) {
	list, err := p.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	for idx := range list {
		if list[idx].ID == idOrName || list[idx].Name == idOrName {
			return &list[idx], nil
		}
	}
	return nil, interfaces.ErrInstanceNotFound
}

func (p *IncusProvider) CreateInstance(ctx context.Context, inst *interfaces.Instance) (*interfaces.Instance, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}
	if inst == nil || strings.TrimSpace(inst.Name) == "" {
		return nil, interfaces.ErrInvalidConfiguration
	}

	// Check if already exists
	if _, err := p.GetInstance(ctx, inst.Name); err == nil {
		return nil, interfaces.ErrInstanceExists
	}

	// Execute incus launch
	args := []string{"launch"}
	if inst.Labels != nil && inst.Labels["image"] != "" {
		args = append(args, inst.Labels["image"])
	} else {
		args = append(args, "images:ubuntu/24.04")
	}
	args = append(args, inst.Name)

	if _, err := p.runCmd(ctx, "incus", args...); err != nil {
		return nil, fmt.Errorf("incus launch failed: %w", err)
	}

	// Configure CPU & Memory if specified
	if inst.Limits.CPUCores > 0 {
		_, _ = p.runCmd(ctx, "incus", "config", "set", inst.Name, "limits.cpu", strconv.Itoa(inst.Limits.CPUCores))
	}
	if inst.Limits.MemoryBytes > 0 {
		memMB := inst.Limits.MemoryBytes / (1024 * 1024)
		_, _ = p.runCmd(ctx, "incus", "config", "set", inst.Name, "limits.memory", fmt.Sprintf("%dMB", memMB))
	}

	// Set Mystic ownership labels if present
	if inst.Labels != nil {
		for k, v := range inst.Labels {
			if strings.HasPrefix(k, "user.mystic.") || strings.HasPrefix(k, "mystic.") {
				_, _ = p.runCmd(ctx, "incus", "config", "set", inst.Name, k, v)
			}
		}
	}

	return p.GetInstance(ctx, inst.Name)
}

func (p *IncusProvider) StartInstance(ctx context.Context, idOrName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	if _, err := p.runCmd(ctx, "incus", "start", idOrName); err != nil {
		return fmt.Errorf("failed to start instance %s: %w", idOrName, err)
	}
	return nil
}

func (p *IncusProvider) StopInstance(ctx context.Context, idOrName string, force bool) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	args := []string{"stop", idOrName}
	if force {
		args = append(args, "--force")
	}
	if _, err := p.runCmd(ctx, "incus", args...); err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", idOrName, err)
	}
	return nil
}

func (p *IncusProvider) RestartInstance(ctx context.Context, idOrName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	if _, err := p.runCmd(ctx, "incus", "restart", idOrName); err != nil {
		return fmt.Errorf("failed to restart instance %s: %w", idOrName, err)
	}
	return nil
}

func (p *IncusProvider) DeleteInstance(ctx context.Context, idOrName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	if _, err := p.runCmd(ctx, "incus", "delete", idOrName, "--force"); err != nil {
		return fmt.Errorf("failed to delete instance %s: %w", idOrName, err)
	}
	return nil
}

func (p *IncusProvider) RenameInstance(ctx context.Context, oldName, newName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	_, err := p.runCmd(ctx, "incus", "rename", oldName, newName)
	return err
}

func (p *IncusProvider) ResizeInstance(ctx context.Context, idOrName string, limits interfaces.ResourceLimits) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	if limits.CPUCores > 0 {
		_, _ = p.runCmd(ctx, "incus", "config", "set", idOrName, "limits.cpu", strconv.Itoa(limits.CPUCores))
	}
	if limits.MemoryBytes > 0 {
		memMB := limits.MemoryBytes / (1024 * 1024)
		_, _ = p.runCmd(ctx, "incus", "config", "set", idOrName, "limits.memory", fmt.Sprintf("%dMB", memMB))
	}
	return nil
}

func (p *IncusProvider) GetInstanceMetrics(ctx context.Context, idOrName string) (*interfaces.InstanceMetrics, error) {
	inst, err := p.GetInstance(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	return &interfaces.InstanceMetrics{
		InstanceID: inst.ID,
		Timestamp:  time.Now(),
	}, nil
}

func (p *IncusProvider) AdoptInstance(ctx context.Context, name string, workloadID string) (*interfaces.Instance, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}
	inst, err := p.GetInstance(ctx, name)
	if err != nil {
		return nil, err
	}

	if inst.Labels != nil && (inst.Labels["user.mystic.owned"] == "true" || inst.Labels["mystic.owned"] == "true") {
		existingWL := inst.Labels["user.mystic.workload_id"]
		if existingWL == "" {
			existingWL = inst.Labels["mystic.workload_id"]
		}
		if existingWL == workloadID && workloadID != "" {
			return inst, nil
		}
	}

	if _, err := p.runCmd(ctx, "incus", "config", "set", name, "user.mystic.owned", "true"); err != nil {
		return nil, fmt.Errorf("failed to set ownership label on incus instance %s: %w", name, err)
	}
	if workloadID != "" {
		if _, err := p.runCmd(ctx, "incus", "config", "set", name, "user.mystic.workload_id", workloadID); err != nil {
			return nil, fmt.Errorf("failed to set workload_id label on incus instance %s: %w", name, err)
		}
	}

	return p.GetInstance(ctx, name)
}

// --- ImageProvider Implementation ---

func (p *IncusProvider) ListImages(ctx context.Context) ([]interfaces.Image, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}

	outBytes, err := p.runCmd(ctx, "incus", "image", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to query incus images: %w", err)
	}

	var raw []incusRawImageDetail
	if err := json.Unmarshal(outBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse incus images json: %w", err)
	}

	imagesList := make([]interfaces.Image, 0, len(raw))
	for _, item := range raw {
		aliasName := "none"
		if len(item.Aliases) > 0 {
			aliasName = item.Aliases[0].Name
		}

		createdTime, _ := time.Parse(time.RFC3339, item.CreatedAt)

		imagesList = append(imagesList, interfaces.Image{
			ID:           item.Fingerprint,
			Fingerprint:  item.Fingerprint,
			Alias:        aliasName,
			Architecture: item.Architecture,
			Type:         item.Type,
			SizeBytes:    item.Size,
			CreatedAt:    createdTime,
		})
	}

	return imagesList, nil
}

func (p *IncusProvider) GetImage(ctx context.Context, fingerprintOrAlias string) (*interfaces.Image, error) {
	images, err := p.ListImages(ctx)
	if err != nil {
		return nil, err
	}
	for idx := range images {
		if images[idx].Fingerprint == fingerprintOrAlias || images[idx].Alias == fingerprintOrAlias {
			return &images[idx], nil
		}
	}
	return nil, fmt.Errorf("image %s not found", fingerprintOrAlias)
}

func (p *IncusProvider) DownloadImage(ctx context.Context, server, alias string) (*interfaces.Image, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}
	if _, err := p.runCmd(ctx, "incus", "image", "copy", fmt.Sprintf("%s:%s", server, alias), "local:", "--alias", alias); err != nil {
		return nil, fmt.Errorf("failed to copy image %s from %s: %w", alias, server, err)
	}
	return p.GetImage(ctx, alias)
}

func (p *IncusProvider) DeleteImage(ctx context.Context, fingerprint string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	_, err := p.runCmd(ctx, "incus", "image", "delete", fingerprint)
	return err
}

// --- SnapshotProvider Implementation ---

func (p *IncusProvider) ListSnapshots(ctx context.Context, instanceID string) ([]interfaces.Snapshot, error) {
	return []interfaces.Snapshot{}, nil
}

func (p *IncusProvider) CreateSnapshot(ctx context.Context, instanceID, snapshotName string, stateful bool) (*interfaces.Snapshot, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}
	args := []string{"snapshot", "create", instanceID, snapshotName}
	if stateful {
		args = append(args, "--stateful")
	}
	if _, err := p.runCmd(ctx, "incus", args...); err != nil {
		return nil, err
	}
	return &interfaces.Snapshot{
		Name:       snapshotName,
		InstanceID: instanceID,
		Stateful:   stateful,
		CreatedAt:  time.Now(),
	}, nil
}

func (p *IncusProvider) RestoreSnapshot(ctx context.Context, instanceID, snapshotName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	_, err := p.runCmd(ctx, "incus", "snapshot", "restore", instanceID, snapshotName)
	return err
}

func (p *IncusProvider) DeleteSnapshot(ctx context.Context, instanceID, snapshotName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	_, err := p.runCmd(ctx, "incus", "snapshot", "delete", instanceID, snapshotName)
	return err
}

// --- StorageProvider Implementation ---

func (p *IncusProvider) ListStoragePools(ctx context.Context) ([]interfaces.StoragePool, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}

	outBytes, err := p.runCmd(ctx, "incus", "storage", "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	var raw []incusRawStorageDetail
	if err := json.Unmarshal(outBytes, &raw); err != nil {
		return nil, err
	}

	pools := make([]interfaces.StoragePool, 0, len(raw))
	for _, item := range raw {
		pools = append(pools, interfaces.StoragePool{
			Name:   item.Name,
			Driver: item.Driver,
		})
	}
	return pools, nil
}

func (p *IncusProvider) GetStoragePool(ctx context.Context, name string) (*interfaces.StoragePool, error) {
	pools, err := p.ListStoragePools(ctx)
	if err != nil {
		return nil, err
	}
	for idx := range pools {
		if pools[idx].Name == name {
			return &pools[idx], nil
		}
	}
	return nil, fmt.Errorf("storage pool %s not found", name)
}

// --- NetworkProvider Implementation ---

func (p *IncusProvider) ListNetworks(ctx context.Context) ([]interfaces.Network, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}

	outBytes, err := p.runCmd(ctx, "incus", "network", "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	var raw []incusRawNetworkDetail
	if err := json.Unmarshal(outBytes, &raw); err != nil {
		return nil, err
	}

	nets := make([]interfaces.Network, 0, len(raw))
	for _, item := range raw {
		nets = append(nets, interfaces.Network{
			Name:      item.Name,
			Type:      item.Type,
			ManagedBy: "incus",
			CIDR:      item.Config["ipv4.address"],
		})
	}
	return nets, nil
}

func (p *IncusProvider) GetNetwork(ctx context.Context, name string) (*interfaces.Network, error) {
	nets, err := p.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}
	for idx := range nets {
		if nets[idx].Name == name {
			return &nets[idx], nil
		}
	}
	return nil, fmt.Errorf("network %s not found", name)
}

// Helper utilities
func parseIncusState(status string) interfaces.InstanceState {
	switch strings.ToUpper(status) {
	case "RUNNING":
		return interfaces.StateRunning
	case "STOPPED":
		return interfaces.StateStopped
	case "FROZEN":
		return interfaces.StateFrozen
	case "ERROR":
		return interfaces.StateError
	default:
		return interfaces.StateUnknown
	}
}

type ipCandidate struct {
	ifaceName string
	ip        string
	score     int
}

func extractPrimaryIP(item incusRawInstance) string {
	// Build map of configured Incus NIC devices
	incusNics := make(map[string]bool) // ifaceName -> hasIncusNetwork

	checkDeviceMap := func(devMap map[string]incusDevice) {
		for key, dev := range devMap {
			if dev.Type == "nic" || dev.Type == "" {
				iface := dev.Name
				if iface == "" {
					iface = key
				}
				hasNet := dev.Network != "" || dev.Parent != ""
				if currentHasNet, exists := incusNics[iface]; exists {
					incusNics[iface] = currentHasNet || hasNet
				} else {
					incusNics[iface] = hasNet
				}
			}
		}
	}

	checkDeviceMap(item.ExpandedDevices)
	checkDeviceMap(item.Devices)

	var candidates []ipCandidate

	for ifaceName, netObj := range item.State.Network {
		if ifaceName == "lo" {
			continue
		}
		for _, addr := range netObj.Addresses {
			if addr.Family != "inet" || addr.Scope == "local" || strings.HasPrefix(addr.Address, "127.") {
				continue
			}

			score := 0
			hasNet, isConfiguredNic := incusNics[ifaceName]
			if isConfiguredNic {
				if hasNet {
					score = 100 // Incus-configured NIC connected to Incus network/bridge
				} else {
					score = 80 // Incus-configured NIC device
				}
			} else if isStandardIface(ifaceName) && !isInternalBridge(ifaceName) {
				score = 60 // Standard interface name (eth*, en*, net*) and not internal bridge
			} else if !isInternalBridge(ifaceName) {
				score = 40 // Non-bridge interface
			} else {
				score = 20 // Internal container/bridge interface (docker0, cni0, etc.)
			}

			candidates = append(candidates, ipCandidate{
				ifaceName: ifaceName,
				ip:        addr.Address,
				score:     score,
			})
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// Sort candidates by score desc, ifaceName asc, ip asc
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].ifaceName != candidates[j].ifaceName {
			return candidates[i].ifaceName < candidates[j].ifaceName
		}
		return candidates[i].ip < candidates[j].ip
	})

	return candidates[0].ip
}

func isStandardIface(ifaceName string) bool {
	lower := strings.ToLower(ifaceName)
	return strings.HasPrefix(lower, "eth") || strings.HasPrefix(lower, "en") ||
		strings.HasPrefix(lower, "wl") || strings.HasPrefix(lower, "net")
}

func isInternalBridge(ifaceName string) bool {
	lower := strings.ToLower(ifaceName)
	return lower == "docker0" || strings.HasPrefix(lower, "docker") ||
		strings.HasPrefix(lower, "cni") || strings.HasPrefix(lower, "flannel") ||
		strings.HasPrefix(lower, "br-") || strings.HasPrefix(lower, "veth") ||
		strings.HasPrefix(lower, "kube") || strings.HasPrefix(lower, "virbr") ||
		strings.HasPrefix(lower, "dummy")
}

func parseIncusByteSize(val string) (int64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, nil
	}

	if bytesVal, err := strconv.ParseInt(val, 10, 64); err == nil {
		if bytesVal < 0 {
			return 0, fmt.Errorf("byte size cannot be negative: %s", val)
		}
		return bytesVal, nil
	}

	upper := strings.ToUpper(val)
	idx := 0
	for idx < len(upper) && ((upper[idx] >= '0' && upper[idx] <= '9') || upper[idx] == '.') {
		idx++
	}
	numStr := upper[:idx]
	unit := strings.TrimSpace(upper[idx:])

	if numStr == "" {
		return 0, fmt.Errorf("invalid byte size format '%s'", val)
	}

	floatVal, err := strconv.ParseFloat(numStr, 64)
	if err != nil || floatVal < 0 {
		return 0, fmt.Errorf("invalid numeric portion in byte size '%s'", val)
	}

	var multiplier float64
	switch unit {
	case "B", "":
		multiplier = 1
	case "K", "KB", "KIB":
		if unit == "KB" {
			multiplier = 1000
		} else {
			multiplier = 1024
		}
	case "M", "MB", "MIB":
		if unit == "MB" {
			multiplier = 1000 * 1000
		} else {
			multiplier = 1024 * 1024
		}
	case "G", "GB", "GIB":
		if unit == "GB" {
			multiplier = 1000 * 1000 * 1000
		} else {
			multiplier = 1024 * 1024 * 1024
		}
	case "T", "TB", "TIB":
		if unit == "TB" {
			multiplier = 1000 * 1000 * 1000 * 1000
		} else {
			multiplier = 1024 * 1024 * 1024 * 1024
		}
	default:
		return 0, fmt.Errorf("unrecognized byte unit '%s' in value '%s'", unit, val)
	}

	return int64(floatVal * multiplier), nil
}

func parseIncusCPULimit(val string) (int, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, nil
	}

	if cores, err := strconv.Atoi(val); err == nil {
		if cores <= 0 {
			return 0, fmt.Errorf("cpu limit must be positive: %s", val)
		}
		return cores, nil
	}

	if strings.Contains(val, "-") {
		parts := strings.Split(val, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 == nil && err2 == nil && end >= start {
				return (end - start) + 1, nil
			}
		}
	}

	if strings.Contains(val, ",") {
		parts := strings.Split(val, ",")
		validCount := 0
		for _, p := range parts {
			if _, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				validCount++
			}
		}
		if validCount > 0 {
			return validCount, nil
		}
	}

	return 0, fmt.Errorf("invalid CPU limit format '%s'", val)
}

func extractStorageBytes(item incusRawInstance) int64 {
	checkDevices := func(devMap map[string]incusDevice) int64 {
		for key, dev := range devMap {
			if dev.Type == "disk" || key == "root" {
				if dev.Size != "" {
					if bytes, err := parseIncusByteSize(dev.Size); err == nil && bytes > 0 {
						return bytes
					}
				}
			}
		}
		return 0
	}

	if bytes := checkDevices(item.ExpandedDevices); bytes > 0 {
		return bytes
	}
	if bytes := checkDevices(item.Devices); bytes > 0 {
		return bytes
	}
	if sz, ok := item.Config["root.size"]; ok && sz != "" {
		if bytes, err := parseIncusByteSize(sz); err == nil && bytes > 0 {
			return bytes
		}
	}
	return 0
}
