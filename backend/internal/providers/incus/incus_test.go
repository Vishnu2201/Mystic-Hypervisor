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
