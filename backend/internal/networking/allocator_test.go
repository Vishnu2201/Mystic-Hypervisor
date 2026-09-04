package networking

import (
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
)

func TestAllocatorSinglePortValid(t *testing.T) {
	engine := NewAllocatorEngine()

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		Mode:              AllocationModeSingle,
		ExternalStartPort: 20022,
		InternalStartPort: 22,
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	workloads := []WorkloadNetworkConfig{
		{ID: "wl-lxc-01", WorkloadID: "wl-lxc-01", PrivateIPv4: "10.0.0.151"},
	}

	res := engine.ValidateAllocation(req, nil, nil, workloads, nil, nil)
	if !res.IsValid {
		t.Fatalf("Expected valid allocation, got blockers: %v", res.Blockers)
	}

	if len(res.NormalizedMappings) != 1 {
		t.Fatalf("Expected 1 normalized mapping, got %d", len(res.NormalizedMappings))
	}

	if res.NormalizedMappings[0].ExternalPort != 20022 || res.NormalizedMappings[0].InternalPort != 22 {
		t.Fatalf("Unexpected port mapping: %+v", res.NormalizedMappings[0])
	}
}

func TestAllocatorInvalidPortBounds(t *testing.T) {
	engine := NewAllocatorEngine()

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		Mode:              AllocationModeSingle,
		ExternalStartPort: 70000,
		InternalStartPort: 0,
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	res := engine.ValidateAllocation(req, nil, nil, nil, nil, nil)
	if res.IsValid {
		t.Fatalf("Expected invalid allocation due to port bounds out of 1-65535")
	}
}

func TestAllocatorRangeMismatchedSizes(t *testing.T) {
	engine := NewAllocatorEngine()

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		Mode:              AllocationModeRange,
		ExternalStartPort: 20000,
		ExternalEndPort:   20010, // 11 ports
		InternalStartPort: 3000,
		InternalEndPort:   3005, // 6 ports
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	res := engine.ValidateAllocation(req, nil, nil, nil, nil, nil)
	if res.IsValid {
		t.Fatalf("Expected invalid allocation due to mismatched range sizes")
	}
}

func TestAllocatorRangeReversed(t *testing.T) {
	engine := NewAllocatorEngine()

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		Mode:              AllocationModeRange,
		ExternalStartPort: 20010,
		ExternalEndPort:   20000,
		InternalStartPort: 3000,
		InternalEndPort:   3010,
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	res := engine.ValidateAllocation(req, nil, nil, nil, nil, nil)
	if res.IsValid {
		t.Fatalf("Expected invalid allocation due to reversed start/end range")
	}
}

func TestAllocatorDuplicateForwardingRule(t *testing.T) {
	engine := NewAllocatorEngine()

	existing := []hosts.ForwardingRule{
		{
			ID:           "rule-01",
			ExternalPort: 20022,
			InternalPort: 22,
			Protocol:     hosts.ProtocolTCP,
			WorkloadID:   "wl-other",
			Owner:        hosts.OwnerMystic,
		},
	}

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		Mode:              AllocationModeSingle,
		ExternalStartPort: 20022,
		InternalStartPort: 22,
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	res := engine.ValidateAllocation(req, nil, existing, nil, nil, nil)
	if res.IsValid {
		t.Fatalf("Expected allocation conflict due to duplicate external port 20022")
	}

	if res.Status != ConflictAlreadyAllocatedByMystic {
		t.Fatalf("Expected ConflictAlreadyAllocatedByMystic status, got %s", res.Status)
	}
}

func TestAllocatorManagementPortConflict(t *testing.T) {
	engine := NewAllocatorEngine()

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		Mode:              AllocationModeSingle,
		ExternalStartPort: 22,
		InternalStartPort: 22,
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	res := engine.ValidateAllocation(req, nil, nil, nil, nil, nil)
	// Management port adds a warning and sets status to RESERVED_MANAGEMENT
	if res.Status != ConflictReservedManagement {
		t.Fatalf("Expected status ConflictReservedManagement for SSH port 22, got %s", res.Status)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("Expected warning for SSH management port 22")
	}
}

func TestAllocatorUnconfiguredPool(t *testing.T) {
	engine := NewAllocatorEngine()

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		Mode:              AllocationModeRange,
		ExternalStartPort: 0, // Auto allocation from pool
		ExternalEndPort:   0,
		InternalStartPort: 3000,
		InternalEndPort:   3005,
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	res := engine.ValidateAllocation(req, nil, nil, nil, nil, nil)
	if res.IsValid {
		t.Fatalf("Expected failure due to unconfigured allocation pool")
	}

	if res.Status != ConflictPoolUnconfigured {
		t.Fatalf("Expected status ALLOCATION_POOL_UNCONFIGURED, got %s", res.Status)
	}
}

func TestAllocatorConfiguredPoolAutoAllocation(t *testing.T) {
	engine := NewAllocatorEngine()

	pool := &AllocationPool{
		ID:           "pool-main",
		Name:         "Default External Pool",
		StartPort:    20000,
		EndPort:      30000,
		Protocol:     hosts.ProtocolTCP,
		IsConfigured: true,
	}

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		Mode:              AllocationModeRange,
		ExternalStartPort: 0,
		ExternalEndPort:   0,
		InternalStartPort: 3000,
		InternalEndPort:   3002, // 3 ports
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	res := engine.ValidateAllocation(req, nil, nil, nil, nil, pool)
	if !res.IsValid {
		t.Fatalf("Expected valid pool allocation, got blockers: %v", res.Blockers)
	}

	if len(res.NormalizedMappings) != 3 {
		t.Fatalf("Expected 3 normalized mappings, got %d", len(res.NormalizedMappings))
	}

	if res.NormalizedMappings[0].ExternalPort != 20000 {
		t.Fatalf("Expected first auto port to be 20000, got %d", res.NormalizedMappings[0].ExternalPort)
	}
}

func TestAllocatorExternallyManagedGatewayWarning(t *testing.T) {
	engine := NewAllocatorEngine()

	gateways := []GatewayConfig{
		{
			ID:                   "gw-ext-01",
			Name:                 "Secondary VPS Gateway",
			Type:                 hosts.GatewayExternalVPS,
			ManagementCapability: GatewayExternallyManaged,
		},
	}

	req := PortAllocationRequest{
		WorkloadID:        "wl-lxc-01",
		HostID:            "host-main",
		GatewayID:         "gw-ext-01",
		Mode:              AllocationModeSingle,
		ExternalStartPort: 20022,
		InternalStartPort: 22,
		Protocol:          hosts.ProtocolTCP,
		DestinationIP:     "10.0.0.151",
	}

	res := engine.ValidateAllocation(req, nil, nil, nil, gateways, nil)
	if !res.IsValid {
		t.Fatalf("Expected valid allocation with warning, got blockers: %v", res.Blockers)
	}

	hasWarning := false
	for _, w := range res.Warnings {
		if len(w) > 0 {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Fatalf("Expected warning for externally managed gateway")
	}
}
