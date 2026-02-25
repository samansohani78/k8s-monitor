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

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEscalatorCreation tests escalator creation
func TestEscalatorCreation(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	require.NotNil(t, escalator)
	assert.NotNil(t, escalator.config)
	assert.NotNil(t, escalator.policies)
	assert.Equal(t, 5*time.Minute, config.DefaultEscalationDelay)
	assert.Equal(t, 3, config.MaxEscalationLevel)
}

// TestDefaultEscalatorConfig tests default configuration
func TestDefaultEscalatorConfig(t *testing.T) {
	config := DefaultEscalatorConfig()

	assert.Equal(t, 5*time.Minute, config.DefaultEscalationDelay)
	assert.Equal(t, 3, config.MaxEscalationLevel)
}

// TestAddPolicy tests adding escalation policies
func TestAddPolicy(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	policy := &EscalationPolicy{
		Name: "test-policy",
		Levels: []EscalationLevel{
			{Level: 0, Delay: 5 * time.Minute, Channels: []string{"slack"}},
		},
	}

	escalator.AddPolicy(policy)

	// Verify policy was added
	retrieved, exists := escalator.GetPolicy("test-policy")
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-policy", retrieved.Name)
}

// TestGetPolicy tests retrieving policies
func TestGetPolicy(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Add policy
	policy := &EscalationPolicy{
		Name: "critical-policy",
		Levels: []EscalationLevel{
			{Level: 0, Delay: 5 * time.Minute, Channels: []string{"pagerduty"}},
		},
	}
	escalator.AddPolicy(policy)

	// Get policy
	retrieved, exists := escalator.GetPolicy("critical-policy")
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "critical-policy", retrieved.Name)

	// Get non-existent policy
	_, exists = escalator.GetPolicy("non-existent")
	assert.False(t, exists)
}

// TestEscalate tests alert escalation
func TestEscalate(t *testing.T) {
	ctx := context.Background()
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Add policy
	policy := &EscalationPolicy{
		Name: "default-critical",
		Levels: []EscalationLevel{
			{Level: 0, Delay: 0, Channels: []string{"slack"}, NotifyOnCall: false},
			{Level: 1, Delay: 5 * time.Minute, Channels: []string{"pagerduty"}, NotifyOnCall: true},
		},
	}
	escalator.AddPolicy(policy)

	// Create alert
	alert := &Alert{
		AlertID:      "test-alert",
		Severity:     AlertSeverityCritical,
		FiredAt:      time.Now().Add(-10 * time.Minute),
		Status:       AlertStateFiring,
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}

	// Escalate alert - level 0
	err := escalator.Escalate(ctx, alert, 0)
	assert.NoError(t, err)

	// Escalate alert - level 1
	err = escalator.Escalate(ctx, alert, 1)
	assert.NoError(t, err)
}

// TestEscalate_MaxLevelReached tests escalation with max level reached
func TestEscalate_MaxLevelReached(t *testing.T) {
	ctx := context.Background()
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Add policy with single level
	policy := &EscalationPolicy{
		Name: "single-level",
		Levels: []EscalationLevel{
			{Level: 0, Delay: 0, Channels: []string{"slack"}},
		},
	}
	escalator.AddPolicy(policy)

	alert := &Alert{
		AlertID:      "test-alert",
		Severity:     AlertSeverityWarning,
		FiredAt:      time.Now(),
		Status:       AlertStateFiring,
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}

	// Try to escalate beyond max configured level (3)
	err := escalator.Escalate(ctx, alert, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum escalation level")
}

// TestGetNextEscalationTime tests escalation time calculation
func TestGetNextEscalationTime(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Add policy
	policy := &EscalationPolicy{
		Name: "default-critical",
		Levels: []EscalationLevel{
			{Level: 0, Delay: 0, Channels: []string{"slack"}},
			{Level: 1, Delay: 5 * time.Minute, Channels: []string{"pagerduty"}},
			{Level: 2, Delay: 15 * time.Minute, Channels: []string{"pagerduty"}},
		},
	}
	escalator.AddPolicy(policy)

	now := time.Now()
	alert := &Alert{
		AlertID:      "test-alert",
		Severity:     AlertSeverityCritical,
		FiredAt:      now,
		Status:       AlertStateFiring,
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}

	// Get next escalation time from level 0
	nextTime := escalator.GetNextEscalationTime(alert, 0)
	assert.NotZero(t, nextTime)
	assert.Equal(t, now.Add(5*time.Minute).Unix(), nextTime.Unix())

	// Get next escalation time from level 1
	nextTime = escalator.GetNextEscalationTime(alert, 1)
	assert.NotZero(t, nextTime)
	assert.Equal(t, now.Add(15*time.Minute).Unix(), nextTime.Unix())

	// Get next escalation time from level 2 (no more levels)
	nextTime = escalator.GetNextEscalationTime(alert, 2)
	assert.Zero(t, nextTime)
}

// TestFindPolicy tests policy lookup
func TestFindPolicy(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Add policies
	escalator.AddPolicy(&EscalationPolicy{
		Name:   "default-critical",
		Levels: []EscalationLevel{{Level: 0, Delay: 0, Channels: []string{"slack"}}},
	})
	escalator.AddPolicy(&EscalationPolicy{
		Name:   "default-warning",
		Levels: []EscalationLevel{{Level: 0, Delay: 0, Channels: []string{"email"}}},
	})
	escalator.AddPolicy(&EscalationPolicy{
		Name:   "default",
		Levels: []EscalationLevel{{Level: 0, Delay: 0, Channels: []string{"slack"}}},
	})

	// Find policy for critical alert
	criticalAlert := &Alert{
		Severity:     AlertSeverityCritical,
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	policy := escalator.findPolicy(criticalAlert)
	assert.NotNil(t, policy)
	assert.Equal(t, "default-critical", policy.Name)

	// Find policy for warning alert
	warningAlert := &Alert{
		Severity:     AlertSeverityWarning,
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	policy = escalator.findPolicy(warningAlert)
	assert.NotNil(t, policy)
	assert.Equal(t, "default-warning", policy.Name)

	// Find policy for info alert (should use default)
	infoAlert := &Alert{
		Severity:     AlertSeverityInfo,
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}
	policy = escalator.findPolicy(infoAlert)
	assert.NotNil(t, policy)
	assert.Equal(t, "default", policy.Name)
}

// TestCreateDefaultPolicies tests default policy creation
func TestCreateDefaultPolicies(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Create default policies
	escalator.CreateDefaultPolicies()

	// Verify policies exist
	policy, exists := escalator.GetPolicy("default-critical")
	assert.True(t, exists)
	assert.NotNil(t, policy)
	assert.Len(t, policy.Levels, 3)

	policy, exists = escalator.GetPolicy("default-warning")
	assert.True(t, exists)
	assert.NotNil(t, policy)
	assert.Len(t, policy.Levels, 2)

	policy, exists = escalator.GetPolicy("default")
	assert.True(t, exists)
	assert.NotNil(t, policy)
	assert.Len(t, policy.Levels, 1)
}

// TestEscalationTimeCalculation tests time-based escalation
func TestEscalationTimeCalculation(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Add policy with correct name matching
	policy := &EscalationPolicy{
		Name: "default-critical",
		Levels: []EscalationLevel{
			{Level: 0, Delay: 0, Channels: []string{"slack"}},
			{Level: 1, Delay: 5 * time.Minute, Channels: []string{"email"}},
			{Level: 2, Delay: 15 * time.Minute, Channels: []string{"pagerduty"}},
		},
	}
	escalator.AddPolicy(policy)

	now := time.Now()

	// Test alert fired at time zero
	alert1 := &Alert{
		AlertID:      "alert-1",
		Severity:     AlertSeverityCritical,
		FiredAt:      now,
		Status:       AlertStateFiring,
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}

	// Next escalation from level 0 should be at firedAt + level[1].Delay
	nextTime1 := escalator.GetNextEscalationTime(alert1, 0)
	assert.NotZero(t, nextTime1)
	expectedTime1 := now.Add(5 * time.Minute)
	assert.InDelta(t, expectedTime1.Unix(), nextTime1.Unix(), 1)

	// Next escalation from level 1 should be at firedAt + level[2].Delay
	nextTime2 := escalator.GetNextEscalationTime(alert1, 1)
	assert.NotZero(t, nextTime2)
	expectedTime2 := now.Add(15 * time.Minute)
	assert.InDelta(t, expectedTime2.Unix(), nextTime2.Unix(), 1)
}

// TestFindPolicy_ByBlastRadius tests policy lookup by blast radius
func TestFindPolicy_ByBlastRadius(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Add blast radius policy
	escalator.AddPolicy(&EscalationPolicy{
		Name:   "blast-cluster",
		Levels: []EscalationLevel{{Level: 0, Delay: 0, Channels: []string{"pagerduty"}}},
	})

	alert := &Alert{
		Severity:     AlertSeverityCritical,
		BlastRadius:  "cluster",
		Target:       k8swatchv1.TargetRef{Name: "test", Namespace: "default", Type: "http"},
		FailureLayer: "L2",
		FailureCode:  "tcp_refused",
	}

	policy := escalator.findPolicy(alert)
	assert.NotNil(t, policy)
	assert.Equal(t, "blast-cluster", policy.Name)
}

// TestFindPolicy_ByTargetType tests policy lookup by target type
func TestFindPolicy_ByTargetType(t *testing.T) {
	config := DefaultEscalatorConfig()
	escalator := NewEscalator(config)

	// Add target type policy
	escalator.AddPolicy(&EscalationPolicy{
		Name:   "target-postgresql",
		Levels: []EscalationLevel{{Level: 0, Delay: 0, Channels: []string{"email"}}},
	})

	alert := &Alert{
		Severity:     AlertSeverityWarning,
		Target:       k8swatchv1.TargetRef{Name: "db", Namespace: "default", Type: "postgresql"},
		FailureLayer: "L4",
		FailureCode:  "protocol_error",
	}

	policy := escalator.findPolicy(alert)
	assert.NotNil(t, policy)
	assert.Equal(t, "target-postgresql", policy.Name)
}
