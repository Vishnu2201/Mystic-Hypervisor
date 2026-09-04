package api

import (
	"encoding/json"
	"net/http"

	"github.com/mystic-hypervisor/mystic/backend/internal/config"
	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// Router sets up HTTP route handlers for mysticd API.
type Router struct {
	cfg *config.Config
	mux *http.ServeMux
}

// NewRouter creates a new API router using Go standard library net/http.
func NewRouter(cfg *config.Config) *Router {
	r := &Router{
		cfg: cfg,
		mux: http.NewServeMux(),
	}
	r.registerRoutes()
	return r
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
	r.mux.HandleFunc("GET /api/v1/instances", r.handleInstances)
	r.mux.HandleFunc("GET /api/v1/hosts", r.handleUnimplemented)
	r.mux.HandleFunc("GET /api/v1/storage", r.handleUnimplemented)
	r.mux.HandleFunc("GET /api/v1/networks", r.handleUnimplemented)
	r.mux.HandleFunc("GET /api/v1/users", r.handleUnimplemented)
	r.mux.HandleFunc("GET /api/v1/audit", r.handleUnimplemented)
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "mysticd",
	})
}

func (r *Router) handleVersion(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"version": "0.1.0-foundation",
		"target":  "Milestone 1 — Engineering Foundation",
	})
}

func (r *Router) handleDoctor(w http.ResponseWriter, req *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"overall_health": "FOUNDATION_READY",
		"checks": []map[string]string{
			{"component": "config", "status": "OK"},
			{"component": "logging", "status": "OK"},
			{"component": "provider_abstraction", "status": "OK"},
		},
	})
}

func (r *Router) handleProviders(w http.ResponseWriter, req *http.Request) {
	providersMap := interfaces.ListProviders()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"providers": providersMap,
	})
}

func (r *Router) handleInstances(w http.ResponseWriter, req *http.Request) {
	// Milestone 1: Return empty list adhering to NO FAKE DATA rule
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"instances": []interfaces.Instance{},
		"message":   "No instances registered. Hypervisor providers are non-destructive stubs in Milestone 1.",
	})
}

func (r *Router) handleUnimplemented(w http.ResponseWriter, req *http.Request) {
	logging.GetLogger().Warn("Endpoint hit for unimplemented feature", "path", req.URL.Path)
	jsonResponse(w, http.StatusNotImplemented, map[string]interface{}{
		"error":   "not_implemented",
		"message": "This endpoint is defined in the architecture but not implemented in Milestone 1.",
	})
}

func jsonResponse(w http.ResponseWriter, code int, data interface{}) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
