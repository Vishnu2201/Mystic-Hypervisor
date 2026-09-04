package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// IncusProvider implements the Provider interface for Incus hypervisors.
type IncusProvider struct {
	socketPath string
	caps       interfaces.CapabilitySet
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

func (p *IncusProvider) Name() string {
	return "incus"
}

func (p *IncusProvider) Capabilities() interfaces.CapabilitySet {
	return p.caps
}

func (p *IncusProvider) Ping(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "incus", "version")
	if err := cmd.Run(); err != nil {
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

// --- InstanceProvider Implementation ---

type incusRawInstance struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Type    string `json:"type"`
	State   struct {
		Status    string `json:"status"`
		StatusCode int   `json:"status_code"`
		Network   map[string]struct {
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

func (p *IncusProvider) ListInstances(ctx context.Context) ([]interfaces.Instance, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}

	cmd := exec.CommandContext(ctx, "incus", "list", "--format", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to query incus instances: %w", err)
	}

	var raw []incusRawInstance
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
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

		cores, _ := strconv.Atoi(item.Config["limits.cpu"])
		memBytes, _ := strconv.ParseInt(item.Config["limits.memory"], 10, 64)

		createdTime, _ := time.Parse(time.RFC3339, item.Created)

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
			},
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

	cmd := exec.CommandContext(ctx, "incus", args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("incus launch failed: %w", err)
	}

	// Configure CPU & Memory if specified
	if inst.Limits.CPUCores > 0 {
		_ = exec.CommandContext(ctx, "incus", "config", "set", inst.Name, "limits.cpu", strconv.Itoa(inst.Limits.CPUCores)).Run()
	}
	if inst.Limits.MemoryBytes > 0 {
		memMB := inst.Limits.MemoryBytes / (1024 * 1024)
		_ = exec.CommandContext(ctx, "incus", "config", "set", inst.Name, "limits.memory", fmt.Sprintf("%dMB", memMB)).Run()
	}

	return p.GetInstance(ctx, inst.Name)
}

func (p *IncusProvider) StartInstance(ctx context.Context, idOrName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	cmd := exec.CommandContext(ctx, "incus", "start", idOrName)
	if err := cmd.Run(); err != nil {
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
	cmd := exec.CommandContext(ctx, "incus", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", idOrName, err)
	}
	return nil
}

func (p *IncusProvider) RestartInstance(ctx context.Context, idOrName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	cmd := exec.CommandContext(ctx, "incus", "restart", idOrName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart instance %s: %w", idOrName, err)
	}
	return nil
}

func (p *IncusProvider) DeleteInstance(ctx context.Context, idOrName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	cmd := exec.CommandContext(ctx, "incus", "delete", idOrName, "--force")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete instance %s: %w", idOrName, err)
	}
	return nil
}

func (p *IncusProvider) RenameInstance(ctx context.Context, oldName, newName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	cmd := exec.CommandContext(ctx, "incus", "rename", oldName, newName)
	return cmd.Run()
}

func (p *IncusProvider) ResizeInstance(ctx context.Context, idOrName string, limits interfaces.ResourceLimits) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	if limits.CPUCores > 0 {
		_ = exec.CommandContext(ctx, "incus", "config", "set", idOrName, "limits.cpu", strconv.Itoa(limits.CPUCores)).Run()
	}
	if limits.MemoryBytes > 0 {
		memMB := limits.MemoryBytes / (1024 * 1024)
		_ = exec.CommandContext(ctx, "incus", "config", "set", idOrName, "limits.memory", fmt.Sprintf("%dMB", memMB)).Run()
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

// --- ImageProvider Implementation ---

type incusRawImage struct {
	Fingerprint string   `json:"fingerprint"`
	Aliases     []struct {
		Name string `json:"name"`
	} `json:"aliases"`
	Architecture string `json:"architecture"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	CreatedAt    string `json:"created_at"`
}

func (p *IncusProvider) ListImages(ctx context.Context) ([]interfaces.Image, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}

	cmd := exec.CommandContext(ctx, "incus", "image", "list", "--format", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to query incus images: %w", err)
	}

	var raw []incusRawImage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
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
	cmd := exec.CommandContext(ctx, "incus", "image", "copy", fmt.Sprintf("%s:%s", server, alias), "local:", "--alias", alias)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to copy image %s from %s: %w", alias, server, err)
	}
	return p.GetImage(ctx, alias)
}

func (p *IncusProvider) DeleteImage(ctx context.Context, fingerprint string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	cmd := exec.CommandContext(ctx, "incus", "image", "delete", fingerprint)
	return cmd.Run()
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
	cmd := exec.CommandContext(ctx, "incus", args...)
	if err := cmd.Run(); err != nil {
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
	cmd := exec.CommandContext(ctx, "incus", "snapshot", "restore", instanceID, snapshotName)
	return cmd.Run()
}

func (p *IncusProvider) DeleteSnapshot(ctx context.Context, instanceID, snapshotName string) error {
	if err := p.Ping(ctx); err != nil {
		return interfaces.ErrProviderUnavailable
	}
	cmd := exec.CommandContext(ctx, "incus", "snapshot", "delete", instanceID, snapshotName)
	return cmd.Run()
}

// --- StorageProvider Implementation ---

type incusRawStoragePool struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

func (p *IncusProvider) ListStoragePools(ctx context.Context) ([]interfaces.StoragePool, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}

	cmd := exec.CommandContext(ctx, "incus", "storage", "list", "--format", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var raw []incusRawStoragePool
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
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

type incusRawNetwork struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (p *IncusProvider) ListNetworks(ctx context.Context) ([]interfaces.Network, error) {
	if err := p.Ping(ctx); err != nil {
		return nil, interfaces.ErrProviderUnavailable
	}

	cmd := exec.CommandContext(ctx, "incus", "network", "list", "--format", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var raw []incusRawNetwork
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		return nil, err
	}

	nets := make([]interfaces.Network, 0, len(raw))
	for _, item := range raw {
		nets = append(nets, interfaces.Network{
			Name:      item.Name,
			Type:      item.Type,
			ManagedBy: "incus",
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

func extractPrimaryIP(item incusRawInstance) string {
	for _, netObj := range item.State.Network {
		for _, addr := range netObj.Addresses {
			if addr.Family == "inet" && addr.Scope == "global" {
				return addr.Address
			}
		}
	}
	return ""
}
