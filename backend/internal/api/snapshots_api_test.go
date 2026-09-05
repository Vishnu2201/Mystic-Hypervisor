package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/config"
	"github.com/mystic-hypervisor/mystic/backend/internal/snapshots"
	"github.com/mystic-hypervisor/mystic/backend/internal/workloads"
)

func setupSnapshotTestRouter(t *testing.T) (*Router, string, string) {
	tempDir, err := os.MkdirTemp("", "mystic-snap-api-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cfg := &config.Config{}
	cfg.Database.WorkloadStorePath = filepath.Join(tempDir, "workloads.json")
	t.Setenv("MYSTIC_EXPOSURE_STORE_PATH", filepath.Join(tempDir, "exposures.json"))
	t.Setenv("MYSTIC_SERVICE_STORE_PATH", filepath.Join(tempDir, "services.json"))
	t.Setenv("MYSTIC_CONNECTION_STORE_PATH", filepath.Join(tempDir, "connection_profiles.json"))
	t.Setenv("MYSTIC_SNAPSHOT_STORE_PATH", filepath.Join(tempDir, "snapshots.json"))

	router := NewRouter(cfg)

	// Create a dummy workload draft so snapshot creation passes workload lookup
	ctx := context.Background()
	wl, err := router.workloadManager.CreateWorkload(ctx, workloads.WorkloadSpec{
		Name:        "test-nano",
		Type:        workloads.TypeIncusContainer,
		CPU:         1,
		MemoryMB:    1024,
		StorageGB:   10,
		Image:       "images:debian/13",
		NetworkName: "incusbr0",
	})
	if err != nil {
		t.Fatalf("Failed to create test workload: %v", err)
	}

	return router, tempDir, wl.ID
}

func TestSnapshotAPIRoundTrip(t *testing.T) {
	router, tempDir, wlID := setupSnapshotTestRouter(t)
	defer os.RemoveAll(tempDir)

	// 1. GET /api/v1/workloads/{wlID}/snapshots - Initial Empty List
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workloads/"+wlID+"/snapshots", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected StatusOK (200), got %d: %s", rec.Code, rec.Body.String())
	}

	var listRes struct {
		Count     int                  `json:"count"`
		Snapshots []snapshots.Snapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listRes); err != nil {
		t.Fatalf("Failed to decode list JSON: %v", err)
	}

	// 2. POST /api/v1/workloads/{wlID}/snapshots - Option Injection Rejection
	badBody := map[string]interface{}{"name": "--option-flag", "stateful": false}
	badBytes, _ := json.Marshal(badBody)
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/workloads/"+wlID+"/snapshots", bytes.NewReader(badBytes))
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, badReq)

	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("Expected StatusBadRequest (400) for option flag, got %d: %s", badRec.Code, badRec.Body.String())
	}

	// 3. POST /api/v1/workloads/wl-nonexistent/snapshots - Workload Not Found
	notFoundReq := httptest.NewRequest(http.MethodPost, "/api/v1/workloads/wl-nonexistent/snapshots", bytes.NewReader([]byte(`{"name":"snap0"}`)))
	notFoundRec := httptest.NewRecorder()
	router.ServeHTTP(notFoundRec, notFoundReq)

	if notFoundRec.Code != http.StatusNotFound {
		t.Fatalf("Expected StatusNotFound (404) for nonexistent workload, got %d: %s", notFoundRec.Code, notFoundRec.Body.String())
	}
}
