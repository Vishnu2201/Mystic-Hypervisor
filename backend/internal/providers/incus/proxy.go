package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
)

var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol for Incus proxy device")
	ErrInvalidExposureID   = errors.New("invalid exposure ID")
	ErrInstanceNotFound    = errors.New("target Incus instance not found")
)

// IncusProxyDriver implements networking.ProviderExposureDriver using Incus native proxy devices.
type IncusProxyDriver struct {
	mu         sync.RWMutex
	socketPath string
	execRunner func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// NewIncusProxyDriver constructs an IncusProxyDriver.
func NewIncusProxyDriver(socketPath string) *IncusProxyDriver {
	if socketPath == "" {
		socketPath = "/var/lib/incus/unix.socket"
	}
	return &IncusProxyDriver{socketPath: socketPath}
}

// SetExecRunner allows injecting a custom command runner (primarily for unit testing offline).
func (d *IncusProxyDriver) SetExecRunner(runner func(ctx context.Context, name string, args ...string) ([]byte, error)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.execRunner = runner
}

func (d *IncusProxyDriver) runCmd(ctx context.Context, name string, args ...string) ([]byte, error) {
	d.mu.RLock()
	runner := d.execRunner
	d.mu.RUnlock()

	if runner != nil {
		return runner(ctx, name, args...)
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

// DeviceNameForExposure computes the deterministic Incus device name for an exposure ID.
func DeviceNameForExposure(exposureID string) string {
	cleaned := strings.ReplaceAll(exposureID, "/", "-")
	cleaned = strings.ReplaceAll(cleaned, " ", "-")
	return fmt.Sprintf("mystic-exp-%s", cleaned)
}

func resolveInstanceName(exp *networking.NetworkExposure) (string, error) {
	instName := strings.TrimSpace(exp.WorkloadName)
	if instName == "" {
		instName = strings.TrimSpace(exp.WorkloadID)
	}
	if instName == "" {
		return "", fmt.Errorf("target workload/instance name is required: %w", ErrInvalidExposureID)
	}
	// Sanitize against option injection
	if strings.HasPrefix(instName, "-") {
		return "", fmt.Errorf("invalid instance name '%s': option injection rejected", instName)
	}
	return instName, nil
}

type incusRawInstanceConfig struct {
	Name    string                       `json:"name"`
	Type    string                       `json:"type"`
	Devices map[string]map[string]string `json:"devices"`
}

func (d *IncusProxyDriver) getInstanceConfig(ctx context.Context, instName string) (*incusRawInstanceConfig, error) {
	output, err := d.runCmd(ctx, "incus", "config", "show", instName, "--expanded", "--format", "json")
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not Found") {
			return nil, ErrInstanceNotFound
		}
		return nil, fmt.Errorf("failed to query Incus instance config for '%s': %w", instName, err)
	}

	var raw incusRawInstanceConfig
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse Incus instance config JSON for '%s': %w", instName, err)
	}
	return &raw, nil
}

// CreateExposure applies or updates an Incus native proxy device for an exposure.
func (d *IncusProxyDriver) CreateExposure(ctx context.Context, exp *networking.NetworkExposure) error {
	if exp == nil {
		return fmt.Errorf("exposure configuration cannot be nil")
	}

	// Protocol Safety Rule: Reject UDP in Phase 3
	if exp.Protocol == hosts.ProtocolUDP {
		return fmt.Errorf("protocol UDP is not supported by Incus proxy devices in Phase 3: %w", ErrUnsupportedProtocol)
	}

	instName, err := resolveInstanceName(exp)
	if err != nil {
		return err
	}

	deviceName := DeviceNameForExposure(exp.ID)
	expectedListen := fmt.Sprintf("tcp:%s:%d", exp.PublicIP, exp.PublicPort)
	if exp.PublicIP == "" {
		expectedListen = fmt.Sprintf("tcp:0.0.0.0:%d", exp.PublicPort)
	}
	expectedConnect := fmt.Sprintf("tcp:%s:%d", exp.InternalIP, exp.InternalPort)

	// Idempotency check: Inspect existing instance configuration
	rawConfig, err := d.getInstanceConfig(ctx, instName)
	if err != nil {
		return err
	}

	if rawConfig.Devices != nil {
		if existingDev, exists := rawConfig.Devices[deviceName]; exists {
			devType := existingDev["type"]
			devListen := existingDev["listen"]
			devConnect := existingDev["connect"]
			devNat := existingDev["nat"]

			// If identical, return success immediately (no-op)
			if devType == "proxy" && (devListen == expectedListen || devListen == fmt.Sprintf("tcp::%d", exp.PublicPort)) && devConnect == expectedConnect && (devNat == "true" || devNat == "") {
				return nil
			}

			// Device exists but configuration differs -> update safely
			_, err = d.runCmd(ctx, "incus", "config", "device", "set", instName, deviceName, "listen="+expectedListen, "connect="+expectedConnect, "nat=true")
			if err != nil {
				return fmt.Errorf("failed to update Incus proxy device '%s' on '%s': %w", deviceName, instName, err)
			}
			return nil
		}
	}

	// Device does not exist -> create it
	_, err = d.runCmd(ctx, "incus", "config", "device", "add", instName, deviceName, "proxy", "listen="+expectedListen, "connect="+expectedConnect, "nat=true")
	if err != nil {
		return fmt.Errorf("failed to create Incus proxy device '%s' on '%s': %w", deviceName, instName, err)
	}

	return nil
}

// DeleteExposure removes the native Incus proxy device.
func (d *IncusProxyDriver) DeleteExposure(ctx context.Context, exp *networking.NetworkExposure) error {
	if exp == nil {
		return nil
	}

	instName, err := resolveInstanceName(exp)
	if err != nil {
		return nil // Invalid instance name cannot exist on Incus
	}

	deviceName := DeviceNameForExposure(exp.ID)

	_, err = d.runCmd(ctx, "incus", "config", "device", "remove", instName, deviceName)
	if err != nil {
		errMsg := err.Error()
		// Idempotent: treat missing device/instance as success
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "Not Found") || strings.Contains(errMsg, "doesn't exist") {
			return nil
		}
		return fmt.Errorf("failed to remove Incus proxy device '%s' from '%s': %w", deviceName, instName, err)
	}

	return nil
}

// GetExposure queries the actual Incus instance configuration and returns observed exposure status.
func (d *IncusProxyDriver) GetExposure(ctx context.Context, exp *networking.NetworkExposure) (*networking.NetworkExposureStatus, error) {
	if exp == nil {
		return &networking.NetworkExposureStatus{Active: false, State: hosts.ExposureStateUnconfigured}, nil
	}

	instName, err := resolveInstanceName(exp)
	if err != nil {
		return &networking.NetworkExposureStatus{Active: false, State: hosts.ExposureStateUnconfigured}, nil
	}

	deviceName := DeviceNameForExposure(exp.ID)
	rawConfig, err := d.getInstanceConfig(ctx, instName)
	if err != nil {
		return &networking.NetworkExposureStatus{Active: false, State: hosts.ExposureStateUnconfigured}, nil
	}

	if rawConfig.Devices == nil {
		return &networking.NetworkExposureStatus{Active: false, State: hosts.ExposureStateUnconfigured}, nil
	}

	dev, exists := rawConfig.Devices[deviceName]
	if !exists {
		return &networking.NetworkExposureStatus{Active: false, State: hosts.ExposureStateUnconfigured}, nil
	}

	devType := dev["type"]
	devListen := dev["listen"]
	devConnect := dev["connect"]

	expectedListen := fmt.Sprintf("tcp:%s:%d", exp.PublicIP, exp.PublicPort)
	if exp.PublicIP == "" {
		expectedListen = fmt.Sprintf("tcp:0.0.0.0:%d", exp.PublicPort)
	}
	expectedConnect := fmt.Sprintf("tcp:%s:%d", exp.InternalIP, exp.InternalPort)

	isActive := devType == "proxy" && (devListen == expectedListen || devListen == fmt.Sprintf("tcp::%d", exp.PublicPort)) && devConnect == expectedConnect
	if isActive {
		return &networking.NetworkExposureStatus{
			Active:    true,
			State:     hosts.ExposureStateApplied,
			IPAddress: exp.PublicIP,
			Port:      exp.PublicPort,
		}, nil
	}

	// Device exists but configuration differs (drifted)
	return &networking.NetworkExposureStatus{
		Active:    false,
		State:     hosts.ExposureStateUnconfigured,
		IPAddress: exp.PublicIP,
		Port:      exp.PublicPort,
	}, nil
}
