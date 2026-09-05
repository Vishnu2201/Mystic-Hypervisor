package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
)

func TestIncusProxyDriverCreateMissingDevice(t *testing.T) {
	ctx := context.Background()
	driver := NewIncusProxyDriver("")

	var executedCmds [][]string
	driver.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		full := append([]string{name}, args...)
		executedCmds = append(executedCmds, full)

		// Mock incus config show test-nano --expanded --format json
		if name == "incus" && len(args) >= 3 && args[0] == "config" && args[1] == "show" {
			return []byte(`{"name":"test-nano","type":"container","devices":{}}`), nil
		}

		// Mock incus config device add test-nano mystic-exp-exp-01 proxy listen=tcp:51.162.178.199:2222 connect=tcp:10.170.92.70:22 nat=true
		if name == "incus" && len(args) >= 3 && args[0] == "config" && args[1] == "device" && args[2] == "add" {
			return []byte("Device mystic-exp-exp-01 added to test-nano"), nil
		}

		return []byte(""), nil
	})

	exp := &networking.NetworkExposure{
		ID:           "exp-01",
		WorkloadID:   "wl-01",
		WorkloadName: "test-nano",
		PublicIP:     "51.162.178.199",
		PublicPort:   2222,
		InternalIP:   "10.170.92.70",
		InternalPort: 22,
		Protocol:     hosts.ProtocolTCP,
	}

	err := driver.CreateExposure(ctx, exp)
	if err != nil {
		t.Fatalf("CreateExposure failed: %v", err)
	}

	if len(executedCmds) != 2 {
		t.Fatalf("Expected 2 command executions (show + device add), got %d", len(executedCmds))
	}

	addCmd := executedCmds[1]
	expectedArgs := []string{"incus", "config", "device", "add", "test-nano", "mystic-exp-exp-01", "proxy", "listen=tcp:51.162.178.199:2222", "connect=tcp:10.170.92.70:22", "nat=true"}
	for i, arg := range expectedArgs {
		if addCmd[i] != arg {
			t.Fatalf("Command arg mismatch at index %d: expected '%s', got '%s'", i, arg, addCmd[i])
		}
	}
}

func TestIncusProxyDriverCreateIdenticalDeviceNoOp(t *testing.T) {
	ctx := context.Background()
	driver := NewIncusProxyDriver("")

	var executedCmds [][]string
	driver.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		full := append([]string{name}, args...)
		executedCmds = append(executedCmds, full)

		if name == "incus" && len(args) >= 3 && args[0] == "config" && args[1] == "show" {
			return []byte(`{
				"name":"test-nano",
				"devices":{
					"mystic-exp-exp-01": {
						"type": "proxy",
						"listen": "tcp:51.162.178.199:2222",
						"connect": "tcp:10.170.92.70:22",
						"nat": "true"
					}
				}
			}`), nil
		}
		return []byte(""), nil
	})

	exp := &networking.NetworkExposure{
		ID:           "exp-01",
		WorkloadName: "test-nano",
		PublicIP:     "51.162.178.199",
		PublicPort:   2222,
		InternalIP:   "10.170.92.70",
		InternalPort: 22,
		Protocol:     hosts.ProtocolTCP,
	}

	err := driver.CreateExposure(ctx, exp)
	if err != nil {
		t.Fatalf("CreateExposure failed: %v", err)
	}

	// Should be no-op after config show
	if len(executedCmds) != 1 {
		t.Fatalf("Expected only 1 command execution (config show), got %d", len(executedCmds))
	}
}

func TestIncusProxyDriverUpdateDifferingDevice(t *testing.T) {
	ctx := context.Background()
	driver := NewIncusProxyDriver("")

	var executedCmds [][]string
	driver.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		full := append([]string{name}, args...)
		executedCmds = append(executedCmds, full)

		if name == "incus" && len(args) >= 3 && args[0] == "config" && args[1] == "show" {
			return []byte(`{
				"name":"test-nano",
				"devices":{
					"mystic-exp-exp-01": {
						"type": "proxy",
						"listen": "tcp:51.162.178.199:1111",
						"connect": "tcp:10.170.92.70:22",
						"nat": "true"
					}
				}
			}`), nil
		}
		return []byte(""), nil
	})

	exp := &networking.NetworkExposure{
		ID:           "exp-01",
		WorkloadName: "test-nano",
		PublicIP:     "51.162.178.199",
		PublicPort:   2222, // Changed port to 2222
		InternalIP:   "10.170.92.70",
		InternalPort: 22,
		Protocol:     hosts.ProtocolTCP,
	}

	err := driver.CreateExposure(ctx, exp)
	if err != nil {
		t.Fatalf("CreateExposure failed: %v", err)
	}

	if len(executedCmds) != 2 {
		t.Fatalf("Expected 2 command executions (show + device set), got %d", len(executedCmds))
	}
	if executedCmds[1][3] != "set" {
		t.Fatalf("Expected 'config device set' command for update, got %v", executedCmds[1])
	}
}

func TestIncusProxyDriverDelete(t *testing.T) {
	ctx := context.Background()
	driver := NewIncusProxyDriver("")

	// Case 1: Existing device deletion
	driver.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("Device mystic-exp-exp-01 removed from test-nano"), nil
	})

	exp := &networking.NetworkExposure{
		ID:           "exp-01",
		WorkloadName: "test-nano",
	}

	err := driver.DeleteExposure(ctx, exp)
	if err != nil {
		t.Fatalf("DeleteExposure failed: %v", err)
	}

	// Case 2: Missing device deletion (idempotent success)
	driver.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("Error: The device doesn't exist")
	})

	err = driver.DeleteExposure(ctx, exp)
	if err != nil {
		t.Fatalf("DeleteExposure on missing device failed, expected idempotent success: %v", err)
	}
}

func TestIncusProxyDriverGetExposureAndDrift(t *testing.T) {
	ctx := context.Background()
	driver := NewIncusProxyDriver("")

	exp := &networking.NetworkExposure{
		ID:           "exp-01",
		WorkloadName: "test-nano",
		PublicIP:     "51.162.178.199",
		PublicPort:   2222,
		InternalIP:   "10.170.92.70",
		InternalPort: 22,
	}

	// Case 1: Active & In-Sync
	driver.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{
			"name":"test-nano",
			"devices":{
				"mystic-exp-exp-01": {
					"type": "proxy",
					"listen": "tcp:51.162.178.199:2222",
					"connect": "tcp:10.170.92.70:22",
					"nat": "true"
				}
			}
		}`), nil
	})

	status, err := driver.GetExposure(ctx, exp)
	if err != nil || !status.Active || status.State != hosts.ExposureStateApplied {
		t.Fatalf("Expected active applied exposure status, got active=%v, state=%s (err: %v)", status.Active, status.State, err)
	}

	// Case 2: Provider Missing (device absent)
	driver.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{"name":"test-nano","devices":{}}`), nil
	})

	statusMissing, err := driver.GetExposure(ctx, exp)
	if err != nil || statusMissing.Active {
		t.Fatalf("Expected inactive status for missing device, got active=%v (err: %v)", statusMissing.Active, err)
	}

	// Case 3: Configuration Drift (port mismatched)
	driver.SetExecRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{
			"name":"test-nano",
			"devices":{
				"mystic-exp-exp-01": {
					"type": "proxy",
					"listen": "tcp:51.162.178.199:3333",
					"connect": "tcp:10.170.92.70:22"
				}
			}
		}`), nil
	})

	statusDrift, err := driver.GetExposure(ctx, exp)
	if err != nil || statusDrift.Active {
		t.Fatalf("Expected inactive/drifted status when listen port differs, got active=%v", statusDrift.Active)
	}
}

func TestIncusProxyDriverRejectUDP(t *testing.T) {
	ctx := context.Background()
	driver := NewIncusProxyDriver("")

	exp := &networking.NetworkExposure{
		ID:           "exp-udp-01",
		WorkloadName: "test-nano",
		PublicPort:   5353,
		InternalPort: 53,
		Protocol:     hosts.ProtocolUDP,
	}

	err := driver.CreateExposure(ctx, exp)
	if err == nil || !errors.Is(err, ErrUnsupportedProtocol) {
		t.Fatalf("Expected ErrUnsupportedProtocol when requesting UDP exposure, got: %v", err)
	}
}

func TestIncusProxyDriverArgumentInjectionProtection(t *testing.T) {
	ctx := context.Background()
	driver := NewIncusProxyDriver("")

	// Malicious workload name attempting flag injection
	exp := &networking.NetworkExposure{
		ID:           "exp-01",
		WorkloadName: "--danger-flag",
		PublicPort:   8080,
		InternalPort: 80,
		Protocol:     hosts.ProtocolTCP,
	}

	err := driver.CreateExposure(ctx, exp)
	if err == nil || !strings.Contains(err.Error(), "option injection rejected") {
		t.Fatalf("Expected option injection error for workload name '--danger-flag', got: %v", err)
	}
}
