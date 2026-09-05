package services

import (
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
)

// ServiceType defines supported application service endpoint types.
type ServiceType string

const (
	ServiceTypeSSH     ServiceType = "SSH"
	ServiceTypeHTTP    ServiceType = "HTTP"
	ServiceTypeHTTPS   ServiceType = "HTTPS"
	ServiceTypeTCP     ServiceType = "TCP"
	ServiceTypeUDP     ServiceType = "UDP"
	ServiceTypeConsole ServiceType = "CONSOLE"
)

// Service represents an application service endpoint associated with a workload.
type Service struct {
	ID           string               `json:"id"`
	WorkloadID   string               `json:"workload_id"`
	WorkloadName string               `json:"workload_name,omitempty"`
	Name         string               `json:"name"`
	Type         ServiceType          `json:"type"`
	InternalIP   string               `json:"internal_ip"`
	InternalPort int                  `json:"internal_port"`
	Protocol     hosts.Protocol       `json:"protocol"`
	ExposureID   string               `json:"exposure_id,omitempty"`
	DesiredState string               `json:"desired_state"`
	ActualState  string               `json:"actual_state"`
	SyncStatus   instances.SyncStatus `json:"sync_status"`
	IsPublic     bool                 `json:"is_public"`
	Description  string               `json:"description,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}
