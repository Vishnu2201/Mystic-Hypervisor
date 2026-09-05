package incus

import (
	"context"
	"strings"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

func TestIncusProviderPreflightUnavailable(t *testing.T) {
	p := NewIncusProvider("/invalid/path/socket")
	p.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, interfaces.ErrProviderUnavailable
	})

	res, err := p.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight returned unexpected error: %v", err)
	}

	if res.Availability != interfaces.AvailabilityUnavailable {
		t.Errorf("Expected Availability UNAVAILABLE, got %s", res.Availability)
	}
	if res.HealthStatus.Reachable {
		t.Errorf("Expected Reachable false when execution runner fails")
	}
	if len(res.Blockers) == 0 {
		t.Errorf("Expected at least 1 blocker when provider is unreachable")
	}
}

func TestIncusProviderPreflightDiscovery(t *testing.T) {
	p := NewIncusProvider("")

	p.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdStr := strings.Join(args, " ")
		switch {
		case strings.Contains(cmdStr, "query /1.0"):
			return []byte(`{
				"type": "sync",
				"status": "Success",
				"status_code": 200,
				"metadata": {
					"environment": {
						"addresses": ["10.0.0.250:8443"],
						"architecture": "x86_64",
						"driver": "qemu",
						"kernel_version": "6.12.38+deb13-cloud-amd64",
						"os_name": "Debian GNU/Linux",
						"os_version": "13 (trixie)",
						"server_version": "6.0.4"
					}
				}
			}`), nil
		case strings.Contains(cmdStr, "network list"):
			return []byte(`[
				{
					"name": "incusbr0",
					"type": "bridge",
					"managed": true,
					"status": "Created",
					"config": {
						"ipv4.address": "10.0.100.1/24"
					}
				}
			]`), nil
		case strings.Contains(cmdStr, "storage list"):
			return []byte(`[
				{
					"name": "default",
					"driver": "dir",
					"status": "Created"
				}
			]`), nil
		case strings.Contains(cmdStr, "image list"):
			return []byte(`[
				{
					"fingerprint": "abc123def456",
					"architecture": "x86_64",
					"type": "container",
					"size": 104857600,
					"aliases": [{"name": "debian/13", "description": "Debian Trixie"}],
					"properties": {"os": "Debian", "release": "13", "description": "Debian 13 Minimal"}
				}
			]`), nil
		case strings.Contains(cmdStr, "list --format json"):
			return []byte(`[
				{
					"name": "external-vm-1",
					"status": "Running",
					"type": "virtual-machine",
					"config": {},
					"created_at": "2026-09-01T10:00:00Z"
				},
				{
					"name": "mystic-test-container",
					"status": "Running",
					"type": "container",
					"config": {
						"user.mystic.owned": "true",
						"user.mystic.workload_id": "wl-100"
					},
					"created_at": "2026-09-02T12:00:00Z"
				}
			]`), nil
		default:
			return []byte("{}"), nil
		}
	})

	res, err := p.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	if res.Availability != interfaces.AvailabilityAvailable {
		t.Errorf("Expected Availability AVAILABLE, got %s", res.Availability)
	}
	if !res.HealthStatus.Installed || !res.HealthStatus.Reachable || !res.HealthStatus.Operational || !res.HealthStatus.Capable {
		t.Errorf("Expected all health flags true, got %+v", res.HealthStatus)
	}
	if res.ServerInfo.ServerVersion != "6.0.4" {
		t.Errorf("Expected ServerVersion 6.0.4, got %s", res.ServerInfo.ServerVersion)
	}

	// Verify instances and ownership classification
	if len(res.ExistingInstances) != 2 {
		t.Fatalf("Expected 2 existing instances, got %d", len(res.ExistingInstances))
	}
	if res.ExistingInstances[0].Ownership != interfaces.OwnershipExternal {
		t.Errorf("Expected external-vm-1 to be classified EXTERNAL, got %s", res.ExistingInstances[0].Ownership)
	}
	if res.ExistingInstances[1].Ownership != interfaces.OwnershipMysticOwned {
		t.Errorf("Expected mystic-test-container to be classified MYSTIC_OWNED, got %s", res.ExistingInstances[1].Ownership)
	}

	// Verify networks, storage, images
	if len(res.Networks) != 1 || res.Networks[0].Name != "incusbr0" {
		t.Errorf("Expected network incusbr0, got %+v", res.Networks)
	}
	if len(res.StoragePools) != 1 || res.StoragePools[0].Name != "default" {
		t.Errorf("Expected storage pool default, got %+v", res.StoragePools)
	}
	if len(res.Images) != 1 || res.Images[0].Alias != "debian/13" {
		t.Errorf("Expected image alias debian/13, got %+v", res.Images)
	}

	// Verify existing external resource warning
	if len(res.Warnings) == 0 {
		t.Errorf("Expected warning for external resources detected")
	}
}

func TestIncusProviderPreflightIncus6DirectResponse(t *testing.T) {
	p := NewIncusProvider("")
	p.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdStr := strings.Join(args, " ")
		switch {
		case strings.Contains(cmdStr, "query /1.0"):
			return []byte(`{
				"api_status": "stable",
				"api_version": "1.0",
				"auth": "trusted",
				"environment": {
					"architectures": ["x86_64", "i686"],
					"driver": "lxc",
					"driver_version": "6.0.4",
					"kernel_version": "6.12.38+deb13-cloud-amd64",
					"os_name": "Debian GNU/Linux",
					"os_version": "13",
					"server_name": "mysticservers",
					"server_version": "6.0.4",
					"storage": "dir"
				}
			}`), nil
		default:
			return []byte("[]"), nil
		}
	})

	res, err := p.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	if res.Availability != interfaces.AvailabilityAvailable {
		t.Errorf("Expected Availability AVAILABLE, got %s", res.Availability)
	}
	if !res.HealthStatus.Installed || !res.HealthStatus.Reachable || !res.HealthStatus.Operational || !res.HealthStatus.Capable {
		t.Errorf("Expected all health flags true, got %+v", res.HealthStatus)
	}
	if res.ServerInfo.ServerVersion != "6.0.4" {
		t.Errorf("Expected ServerVersion 6.0.4, got %s", res.ServerInfo.ServerVersion)
	}
	if res.ServerInfo.OS != "Debian GNU/Linux 13" {
		t.Errorf("Expected OS 'Debian GNU/Linux 13', got '%s'", res.ServerInfo.OS)
	}
	if res.ServerInfo.Architecture != "x86_64" {
		t.Errorf("Expected Architecture 'x86_64', got '%s'", res.ServerInfo.Architecture)
	}
	if res.ServerInfo.KVMSupported {
		t.Errorf("Expected KVMSupported false for LXC driver, got true")
	}
}

func TestExtractPrimaryIP(t *testing.T) {
	// a. Instance with eth0 on incusbr0 (10.170.92.70) and docker0 (172.17.0.1) -> expect 10.170.92.70
	t.Run("IncusNicVsDocker0", func(t *testing.T) {
		inst := incusRawInstance{
			Name: "test-nano",
			ExpandedDevices: map[string]incusDevice{
				"eth0": {Type: "nic", Name: "eth0", Network: "incusbr0"},
			},
		}
		inst.State.Network = map[string]struct {
			Addresses []struct {
				Family  string `json:"family"`
				Address string `json:"address"`
				Scope   string `json:"scope"`
			} `json:"addresses"`
		}{
			"docker0": {
				Addresses: []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				}{
					{Family: "inet", Address: "172.17.0.1", Scope: "global"},
				},
			},
			"eth0": {
				Addresses: []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				}{
					{Family: "inet", Address: "10.170.92.70", Scope: "global"},
				},
			},
			"lo": {
				Addresses: []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				}{
					{Family: "inet", Address: "127.0.0.1", Scope: "local"},
				},
			},
		}

		ip := extractPrimaryIP(inst)
		if ip != "10.170.92.70" {
			t.Errorf("Expected primary IP 10.170.92.70, got %s", ip)
		}
	})

	// b. Instance with custom non-eth0 Incus-connected interface (e.g. net0 with 10.0.0.55) -> expect 10.0.0.55
	t.Run("CustomNicNameIncusConnected", func(t *testing.T) {
		inst := incusRawInstance{
			Name: "custom-net-inst",
			ExpandedDevices: map[string]incusDevice{
				"net0": {Type: "nic", Name: "net0", Network: "custombr0"},
			},
		}
		inst.State.Network = map[string]struct {
			Addresses []struct {
				Family  string `json:"family"`
				Address string `json:"address"`
				Scope   string `json:"scope"`
			} `json:"addresses"`
		}{
			"docker0": {
				Addresses: []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				}{
					{Family: "inet", Address: "172.18.0.1", Scope: "global"},
				},
			},
			"net0": {
				Addresses: []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				}{
					{Family: "inet", Address: "10.0.0.55", Scope: "global"},
				},
			},
		}

		ip := extractPrimaryIP(inst)
		if ip != "10.0.0.55" {
			t.Errorf("Expected primary IP 10.0.0.55, got %s", ip)
		}
	})

	// c. Instance with single IPv4 address -> expected that address remains selected
	t.Run("SingleIPv4Address", func(t *testing.T) {
		inst := incusRawInstance{
			Name: "single-ip-inst",
		}
		inst.State.Network = map[string]struct {
			Addresses []struct {
				Family  string `json:"family"`
				Address string `json:"address"`
				Scope   string `json:"scope"`
			} `json:"addresses"`
		}{
			"eth0": {
				Addresses: []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				}{
					{Family: "inet", Address: "192.168.1.100", Scope: "global"},
				},
			},
		}

		ip := extractPrimaryIP(inst)
		if ip != "192.168.1.100" {
			t.Errorf("Expected primary IP 192.168.1.100, got %s", ip)
		}
	})

	// d. Instance without usable IPv4 addresses -> expected empty string
	t.Run("NoUsableIPv4", func(t *testing.T) {
		inst := incusRawInstance{
			Name: "no-ip-inst",
		}
		inst.State.Network = map[string]struct {
			Addresses []struct {
				Family  string `json:"family"`
				Address string `json:"address"`
				Scope   string `json:"scope"`
			} `json:"addresses"`
		}{
			"lo": {
				Addresses: []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				}{
					{Family: "inet", Address: "127.0.0.1", Scope: "local"},
				},
			},
			"eth0": {
				Addresses: []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				}{
					{Family: "inet6", Address: "fe80::1", Scope: "link"},
				},
			},
		}

		ip := extractPrimaryIP(inst)
		if ip != "" {
			t.Errorf("Expected empty primary IP, got %s", ip)
		}
	})
}

func TestIncusProviderAdoptInstance(t *testing.T) {
	p := NewIncusProvider("")

	executedCmds := make([]string, 0)
	p.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdStr := strings.Join(args, " ")
		executedCmds = append(executedCmds, cmdStr)

		switch {
		case strings.Contains(cmdStr, "version"):
			return []byte("6.0.4"), nil
		case strings.Contains(cmdStr, "list --format json"):
			return []byte(`[
				{
					"name": "ext-01",
					"status": "Running",
					"type": "container",
					"config": {},
					"created_at": "2026-09-01T10:00:00Z"
				}
			]`), nil
		case strings.Contains(cmdStr, "config set ext-01"):
			return []byte(""), nil
		default:
			return []byte("{}"), nil
		}
	})

	inst, err := p.AdoptInstance(context.Background(), "ext-01", "wl-9999")
	if err != nil {
		t.Fatalf("AdoptInstance failed: %v", err)
	}

	if inst.Name != "ext-01" {
		t.Errorf("Expected instance name ext-01, got %s", inst.Name)
	}

	setOwnedCalled := false
	setWlIDCalled := false
	for _, cmd := range executedCmds {
		if strings.Contains(cmd, "config set ext-01 user.mystic.owned true") {
			setOwnedCalled = true
		}
		if strings.Contains(cmd, "config set ext-01 user.mystic.workload_id wl-9999") {
			setWlIDCalled = true
		}
		if strings.HasPrefix(cmd, "delete") || strings.HasPrefix(cmd, "stop") ||
			strings.HasPrefix(cmd, "start") || strings.HasPrefix(cmd, "restart") {
			t.Errorf("Forbidden destructive command executed during adoption: %s", cmd)
		}
	}

	if !setOwnedCalled {
		t.Errorf("Expected config set user.mystic.owned true to be executed")
	}
	if !setWlIDCalled {
		t.Errorf("Expected config set user.mystic.workload_id wl-9999 to be executed")
	}
}

func TestParseIncusByteSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"3GiB", 3 * 1024 * 1024 * 1024, false},
		{"512MiB", 512 * 1024 * 1024, false},
		{"1GiB", 1 * 1024 * 1024 * 1024, false},
		{"4096MiB", 4096 * 1024 * 1024, false},
		{"4GB", 4000000000, false},
		{"512MB", 512000000, false},
		{"3221225472", 3221225472, false},
		{"", 0, false},
		{"invalid-unit-xyz", 0, true},
		{"-500MB", 0, true},
	}

	for _, tt := range tests {
		got, err := parseIncusByteSize(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseIncusByteSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("parseIncusByteSize(%q) = %d, expected %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseIncusCPULimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"1", 1, false},
		{"4", 4, false},
		{"0-3", 4, false},
		{"0,2,4", 3, false},
		{"", 0, false},
		{"invalid-cpu", 0, true},
		{"-2", 0, true},
	}

	for _, tt := range tests {
		got, err := parseIncusCPULimit(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseIncusCPULimit(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("parseIncusCPULimit(%q) = %d, expected %d", tt.input, got, tt.expected)
		}
	}
}

func TestIncusListInstancesMemoryParsing(t *testing.T) {
	p := NewIncusProvider("")
	p.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdStr := strings.Join(args, " ")
		switch {
		case strings.Contains(cmdStr, "version"):
			return []byte("6.0.4"), nil
		case strings.Contains(cmdStr, "list --format json"):
			return []byte(`[
				{
					"name": "nano-3gib",
					"status": "Running",
					"type": "container",
					"config": {
						"limits.cpu": "1",
						"limits.memory": "3GiB"
					}
				}
			]`), nil
		default:
			return []byte("{}"), nil
		}
	})

	insts, err := p.ListInstances(context.Background())
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}
	if len(insts) != 1 {
		t.Fatalf("Expected 1 instance, got %d", len(insts))
	}
	if insts[0].Limits.MemoryBytes != 3*1024*1024*1024 {
		t.Errorf("Expected 3GiB (%d bytes), got %d bytes", 3*1024*1024*1024, insts[0].Limits.MemoryBytes)
	}
}

func TestIncusListInstancesInvalidMemoryReturnsError(t *testing.T) {
	p := NewIncusProvider("")
	p.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdStr := strings.Join(args, " ")
		switch {
		case strings.Contains(cmdStr, "version"):
			return []byte("6.0.4"), nil
		case strings.Contains(cmdStr, "list --format json"):
			return []byte(`[
				{
					"name": "bad-mem",
					"status": "Running",
					"type": "container",
					"config": {
						"limits.memory": "unparseable-xyz"
					}
				}
			]`), nil
		default:
			return []byte("{}"), nil
		}
	})

	_, err := p.ListInstances(context.Background())
	if err == nil {
		t.Fatalf("Expected ListInstances to fail on unparseable memory limit, got nil error")
	}
}
