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
	"sort"
	"sync"
)

// MemoryStore is an in-memory alert store (for testing and development)
type MemoryStore struct {
	mu     sync.RWMutex
	alerts map[string]*Alert
	events map[string][]AlertEvent // alertId -> events
	closed bool
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		alerts: make(map[string]*Alert),
		events: make(map[string][]AlertEvent),
	}
}

// Create creates a new alert
func (s *MemoryStore) Create(ctx context.Context, alert *Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if _, exists := s.alerts[alert.AlertID]; exists {
		return fmt.Errorf("alert already exists: %s", alert.AlertID)
	}

	// Store a copy
	s.alerts[alert.AlertID] = copyAlert(alert)
	return nil
}

// Update updates an existing alert
func (s *MemoryStore) Update(ctx context.Context, alert *Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if _, exists := s.alerts[alert.AlertID]; !exists {
		return fmt.Errorf("alert not found: %s", alert.AlertID)
	}

	s.alerts[alert.AlertID] = copyAlert(alert)
	return nil
}

// Get retrieves an alert by ID
func (s *MemoryStore) Get(ctx context.Context, alertID string) (*Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	alert, exists := s.alerts[alertID]
	if !exists {
		return nil, fmt.Errorf("alert not found: %s", alertID)
	}

	return copyAlert(alert), nil
}

// List lists alerts matching the filter
func (s *MemoryStore) List(ctx context.Context, filter AlertFilter) (*AlertQueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	var alerts []Alert

	for _, alert := range s.alerts {
		if !matchesFilter(alert, filter) {
			continue
		}
		alerts = append(alerts, *alert)
	}

	// Sort by FiredAt descending
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].FiredAt.After(alerts[j].FiredAt)
	})

	total := len(alerts)

	// Apply limit
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > total {
		limit = total
	}

	return &AlertQueryResult{
		Alerts: alerts[:limit],
		Total:  total,
	}, nil
}

// Delete deletes an alert
func (s *MemoryStore) Delete(ctx context.Context, alertID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if _, exists := s.alerts[alertID]; !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	delete(s.alerts, alertID)
	delete(s.events, alertID)
	return nil
}

// CreateEvent creates an alert event
func (s *MemoryStore) CreateEvent(ctx context.Context, event *AlertEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	s.events[event.AlertID] = append(s.events[event.AlertID], *event)
	return nil
}

// ListEvents lists events for an alert
func (s *MemoryStore) ListEvents(ctx context.Context, alertID string) ([]AlertEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	events, exists := s.events[alertID]
	if !exists {
		return []AlertEvent{}, nil
	}

	result := make([]AlertEvent, len(events))
	copy(result, events)
	return result, nil
}

// Close closes the store
func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// matchesFilter checks if an alert matches the filter
func matchesFilter(alert *Alert, filter AlertFilter) bool {
	// Filter by status
	if len(filter.Status) > 0 {
		found := false
		for _, status := range filter.Status {
			if alert.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by severity
	if len(filter.Severity) > 0 {
		found := false
		for _, severity := range filter.Severity {
			if alert.Severity == severity {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by target name
	if filter.TargetName != "" && alert.Target.Name != filter.TargetName {
		return false
	}

	// Filter by namespace
	if filter.Namespace != "" && alert.Target.Namespace != filter.Namespace {
		return false
	}

	// Filter by blast radius
	if filter.BlastRadius != "" && alert.BlastRadius != filter.BlastRadius {
		return false
	}

	// Filter by time range
	if !filter.From.IsZero() && alert.FiredAt.Before(filter.From) {
		return false
	}
	if !filter.To.IsZero() && alert.FiredAt.After(filter.To) {
		return false
	}

	return true
}

// copyAlert creates a deep copy of an alert
func copyAlert(alert *Alert) *Alert {
	if alert == nil {
		return nil
	}

	c := *alert
	c.AffectedNodes = make([]string, len(alert.AffectedNodes))
	copy(c.AffectedNodes, alert.AffectedNodes)

	if alert.Labels != nil {
		c.Labels = make(map[string]string)
		for k, v := range alert.Labels {
			c.Labels[k] = v
		}
	}

	if alert.Annotations != nil {
		c.Annotations = make(map[string]string)
		for k, v := range alert.Annotations {
			c.Annotations[k] = v
		}
	}

	return &c
}
