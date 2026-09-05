package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
)

type mockExposureResolver struct {
	exposures map[string]*networking.NetworkExposure
}

func (m *mockExposureResolver) GetNetworkExposure(ctx context.Context, id string) (*networking.NetworkExposure, error) {
	exp, exists := m.exposures[id]
	if !exists {
		return nil, networking.ErrExposureNotFound
	}
	return exp, nil
}

func TestServiceCRUDAndAtomicPersistence(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "mystic-svc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "services.json")
	store := NewFileServiceStore(storePath)
	sm := NewServiceManager(store, nil)

	// 1. Create Service
	svc := &Service{
		WorkloadID:   "wl-01",
		WorkloadName: "test-workload",
		Name:         "SSH Access",
		Type:         ServiceTypeSSH,
		InternalIP:   "10.0.0.50",
		InternalPort: 22,
		Protocol:     hosts.ProtocolTCP,
	}

	created, err := sm.CreateService(ctx, svc)
	if err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("Expected non-empty ID")
	}

	// 2. Get Service
	fetched, err := sm.GetService(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetService failed: %v", err)
	}
	if fetched.Name != "SSH Access" {
		t.Fatalf("Expected name 'SSH Access', got '%s'", fetched.Name)
	}

	// 3. Round-trip Persistence Check
	sm2 := NewServiceManager(store, nil)
	loaded, err := sm2.GetService(ctx, created.ID)
	if err != nil {
		t.Fatalf("Failed to load service after store re-initialization: %v", err)
	}
	if loaded.InternalIP != "10.0.0.50" {
		t.Fatalf("Expected internal IP 10.0.0.50, got %s", loaded.InternalIP)
	}

	// 4. List Workload Services
	svcs, err := sm.ListWorkloadServices(ctx, "wl-01")
	if err != nil || len(svcs) != 1 {
		t.Fatalf("Expected 1 service for workload wl-01, got %d (err: %v)", len(svcs), err)
	}

	// 5. Delete Service
	err = sm.DeleteService(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteService failed: %v", err)
	}
	_, err = sm.GetService(ctx, created.ID)
	if err == nil {
		t.Fatalf("Expected error after deletion, got nil")
	}
}

func TestServiceValidationAndExposureCheck(t *testing.T) {
	ctx := context.Background()
	expRes := &mockExposureResolver{
		exposures: map[string]*networking.NetworkExposure{
			"exp-01": {
				ID:           "exp-01",
				WorkloadID:   "wl-01",
				PublicPort:   2222,
				InternalPort: 22,
				InternalIP:   "10.0.0.50",
				Protocol:     hosts.ProtocolTCP,
			},
			"exp-other": {
				ID:           "exp-other",
				WorkloadID:   "wl-other",
				PublicPort:   8080,
				InternalPort: 80,
				InternalIP:   "10.0.0.99",
				Protocol:     hosts.ProtocolTCP,
			},
		},
	}

	tempDir, _ := os.MkdirTemp("", "mystic-svc-val-*")
	defer os.RemoveAll(tempDir)
	sm := NewServiceManager(NewFileServiceStore(filepath.Join(tempDir, "svcs.json")), expRes)

	// Test 1: Invalid Service Type
	invType := &Service{WorkloadID: "wl-01", Name: "Bad Svc", Type: "INVALID_TYPE", InternalPort: 80}
	_, err := sm.CreateService(ctx, invType)
	if err == nil {
		t.Fatalf("Expected error for invalid service type")
	}

	// Test 2: Protocol mismatch for HTTP (UDP given)
	badProto := &Service{WorkloadID: "wl-01", Name: "HTTP Svc", Type: ServiceTypeHTTP, InternalPort: 80, Protocol: hosts.ProtocolUDP}
	_, err = sm.CreateService(ctx, badProto)
	if err == nil {
		t.Fatalf("Expected error for HTTP with UDP protocol")
	}

	// Test 3: Referenced exposure belongs to different workload
	diffWlExp := &Service{WorkloadID: "wl-01", Name: "SSH Svc", Type: ServiceTypeSSH, InternalPort: 22, ExposureID: "exp-other"}
	_, err = sm.CreateService(ctx, diffWlExp)
	if err == nil {
		t.Fatalf("Expected error for cross-workload exposure reference")
	}

	// Test 4: Valid service with matching exposure
	validSvc := &Service{WorkloadID: "wl-01", Name: "SSH Svc", Type: ServiceTypeSSH, InternalPort: 22, ExposureID: "exp-01"}
	created, err := sm.CreateService(ctx, validSvc)
	if err != nil {
		t.Fatalf("Expected valid service creation, got error: %v", err)
	}
	if !created.IsPublic {
		t.Fatalf("Expected IsPublic=true when exposure is linked")
	}
}
