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
	"github.com/mystic-hypervisor/mystic/backend/internal/hosts"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
	"github.com/mystic-hypervisor/mystic/backend/internal/services"
)

func setupTestRouter(t *testing.T) (*Router, string) {
	tempDir, err := os.MkdirTemp("", "mystic-api-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cfg := &config.Config{}
	cfg.Database.WorkloadStorePath = filepath.Join(tempDir, "workloads.json")
	t.Setenv("MYSTIC_EXPOSURE_STORE_PATH", filepath.Join(tempDir, "exposures.json"))
	t.Setenv("MYSTIC_SERVICE_STORE_PATH", filepath.Join(tempDir, "services.json"))
	t.Setenv("MYSTIC_CONNECTION_STORE_PATH", filepath.Join(tempDir, "connection_profiles.json"))

	router := NewRouter(cfg)
	return router, tempDir
}

func TestServicesAPIRoundTrip(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	// 1. POST /api/v1/services - Create SSH Service
	body := map[string]interface{}{
		"workload_id":   "wl-api-01",
		"workload_name": "test-vps",
		"name":          "SSH Access",
		"type":          "SSH",
		"internal_ip":   "10.0.0.50",
		"internal_port": 22,
		"protocol":      "TCP",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected StatusCreated (201), got %d: %s", rec.Code, rec.Body.String())
	}

	var createRes struct {
		Service services.Service `json:"service"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &createRes)
	svcID := createRes.Service.ID
	if svcID == "" {
		t.Fatalf("Expected non-empty service ID")
	}

	// 2. GET /api/v1/services/{id}
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/services/"+svcID, nil)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("Expected StatusOK (200), got %d", recGet.Code)
	}

	// 3. GET /api/v1/workloads/{id}/services
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/workloads/wl-api-01/services", nil)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("Expected StatusOK (200), got %d", recList.Code)
	}

	var listRes struct {
		Services []services.Service `json:"services"`
	}
	_ = json.Unmarshal(recList.Body.Bytes(), &listRes)
	if len(listRes.Services) != 1 {
		t.Fatalf("Expected 1 service for workload wl-api-01, got %d", len(listRes.Services))
	}
}

func TestGenerateConnectionProfileAPI(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create Exposure
	exp, err := router.workloadManager.ExposureManager().CreateNetworkExposure(ctx, &networking.NetworkExposure{
		ID:           "exp-ssh-01",
		WorkloadID:   "wl-api-02",
		PublicIP:     "51.162.178.199",
		PublicPort:   2222,
		InternalIP:   "10.0.0.60",
		InternalPort: 22,
		Protocol:     hosts.ProtocolTCP,
	})
	if err != nil {
		t.Fatalf("Failed to set up exposure: %v", err)
	}

	// Create Service linking to Exposure
	svc, err := router.workloadManager.ServiceManager().CreateService(ctx, &services.Service{
		WorkloadID:   "wl-api-02",
		WorkloadName: "production-container",
		Name:         "SSH Access",
		Type:         services.ServiceTypeSSH,
		InternalIP:   "10.0.0.60",
		InternalPort: 22,
		ExposureID:   exp.ID,
		Protocol:     hosts.ProtocolTCP,
	})
	if err != nil {
		t.Fatalf("Failed to set up service: %v", err)
	}

	// POST /api/v1/services/{id}/connection-profile
	genPayload := map[string]interface{}{
		"target_user":  "root",
		"save_profile": true,
	}
	genBytes, _ := json.Marshal(genPayload)
	reqGen := httptest.NewRequest(http.MethodPost, "/api/v1/services/"+svc.ID+"/connection-profile", bytes.NewReader(genBytes))
	recGen := httptest.NewRecorder()

	router.ServeHTTP(recGen, reqGen)
	if recGen.Code != http.StatusOK {
		t.Fatalf("Expected StatusOK (200), got %d: %s", recGen.Code, recGen.Body.String())
	}

	var genRes struct {
		Connection struct {
			ConnectionURL string `json:"connection_url"`
			CLICommand    string `json:"cli_command"`
			EndpointHost  string `json:"endpoint_host"`
			EndpointPort  int    `json:"endpoint_port"`
		} `json:"connection"`
	}
	_ = json.Unmarshal(recGen.Body.Bytes(), &genRes)

	if genRes.Connection.EndpointHost != "51.162.178.199" || genRes.Connection.EndpointPort != 2222 {
		t.Fatalf("Expected resolved exposure endpoint 51.162.178.199:2222, got %s:%d", genRes.Connection.EndpointHost, genRes.Connection.EndpointPort)
	}

	if genRes.Connection.CLICommand != "ssh -p 2222 root@51.162.178.199" {
		t.Fatalf("Expected CLI 'ssh -p 2222 root@51.162.178.199', got '%s'", genRes.Connection.CLICommand)
	}
}

func TestGetExposureProviderAPI(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// 1. Create Exposure
	exp, err := router.workloadManager.ExposureManager().CreateNetworkExposure(ctx, &networking.NetworkExposure{
		ID:           "exp-prov-01",
		WorkloadID:   "wl-prov-01",
		WorkloadName: "test-nano",
		PublicIP:     "51.162.178.199",
		PublicPort:   2222,
		InternalIP:   "10.170.92.70",
		InternalPort: 22,
		Protocol:     hosts.ProtocolTCP,
	})
	if err != nil {
		t.Fatalf("Failed to create exposure: %v", err)
	}

	// 2. GET /api/v1/network/exposures/{id}/provider
	reqProv := httptest.NewRequest(http.MethodGet, "/api/v1/network/exposures/"+exp.ID+"/provider", nil)
	recProv := httptest.NewRecorder()
	router.ServeHTTP(recProv, reqProv)

	if recProv.Code != http.StatusOK {
		t.Fatalf("Expected StatusOK (200), got %d: %s", recProv.Code, recProv.Body.String())
	}

	var provRes struct {
		ProviderStatus networking.NetworkExposureStatus `json:"provider_status"`
		ExposureID     string                           `json:"exposure_id"`
	}
	if err := json.Unmarshal(recProv.Body.Bytes(), &provRes); err != nil {
		t.Fatalf("Failed to unmarshal provider response: %v", err)
	}

	if provRes.ExposureID != exp.ID {
		t.Fatalf("Expected exposure_id %s, got %s", exp.ID, provRes.ExposureID)
	}

	// 3. Test non-existent exposure GET /api/v1/network/exposures/exp-nonexistent/provider
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/network/exposures/exp-nonexistent/provider", nil)
	rec404 := httptest.NewRecorder()
	router.ServeHTTP(rec404, req404)

	if rec404.Code != http.StatusNotFound {
		t.Fatalf("Expected StatusNotFound (404), got %d: %s", rec404.Code, rec404.Body.String())
	}
}
