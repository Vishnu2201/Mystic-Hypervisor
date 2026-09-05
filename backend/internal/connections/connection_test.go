package connections

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
	"github.com/mystic-hypervisor/mystic/backend/internal/services"
)

func TestConnectionProfileGenerationSSH(t *testing.T) {
	cm := NewConnectionManager(nil, nil, nil)

	// Case 1: Exposed SSH Service on custom port 2222
	sshSvc := &services.Service{
		ID:           "svc-ssh-01",
		WorkloadID:   "wl-01",
		WorkloadName: "test-nano",
		Name:         "SSH Access",
		Type:         services.ServiceTypeSSH,
		InternalIP:   "10.0.0.50",
		InternalPort: 22,
		ExposureID:   "exp-01",
	}

	exp := &networking.NetworkExposure{
		ID:           "exp-01",
		WorkloadID:   "wl-01",
		PublicIP:     "51.162.178.199",
		PublicPort:   2222,
		InternalIP:   "10.0.0.50",
		InternalPort: 22,
		Protocol:     hosts.ProtocolTCP,
	}

	profile, err := cm.GenerateConnectionProfile(sshSvc, exp, "root", "")
	if err != nil {
		t.Fatalf("GenerateConnectionProfile failed: %v", err)
	}

	if profile.EndpointHost != "51.162.178.199" || profile.EndpointPort != 2222 {
		t.Fatalf("Expected public endpoint 51.162.178.199:2222, got %s:%d", profile.EndpointHost, profile.EndpointPort)
	}

	expectedURL := "ssh://root@51.162.178.199:2222"
	if profile.ConnectionURL != expectedURL {
		t.Fatalf("Expected URL '%s', got '%s'", expectedURL, profile.ConnectionURL)
	}

	expectedCLI := "ssh -p 2222 root@51.162.178.199"
	if profile.CLICommand != expectedCLI {
		t.Fatalf("Expected CLI '%s', got '%s'", expectedCLI, profile.CLICommand)
	}

	// Case 2: Missing target user for SSH must fail
	_, err = cm.GenerateConnectionProfile(sshSvc, exp, "", "")
	if err == nil {
		t.Fatalf("Expected error when SSH target user is missing")
	}

	// Case 3: Standard SSH port 22
	exp22 := &networking.NetworkExposure{
		ID:           "exp-22",
		WorkloadID:   "wl-01",
		PublicIP:     "51.162.178.199",
		PublicPort:   22,
		InternalIP:   "10.0.0.50",
		InternalPort: 22,
	}
	p22, err := cm.GenerateConnectionProfile(sshSvc, exp22, "mysticadmin", "")
	if err != nil {
		t.Fatalf("Unexpected error for port 22 SSH: %v", err)
	}
	if p22.CLICommand != "ssh mysticadmin@51.162.178.199" {
		t.Fatalf("Expected CLI 'ssh mysticadmin@51.162.178.199', got '%s'", p22.CLICommand)
	}
}

func TestConnectionProfileGenerationHTTPAndHTTPS(t *testing.T) {
	cm := NewConnectionManager(nil, nil, nil)

	// HTTP Internal
	httpSvc := &services.Service{
		ID:           "svc-http-01",
		WorkloadID:   "wl-01",
		Name:         "Web App",
		Type:         services.ServiceTypeHTTP,
		InternalIP:   "10.0.0.50",
		InternalPort: 8080,
	}

	httpProf, err := cm.GenerateConnectionProfile(httpSvc, nil, "", "")
	if err != nil {
		t.Fatalf("GenerateConnectionProfile HTTP failed: %v", err)
	}
	if httpProf.ConnectionURL != "http://10.0.0.50:8080" {
		t.Fatalf("Expected http://10.0.0.50:8080, got %s", httpProf.ConnectionURL)
	}

	// HTTPS Exposed
	httpsSvc := &services.Service{
		ID:           "svc-https-01",
		WorkloadID:   "wl-01",
		Name:         "Secure Portal",
		Type:         services.ServiceTypeHTTPS,
		InternalIP:   "10.0.0.50",
		InternalPort: 443,
		ExposureID:   "exp-https",
	}

	expHTTPS := &networking.NetworkExposure{
		ID:         "exp-https",
		WorkloadID: "wl-01",
		PublicIP:   "51.162.178.199",
		PublicPort: 443,
	}

	httpsProf, err := cm.GenerateConnectionProfile(httpsSvc, expHTTPS, "", "")
	if err != nil {
		t.Fatalf("GenerateConnectionProfile HTTPS failed: %v", err)
	}
	if httpsProf.ConnectionURL != "https://51.162.178.199" {
		t.Fatalf("Expected https://51.162.178.199, got %s", httpsProf.ConnectionURL)
	}
}

func TestConnectionProfileGenerationTCPUDPConsole(t *testing.T) {
	cm := NewConnectionManager(nil, nil, nil)

	// TCP Database
	tcpSvc := &services.Service{
		ID:           "svc-tcp-01",
		WorkloadID:   "wl-01",
		Name:         "PostgreSQL",
		Type:         services.ServiceTypeTCP,
		InternalIP:   "10.0.0.50",
		InternalPort: 5432,
	}
	tcpProf, err := cm.GenerateConnectionProfile(tcpSvc, nil, "", "")
	if err != nil {
		t.Fatalf("Generate TCP failed: %v", err)
	}
	if !strings.HasPrefix(tcpProf.ConnectionURL, "tcp://") || !strings.HasPrefix(tcpProf.CLICommand, "nc -zv") {
		t.Fatalf("Unexpected TCP connection outputs: %s / %s", tcpProf.ConnectionURL, tcpProf.CLICommand)
	}

	// CONSOLE
	consoleSvc := &services.Service{
		ID:           "svc-console-01",
		WorkloadID:   "wl-01",
		WorkloadName: "test-nano",
		Name:         "System Console",
		Type:         services.ServiceTypeConsole,
	}
	cProf, err := cm.GenerateConnectionProfile(consoleSvc, nil, "", "")
	if err != nil {
		t.Fatalf("Generate CONSOLE failed: %v", err)
	}
	if cProf.ConnectionURL != "" {
		t.Fatalf("Expected empty URL state for CONSOLE, got '%s'", cProf.ConnectionURL)
	}
	if cProf.CLICommand != "incus console test-nano" {
		t.Fatalf("Expected 'incus console test-nano', got '%s'", cProf.CLICommand)
	}
}

func TestConnectionProfileCRUDAndAtomicStore(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "mystic-conn-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "connection_profiles.json")
	store := NewFileConnectionStore(storePath)
	cm := NewConnectionManager(store, nil, nil)

	prof := &ConnectionProfile{
		ServiceID:     "svc-01",
		WorkloadID:    "wl-01",
		Label:         "SSH (root)",
		Protocol:      "SSH",
		EndpointHost:  "51.162.178.199",
		EndpointPort:  2222,
		TargetUser:    "root",
		ConnectionURL: "ssh://root@51.162.178.199:2222",
		CLICommand:    "ssh -p 2222 root@51.162.178.199",
	}

	created, err := cm.CreateConnectionProfile(ctx, prof)
	if err != nil {
		t.Fatalf("CreateConnectionProfile failed: %v", err)
	}

	// Reload store
	cm2 := NewConnectionManager(store, nil, nil)
	loaded, err := cm2.GetConnectionProfile(ctx, created.ID)
	if err != nil {
		t.Fatalf("Failed to load connection profile across store reload: %v", err)
	}
	if loaded.CLICommand != "ssh -p 2222 root@51.162.178.199" {
		t.Fatalf("Unexpected CLI command in loaded store: %s", loaded.CLICommand)
	}

	// List service connections
	list, err := cm2.ListServiceConnections(ctx, "svc-01")
	if err != nil || len(list) != 1 {
		t.Fatalf("Expected 1 connection for svc-01, got %d (err: %v)", len(list), err)
	}

	// Delete connection
	err = cm2.DeleteConnectionProfile(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteConnectionProfile failed: %v", err)
	}
}
