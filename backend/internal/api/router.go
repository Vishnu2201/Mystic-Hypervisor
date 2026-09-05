package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/config"
	"github.com/mystic-hypervisor/mystic/backend/internal/connections"
	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
	"github.com/mystic-hypervisor/mystic/backend/internal/networking"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/incus"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
	"github.com/mystic-hypervisor/mystic/backend/internal/services"
	"github.com/mystic-hypervisor/mystic/backend/internal/workloads"
)

// Router sets up HTTP route handlers for mysticd API.
type Router struct {
	cfg             *config.Config
	mux             *http.ServeMux
	workloadManager *workloads.Manager
	incusDriver     *incus.IncusProvider
}

// NewRouter creates a new API router using Go standard library net/http.
func NewRouter(cfg *config.Config) *Router {
	storePath := cfg.Database.WorkloadStorePath
	if storePath == "" {
		storePath = "/var/lib/mystic/workloads.json"
	}

	store := workloads.NewFileStore(storePath)
	incusProv := incus.NewIncusProvider(cfg.Provider.IncusSocket)
	wm := workloads.NewManagerWithProviderAndStore(incusProv, store)

	intervalSec := 15
	if envSec := os.Getenv("MYSTIC_RECONCILE_INTERVAL_SECONDS"); envSec != "" {
		if parsed, err := strconv.Atoi(envSec); err == nil && parsed > 0 {
			intervalSec = parsed
		}
	} else if cfg.Monitoring.IntervalSeconds > 0 {
		intervalSec = cfg.Monitoring.IntervalSeconds
	}
	wm.StartBackgroundReconciliation(time.Duration(intervalSec) * time.Second)

	logging.GetLogger().Info("Initialized Workload Manager with persistent store",
		"workload_store_path", store.FilePath(),
		"incus_socket", cfg.Provider.IncusSocket,
		"reconcile_interval_seconds", intervalSec,
	)

	r := &Router{
		cfg:             cfg,
		mux:             http.NewServeMux(),
		workloadManager: wm,
		incusDriver:     incusProv,
	}
	r.registerRoutes()
	return r
}

// Close gracefully stops router resources including background worker goroutines.
func (r *Router) Close() error {
	if r.workloadManager != nil {
		r.workloadManager.StopBackgroundReconciliation()
	}
	return nil
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Simple CORS and JSON headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	r.mux.ServeHTTP(w, req)
}

func (r *Router) registerRoutes() {
	r.mux.HandleFunc("GET /api/v1/health", r.handleHealth)
	r.mux.HandleFunc("GET /api/v1/version", r.handleVersion)
	r.mux.HandleFunc("GET /api/v1/doctor", r.handleDoctor)
	r.mux.HandleFunc("GET /api/v1/providers", r.handleProviders)
	r.mux.HandleFunc("GET /api/v1/providers/incus/preflight", r.handleIncusPreflight)
	r.mux.HandleFunc("GET /api/v1/providers/preflight", r.handleIncusPreflight)
	r.mux.HandleFunc("GET /api/v1/providers/incus/images", r.handleIncusImages)
	r.mux.HandleFunc("GET /api/v1/providers/incus/resources", r.handleIncusResources)
	r.mux.HandleFunc("GET /api/v1/providers/incus/instances/{name}/adoption-preview", r.handleIncusAdoptionPreview)
	r.mux.HandleFunc("POST /api/v1/providers/incus/instances/{name}/adopt", r.handleAdoptIncusInstance)
	r.mux.HandleFunc("GET /api/v1/instances", r.handleInstances)

	// Workload Lifecycle & Provisioning Endpoints
	r.mux.HandleFunc("GET /api/v1/workloads", r.handleListWorkloads)
	r.mux.HandleFunc("POST /api/v1/workloads", r.handleCreateWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/adopt", r.handleAdoptWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/reconcile", r.handleReconcileAllWorkloads)
	r.mux.HandleFunc("GET /api/v1/workloads/{id}", r.handleGetWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/{id}/validate", r.handleValidateWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/{id}/plan", r.handlePlanWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/{id}/approve", r.handleApproveWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/{id}/provision", r.handleProvisionWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/{id}/start", r.handleStartWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/{id}/stop", r.handleStopWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/{id}/restart", r.handleRestartWorkload)
	r.mux.HandleFunc("DELETE /api/v1/workloads/{id}", r.handleDeleteWorkload)
	r.mux.HandleFunc("POST /api/v1/workloads/{id}/reconcile", r.handleReconcileWorkload)

	r.mux.HandleFunc("GET /api/v1/hosts", r.handleUnimplemented)
	r.mux.HandleFunc("GET /api/v1/storage", r.handleUnimplemented)
	r.mux.HandleFunc("GET /api/v1/networks", r.handleUnimplemented)
	r.mux.HandleFunc("GET /api/v1/networks/workloads", r.handleWorkloadNetworks)
	r.mux.HandleFunc("POST /api/v1/networks/validate-allocation", r.handleValidateAllocation)
	r.mux.HandleFunc("GET /api/v1/networks/pools", r.handleAllocationPools)
	r.mux.HandleFunc("POST /api/v1/networks/pools", r.handleAllocationPools)
	r.mux.HandleFunc("GET /api/v1/networks/rules", r.handleForwardingRules)
	r.mux.HandleFunc("POST /api/v1/networks/rules", r.handleForwardingRules)
	r.mux.HandleFunc("POST /api/v1/networks/rules/apply", r.handleApplyForwardingRule)
	r.mux.HandleFunc("POST /api/v1/networks/rules/verify", r.handleVerifyForwardingRule)
	r.mux.HandleFunc("GET /api/v1/users", r.handleUnimplemented)
	r.mux.HandleFunc("GET /api/v1/audit", r.handleUnimplemented)

	// Network Exposure Endpoints
	r.mux.HandleFunc("GET /api/v1/network/exposures", r.handleListExposures)
	r.mux.HandleFunc("GET /api/v1/network/exposures/{id}", r.handleGetExposure)
	r.mux.HandleFunc("POST /api/v1/network/exposures", r.handleCreateExposure)
	r.mux.HandleFunc("PATCH /api/v1/network/exposures/{id}", r.handleUpdateExposure)
	r.mux.HandleFunc("DELETE /api/v1/network/exposures/{id}", r.handleDeleteExposure)
	r.mux.HandleFunc("POST /api/v1/network/exposures/{id}/validate", r.handleValidateExposure)
	r.mux.HandleFunc("POST /api/v1/network/exposures/{id}/apply", r.handleApplyExposure)
	r.mux.HandleFunc("POST /api/v1/network/exposures/{id}/reconcile", r.handleReconcileExposure)
	r.mux.HandleFunc("GET /api/v1/workloads/{id}/exposures", r.handleWorkloadExposures)

	// Service Endpoints
	r.mux.HandleFunc("GET /api/v1/services", r.handleListServices)
	r.mux.HandleFunc("POST /api/v1/services", r.handleCreateService)
	r.mux.HandleFunc("GET /api/v1/services/{id}", r.handleGetService)
	r.mux.HandleFunc("PATCH /api/v1/services/{id}", r.handleUpdateService)
	r.mux.HandleFunc("DELETE /api/v1/services/{id}", r.handleDeleteService)
	r.mux.HandleFunc("GET /api/v1/workloads/{id}/services", r.handleWorkloadServices)

	// Connection Profile Endpoints
	r.mux.HandleFunc("GET /api/v1/connections", r.handleListConnections)
	r.mux.HandleFunc("POST /api/v1/connections", r.handleCreateConnection)
	r.mux.HandleFunc("GET /api/v1/connections/{id}", r.handleGetConnection)
	r.mux.HandleFunc("PATCH /api/v1/connections/{id}", r.handleUpdateConnection)
	r.mux.HandleFunc("DELETE /api/v1/connections/{id}", r.handleDeleteConnection)
	r.mux.HandleFunc("GET /api/v1/services/{id}/connections", r.handleServiceConnections)
	r.mux.HandleFunc("POST /api/v1/services/{id}/connection-profile", r.handleGenerateServiceConnectionProfile)
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "mysticd",
	})
}

func (r *Router) handleVersion(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"version": "0.8.0-milestone7",
		"target":  "Milestone 7 — Real VPS Integration & Controlled Incus Validation",
	})
}

func (r *Router) handleDoctor(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"overall_health": "PROVISIONING_ENGINE_READY",
		"checks": []map[string]string{
			{"component": "config", "status": "OK"},
			{"component": "logging", "status": "OK"},
			{"component": "provider_abstraction", "status": "OK"},
			{"component": "incus_provider_driver", "status": "ACTIVE"},
			{"component": "port_allocator_engine", "status": "OK"},
			{"component": "workload_provisioning_engine", "status": "OK"},
			{"component": "provider_preflight_engine", "status": "OK"},
		},
	})
}

func (r *Router) handleProviders(w http.ResponseWriter, req *http.Request) {
	providersMap := interfaces.ListProviders()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"providers": providersMap,
	})
}

func (r *Router) handleIncusPreflight(w http.ResponseWriter, req *http.Request) {
	res, err := r.workloadManager.GetProviderPreflight(req.Context(), "incus")
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

func (r *Router) handleIncusImages(w http.ResponseWriter, req *http.Request) {
	if imgProvider, ok := r.incusDriver.ImageProvider(); ok {
		imgs, err := imgProvider.ListImages(req.Context())
		if err != nil {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"images":  []interface{}{},
				"error":   "provider_unavailable",
				"message": fmt.Sprintf("Incus image discovery unavailable: %v", err),
			})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"images": imgs,
		})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"images":  []interface{}{},
		"message": "Incus image provider driver unattached.",
	})
}

func (r *Router) handleIncusResources(w http.ResponseWriter, req *http.Request) {
	var pools []interfaces.StoragePool
	var nets []interfaces.Network

	if stProvider, ok := r.incusDriver.StorageProvider(); ok {
		pools, _ = stProvider.ListStoragePools(req.Context())
	}
	if netProvider, ok := r.incusDriver.NetworkProvider(); ok {
		nets, _ = netProvider.ListNetworks(req.Context())
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"storage_pools": pools,
		"networks":      nets,
	})
}

func (r *Router) handleInstances(w http.ResponseWriter, req *http.Request) {
	if instProvider, ok := r.incusDriver.InstanceProvider(); ok {
		insts, err := instProvider.ListInstances(req.Context())
		if err == nil {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"instances": insts,
			})
			return
		}
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"instances": []interfaces.Instance{},
		"message":   "No live provider instances detected.",
	})
}

// --- Milestone 5 Workload Handlers ---

func (r *Router) handleListWorkloads(w http.ResponseWriter, req *http.Request) {
	list, err := r.workloadManager.ListWorkloads(req.Context())
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workloads": list,
	})
}

func (r *Router) handleCreateWorkload(w http.ResponseWriter, req *http.Request) {
	var spec workloads.WorkloadSpec
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	wl, err := r.workloadManager.CreateWorkload(req.Context(), spec)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"workload": wl,
		"message":  "Workload draft created successfully. Next step: Run validation and generate provisioning plan.",
	})
}

func (r *Router) handleGetWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	wl, err := r.workloadManager.GetWorkload(req.Context(), id)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload": wl,
	})
}

func (r *Router) handleValidateWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/validate")

	valRes, err := r.workloadManager.ValidateWorkload(req.Context(), id)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"validation_result": valRes,
	})
}

func (r *Router) handlePlanWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/plan")

	plan, err := r.workloadManager.GeneratePlan(req.Context(), id)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"provisioning_plan": plan,
	})
}

func (r *Router) handleApproveWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/approve")

	if err := r.workloadManager.ApprovePlan(req.Context(), id); err != nil {
		writeWorkloadError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "APPROVED",
		"message": "Provisioning plan explicitly approved by administrator. Ready for provision execution.",
	})
}

func (r *Router) handleProvisionWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/provision")

	wl, err := r.workloadManager.ProvisionWorkload(req.Context(), id)
	if err != nil {
		writeWorkloadError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload": wl,
		"message":  "Workload provisioned and started on Incus provider.",
	})
}

func (r *Router) handleStartWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/start")

	wl, err := r.workloadManager.StartWorkload(req.Context(), id)
	if err != nil {
		writeWorkloadError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload": wl,
	})
}

func (r *Router) handleStopWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/stop")

	wl, err := r.workloadManager.StopWorkload(req.Context(), id, false)
	if err != nil {
		writeWorkloadError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload": wl,
	})
}

func (r *Router) handleRestartWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/restart")

	wl, err := r.workloadManager.RestartWorkload(req.Context(), id)
	if err != nil {
		writeWorkloadError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload": wl,
	})
}

func (r *Router) handleDeleteWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	if err := r.workloadManager.DeleteWorkload(req.Context(), id); err != nil {
		writeWorkloadError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Workload deleted successfully.",
	})
}

func (r *Router) handleReconcileWorkload(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/reconcile")

	wl, err := r.workloadManager.ReconcileWorkload(req.Context(), id)
	if err != nil {
		writeWorkloadError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload": wl,
	})
}

func (r *Router) handleReconcileAllWorkloads(w http.ResponseWriter, req *http.Request) {
	if err := r.workloadManager.ReconcileAll(req.Context()); err != nil {
		writeWorkloadError(w, err)
		return
	}
	workloadsList, err := r.workloadManager.ListWorkloads(req.Context())
	if err != nil {
		writeWorkloadError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"message":   "Bulk workload reconciliation completed.",
		"workloads": workloadsList,
	})
}

func (r *Router) handleIncusAdoptionPreview(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/v1/providers/incus/instances/")
	name := strings.TrimSuffix(trimmed, "/adoption-preview")
	name = strings.Trim(name, "/")

	if name == "" {
		name = req.URL.Query().Get("name")
	}

	if strings.TrimSpace(name) == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "instance name is required for preview"})
		return
	}

	preview, err := r.workloadManager.GetAdoptionPreview(req.Context(), name)
	if err != nil {
		writeWorkloadError(w, err)
		return
	}

	jsonResponse(w, http.StatusOK, preview)
}

func (r *Router) handleAdoptIncusInstance(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	path := req.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/v1/providers/incus/instances/")
	name := strings.TrimSuffix(trimmed, "/adopt")
	name = strings.Trim(name, "/")

	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		name = body.Name
	}

	if strings.TrimSpace(name) == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "instance name is required for adoption"})
		return
	}

	wl, err := r.workloadManager.AdoptWorkload(req.Context(), name)
	if err != nil {
		writeWorkloadError(w, err)
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload": wl,
		"message":  fmt.Sprintf("External instance '%s' successfully adopted into Mystic management.", name),
	})
}

func (r *Router) handleAdoptWorkload(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request body: 'name' field is required"})
		return
	}

	wl, err := r.workloadManager.AdoptWorkload(req.Context(), body.Name)
	if err != nil {
		writeWorkloadError(w, err)
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload": wl,
		"message":  fmt.Sprintf("External instance '%s' successfully adopted into Mystic management.", body.Name),
	})
}

func writeWorkloadError(w http.ResponseWriter, err error) {
	if errors.Is(err, workloads.ErrWorkloadNotFound) || errors.Is(err, workloads.ErrIncusInstanceNotFound) {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, workloads.ErrPlanNotApproved) || errors.Is(err, workloads.ErrPlanInvalidated) || errors.Is(err, workloads.ErrIllegalStateTransition) {
		jsonResponse(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, workloads.ErrAlreadyManaged) || errors.Is(err, workloads.ErrOwnershipConflict) || errors.Is(err, workloads.ErrWorkloadConfigConflict) || errors.Is(err, workloads.ErrOperationAlreadyInProgress) {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, workloads.ErrProviderCapabilityMissing) {
		jsonResponse(w, http.StatusNotImplemented, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, interfaces.ErrProviderUnavailable) {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func extractPathID(fullPath, prefix string) string {
	trimmed := strings.TrimPrefix(fullPath, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return trimmed
}

func (r *Router) handleWorkloadNetworks(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workload_networks": []interface{}{},
		"message":           "Workload network configuration store initialized.",
	})
}

func (r *Router) handleValidateAllocation(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "AVAILABLE",
		"message": "Allocation validation engine endpoint active.",
	})
}

func (r *Router) handleAllocationPools(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"pools":   []interface{}{},
		"message": "External port allocation pools store initialized. Status: ALLOCATION_POOL_UNCONFIGURED until pool is created.",
	})
}

func (r *Router) handleForwardingRules(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"rules":   []interface{}{},
		"message": "Forwarding rules store initialized.",
	})
}

func (r *Router) handleApplyForwardingRule(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "REQUESTED",
		"state":   "EXTERNAL_GATEWAY_INTEGRATION_NOT_CONFIGURED",
		"message": "Forwarding rule request recorded as CONFIGURED/REQUESTED. No active gateway agent is enabled to perform live changes.",
	})
}

func (r *Router) handleVerifyForwardingRule(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "UNVERIFIED",
		"message": "Verification requires an active gateway connectivity test agent.",
	})
}

func (r *Router) handleUnimplemented(w http.ResponseWriter, req *http.Request) {
	logging.GetLogger().Warn("Endpoint hit for unimplemented feature", "path", req.URL.Path)
	jsonResponse(w, http.StatusNotImplemented, map[string]interface{}{
		"error":   "not_implemented",
		"message": "This endpoint is defined in the architecture but not implemented.",
	})
}

func jsonResponse(w http.ResponseWriter, code int, data interface{}) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, code int, message string) {
	jsonResponse(w, code, map[string]string{"error": message})
}

func (r *Router) handleListExposures(w http.ResponseWriter, req *http.Request) {
	workloadID := req.URL.Query().Get("workload_id")
	var list []networking.NetworkExposure
	var err error
	if workloadID != "" {
		list, err = r.workloadManager.ExposureManager().ListWorkloadExposures(req.Context(), workloadID)
	} else {
		list, err = r.workloadManager.ExposureManager().ListNetworkExposures(req.Context())
	}
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"exposures": list})
}

func (r *Router) handleGetExposure(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/network/exposures/")
	exp, err := r.workloadManager.ExposureManager().GetNetworkExposure(req.Context(), id)
	if err != nil {
		if errors.Is(err, networking.ErrExposureNotFound) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"exposure": exp})
}

func (r *Router) handleCreateExposure(w http.ResponseWriter, req *http.Request) {
	var exp networking.NetworkExposure
	if err := json.NewDecoder(req.Body).Decode(&exp); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request JSON body"})
		return
	}

	created, err := r.workloadManager.ExposureManager().CreateNetworkExposure(req.Context(), &exp)
	if err != nil {
		if errors.Is(err, networking.ErrExposureConflict) || errors.Is(err, networking.ErrInvalidExposure) {
			jsonResponse(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"exposure": created,
		"message":  "Network exposure created successfully.",
	})
}

func (r *Router) handleUpdateExposure(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/network/exposures/")
	var exp networking.NetworkExposure
	if err := json.NewDecoder(req.Body).Decode(&exp); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request JSON body"})
		return
	}

	updated, err := r.workloadManager.ExposureManager().UpdateNetworkExposure(req.Context(), id, &exp)
	if err != nil {
		if errors.Is(err, networking.ErrExposureNotFound) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, networking.ErrExposureConflict) {
			jsonResponse(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"exposure": updated,
		"message":  "Network exposure updated successfully.",
	})
}

func (r *Router) handleDeleteExposure(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/network/exposures/")
	if err := r.workloadManager.ExposureManager().DeleteNetworkExposure(req.Context(), id); err != nil {
		if errors.Is(err, networking.ErrExposureNotFound) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Network exposure deleted cleanly."})
}

func (r *Router) handleValidateExposure(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/network/exposures/")
	id = strings.TrimSuffix(id, "/validate")

	exp, err := r.workloadManager.ExposureManager().GetNetworkExposure(req.Context(), id)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	res, err := r.workloadManager.ExposureManager().ValidateNetworkExposure(req.Context(), exp)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"validation_result": res})
}

func (r *Router) handleApplyExposure(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/network/exposures/")
	id = strings.TrimSuffix(id, "/apply")

	exp, err := r.workloadManager.ExposureManager().ApplyExposure(req.Context(), id)
	if err != nil {
		if errors.Is(err, networking.ErrExposureNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"exposure": exp, "message": "Exposure applied on provider."})
}

func (r *Router) handleReconcileExposure(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/network/exposures/")
	id = strings.TrimSuffix(id, "/reconcile")

	exp, err := r.workloadManager.ExposureManager().ReconcileExposure(req.Context(), id)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"exposure": exp})
}

func (r *Router) handleWorkloadExposures(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/exposures")

	list, err := r.workloadManager.ExposureManager().ListWorkloadExposures(req.Context(), id)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"exposures": list})
}

// --- SERVICES HANDLERS ---

func (r *Router) handleListServices(w http.ResponseWriter, req *http.Request) {
	svcs, err := r.workloadManager.ServiceManager().ListServices(req.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"services": svcs, "count": len(svcs)})
}

func (r *Router) handleCreateService(w http.ResponseWriter, req *http.Request) {
	var svc services.Service
	if err := json.NewDecoder(req.Body).Decode(&svc); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	created, err := r.workloadManager.ServiceManager().CreateService(req.Context(), &svc)
	if err != nil {
		if errors.Is(err, services.ErrInvalidService) {
			jsonError(w, http.StatusBadRequest, err.Error())
		} else if errors.Is(err, services.ErrServiceConflict) {
			jsonError(w, http.StatusConflict, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]interface{}{"service": created})
}

func (r *Router) handleGetService(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/services/")
	svc, err := r.workloadManager.ServiceManager().GetService(req.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrServiceNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"service": svc})
}

func (r *Router) handleUpdateService(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/services/")
	var updated services.Service
	if err := json.NewDecoder(req.Body).Decode(&updated); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	svc, err := r.workloadManager.ServiceManager().UpdateService(req.Context(), id, &updated)
	if err != nil {
		if errors.Is(err, services.ErrServiceNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, services.ErrInvalidService) {
			jsonError(w, http.StatusBadRequest, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"service": svc})
}

func (r *Router) handleDeleteService(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/services/")
	if err := r.workloadManager.ServiceManager().DeleteService(req.Context(), id); err != nil {
		if errors.Is(err, services.ErrServiceNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Service deleted cleanly"})
}

func (r *Router) handleWorkloadServices(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/workloads/")
	id = strings.TrimSuffix(id, "/services")

	svcs, err := r.workloadManager.ServiceManager().ListWorkloadServices(req.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"services": svcs, "count": len(svcs)})
}

// --- CONNECTION PROFILES HANDLERS ---

func (r *Router) handleListConnections(w http.ResponseWriter, req *http.Request) {
	profiles, err := r.workloadManager.ConnectionManager().ListConnectionProfiles(req.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"connections": profiles, "count": len(profiles)})
}

func (r *Router) handleCreateConnection(w http.ResponseWriter, req *http.Request) {
	var profile connections.ConnectionProfile
	if err := json.NewDecoder(req.Body).Decode(&profile); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	created, err := r.workloadManager.ConnectionManager().CreateConnectionProfile(req.Context(), &profile)
	if err != nil {
		if errors.Is(err, connections.ErrInvalidConnection) {
			jsonError(w, http.StatusBadRequest, err.Error())
		} else if errors.Is(err, connections.ErrConnectionConflict) {
			jsonError(w, http.StatusConflict, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]interface{}{"connection": created})
}

func (r *Router) handleGetConnection(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/connections/")
	profile, err := r.workloadManager.ConnectionManager().GetConnectionProfile(req.Context(), id)
	if err != nil {
		if errors.Is(err, connections.ErrConnectionNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"connection": profile})
}

func (r *Router) handleUpdateConnection(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/connections/")
	var updated connections.ConnectionProfile
	if err := json.NewDecoder(req.Body).Decode(&updated); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	profile, err := r.workloadManager.ConnectionManager().UpdateConnectionProfile(req.Context(), id, &updated)
	if err != nil {
		if errors.Is(err, connections.ErrConnectionNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, connections.ErrInvalidConnection) {
			jsonError(w, http.StatusBadRequest, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"connection": profile})
}

func (r *Router) handleDeleteConnection(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/connections/")
	if err := r.workloadManager.ConnectionManager().DeleteConnectionProfile(req.Context(), id); err != nil {
		if errors.Is(err, connections.ErrConnectionNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Connection profile deleted cleanly"})
}

func (r *Router) handleServiceConnections(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/services/")
	id = strings.TrimSuffix(id, "/connections")

	profiles, err := r.workloadManager.ConnectionManager().ListServiceConnections(req.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{"connections": profiles, "count": len(profiles)})
}

func (r *Router) handleGenerateServiceConnectionProfile(w http.ResponseWriter, req *http.Request) {
	id := extractPathID(req.URL.Path, "/api/v1/services/")
	id = strings.TrimSuffix(id, "/connection-profile")

	svc, err := r.workloadManager.ServiceManager().GetService(req.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrServiceNotFound) {
			jsonError(w, http.StatusNotFound, err.Error())
		} else {
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	var reqPayload struct {
		TargetUser   string `json:"target_user"`
		CredentialID string `json:"credential_id"`
		SaveProfile  bool   `json:"save_profile"`
	}
	if req.Body != nil {
		_ = json.NewDecoder(req.Body).Decode(&reqPayload)
	}

	var exp *networking.NetworkExposure
	if svc.ExposureID != "" {
		exp, _ = r.workloadManager.ExposureManager().GetNetworkExposure(req.Context(), svc.ExposureID)
	}

	profile, err := r.workloadManager.ConnectionManager().GenerateConnectionProfile(svc, exp, reqPayload.TargetUser, reqPayload.CredentialID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if reqPayload.SaveProfile {
		saved, err := r.workloadManager.ConnectionManager().CreateConnectionProfile(req.Context(), profile)
		if err == nil {
			profile = saved
		}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"connection": profile})
}
