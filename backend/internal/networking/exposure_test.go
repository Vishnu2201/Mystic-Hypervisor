package networking

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/instances"
)

func TestNetworkExposureJSONSerialization(t *testing.T) {
	exp := &NetworkExposure{
		ID:           "exp-wl-101-2222",
		WorkloadID:   "wl-101",
		WorkloadName: "test-nano",
		ExposureMode: hosts.ExposureNATForwarded,
		PublicIP:     "51.162.178.199",
		PublicPort:   2222,
		InternalIP:   "10.170.92.70",
		InternalPort: 22,
		Protocol:     hosts.ProtocolTCP,
		DesiredState: hosts.ExposureStateConfigured,
		ActualState:  hosts.ExposureStateApplied,
		SyncStatus:   instances.SyncInSync,
		Description:  "SSH Exposure",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("Failed to marshal NetworkExposure: %v", err)
	}

	var unmarshaled NetworkExposure
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal NetworkExposure: %v", err)
	}

	if unmarshaled.ID != exp.ID || unmarshaled.PublicPort != 2222 || unmarshaled.InternalPort != 22 {
		t.Errorf("Mismatch after unmarshal: got %+v, want %+v", unmarshaled, exp)
	}
}

func TestFileExposureStoreAtomicPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mystic-exposure-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "exposures.json")
	store := NewFileExposureStore(storePath)

	exposures, err := store.Load()
	if err != nil {
		t.Fatalf("Load on missing file failed: %v", err)
	}
	if len(exposures) != 0 {
		t.Fatalf("Expected empty map for missing file, got %d", len(exposures))
	}

	testExp := &NetworkExposure{
		ID:           "exp-test-01",
		WorkloadID:   "wl-01",
		PublicPort:   8080,
		InternalPort: 80,
		Protocol:     hosts.ProtocolTCP,
	}
	exposures[testExp.ID] = testExp

	if err := store.Save(exposures); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}
	if len(loaded) != 1 || loaded["exp-test-01"].PublicPort != 8080 {
		t.Errorf("Loaded data mismatch: got %+v", loaded)
	}
}

func TestForwardingRuleConversionAndMigration(t *testing.T) {
	rule := hosts.ForwardingRule{
		ID:           "rule-01",
		GatewayID:    "gw-main",
		PublicIP:     "51.162.178.199",
		PublicPort:   2222,
		Protocol:     hosts.ProtocolTCP,
		InternalIP:   "10.170.92.70",
		InternalPort: 22,
		State:        hosts.ExposureStateConfigured,
		Description:  "Legacy SSH Rule",
	}

	exp := ConvertForwardingRuleToExposure(rule, "wl-nano", "test-nano", hosts.ExposureNATForwarded)
	if exp.ID != "rule-01" || exp.PublicPort != 2222 || exp.InternalPort != 22 {
		t.Errorf("ConvertForwardingRuleToExposure returned unexpected values: %+v", exp)
	}

	convertedRule := exp.ToForwardingRule()
	if convertedRule.PublicPort != 2222 || convertedRule.InternalPort != 22 {
		t.Errorf("ToForwardingRule returned unexpected values: %+v", convertedRule)
	}

	tempDir, _ := os.MkdirTemp("", "mystic-migration-test-*")
	defer os.RemoveAll(tempDir)
	store := NewFileExposureStore(filepath.Join(tempDir, "exposures.json"))
	em := NewExposureManager(store, nil)

	// First migration run
	count := em.MigrateForwardingRules("wl-nano", "test-nano", hosts.ExposureNATForwarded, []hosts.ForwardingRule{rule})
	if count != 1 {
		t.Errorf("Expected 1 migrated rule, got %d", count)
	}

	// Second migration run (idempotent - no duplicate records)
	count2 := em.MigrateForwardingRules("wl-nano", "test-nano", hosts.ExposureNATForwarded, []hosts.ForwardingRule{rule})
	if count2 != 0 {
		t.Errorf("Expected 0 migrated rules on duplicate run, got %d", count2)
	}
}

func TestExposureManagerAllocatorIntegration(t *testing.T) {
	ctx := context.Background()
	tempDir, _ := os.MkdirTemp("", "mystic-alloc-test-*")
	defer os.RemoveAll(tempDir)
	store := NewFileExposureStore(filepath.Join(tempDir, "exposures.json"))
	em := NewExposureManager(store, nil)

	// Test 1: Reserved SSH management port rejection (PublicPort = 22)
	reservedExp := &NetworkExposure{
		WorkloadID:   "wl-01",
		PublicPort:   22, // Reserved SSH port
		InternalPort: 2222,
		InternalIP:   "10.0.0.50",
		Protocol:     hosts.ProtocolTCP,
	}
	_, err := em.CreateNetworkExposure(ctx, reservedExp)
	if err == nil {
		t.Fatalf("Expected error for reserved SSH public port 22, got nil")
	}

	// Test 2: Valid allocation
	validExp := &NetworkExposure{
		ID:           "exp-valid-01",
		WorkloadID:   "wl-01",
		PublicPort:   2222,
		InternalPort: 22,
		InternalIP:   "10.0.0.50",
		Protocol:     hosts.ProtocolTCP,
	}
	created, err := em.CreateNetworkExposure(ctx, validExp)
	if err != nil {
		t.Fatalf("CreateNetworkExposure failed: %v", err)
	}
	if created.PublicPort != 2222 {
		t.Errorf("Expected PublicPort 2222, got %d", created.PublicPort)
	}

	// Test 3: Duplicate external port collision rejection
	dupExp := &NetworkExposure{
		ID:           "exp-dup-01",
		WorkloadID:   "wl-02",
		PublicPort:   2222, // Duplicate port
		InternalPort: 80,
		InternalIP:   "10.0.0.51",
		Protocol:     hosts.ProtocolTCP,
	}
	_, err = em.CreateNetworkExposure(ctx, dupExp)
	if err == nil {
		t.Fatalf("Expected error for duplicate public port 2222, got nil")
	}
}

func TestExposureReconciliationDriftDetection(t *testing.T) {
	ctx := context.Background()
	tempDir, _ := os.MkdirTemp("", "mystic-recon-test-*")
	defer os.RemoveAll(tempDir)
	store := NewFileExposureStore(filepath.Join(tempDir, "exposures.json"))

	em := NewExposureManager(store, nil)

	exp := &NetworkExposure{
		ID:           "exp-drift-01",
		WorkloadID:   "wl-01",
		PublicPort:   3000,
		InternalPort: 3000,
		InternalIP:   "10.0.0.50",
		Protocol:     hosts.ProtocolTCP,
		DesiredState: hosts.ExposureStateApplied,
		ActualState:  hosts.ExposureStateApplied,
		SyncStatus:   instances.SyncInSync,
	}
	_, err := em.CreateNetworkExposure(ctx, exp)
	if err != nil {
		t.Fatalf("CreateNetworkExposure failed: %v", err)
	}

	// Reconcile when driver is nil maintains SyncInSync
	reconciled, err := em.ReconcileExposure(ctx, "exp-drift-01")
	if err != nil {
		t.Fatalf("ReconcileExposure failed: %v", err)
	}
	if reconciled.SyncStatus != instances.SyncInSync {
		t.Errorf("Expected SyncInSync, got %s", reconciled.SyncStatus)
	}
}
