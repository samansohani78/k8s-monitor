/*
Copyright 2026 K8sWatch.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alertmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// API provides the REST API for alert management
type API struct {
	manager *Manager
	mux     *http.ServeMux
}

// NewAPI creates a new API server
func NewAPI(manager *Manager) *API {
	api := &API{
		manager: manager,
		mux:     http.NewServeMux(),
	}
	api.registerRoutes()
	return api
}

// registerRoutes registers HTTP routes
func (a *API) registerRoutes() {
	a.mux.HandleFunc("/api/v1/alerts", a.handleAlerts)
	a.mux.HandleFunc("/api/v1/alerts/", a.handleAlertByID)
	a.mux.HandleFunc("/healthz", a.handleHealth)
	a.mux.HandleFunc("/ready", a.handleReady)
	a.mux.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)
}

// ServeHTTP implements http.Handler
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// handleAlerts handles /api/v1/alerts
func (a *API) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listAlerts(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAlertByID handles /api/v1/alerts/{id}
func (a *API) handleAlertByID(w http.ResponseWriter, r *http.Request) {
	// Extract alert ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/alerts/")
	parts := strings.SplitN(path, "/", 2)
	alertID := parts[0]

	if alertID == "" {
		http.Error(w, "Alert ID required", http.StatusBadRequest)
		return
	}

	// Check for action (acknowledge, silence, resolve)
	var action string
	if len(parts) > 1 {
		action = parts[1]
	}

	switch r.Method {
	case http.MethodGet:
		if action == "" {
			a.getAlert(w, r, alertID)
		} else if action == "events" {
			a.getAlertEvents(w, r, alertID)
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	case http.MethodPost:
		switch action {
		case "acknowledge":
			a.acknowledgeAlert(w, r, alertID)
		case "silence":
			a.silenceAlert(w, r, alertID)
		case "resolve":
			a.resolveAlert(w, r, alertID)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHealth handles /healthz
func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleReady handles /ready
func (a *API) handleReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// listAlerts lists alerts with filtering and pagination
func (a *API) listAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	filter := AlertFilter{}

	if status := r.URL.Query().Get("status"); status != "" {
		for _, s := range strings.Split(status, ",") {
			filter.Status = append(filter.Status, AlertState(s))
		}
	}

	if severity := r.URL.Query().Get("severity"); severity != "" {
		for _, s := range strings.Split(severity, ",") {
			filter.Severity = append(filter.Severity, AlertSeverity(s))
		}
	}

	if targetName := r.URL.Query().Get("targetName"); targetName != "" {
		filter.TargetName = targetName
	}

	if namespace := r.URL.Query().Get("namespace"); namespace != "" {
		filter.Namespace = namespace
	}

	if blastRadius := r.URL.Query().Get("blastRadius"); blastRadius != "" {
		filter.BlastRadius = blastRadius
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}

	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		filter.Cursor = cursor
	}

	// Query alerts
	result, err := a.manager.ListAlerts(ctx, filter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list alerts: %v", err), http.StatusInternalServerError)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/json")
	if result.NextCursor != "" {
		w.Header().Set("X-Next-Cursor", result.NextCursor)
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(result.Total))

	// Write response
	if err := json.NewEncoder(w).Encode(result); err != nil {
		fmt.Printf("Failed to encode response: %v\n", err)
	}
}

// getAlert retrieves a single alert
func (a *API) getAlert(w http.ResponseWriter, r *http.Request, alertID string) {
	ctx := r.Context()

	alert, err := a.manager.GetAlert(ctx, alertID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Alert not found: %s", alertID), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(alert); err != nil {
		fmt.Printf("Failed to encode alert: %v\n", err)
	}
}

// getAlertEvents retrieves events for an alert
func (a *API) getAlertEvents(w http.ResponseWriter, r *http.Request, alertID string) {
	ctx := r.Context()

	events, err := a.manager.GetAlertEvents(ctx, alertID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get events: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(events); err != nil {
		fmt.Printf("Failed to encode events: %v\n", err)
	}
}

// acknowledgeAlert acknowledges an alert
func (a *API) acknowledgeAlert(w http.ResponseWriter, r *http.Request, alertID string) {
	ctx := r.Context()

	var req AcknowledgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.User == "" {
		http.Error(w, "User is required", http.StatusBadRequest)
		return
	}

	if err := a.manager.AcknowledgeAlert(ctx, alertID, &req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to acknowledge alert: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"}); err != nil {
		fmt.Printf("Failed to encode response: %v\n", err)
	}
}

// silenceAlert silences an alert
func (a *API) silenceAlert(w http.ResponseWriter, r *http.Request, alertID string) {
	ctx := r.Context()

	var req SilenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.User == "" {
		http.Error(w, "User is required", http.StatusBadRequest)
		return
	}

	if req.Duration == "" {
		http.Error(w, "Duration is required", http.StatusBadRequest)
		return
	}

	// Validate duration
	if _, err := time.ParseDuration(req.Duration); err != nil {
		http.Error(w, fmt.Sprintf("Invalid duration: %v", err), http.StatusBadRequest)
		return
	}

	if err := a.manager.SilenceAlert(ctx, alertID, &req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to silence alert: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "silenced"}); err != nil {
		fmt.Printf("Failed to encode response: %v\n", err)
	}
}

// resolveAlert resolves an alert
func (a *API) resolveAlert(w http.ResponseWriter, r *http.Request, alertID string) {
	ctx := r.Context()

	var req ResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.User == "" {
		http.Error(w, "User is required", http.StatusBadRequest)
		return
	}

	if err := a.manager.ResolveAlert(ctx, alertID, &req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to resolve alert: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "resolved"}); err != nil {
		fmt.Printf("Failed to encode response: %v\n", err)
	}
}

// Start starts the API server
func (a *API) Start(addr string) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           a,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return server.ListenAndServe()
}
