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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ManagerConfig holds alert manager configuration
type ManagerConfig struct {
	// DefaultCooldown is the minimum time between same alerts
	DefaultCooldown time.Duration
	// AutoResolveTimeout is the timeout for auto-resolving alerts
	AutoResolveTimeout time.Duration
	// MaxActiveAlerts is the maximum number of active alerts to keep in memory
	MaxActiveAlerts int
	// HistoryRetention is how long to keep alert history
	HistoryRetention time.Duration
}

// DefaultManagerConfig returns the default manager configuration
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		DefaultCooldown:    5 * time.Minute,
		AutoResolveTimeout: 24 * time.Hour,
		MaxActiveAlerts:    1000,
		HistoryRetention:   90 * 24 * time.Hour,
	}
}

// Manager manages the alert lifecycle
type Manager struct {
	config     *ManagerConfig
	store      AlertStore
	routing    *Router
	mu         sync.RWMutex
	active     map[string]*Alert // alertId -> Alert
	history    []AlertEvent      // Recent events for audit
	callbacks  []AlertCallback   // Callbacks for alert state changes
	shutdownCh chan struct{}
}

// AlertCallback is called when alert state changes
type AlertCallback func(alert *Alert, event *AlertEvent)

// AlertStore is the interface for alert persistence
type AlertStore interface {
	// Create creates a new alert
	Create(ctx context.Context, alert *Alert) error
	// Update updates an existing alert
	Update(ctx context.Context, alert *Alert) error
	// Get retrieves an alert by ID
	Get(ctx context.Context, alertID string) (*Alert, error)
	// List lists alerts matching the filter
	List(ctx context.Context, filter AlertFilter) (*AlertQueryResult, error)
	// Delete deletes an alert
	Delete(ctx context.Context, alertID string) error
	// CreateEvent creates an alert event
	CreateEvent(ctx context.Context, event *AlertEvent) error
	// ListEvents lists events for an alert
	ListEvents(ctx context.Context, alertID string) ([]AlertEvent, error)
	// Close closes the store
	Close() error
}

// NewManager creates a new alert manager
func NewManager(config *ManagerConfig, store AlertStore, routing *Router) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	if store == nil {
		store = NewMemoryStore()
	}

	m := &Manager{
		config:     config,
		store:      store,
		routing:    routing,
		active:     make(map[string]*Alert),
		history:    make([]AlertEvent, 0),
		callbacks:  make([]AlertCallback, 0),
		shutdownCh: make(chan struct{}),
	}

	return m
}

// Start starts the alert manager
func (m *Manager) Start(ctx context.Context) error {
	// Load existing active alerts from store
	if err := m.loadActiveAlerts(ctx); err != nil {
		return fmt.Errorf("failed to load active alerts: %w", err)
	}

	// Start background cleanup
	go m.cleanupLoop(ctx)

	return nil
}

// Stop stops the alert manager
func (m *Manager) Stop() error {
	close(m.shutdownCh)
	if m.store != nil {
		return m.store.Close()
	}
	return nil
}

// CreateAlert creates a new alert
func (m *Manager) CreateAlert(ctx context.Context, alert *Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Initialize alert
	alert.AlertID = uuid.New().String()
	alert.FiredAt = now
	alert.LastUpdatedAt = now
	alert.Status = AlertStateFiring
	alert.NotificationCount = 0

	// Check for deduplication
	existingAlert := m.findSimilarAlert(alert)
	if existingAlert != nil {
		// Update existing alert instead of creating new one
		existingAlert.LastUpdatedAt = now
		existingAlert.ConsecutiveFailures = alert.ConsecutiveFailures
		return m.store.Update(ctx, existingAlert)
	}

	// Store alert
	if err := m.store.Create(ctx, alert); err != nil {
		return fmt.Errorf("failed to create alert: %w", err)
	}

	// Add to active alerts
	m.active[alert.AlertID] = alert

	// Create event
	event := &AlertEvent{
		EventID:   uuid.New().String(),
		AlertID:   alert.AlertID,
		EventType: "fired",
		ToState:   AlertStateFiring,
		Reason:    "Alert triggered by failure threshold",
		Timestamp: now,
	}
	if err := m.store.CreateEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	m.addHistory(event)

	// Notify callbacks
	m.notifyCallbacks(alert, event)

	// Trigger notifications via routing
	if m.routing != nil {
		if err := m.routing.Route(ctx, alert); err != nil {
			// Log error but don't fail alert creation
			fmt.Printf("Failed to route alert %s: %v\n", alert.AlertID, err)
		}
	}

	return nil
}

// ResolveAlert resolves an alert
func (m *Manager) ResolveAlert(ctx context.Context, alertID string, req *ResolveRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, err := m.getAlert(alertID)
	if err != nil {
		return err
	}

	if alert.Status == AlertStateResolved {
		return fmt.Errorf("alert already resolved")
	}

	now := time.Now()
	oldStatus := alert.Status

	alert.Status = AlertStateResolved
	alert.ResolvedAt = &now
	alert.LastUpdatedAt = now

	// Update store
	if err := m.store.Update(ctx, alert); err != nil {
		return fmt.Errorf("failed to update alert: %w", err)
	}

	// Create event
	event := &AlertEvent{
		EventID:   uuid.New().String(),
		AlertID:   alertID,
		EventType: "resolved",
		FromState: oldStatus,
		ToState:   AlertStateResolved,
		Reason:    req.Comment,
		User:      req.User,
		Timestamp: now,
	}
	if err := m.store.CreateEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	m.addHistory(event)

	// Notify callbacks
	m.notifyCallbacks(alert, event)

	return nil
}

// AcknowledgeAlert acknowledges an alert
func (m *Manager) AcknowledgeAlert(ctx context.Context, alertID string, req *AcknowledgeRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, err := m.getAlert(alertID)
	if err != nil {
		return err
	}

	if alert.Status != AlertStateFiring {
		return fmt.Errorf("can only acknowledge firing alerts")
	}

	now := time.Now()
	oldStatus := alert.Status

	alert.Status = AlertStateAcknowledged
	alert.AcknowledgedAt = &now
	alert.AcknowledgedBy = req.User
	alert.LastUpdatedAt = now

	// Update store
	if err := m.store.Update(ctx, alert); err != nil {
		return fmt.Errorf("failed to update alert: %w", err)
	}

	// Create event
	event := &AlertEvent{
		EventID:   uuid.New().String(),
		AlertID:   alertID,
		EventType: "acknowledged",
		FromState: oldStatus,
		ToState:   AlertStateAcknowledged,
		Reason:    req.Comment,
		User:      req.User,
		Timestamp: now,
	}
	if err := m.store.CreateEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	m.addHistory(event)

	// Notify callbacks
	m.notifyCallbacks(alert, event)

	return nil
}

// SilenceAlert silences an alert
func (m *Manager) SilenceAlert(ctx context.Context, alertID string, req *SilenceRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, err := m.getAlert(alertID)
	if err != nil {
		return err
	}

	now := time.Now()
	duration, err := time.ParseDuration(req.Duration)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	silenceEnds := now.Add(duration)
	oldStatus := alert.Status

	alert.Status = AlertStateSilenced
	alert.SilencedAt = &now
	alert.SilencedBy = req.User
	alert.SilenceReason = req.Reason
	alert.SilenceEndsAt = &silenceEnds
	alert.LastUpdatedAt = now

	// Update store
	if err := m.store.Update(ctx, alert); err != nil {
		return fmt.Errorf("failed to update alert: %w", err)
	}

	// Create event
	event := &AlertEvent{
		EventID:   uuid.New().String(),
		AlertID:   alertID,
		EventType: "silenced",
		FromState: oldStatus,
		ToState:   AlertStateSilenced,
		Reason:    req.Reason,
		User:      req.User,
		Timestamp: now,
	}
	if err := m.store.CreateEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	m.addHistory(event)

	// Notify callbacks
	m.notifyCallbacks(alert, event)

	return nil
}

// GetAlert retrieves an alert by ID
func (m *Manager) GetAlert(ctx context.Context, alertID string) (*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getAlert(alertID)
}

// ListAlerts lists alerts matching the filter
func (m *Manager) ListAlerts(ctx context.Context, filter AlertFilter) (*AlertQueryResult, error) {
	return m.store.List(ctx, filter)
}

// GetAlertEvents retrieves events for an alert
func (m *Manager) GetAlertEvents(ctx context.Context, alertID string) ([]AlertEvent, error) {
	return m.store.ListEvents(ctx, alertID)
}

// OnAlert adds a callback for alert state changes
func (m *Manager) OnAlert(callback AlertCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// getAlert retrieves an alert (must be called with lock held)
func (m *Manager) getAlert(alertID string) (*Alert, error) {
	if alert, ok := m.active[alertID]; ok {
		return alert, nil
	}

	// Try to load from store
	alert, err := m.store.Get(context.Background(), alertID)
	if err != nil {
		return nil, fmt.Errorf("alert not found: %s", alertID)
	}

	return alert, nil
}

// findSimilarAlert finds a similar active alert for deduplication
func (m *Manager) findSimilarAlert(alert *Alert) *Alert {
	for _, a := range m.active {
		if a.Status == AlertStateResolved || a.Status == AlertStateSilenced {
			continue
		}
		if a.Target.Name == alert.Target.Name &&
			a.Target.Namespace == alert.Target.Namespace &&
			a.FailureLayer == alert.FailureLayer &&
			a.FailureCode == alert.FailureCode {
			return a
		}
	}
	return nil
}

// loadActiveAlerts loads active alerts from the store
func (m *Manager) loadActiveAlerts(ctx context.Context) error {
	result, err := m.store.List(ctx, AlertFilter{
		Status: []AlertState{AlertStateFiring, AlertStateAcknowledged},
		Limit:  m.config.MaxActiveAlerts,
	})
	if err != nil {
		return err
	}

	for _, alert := range result.Alerts {
		m.active[alert.AlertID] = &alert
	}

	return nil
}

// addHistory adds an event to history (must be called with lock held)
func (m *Manager) addHistory(event *AlertEvent) {
	m.history = append(m.history, *event)

	// Trim history
	if len(m.history) > 1000 {
		m.history = m.history[len(m.history)-1000:]
	}
}

// notifyCallbacks notifies all callbacks
func (m *Manager) notifyCallbacks(alert *Alert, event *AlertEvent) {
	for _, callback := range m.callbacks {
		go callback(alert, event)
	}
}

// cleanupLoop periodically cleans up old alerts
func (m *Manager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.shutdownCh:
			return
		case <-ticker.C:
			m.cleanup(ctx)
		}
	}
}

// cleanup removes resolved alerts older than retention period
func (m *Manager) cleanup(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.config.HistoryRetention)

	for id, alert := range m.active {
		if alert.Status == AlertStateResolved && alert.ResolvedAt != nil {
			if alert.ResolvedAt.Before(cutoff) {
				delete(m.active, id)
				// Don't delete from store - keep for audit
			}
		}
	}

	// Check for expired silences
	for _, alert := range m.active {
		if alert.Status == AlertStateSilenced && alert.SilenceEndsAt != nil {
			if time.Now().After(*alert.SilenceEndsAt) {
				// Silence expired, return to firing state
				alert.Status = AlertStateFiring
				alert.SilencedAt = nil
				alert.SilenceEndsAt = nil
				alert.LastUpdatedAt = time.Now()
				_ = m.store.Update(ctx, alert)
			}
		}
	}
}
