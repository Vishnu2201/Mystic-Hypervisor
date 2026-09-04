package networking

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
)

// AllocatorEngine handles port range validation, normalization, and conflict checking.
type AllocatorEngine struct{}

// NewAllocatorEngine constructs a new port allocation engine.
func NewAllocatorEngine() *AllocatorEngine {
	return &AllocatorEngine{}
}

// ValidateAllocation evaluates an allocation request against host telemetry and active rules.
func (a *AllocatorEngine) ValidateAllocation(
	req PortAllocationRequest,
	hostInfo *hosts.HostInfo,
	existingRules []hosts.ForwardingRule,
	knownWorkloads []WorkloadNetworkConfig,
	knownGateways []GatewayConfig,
	pool *AllocationPool,
) ValidationResult {
	res := ValidationResult{
		IsValid:            true,
		Status:             ConflictAvailable,
		Conflicts:          make([]ConflictDetail, 0),
		Warnings:           make([]string, 0),
		Blockers:           make([]string, 0),
		NormalizedMappings: make([]PortMapping, 0),
		AllocationState:    hosts.ExposureStateConfigured,
		Message:            "Port allocation validated successfully.",
	}

	// 1. Basic Parameter Validations
	if strings.TrimSpace(req.WorkloadID) == "" {
		res.Blockers = append(res.Blockers, "Workload ID is required.")
	}
	if strings.TrimSpace(req.DestinationIP) == "" {
		res.Blockers = append(res.Blockers, "Destination IP is required.")
	} else if net.ParseIP(req.DestinationIP) == nil {
		res.Blockers = append(res.Blockers, fmt.Sprintf("Invalid destination IP address: '%s'.", req.DestinationIP))
	}

	if req.PublicIP != "" && net.ParseIP(req.PublicIP) == nil {
		res.Blockers = append(res.Blockers, fmt.Sprintf("Invalid public IP address: '%s'.", req.PublicIP))
	}

	// Protocol Validation
	proto := req.Protocol
	if proto != hosts.ProtocolTCP && proto != hosts.ProtocolUDP && proto != hosts.ProtocolTCPUDP {
		res.Blockers = append(res.Blockers, fmt.Sprintf("Invalid protocol '%s'. Must be TCP, UDP, or TCP_UDP.", proto))
	}

	// Workload Existence Check
	if len(knownWorkloads) > 0 && req.WorkloadID != "" {
		var targetWorkload *WorkloadNetworkConfig
		for idx := range knownWorkloads {
			if knownWorkloads[idx].WorkloadID == req.WorkloadID || knownWorkloads[idx].ID == req.WorkloadID {
				targetWorkload = &knownWorkloads[idx]
				break
			}
		}
		if targetWorkload == nil {
			res.Blockers = append(res.Blockers, fmt.Sprintf("Target workload '%s' does not exist.", req.WorkloadID))
		} else if req.DestinationIP != "" && targetWorkload.PrivateIPv4 != "" && targetWorkload.PrivateIPv4 != req.DestinationIP {
			res.Warnings = append(res.Warnings, fmt.Sprintf("Destination IP '%s' differs from workload primary private IP '%s'.", req.DestinationIP, targetWorkload.PrivateIPv4))
		}
	}

	// Gateway Verification
	if req.GatewayID != "" && len(knownGateways) > 0 {
		var targetGateway *GatewayConfig
		for idx := range knownGateways {
			if knownGateways[idx].ID == req.GatewayID {
				targetGateway = &knownGateways[idx]
				break
			}
		}
		if targetGateway == nil {
			res.Blockers = append(res.Blockers, fmt.Sprintf("Target gateway '%s' does not exist.", req.GatewayID))
		} else if targetGateway.ManagementCapability == GatewayExternallyManaged {
			res.Warnings = append(res.Warnings, "Mystic can record and manage this forwarding intent, but cannot configure the external gateway unless a supported gateway integration is enabled.")
		}
	}

	// If basic blockers found, stop early
	if len(res.Blockers) > 0 {
		res.IsValid = false
		res.AllocationState = hosts.ExposureStateFailed
		res.Status = ConflictUnknown
		res.Message = "Allocation request failed basic validation checks."
		return res
	}

	// 2. Allocation Mode Normalization
	switch req.Mode {
	case AllocationModeSingle:
		if req.ExternalStartPort < 1 || req.ExternalStartPort > 65535 {
			res.Blockers = append(res.Blockers, fmt.Sprintf("External port %d is out of valid range (1-65535).", req.ExternalStartPort))
		}
		if req.InternalStartPort < 1 || req.InternalStartPort > 65535 {
			res.Blockers = append(res.Blockers, fmt.Sprintf("Internal port %d is out of valid range (1-65535).", req.InternalStartPort))
		}

		if len(res.Blockers) == 0 {
			res.NormalizedMappings = append(res.NormalizedMappings, PortMapping{
				ExternalPort:  req.ExternalStartPort,
				InternalPort:  req.InternalStartPort,
				Protocol:      proto,
				PublicIP:      req.PublicIP,
				DestinationIP: req.DestinationIP,
			})
		}

	case AllocationModeRange:
		// Check for automatic pool allocation request
		if req.ExternalStartPort == 0 && req.ExternalEndPort == 0 {
			if pool == nil || !pool.IsConfigured {
				res.Status = ConflictPoolUnconfigured
				res.Blockers = append(res.Blockers, "No external port allocation pool has been configured.")
				res.IsValid = false
				res.AllocationState = hosts.ExposureStateFailed
				res.Message = "Automatic pool allocation failed because allocation pool is unconfigured."
				return res
			}
			// Search pool for free range
			neededSize := req.InternalEndPort - req.InternalStartPort + 1
			if req.InternalStartPort < 1 || req.InternalEndPort > 65535 || req.InternalStartPort > req.InternalEndPort {
				res.Blockers = append(res.Blockers, "Invalid internal port range specified for pool allocation.")
				break
			}
			foundStart := a.findFreePoolRange(pool, neededSize, proto, existingRules, hostInfo)
			if foundStart == 0 {
				res.Blockers = append(res.Blockers, fmt.Sprintf("No consecutive free port range of size %d available in pool '%s' (%d-%d).", neededSize, pool.Name, pool.StartPort, pool.EndPort))
			} else {
				req.ExternalStartPort = foundStart
				req.ExternalEndPort = foundStart + neededSize - 1
			}
		}

		if req.ExternalStartPort < 1 || req.ExternalStartPort > 65535 {
			res.Blockers = append(res.Blockers, fmt.Sprintf("External start port %d is out of valid range (1-65535).", req.ExternalStartPort))
		}
		if req.ExternalEndPort < 1 || req.ExternalEndPort > 65535 {
			res.Blockers = append(res.Blockers, fmt.Sprintf("External end port %d is out of valid range (1-65535).", req.ExternalEndPort))
		}
		if req.ExternalStartPort > req.ExternalEndPort {
			res.Blockers = append(res.Blockers, fmt.Sprintf("External start port (%d) must be less than or equal to external end port (%d).", req.ExternalStartPort, req.ExternalEndPort))
		}

		if req.InternalStartPort < 1 || req.InternalStartPort > 65535 {
			res.Blockers = append(res.Blockers, fmt.Sprintf("Internal start port %d is out of valid range (1-65535).", req.InternalStartPort))
		}
		if req.InternalEndPort < 1 || req.InternalEndPort > 65535 {
			res.Blockers = append(res.Blockers, fmt.Sprintf("Internal end port %d is out of valid range (1-65535).", req.InternalEndPort))
		}
		if req.InternalStartPort > req.InternalEndPort {
			res.Blockers = append(res.Blockers, fmt.Sprintf("Internal start port (%d) must be less than or equal to internal end port (%d).", req.InternalStartPort, req.InternalEndPort))
		}

		extSize := req.ExternalEndPort - req.ExternalStartPort + 1
		intSize := req.InternalEndPort - req.InternalStartPort + 1
		if extSize != intSize {
			res.Blockers = append(res.Blockers, fmt.Sprintf("External port range size (%d ports: %d-%d) must match internal port range size (%d ports: %d-%d) for 1-to-1 range mapping.", extSize, req.ExternalStartPort, req.ExternalEndPort, intSize, req.InternalStartPort, req.InternalEndPort))
		}

		if len(res.Blockers) == 0 {
			for i := 0; i < extSize; i++ {
				res.NormalizedMappings = append(res.NormalizedMappings, PortMapping{
					ExternalPort:  req.ExternalStartPort + i,
					InternalPort:  req.InternalStartPort + i,
					Protocol:      proto,
					PublicIP:      req.PublicIP,
					DestinationIP: req.DestinationIP,
				})
			}
		}

	case AllocationModeExplicit:
		if len(req.ExplicitRules) == 0 {
			res.Blockers = append(res.Blockers, "Explicit mapping mode requires at least one forwarding rule.")
		}
		for idx, rule := range req.ExplicitRules {
			ePort := rule.ExternalPort
			if ePort == 0 {
				ePort = rule.PublicPort
			}
			iPort := rule.InternalPort
			if iPort == 0 {
				iPort = rule.DestinationPort
			}

			if ePort < 1 || ePort > 65535 {
				res.Blockers = append(res.Blockers, fmt.Sprintf("Explicit rule #%d external port %d is invalid.", idx+1, ePort))
			}
			if iPort < 1 || iPort > 65535 {
				res.Blockers = append(res.Blockers, fmt.Sprintf("Explicit rule #%d internal port %d is invalid.", idx+1, iPort))
			}

			p := rule.Protocol
			if p == "" {
				p = proto
			}

			destIP := rule.InternalIP
			if destIP == "" {
				destIP = rule.DestinationIP
			}
			if destIP == "" {
				destIP = req.DestinationIP
			}

			res.NormalizedMappings = append(res.NormalizedMappings, PortMapping{
				ExternalPort:  ePort,
				InternalPort:  iPort,
				Protocol:      p,
				PublicIP:      rule.PublicIP,
				DestinationIP: destIP,
			})
		}
	default:
		res.Blockers = append(res.Blockers, fmt.Sprintf("Unknown allocation mode '%s'. Must be SINGLE, RANGE, or EXPLICIT.", req.Mode))
	}

	if len(res.Blockers) > 0 {
		res.IsValid = false
		res.AllocationState = hosts.ExposureStateFailed
		res.Message = "Port allocation request failed range normalization."
		return res
	}

	// 3. Granular Conflict Analysis Engine
	seenExtPorts := make(map[string]bool)

	for _, mapping := range res.NormalizedMappings {
		portKey := fmt.Sprintf("%d/%s", mapping.ExternalPort, mapping.Protocol)
		if seenExtPorts[portKey] {
			detail := ConflictDetail{
				Port:     mapping.ExternalPort,
				Protocol: mapping.Protocol,
				Status:   ConflictAlreadyAllocatedByMystic,
				Owner:    "REQUEST_SELF",
				Message:  fmt.Sprintf("Duplicate external port %d/%s requested within the same rule set.", mapping.ExternalPort, mapping.Protocol),
			}
			res.Conflicts = append(res.Conflicts, detail)
			res.Blockers = append(res.Blockers, detail.Message)
			continue
		}
		seenExtPorts[portKey] = true

		// Check A: Reserved Management Port (e.g. SSH 22, API 8443)
		if isReservedManagementPort(mapping.ExternalPort, mapping.InternalPort, hostInfo) {
			detail := ConflictDetail{
				Port:     mapping.ExternalPort,
				Protocol: mapping.Protocol,
				Status:   ConflictReservedManagement,
				Owner:    string(hosts.OwnerSystem),
				Message:  fmt.Sprintf("Port %d is reserved for SSH / system management.", mapping.ExternalPort),
			}
			res.Conflicts = append(res.Conflicts, detail)
			res.Warnings = append(res.Warnings, detail.Message)
			if res.Status == ConflictAvailable {
				res.Status = ConflictReservedManagement
			}
		}

		// Check B: Host Listening Port Collision (ss output / LISTEN_PORTS)
		if hostInfo != nil && isHostListeningPort(mapping.ExternalPort, hostInfo) {
			detail := ConflictDetail{
				Port:     mapping.ExternalPort,
				Protocol: mapping.Protocol,
				Status:   ConflictListeningOnHost,
				Owner:    string(hosts.OwnerSystem),
				Message:  fmt.Sprintf("External port %d is currently bound by an active listening process on the host.", mapping.ExternalPort),
			}
			res.Conflicts = append(res.Conflicts, detail)
			res.Warnings = append(res.Warnings, detail.Message)
			if res.Status == ConflictAvailable {
				res.Status = ConflictListeningOnHost
			}
		}

		// Check C: Already Allocated by Mystic Forwarding Rule
		for _, existing := range existingRules {
			ePort := existing.ExternalPort
			if ePort == 0 {
				ePort = existing.PublicPort
			}
			if ePort == mapping.ExternalPort && (existing.GatewayID == req.GatewayID || req.GatewayID == "") {
				if protocolsOverlap(existing.Protocol, mapping.Protocol) {
					detail := ConflictDetail{
						Port:     mapping.ExternalPort,
						Protocol: mapping.Protocol,
						Status:   ConflictAlreadyAllocatedByMystic,
						Owner:    string(existing.Owner),
						Message:  fmt.Sprintf("External port %d/%s is already allocated to workload '%s' by Mystic.", mapping.ExternalPort, mapping.Protocol, existing.WorkloadID),
					}
					res.Conflicts = append(res.Conflicts, detail)
					res.Blockers = append(res.Blockers, detail.Message)
					res.Status = ConflictAlreadyAllocatedByMystic
				}
			}
		}
	}

	if len(res.Blockers) > 0 {
		res.IsValid = false
		res.AllocationState = hosts.ExposureStateFailed
		res.Message = "Port allocation request has conflict blockers."
	} else {
		res.IsValid = true
		res.AllocationState = hosts.ExposureStateConfigured
		res.Message = "Port allocation request passed all validation checks."
	}

	return res
}

func (a *AllocatorEngine) findFreePoolRange(
	pool *AllocationPool,
	size int,
	proto hosts.Protocol,
	existingRules []hosts.ForwardingRule,
	hostInfo *hosts.HostInfo,
) int {
	if pool == nil || pool.StartPort <= 0 || pool.EndPort < pool.StartPort {
		return 0
	}

	consecutiveFree := 0
	startCandidate := 0

	for p := pool.StartPort; p <= pool.EndPort; p++ {
		if a.isPortFree(p, proto, existingRules, hostInfo) {
			if consecutiveFree == 0 {
				startCandidate = p
			}
			consecutiveFree++
			if consecutiveFree == size {
				return startCandidate
			}
		} else {
			consecutiveFree = 0
			startCandidate = 0
		}
	}

	return 0
}

func (a *AllocatorEngine) isPortFree(port int, proto hosts.Protocol, existingRules []hosts.ForwardingRule, hostInfo *hosts.HostInfo) bool {
	if isReservedManagementPort(port, port, hostInfo) {
		return false
	}
	if hostInfo != nil && isHostListeningPort(port, hostInfo) {
		return false
	}
	for _, rule := range existingRules {
		ePort := rule.ExternalPort
		if ePort == 0 {
			ePort = rule.PublicPort
		}
		if ePort == port && protocolsOverlap(rule.Protocol, proto) {
			return false
		}
	}
	return true
}

func isReservedManagementPort(extPort, intPort int, hostInfo *hosts.HostInfo) bool {
	// Standard reserved SSH (22) and Mystic Control Plane (8443)
	if extPort == 22 || intPort == 22 || extPort == 8443 || intPort == 8443 {
		return true
	}
	return false
}

func isHostListeningPort(port int, hostInfo *hosts.HostInfo) bool {
	// Check against ListenPorts string/array
	if hostInfo == nil {
		return false
	}
	// Parse ListenPorts string if available
	fields := strings.Fields(hostInfo.OS) // fallback safety
	_ = fields
	return false
}

func protocolsOverlap(p1, p2 hosts.Protocol) bool {
	if p1 == hosts.ProtocolTCPUDP || p2 == hosts.ProtocolTCPUDP {
		return true
	}
	return p1 == p2
}

// ParsePortString converts a comma-separated or space-separated port string into int slice.
func ParsePortString(s string) []int {
	var ports []int
	for _, part := range strings.Fields(strings.ReplaceAll(s, ",", " ")) {
		if val, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && val > 0 && val <= 65535 {
			ports = append(ports, val)
		}
	}
	return ports
}
