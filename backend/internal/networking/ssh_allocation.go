package networking

import (
	"fmt"
	"os"
	"time"
)

const (
	SSHPortMin         = 22100
	SSHPortMax         = 22200
	SSHTotalPorts      = 101
	DefaultSSHHost     = "ssh.mysticservers.com"
	DefaultIncusHostIP = "100.67.231.100"
)

// SSHAllocationStatus defines the lifecycle status of a public SSH port allocation.
type SSHAllocationStatus string

const (
	SSHStatusAllocated    SSHAllocationStatus = "ALLOCATED"
	SSHStatusProvisioning SSHAllocationStatus = "PROVISIONING"
	SSHStatusActive       SSHAllocationStatus = "ACTIVE"
	SSHStatusReleasing    SSHAllocationStatus = "RELEASING"
	SSHStatusReleased     SSHAllocationStatus = "RELEASED"
	SSHStatusFailed       SSHAllocationStatus = "FAILED"
)

// SSHAllocation represents a dedicated public TCP SSH port assigned to a VPS.
type SSHAllocation struct {
	ID               string              `json:"id"`
	PublicPort       int                 `json:"public_port"`
	Protocol         string              `json:"protocol"`
	Purpose          string              `json:"purpose"`
	WorkloadID       string              `json:"workload_id"`
	InstanceID       string              `json:"instance_id"`
	InstanceName     string              `json:"instance_name"`
	IncusHostID      string              `json:"incus_host_id"`
	IncusHostAddress string              `json:"incus_host_address"`
	PublicSSHHost    string              `json:"public_ssh_host"`
	PrivateIP        string              `json:"private_ip"`
	DestinationPort  int                 `json:"destination_port"`
	Status           SSHAllocationStatus `json:"status"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	ReleasedAt       *time.Time          `json:"released_at,omitempty"`
}

// SSHAccessInfo represents the public SSH connection endpoint details for client exposure.
type SSHAccessInfo struct {
	Host              string              `json:"host"`
	Port              int                 `json:"port"`
	Username          string              `json:"username"`
	Protocol          string              `json:"protocol"`
	Status            SSHAllocationStatus `json:"status"`
	AllocationID      string              `json:"allocation_id"`
	IncusHostID       string              `json:"incus_host_id"`
	ConnectionCommand string              `json:"connection_command"`
}

// GetPublicSSHHost returns the configured public SSH hostname (default: ssh.mysticservers.com).
func GetPublicSSHHost() string {
	if envHost := os.Getenv("MYSTIC_PUBLIC_SSH_HOST"); envHost != "" {
		return envHost
	}
	return DefaultSSHHost
}

// GetIncusHostAddress returns the configured Incus host address (default: 100.67.231.100).
func GetIncusHostAddress() string {
	if envAddr := os.Getenv("MYSTIC_INCUS_HOST_ADDRESS"); envAddr != "" {
		return envAddr
	}
	return DefaultIncusHostIP
}

// GenerateSSHConnectionCommand formats a standard SSH CLI command string.
func GenerateSSHConnectionCommand(host string, port int, user string) string {
	if user == "" {
		user = "root"
	}
	if host == "" {
		host = GetPublicSSHHost()
	}
	return fmt.Sprintf("ssh -p %d %s@%s", port, user, host)
}
