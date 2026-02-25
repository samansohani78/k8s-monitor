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
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLocalTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping test: cannot bind local test server: %v", err)
	}
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	return server
}

// TestAPICreation tests API server creation
func TestAPICreation(t *testing.T) {
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	api := NewAPI(manager)

	require.NotNil(t, api)
	assert.NotNil(t, api.manager)
	assert.NotNil(t, api.mux)
}

// TestAPIRoutes tests route registration
func TestAPIRoutes(t *testing.T) {
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	api := NewAPI(manager)

	// Test routes exist
	assert.NotNil(t, api.mux)

	// Create test requests to verify routes are registered
	tests := []struct {
		path   string
		method string
	}{
		{"/api/v1/alerts", http.MethodGet},
		{"/healthz", http.MethodGet},
		{"/ready", http.MethodGet},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		// Should not return 404
		assert.NotEqual(t, http.StatusNotFound, w.Code, "Route %s should exist", tt.path)
	}
}

// TestServeHTTP tests HTTP handler
func TestServeHTTP(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHandleAlerts_GET tests listing alerts via API
func TestHandleAlerts_GET(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alert
	alert := &Alert{
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		Severity:     AlertSeverityCritical,
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	_ = manager.CreateAlert(ctx, alert)

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result struct {
		Alerts []Alert `json:"alerts"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Alerts)
}

// TestHandleAlertByID_Found tests getting a specific alert
func TestHandleAlertByID_Found(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alert
	alert := &Alert{
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		Severity:     AlertSeverityCritical,
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	_ = manager.CreateAlert(ctx, alert)

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/"+alert.AlertID, nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHandleAlertByID_NotFound tests getting non-existent alert
func TestHandleAlertByID_NotFound(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/non-existent-id", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleAlertByID_InvalidID tests getting alert with invalid ID
func TestHandleAlertByID_InvalidID(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAcknowledgeAlertAPI tests acknowledging alert via API
func TestAcknowledgeAlertAPI(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alert
	alert := &Alert{
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		Severity:     AlertSeverityCritical,
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	_ = manager.CreateAlert(ctx, alert)

	api := NewAPI(manager)

	// Create acknowledge request
	reqBody := map[string]string{
		"user":    "test-user",
		"comment": "Investigating",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/"+alert.AlertID+"/acknowledge", bytes.NewReader(body))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestSilenceAlertAPI tests silencing alert via API
func TestSilenceAlertAPI(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alert
	alert := &Alert{
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		Severity:     AlertSeverityCritical,
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	_ = manager.CreateAlert(ctx, alert)

	api := NewAPI(manager)

	// Create silence request
	reqBody := map[string]string{
		"user":     "test-user",
		"duration": "1h",
		"reason":   "Maintenance",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/"+alert.AlertID+"/silence", bytes.NewReader(body))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestResolveAlertAPI tests resolving alert via API
func TestResolveAlertAPI(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alert
	alert := &Alert{
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		Severity:     AlertSeverityCritical,
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	_ = manager.CreateAlert(ctx, alert)

	api := NewAPI(manager)

	// Create resolve request
	reqBody := map[string]string{
		"user":    "test-user",
		"comment": "Issue fixed",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/"+alert.AlertID+"/resolve", bytes.NewReader(body))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHealthEndpoint tests health check endpoint
func TestHealthEndpoint(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

// TestReadyEndpoint tests readiness endpoint
func TestReadyEndpoint(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ready")
}

// TestInvalidRequests_BadJSON tests handling invalid JSON
func TestInvalidRequests_BadJSON(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alert
	alert := &Alert{
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		Severity:     AlertSeverityCritical,
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	_ = manager.CreateAlert(ctx, alert)

	api := NewAPI(manager)

	// Send invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/"+alert.AlertID+"/acknowledge", bytes.NewReader([]byte("invalid-json")))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestInvalidRequests_BadMethod tests handling unsupported HTTP methods
func TestInvalidRequests_BadMethod(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	api := NewAPI(manager)

	// Try DELETE on alerts endpoint
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// TestListAlerts_FilterByState tests filtering alerts by state via API
func TestListAlerts_FilterByState(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alerts
	for i := 0; i < 3; i++ {
		alert := &Alert{
			Target:       k8swatchv1.TargetRef{Name: "test-" + string(rune('A'+i)), Namespace: "default", Type: "http"},
			Severity:     AlertSeverityCritical,
			FailureLayer: "L2",
			FailureCode:  "tcp_refused",
		}
		_ = manager.CreateAlert(ctx, alert)
	}

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?status=firing", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestListAlerts_FilterByTarget tests filtering alerts by target via API
func TestListAlerts_FilterByTarget(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alerts
	alert := &Alert{
		Target:       k8swatchv1.TargetRef{Name: "specific-target", Namespace: "default", Type: "http"},
		Severity:     AlertSeverityCritical,
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	_ = manager.CreateAlert(ctx, alert)

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?targetName=specific-target", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestListAlerts_Pagination tests alert pagination via API
func TestListAlerts_Pagination(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create multiple test alerts
	for i := 0; i < 10; i++ {
		alert := &Alert{
			Target:       k8swatchv1.TargetRef{Name: "test-" + string(rune('A'+i)), Namespace: "default", Type: "http"},
			Severity:     AlertSeverityCritical,
			FailureLayer: "L2",
			FailureCode:  "tcp_refused",
		}
		_ = manager.CreateAlert(ctx, alert)
	}

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?limit=5", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetAlertEvents tests getting alert events via API
func TestGetAlertEvents(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	// Create test alert
	alert := &Alert{
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		Severity:     AlertSeverityCritical,
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	_ = manager.CreateAlert(ctx, alert)

	// Acknowledge to create event
	_ = manager.AcknowledgeAlert(ctx, alert.AlertID, &AcknowledgeRequest{
		User:    "test-user",
		Comment: "ACK",
	})

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/"+alert.AlertID+"/events", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAPI_StartStop tests API server lifecycle
func TestAPI_StartStop(t *testing.T) {
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	api := NewAPI(manager)
	require.NotNil(t, api)

	// Start server
	server := newLocalTestServer(t, api)
	defer server.Close()

	// Make request to verify server is running
	resp, err := http.Get(server.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestAPIMetricsEndpoint tests metrics endpoint
func TestAPIMetricsEndpoint(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)
	_ = manager.Start(ctx)
	defer func() { _ = manager.Stop() }()

	api := NewAPI(manager)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	// Metrics endpoint should return 200
	assert.Equal(t, http.StatusOK, w.Code)
}
