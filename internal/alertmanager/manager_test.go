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
	"sync"
	"testing"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultManagerConfig tests the default configuration
func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	assert.Equal(t, 5*time.Minute, config.DefaultCooldown)
	assert.Equal(t, 24*time.Hour, config.AutoResolveTimeout)
	assert.Equal(t, 1000, config.MaxActiveAlerts)
	assert.Equal(t, 90*24*time.Hour, config.HistoryRetention)
}

// TestNewManager tests manager creation
func TestNewManager(t *testing.T) {
	t.Run("WithNilConfig", func(t *testing.T) {
		manager := NewManager(nil, nil, nil)
		require.NotNil(t, manager)
		assert.NotNil(t, manager.config)
	})

	t.Run("WithCustomConfig", func(t *testing.T) {
		config := &ManagerConfig{
			DefaultCooldown:    10 * time.Minute,
			AutoResolveTimeout: 12 * time.Hour,
			MaxActiveAlerts:    500,
			HistoryRetention:   30 * 24 * time.Hour,
		}
		store := NewMemoryStore()
		routing := NewRouter(DefaultRouterConfig())

		manager := NewManager(config, store, routing)
		require.NotNil(t, manager)
		assert.Equal(t, config, manager.config)
		assert.NotNil(t, manager.store)
		assert.NotNil(t, manager.routing)
		assert.NotNil(t, manager.active)
		assert.NotNil(t, manager.history)
		assert.NotNil(t, manager.callbacks)
		assert.NotNil(t, manager.shutdownCh)
	})

	t.Run("WithNilStore", func(t *testing.T) {
		config := DefaultManagerConfig()
		manager := NewManager(config, nil, nil)
		require.NotNil(t, manager)
		assert.NotNil(t, manager.store)
	})
}

// TestManagerLifecycle tests start and stop operations
func TestManagerLifecycle(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	// Start manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Stop manager
	err = manager.Stop()
	assert.NoError(t, err)
}

// TestCreateAlert tests alert creation
func TestCreateAlert(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1", "node-2"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Verify alert was created
	assert.NotEmpty(t, alert.AlertID)
	assert.Equal(t, AlertStateFiring, alert.Status)
	assert.NotEmpty(t, alert.FiredAt)
	assert.NotEmpty(t, alert.LastUpdatedAt)

	// Verify alert is in active map
	manager.mu.RLock()
	activeAlert, exists := manager.active[alert.AlertID]
	manager.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, alert.AlertID, activeAlert.AlertID)
}

// TestCreateAlert_Deduplication tests alert deduplication
func TestCreateAlert_Deduplication(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create first alert
	alert1 := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert1)
	require.NoError(t, err)
	firstAlertID := alert1.AlertID

	// Create similar alert (should be deduplicated)
	alert2 := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 5, // Different failure count
		AffectedNodes:       []string{"node-2"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert2)
	require.NoError(t, err)

	// Deduplication: second alert should not create a new alert
	// The existing alert is updated instead
	assert.NotEqual(t, "", firstAlertID, "First alert should have ID")

	// Verify only one alert exists (deduplication worked)
	manager.mu.RLock()
	assert.Len(t, manager.active, 1)
	manager.mu.RUnlock()

	// Verify the existing alert was updated with new failure count
	manager.mu.RLock()
	updatedAlert := manager.active[firstAlertID]
	manager.mu.RUnlock()
	assert.Equal(t, int32(5), updatedAlert.ConsecutiveFailures, "Should have updated failure count")
}

// TestResolveAlert tests alert resolution
func TestResolveAlert(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Resolve alert
	resolveReq := &ResolveRequest{
		Comment: "Issue resolved",
		User:    "test-user",
	}

	err = manager.ResolveAlert(ctx, alert.AlertID, resolveReq)
	require.NoError(t, err)

	// Verify alert is resolved
	manager.mu.RLock()
	resolvedAlert, exists := manager.active[alert.AlertID]
	manager.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, AlertStateResolved, resolvedAlert.Status)
	assert.NotNil(t, resolvedAlert.ResolvedAt)
}

// TestResolveAlert_AlreadyResolved tests resolving an already resolved alert
func TestResolveAlert_AlreadyResolved(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create and resolve alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	err = manager.ResolveAlert(ctx, alert.AlertID, &ResolveRequest{Comment: "Resolved", User: "user"})
	require.NoError(t, err)

	// Try to resolve again
	err = manager.ResolveAlert(ctx, alert.AlertID, &ResolveRequest{Comment: "Already resolved", User: "user"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already resolved")
}

// TestAcknowledgeAlert tests alert acknowledgment
func TestAcknowledgeAlert(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Acknowledge alert
	ackReq := &AcknowledgeRequest{
		Comment: "Investigating",
		User:    "oncall-user",
	}

	err = manager.AcknowledgeAlert(ctx, alert.AlertID, ackReq)
	require.NoError(t, err)

	// Verify alert is acknowledged
	manager.mu.RLock()
	ackAlert, exists := manager.active[alert.AlertID]
	manager.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, AlertStateAcknowledged, ackAlert.Status)
	assert.Equal(t, "oncall-user", ackAlert.AcknowledgedBy)
	assert.NotNil(t, ackAlert.AcknowledgedAt)
}

// TestSilenceAlert tests alert silencing
func TestSilenceAlert(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Silence alert
	silenceReq := &SilenceRequest{
		Duration: "1h",
		Reason:   "Maintenance window",
		User:     "admin-user",
	}

	err = manager.SilenceAlert(ctx, alert.AlertID, silenceReq)
	require.NoError(t, err)

	// Verify alert is silenced
	manager.mu.RLock()
	silencedAlert, exists := manager.active[alert.AlertID]
	manager.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, AlertStateSilenced, silencedAlert.Status)
	assert.Equal(t, "admin-user", silencedAlert.SilencedBy)
	assert.Equal(t, "Maintenance window", silencedAlert.SilenceReason)
	assert.NotNil(t, silencedAlert.SilencedAt)
	assert.NotNil(t, silencedAlert.SilenceEndsAt)
}

// TestSilenceAlert_WithDuration tests silencing with different durations
func TestSilenceAlert_WithDuration(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Test various durations
	testCases := []struct {
		duration string
		valid    bool
	}{
		{"30m", true},
		{"1h", true},
		{"2h", true},
		{"invalid", false},
	}

	for _, tc := range testCases {
		t.Run(tc.duration, func(t *testing.T) {
			// Create new alert for each test
			newAlert := &Alert{
				Target: k8swatchv1.TargetRef{
					Name:      "test-target-" + tc.duration,
					Namespace: "default",
					Type:      "http",
				},
				Severity:            "critical",
				FailureLayer:        "L2",
				FailureCode:         "tcp_refused",
				ConsecutiveFailures: 3,
			}
			err = manager.CreateAlert(ctx, newAlert)
			require.NoError(t, err)

			silenceReq := &SilenceRequest{
				Duration: tc.duration,
				Reason:   "Test",
				User:     "test",
			}

			err = manager.SilenceAlert(ctx, newAlert.AlertID, silenceReq)
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// TestGetAlert tests alert retrieval
func TestGetAlert(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Get alert
	retrieved, err := manager.GetAlert(ctx, alert.AlertID)
	require.NoError(t, err)
	assert.Equal(t, alert.AlertID, retrieved.AlertID)
	assert.Equal(t, "test-target", retrieved.Target.Name)
}

// TestGetAlert_NotFound tests getting a non-existent alert
func TestGetAlert_NotFound(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	_, err = manager.GetAlert(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestListAlerts tests listing alerts
func TestListAlerts(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create multiple alerts
	for i := 0; i < 5; i++ {
		alert := &Alert{
			Target: k8swatchv1.TargetRef{
				Name:      "test-target-" + string(rune('A'+i)),
				Namespace: "default",
				Type:      "http",
			},
			Severity:            "critical",
			FailureLayer:        "L2",
			FailureCode:         "tcp_refused",
			ConsecutiveFailures: int32(i) + 1, // nolint:gosec // test value, no overflow risk
			AffectedNodes:       []string{"node-1"},
			BlastRadius:         "node",
		}
		err = manager.CreateAlert(ctx, alert)
		require.NoError(t, err)
	}

	// List all alerts
	result, err := manager.ListAlerts(ctx, AlertFilter{})
	require.NoError(t, err)
	assert.Len(t, result.Alerts, 5)

	// List with state filter
	result, err = manager.ListAlerts(ctx, AlertFilter{
		Status: []AlertState{AlertStateFiring},
	})
	require.NoError(t, err)
	assert.Len(t, result.Alerts, 5)
}

// TestListAlerts_FilteredByState tests filtering by state
func TestListAlerts_FilteredByState(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alerts with different states
	alerts := make([]*Alert, 3)
	for i := 0; i < 3; i++ {
		alert := &Alert{
			Target: k8swatchv1.TargetRef{
				Name:      "test-target-" + string(rune('A'+i)),
				Namespace: "default",
				Type:      "http",
			},
			Severity:            "critical",
			FailureLayer:        "L2",
			FailureCode:         "tcp_refused",
			ConsecutiveFailures: 3,
		}
		err = manager.CreateAlert(ctx, alert)
		require.NoError(t, err)
		alerts[i] = alert
	}

	// Resolve one alert
	err = manager.ResolveAlert(ctx, alerts[0].AlertID, &ResolveRequest{Comment: "Resolved", User: "test"})
	require.NoError(t, err)

	// Acknowledge one alert
	err = manager.AcknowledgeAlert(ctx, alerts[1].AlertID, &AcknowledgeRequest{Comment: "ACK", User: "test"})
	require.NoError(t, err)

	// List firing alerts only
	result, err := manager.ListAlerts(ctx, AlertFilter{
		Status: []AlertState{AlertStateFiring},
	})
	require.NoError(t, err)
	assert.Len(t, result.Alerts, 1)
	assert.Equal(t, alerts[2].AlertID, result.Alerts[0].AlertID)

	// List acknowledged alerts
	result, err = manager.ListAlerts(ctx, AlertFilter{
		Status: []AlertState{AlertStateAcknowledged},
	})
	require.NoError(t, err)
	assert.Len(t, result.Alerts, 1)
}

// TestFindSimilarAlert tests deduplication logic
func TestFindSimilarAlert(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create base alert
	baseAlert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}

	err = manager.CreateAlert(ctx, baseAlert)
	require.NoError(t, err)

	// Test finding similar alert
	similarAlert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 5,
		AffectedNodes:       []string{"node-2"},
		BlastRadius:         "node",
	}

	foundAlert := manager.findSimilarAlert(similarAlert)
	assert.NotNil(t, foundAlert)
	assert.Equal(t, baseAlert.AlertID, foundAlert.AlertID)

	// Test with different target (should not match)
	differentAlert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "different-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
	}

	foundAlert = manager.findSimilarAlert(differentAlert)
	assert.Nil(t, foundAlert)
}

// TestFindSimilarAlert_MultipleMatches tests deduplication with multiple similar alerts
func TestFindSimilarAlert_MultipleMatches(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create first alert
	alert1 := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 1,
		AffectedNodes:       []string{"node-1"},
		BlastRadius:         "node",
	}
	err = manager.CreateAlert(ctx, alert1)
	require.NoError(t, err)

	// Try to create similar alert (should be deduplicated)
	alert2 := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 2,
		AffectedNodes:       []string{"node-2"},
		BlastRadius:         "node",
	}
	err = manager.CreateAlert(ctx, alert2)
	require.NoError(t, err)

	// Should have only 1 alert (deduplicated)
	manager.mu.RLock()
	assert.Len(t, manager.active, 1)
	manager.mu.RUnlock()

	// Verify the alert was updated with new failure count
	manager.mu.RLock()
	updatedAlert := manager.active[alert1.AlertID]
	manager.mu.RUnlock()
	assert.NotNil(t, updatedAlert)
	assert.Equal(t, int32(2), updatedAlert.ConsecutiveFailures)
}

// TestAlertHistory tests event history tracking
func TestAlertHistory(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Acknowledge alert (creates event)
	err = manager.AcknowledgeAlert(ctx, alert.AlertID, &AcknowledgeRequest{Comment: "ACK", User: "test"})
	require.NoError(t, err)

	// Verify history
	manager.mu.RLock()
	assert.Len(t, manager.history, 2) // fired + acknowledged
	manager.mu.RUnlock()
}

// TestCleanupExpired tests cleanup of old alerts
func TestCleanupExpired(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DefaultCooldown:    5 * time.Minute,
		AutoResolveTimeout: 24 * time.Hour,
		MaxActiveAlerts:    1000,
		HistoryRetention:   1 * time.Hour, // Short retention for testing
	}
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create and resolve alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	err = manager.ResolveAlert(ctx, alert.AlertID, &ResolveRequest{Comment: "Resolved", User: "test"})
	require.NoError(t, err)

	// Manually set resolved time to past
	manager.mu.Lock()
	oldTime := time.Now().Add(-2 * time.Hour)
	alert.ResolvedAt = &oldTime
	manager.mu.Unlock()

	// Run cleanup
	manager.cleanup(ctx)

	// Alert should be removed from active map
	manager.mu.RLock()
	_, exists := manager.active[alert.AlertID]
	manager.mu.RUnlock()
	assert.False(t, exists)
}

// TestCleanupLoop tests the background cleanup loop
func TestCleanupLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Cancel context to stop cleanup loop
	cancel()

	// Give goroutine time to exit
	time.Sleep(100 * time.Millisecond)

	// Stop should not error
	err = manager.Stop()
	assert.NoError(t, err)
}

// TestCallbackNotifications tests callback notifications
func TestCallbackNotifications(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Track callbacks
	var mu sync.Mutex
	callbackCalled := false
	var callbackAlert *Alert
	var callbackEvent *AlertEvent

	// Register callback
	manager.OnAlert(func(alert *Alert, event *AlertEvent) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		callbackAlert = alert
		callbackEvent = event
	})

	// Create alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Wait for callback
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.True(t, callbackCalled)
	assert.NotNil(t, callbackAlert)
	assert.NotNil(t, callbackEvent)
	mu.Unlock()
}

// TestConcurrentAlertOperations tests concurrent alert operations
func TestConcurrentAlertOperations(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alerts concurrently with unique names
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			alert := &Alert{
				Target: k8swatchv1.TargetRef{
					Name:      "test-target-" + string(rune('A'+idx%26)) + "-" + string(rune('0'+idx/26)),
					Namespace: "default",
					Type:      "http",
				},
				Severity:            "critical",
				FailureLayer:        "L2",
				FailureCode:         "tcp_refused",
				ConsecutiveFailures: 3,
			}
			err := manager.CreateAlert(ctx, alert)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Verify alerts were created
	manager.mu.RLock()
	activeCount := len(manager.active)
	manager.mu.RUnlock()

	assert.Greater(t, activeCount, 0)
	assert.Equal(t, activeCount, successCount, "Active count should match successful creations")
}

// TestManager_WithRedisBackend tests manager with Redis backend (mock)
func TestManager_WithRedisBackend(t *testing.T) {
	ctx := context.Background()
	config := DefaultManagerConfig()

	// Use memory store as Redis mock
	store := NewMemoryStore()
	routing := NewRouter(DefaultRouterConfig())
	manager := NewManager(config, store, routing)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = manager.Stop() }()

	// Create alert
	alert := &Alert{
		Target: k8swatchv1.TargetRef{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Severity:            "critical",
		FailureLayer:        "L2",
		FailureCode:         "tcp_refused",
		ConsecutiveFailures: 3,
	}

	err = manager.CreateAlert(ctx, alert)
	require.NoError(t, err)

	// Verify alert persists in store
	retrieved, err := store.Get(ctx, alert.AlertID)
	require.NoError(t, err)
	assert.Equal(t, alert.AlertID, retrieved.AlertID)

	// List events
	events, err := store.ListEvents(ctx, alert.AlertID)
	require.NoError(t, err)
	assert.NotEmpty(t, events)
}
