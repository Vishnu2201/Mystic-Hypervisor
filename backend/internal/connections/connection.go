package connections

import (
	"time"
)

// ConnectionProfile represents how an administrator or user connects to a service.
type ConnectionProfile struct {
	ID            string    `json:"id"`
	ServiceID     string    `json:"service_id"`
	WorkloadID    string    `json:"workload_id"`
	Label         string    `json:"label"`
	Protocol      string    `json:"protocol"`
	EndpointHost  string    `json:"endpoint_host"`
	EndpointPort  int       `json:"endpoint_port"`
	TargetUser    string    `json:"target_user,omitempty"`
	CredentialID  string    `json:"credential_id,omitempty"`
	ConnectionURL string    `json:"connection_url,omitempty"`
	CLICommand    string    `json:"cli_command,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
