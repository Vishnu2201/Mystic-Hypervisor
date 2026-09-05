package networking

import (
	"context"
	"fmt"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
)

// NetworkExposure represents a first-class network exposure and port-forwarding rule.
type NetworkExposure struct {
	ID           string               `json:"id"`
	WorkloadID   string               `json:"workload_id"`
	WorkloadName string               `json:"workload_name,omitempty"`
	GatewayID    string               `json:"gateway_id,omitempty"`
	ExposureMode hosts.ExposureMode   `json:"exposure_mode"`
	PublicIP     string               `json:"public_ip,omitempty"`
	PublicPort   int                  `json:"public_port"`
	InternalIP   string               `json:"internal_ip"`
	InternalPort int                  `json:"internal_port"`
	Protocol     hosts.Protocol       `json:"protocol"`
	DesiredState hosts.ExposureState  `json:"desired_state"`
	ActualState  hosts.ExposureState  `json:"actual_state"`
	SyncStatus   instances.SyncStatus `json:"sync_status"`
	Description  string               `json:"description,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
	LastSync     time.Time            `json:"last_sync,omitempty"`
}

// NetworkExposureStatus summarizes observed provider-side exposure runtime facts.
type NetworkExposureStatus struct {
	Active       bool                `json:"active"`
	State        hosts.ExposureState `json:"state"`
	IPAddress    string              `json:"ip_address,omitempty"`
	Port         int                 `json:"port,omitempty"`
	DeviceName   string              `json:"device_name,omitempty"`
	DeviceType   string              `json:"device_type,omitempty"`
	Listen       string              `json:"listen,omitempty"`
	Connect      string              `json:"connect,omitempty"`
	NAT          string              `json:"nat,omitempty"`
	InstanceName string              `json:"instance_name,omitempty"`
	RawDevice    map[string]string   `json:"raw_device,omitempty"`
}

// ProviderExposureDriver is an optional interface implemented by virtualization drivers
// to manage and inspect native port forwarding devices or rules.
type ProviderExposureDriver interface {
	CreateExposure(ctx context.Context, exp *NetworkExposure) error
	DeleteExposure(ctx context.Context, exp *NetworkExposure) error
	GetExposure(ctx context.Context, exp *NetworkExposure) (*NetworkExposureStatus, error)
}

// ConvertForwardingRuleToExposure converts a legacy hosts.ForwardingRule to a NetworkExposure.
func ConvertForwardingRuleToExposure(rule hosts.ForwardingRule, workloadID, workloadName string, mode hosts.ExposureMode) *NetworkExposure {
	pubPort := rule.ExternalPort
	if pubPort == 0 {
		pubPort = rule.PublicPort
	}
	intPort := rule.InternalPort
	if intPort == 0 {
		intPort = rule.DestinationPort
	}
	intIP := rule.InternalIP
	if intIP == "" {
		intIP = rule.DestinationIP
	}

	id := rule.ID
	if id == "" {
		id = fmt.Sprintf("exp-%s-%d", workloadID, pubPort)
	}

	desired := rule.State
	if desired == "" {
		desired = hosts.ExposureStateConfigured
	}

	return &NetworkExposure{
		ID:           id,
		WorkloadID:   workloadID,
		WorkloadName: workloadName,
		GatewayID:    rule.GatewayID,
		ExposureMode: mode,
		PublicIP:     rule.PublicIP,
		PublicPort:   pubPort,
		InternalIP:   intIP,
		InternalPort: intPort,
		Protocol:     rule.Protocol,
		DesiredState: desired,
		ActualState:  hosts.ExposureStateApplied,
		SyncStatus:   instances.SyncInSync,
		Description:  rule.Description,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// ToForwardingRule converts a NetworkExposure to a legacy hosts.ForwardingRule for backwards compatibility.
func (ne *NetworkExposure) ToForwardingRule() hosts.ForwardingRule {
	return hosts.ForwardingRule{
		ID:                ne.ID,
		GatewayID:         ne.GatewayID,
		PublicIP:          ne.PublicIP,
		PublicPort:        ne.PublicPort,
		Protocol:          ne.Protocol,
		DestinationHostID: "",
		DestinationIP:     ne.InternalIP,
		DestinationPort:   ne.InternalPort,
		WorkloadID:        ne.WorkloadID,
		State:             ne.DesiredState,
		Owner:             hosts.OwnerMystic,
		Description:       ne.Description,
		ExternalPort:      ne.PublicPort,
		InternalIP:        ne.InternalIP,
		InternalPort:      ne.InternalPort,
	}
}
