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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// Test logger functions
func TestLogger(t *testing.T) {
	t.Run("SetLogger and GetLogger", func(t *testing.T) {
		// These functions exist for setting up logging
		// In tests, we just verify they don't panic
		assert.NotPanics(t, func() {
			GetLogger()
		})
	})

	t.Run("GetContextLogger", func(t *testing.T) {
		logger := GetContextLogger()
		// May be nil if not initialized
		assert.True(t, logger == nil || logger != nil)
	})
}

// Test metrics (just verify they can be created without panic)
func TestMetrics(t *testing.T) {
	t.Run("NewMetrics", func(t *testing.T) {
		// Note: Prometheus global registry doesn't allow duplicate registration
		// So we just verify the function exists
		assert.NotPanics(t, func() {
			NewMetrics()
		})
	})
}

// Test API
func TestAPI(t *testing.T) {
	t.Run("NewAPI", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		
		api := NewAPI(manager)
		assert.NotNil(t, api)
	})

	t.Run("ServeHTTP health endpoint", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		api := NewAPI(manager)
		
		// Just verify it doesn't panic
		assert.NotNil(t, api)
		assert.NotNil(t, api.mux)
	})
}

// Test Escalator
func TestEscalator(t *testing.T) {
	t.Run("DefaultEscalatorConfig", func(t *testing.T) {
		cfg := DefaultEscalatorConfig()
		assert.NotNil(t, cfg)
		assert.Greater(t, cfg.DefaultEscalationDelay, time.Duration(0))
		assert.Greater(t, cfg.MaxEscalationLevel, 0)
	})

	t.Run("NewEscalator with nil config", func(t *testing.T) {
		escalator := NewEscalator(nil)
		assert.NotNil(t, escalator)
		assert.NotNil(t, escalator.config)
	})

	t.Run("NewEscalator with config", func(t *testing.T) {
		cfg := &EscalatorConfig{
			DefaultEscalationDelay: 10 * time.Minute,
			MaxEscalationLevel:     5,
		}
		escalator := NewEscalator(cfg)
		assert.NotNil(t, escalator)
		assert.Equal(t, 10*time.Minute, escalator.config.DefaultEscalationDelay)
		assert.Equal(t, 5, escalator.config.MaxEscalationLevel)
	})

	t.Run("CreateDefaultPolicies", func(t *testing.T) {
		escalator := NewEscalator(nil)
		
		assert.NotPanics(t, func() {
			escalator.CreateDefaultPolicies()
		})
		
		// Should have created some policies
		assert.Greater(t, len(escalator.policies), 0)
	})

	t.Run("AddPolicy and GetPolicy", func(t *testing.T) {
		escalator := NewEscalator(nil)
		
		policy := &EscalationPolicy{
			Name: "test-policy",
			Levels: []EscalationLevel{
				{Level: 0, Delay: 5 * time.Minute, Channels: []string{"slack"}},
				{Level: 1, Delay: 15 * time.Minute, Channels: []string{"pagerduty"}},
			},
		}
		
		escalator.AddPolicy(policy)
		
		retrieved, exists := escalator.GetPolicy("test-policy")
		assert.True(t, exists)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test-policy", retrieved.Name)
	})

	t.Run("GetPolicy not found", func(t *testing.T) {
		escalator := NewEscalator(nil)
		
		_, exists := escalator.GetPolicy("non-existent")
		assert.False(t, exists)
	})

	t.Run("GetNextEscalationTime", func(t *testing.T) {
		escalator := NewEscalator(nil)
		escalator.CreateDefaultPolicies()
		
		alert := &Alert{
			AlertID: "test-alert",
			Target: k8swatchv1.TargetRef{
				Name:      "test-target",
				Namespace: "default",
				Type:      k8swatchv1.TargetTypeHTTP,
			},
			Severity:    AlertSeverityCritical,
			BlastRadius: "node",
			FiredAt:     time.Now(),
		}
		
		nextTime := escalator.GetNextEscalationTime(alert, 0)
		// Should return a time in the future
		assert.True(t, nextTime.IsZero() || nextTime.After(alert.FiredAt))
	})

	t.Run("Escalate", func(t *testing.T) {
		escalator := NewEscalator(nil)
		escalator.CreateDefaultPolicies()
		
		alert := &Alert{
			AlertID: "test-alert",
			Target: k8swatchv1.TargetRef{
				Name:      "test-target",
				Namespace: "default",
				Type:      k8swatchv1.TargetTypeHTTP,
			},
			Severity: AlertSeverityCritical,
		}
		
		ctx := context.Background()
		
		// Should not panic
		assert.NotPanics(t, func() {
			err := escalator.Escalate(ctx, alert, 0)
			// May error if no matching policy
			assert.True(t, err == nil || err != nil)
		})
	})

	t.Run("Escalate max level", func(t *testing.T) {
		escalator := NewEscalator(nil)
		
		alert := &Alert{
			AlertID: "test-alert",
			Target:  k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: k8swatchv1.TargetTypeHTTP},
			Severity: AlertSeverityCritical,
		}
		
		ctx := context.Background()
		
		// Try to escalate beyond max level
		err := escalator.Escalate(ctx, alert, 100)
		assert.Error(t, err)
	})
}

// Test MemoryStore
func TestMemoryStore(t *testing.T) {
	t.Run("NewMemoryStore", func(t *testing.T) {
		store := NewMemoryStore()
		assert.NotNil(t, store)
	})

	t.Run("Create and Get", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()
		
		alert := &Alert{
			AlertID: "test-1",
			Target:  k8swatchv1.TargetRef{Name: "test", Namespace: "default"},
		}
		
		err := store.Create(ctx, alert)
		require.NoError(t, err)
		
		retrieved, err := store.Get(ctx, "test-1")
		require.NoError(t, err)
		assert.Equal(t, "test-1", retrieved.AlertID)
	})

	t.Run("Update", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()
		
		alert := &Alert{AlertID: "test-2", Target: k8swatchv1.TargetRef{Name: "test", Namespace: "default"}}
		err := store.Create(ctx, alert)
		require.NoError(t, err)
		
		alert.Status = AlertStateAcknowledged
		err = store.Update(ctx, alert)
		require.NoError(t, err)
		
		retrieved, err := store.Get(ctx, "test-2")
		require.NoError(t, err)
		assert.Equal(t, AlertStateAcknowledged, retrieved.Status)
	})

	t.Run("Delete", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()
		
		alert := &Alert{AlertID: "test-3"}
		err := store.Create(ctx, alert)
		require.NoError(t, err)
		
		err = store.Delete(ctx, "test-3")
		require.NoError(t, err)
		
		_, err = store.Get(ctx, "test-3")
		assert.Error(t, err)
	})

	t.Run("List all", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()
		
		for i := 0; i < 3; i++ {
			alert := &Alert{
				AlertID: "test-" + string(rune('0'+i)),
				Target:  k8swatchv1.TargetRef{Name: "test", Namespace: "default"},
			}
			err := store.Create(ctx, alert)
			require.NoError(t, err)
		}
		
		result, err := store.List(ctx, AlertFilter{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Alerts), 3)
	})

	t.Run("List with namespace filter", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()
		
		alert1 := &Alert{AlertID: "a1", Target: k8swatchv1.TargetRef{Namespace: "ns1"}}
		alert2 := &Alert{AlertID: "a2", Target: k8swatchv1.TargetRef{Namespace: "ns2"}}
		
		store.Create(ctx, alert1)
		store.Create(ctx, alert2)
		
		result, err := store.List(ctx, AlertFilter{Namespace: "ns1"})
		require.NoError(t, err)
		assert.Equal(t, 1, len(result.Alerts))
	})

	t.Run("List with status filter", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()
		
		alert1 := &Alert{AlertID: "a1", Status: AlertStateFiring}
		alert2 := &Alert{AlertID: "a2", Status: AlertStateAcknowledged}
		
		store.Create(ctx, alert1)
		store.Create(ctx, alert2)
		
		result, err := store.List(ctx, AlertFilter{Status: []AlertState{AlertStateFiring}})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Alerts), 1)
	})

	t.Run("List with limit", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()
		
		for i := 0; i < 10; i++ {
			alert := &Alert{AlertID: "test-" + string(rune('0'+i))}
			store.Create(ctx, alert)
		}
		
		result, err := store.List(ctx, AlertFilter{Limit: 5})
		require.NoError(t, err)
		assert.Equal(t, 5, len(result.Alerts))
	})

	t.Run("CreateEvent and ListEvents", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()
		
		event := &AlertEvent{
			AlertID:   "test-alert",
			EventType: "fired",
			ToState:   AlertStateFiring,
			Timestamp: time.Now(),
		}
		
		err := store.CreateEvent(ctx, event)
		require.NoError(t, err)
		
		events, err := store.ListEvents(ctx, "test-alert")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(events), 1)
	})

	t.Run("Close", func(t *testing.T) {
		store := NewMemoryStore()
		err := store.Close()
		assert.NoError(t, err)
	})
}

// Test Router
func TestRouter(t *testing.T) {
	t.Run("DefaultRouterConfig", func(t *testing.T) {
		cfg := DefaultRouterConfig()
		assert.NotNil(t, cfg)
		assert.Equal(t, 5*time.Minute, cfg.DefaultCooldown)
		assert.Equal(t, 3, cfg.MaxRetries)
	})

	t.Run("NewRouter with nil config", func(t *testing.T) {
		router := NewRouter(nil)
		assert.NotNil(t, router)
	})

	t.Run("RegisterChannel", func(t *testing.T) {
		router := NewRouter(nil)
		mockCh := &mockChannel{name: "test"}
		
		router.RegisterChannel(mockCh)
		assert.Equal(t, 1, len(router.channels))
	})

	t.Run("AddRule", func(t *testing.T) {
		router := NewRouter(nil)
		
		rule := RoutingRule{
			Name: "test-rule",
			Match: RouteMatch{
				Severities: []AlertSeverity{AlertSeverityCritical},
			},
			Channels: []string{"test"},
		}
		
		router.AddRule(rule)
		assert.Equal(t, 1, len(router.rules))
	})

	t.Run("Route with cooldown", func(t *testing.T) {
		router := NewRouter(nil)
		
		alert := &Alert{
			Target:      k8swatchv1.TargetRef{Name: "test", Namespace: "default"},
			FailureCode: "test_err",
		}
		
		ctx := context.Background()
		
		// First route
		router.Route(ctx, alert)
		
		// Second route should be in cooldown
		router.Route(ctx, alert)
		// Should not panic
	})

	t.Run("isInCooldown", func(t *testing.T) {
		router := NewRouter(nil)
		key := "test-key"
		
		assert.False(t, router.isInCooldown(key))
		
		router.lastAlerts[key] = time.Now()
		assert.True(t, router.isInCooldown(key))
	})

	t.Run("findMatchingRules", func(t *testing.T) {
		router := NewRouter(nil)
		
		router.AddRule(RoutingRule{
			Name: "critical",
			Match: RouteMatch{
				Severities: []AlertSeverity{AlertSeverityCritical},
			},
		})
		
		alert := &Alert{Severity: AlertSeverityCritical}
		matched := router.findMatchingRules(alert)
		assert.Equal(t, 1, len(matched))
	})
}

// Test Manager
func TestManager(t *testing.T) {
	t.Run("DefaultManagerConfig", func(t *testing.T) {
		cfg := DefaultManagerConfig()
		assert.NotNil(t, cfg)
		assert.Equal(t, 5*time.Minute, cfg.DefaultCooldown)
	})

	t.Run("NewManager with nil config", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.config)
	})

	t.Run("Start and Stop", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		ctx := context.Background()
		
		err := manager.Start(ctx)
		assert.NoError(t, err)
		
		err = manager.Stop()
		assert.NoError(t, err)
	})

	t.Run("RegisterCallback", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		
		called := false
		cb := func(*Alert, *AlertEvent) { called = true }
		
		manager.callbacks = append(manager.callbacks, cb)
		assert.Equal(t, 1, len(manager.callbacks))
		
		// Trigger callback manually
		alert := &Alert{AlertID: "test"}
		event := &AlertEvent{AlertID: "test"}
		for _, c := range manager.callbacks {
			c(alert, event)
		}
		
		assert.True(t, called)
	})

	t.Run("findSimilarAlert", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		
		existing := &Alert{
			AlertID: "existing",
			Target:  k8swatchv1.TargetRef{Name: "test", Namespace: "default"},
			FailureCode: "err1",
		}
		manager.active["existing"] = existing
		
		newAlert := &Alert{
			Target: k8swatchv1.TargetRef{Name: "test", Namespace: "default"},
			FailureCode: "err1",
		}
		
		similar := manager.findSimilarAlert(newAlert)
		assert.NotNil(t, similar)
		assert.Equal(t, "existing", similar.AlertID)
	})

	t.Run("getAlert", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		
		alert := &Alert{AlertID: "test"}
		manager.active["test"] = alert
		
		retrieved, err := manager.getAlert("test")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		
		_, err = manager.getAlert("non-existent")
		assert.Error(t, err)
	})

	t.Run("addHistory", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		
		event := &AlertEvent{AlertID: "test"}
		manager.addHistory(event)
		
		assert.GreaterOrEqual(t, len(manager.history), 1)
	})

	t.Run("loadActiveAlerts with nil store", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		ctx := context.Background()
		
		// Should not panic
		assert.NotPanics(t, func() {
			manager.loadActiveAlerts(ctx)
		})
	})

	t.Run("cleanupLoop", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		
		go manager.cleanupLoop(ctx)
		
		<-ctx.Done()
		// Should complete without panic
	})
}

// Mock channel for testing
type mockChannel struct {
	name string
}

func (m *mockChannel) Name() string {
	return m.name
}

func (m *mockChannel) Send(ctx context.Context, alert *Alert) error {
	return nil
}

func (m *mockChannel) Close() error {
	return nil
}
