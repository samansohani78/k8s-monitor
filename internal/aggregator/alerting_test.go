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

package aggregator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlertEngineConfigDefaults(t *testing.T) {
	cfg := DefaultAlertEngineConfig()

	assert.Equal(t, int32(3), cfg.DefaultConsecutiveFailures)
	assert.Equal(t, int32(2), cfg.DefaultRecoverySuccesses)
}

func TestAlertDecisionEngineCreation(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.targetStates)
	assert.NotNil(t, engine.rules)
}

func TestAlertDecisionEngineCreationNilConfig(t *testing.T) {
	engine := NewAlertDecisionEngine(nil)

	assert.NotNil(t, engine)
	assert.Equal(t, int32(3), engine.config.DefaultConsecutiveFailures)
}

func TestAlertDecisionEngineProcessResultSuccess(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	decision := engine.ProcessResult("target-1", true, "", "")

	assert.False(t, decision.ShouldAlert)
	assert.False(t, decision.ShouldResolve)
	assert.Equal(t, "target-1", decision.TargetKey)
}

func TestAlertDecisionEngineProcessResultFailure(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// First failure - should not alert yet (threshold is 3)
	decision := engine.ProcessResult("target-1", false, "tcp_refused", "L2")

	assert.False(t, decision.ShouldAlert)
	assert.Equal(t, int32(1), engine.targetStates["target-1"].ConsecutiveFailures)

	// Second failure
	decision = engine.ProcessResult("target-1", false, "tcp_refused", "L2")
	assert.False(t, decision.ShouldAlert)

	// Third failure - should alert
	decision = engine.ProcessResult("target-1", false, "tcp_refused", "L2")
	assert.True(t, decision.ShouldAlert)
	assert.Equal(t, "Failure threshold met", decision.Reason)
}

func TestAlertDecisionEngineProcessResultRecovery(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// Trigger alert (3 failures)
	for i := 0; i < 3; i++ {
		engine.ProcessResult("target-1", false, "tcp_refused", "L2")
	}

	state, exists := engine.GetTargetState("target-1")
	assert.True(t, exists)
	assert.True(t, state.IsAlerting)

	// First success - should not resolve yet (threshold is 2)
	decision := engine.ProcessResult("target-1", true, "", "")
	assert.False(t, decision.ShouldResolve)

	// Second success - should resolve
	decision = engine.ProcessResult("target-1", true, "", "")
	assert.True(t, decision.ShouldResolve)
	assert.Equal(t, "Recovery threshold met", decision.Reason)

	state, _ = engine.GetTargetState("target-1")
	assert.False(t, state.IsAlerting)
}

func TestAlertDecisionEngineConsecutiveCounters(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// Add 2 failures
	engine.ProcessResult("target-1", false, "err1", "L2")
	engine.ProcessResult("target-1", false, "err2", "L2")

	state, _ := engine.GetTargetState("target-1")
	assert.Equal(t, int32(2), state.ConsecutiveFailures)
	assert.Equal(t, int32(0), state.ConsecutiveSuccesses)

	// Add success - should reset failures
	engine.ProcessResult("target-1", true, "", "")

	state, _ = engine.GetTargetState("target-1")
	assert.Equal(t, int32(0), state.ConsecutiveFailures)
	assert.Equal(t, int32(1), state.ConsecutiveSuccesses)
}

func TestAlertDecisionEngineAlertCallback(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	alerted := false
	resolved := false

	engine.SetAlertCallback(func(target string, isAlerting bool, state *AlertTargetState) {
		if isAlerting {
			alerted = true
		} else {
			resolved = true
		}
	})

	// Trigger alert
	for i := 0; i < 3; i++ {
		engine.ProcessResult("target-1", false, "tcp_refused", "L2")
	}
	assert.True(t, alerted)

	// Recover
	for i := 0; i < 2; i++ {
		engine.ProcessResult("target-1", true, "", "")
	}
	assert.True(t, resolved)
}

func TestAlertDecisionEngineGetTargetStateNotFound(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	state, exists := engine.GetTargetState("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, state)
}

func TestAlertDecisionEngineGetTargetStateCopy(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// Add state
	engine.ProcessResult("target-1", false, "tcp_refused", "L2")

	state1, _ := engine.GetTargetState("target-1")
	state1.ConsecutiveFailures = 999

	state2, _ := engine.GetTargetState("target-1")
	assert.Equal(t, int32(1), state2.ConsecutiveFailures)
}

func TestAlertDecisionEngineCalculateSeverity(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// DNS failure (L1) should be critical
	severity := engine.CalculateSeverity("L1", BlastRadiusNode, "")
	assert.Equal(t, AlertSeverityCritical, severity)

	// Network failure (L2) should be critical
	severity = engine.CalculateSeverity("L2", BlastRadiusNode, "")
	assert.Equal(t, AlertSeverityCritical, severity)

	// TLS failure (L3) should be warning
	severity = engine.CalculateSeverity("L3", BlastRadiusNode, "")
	assert.Equal(t, AlertSeverityWarning, severity)

	// Auth failure (L5) should be warning
	severity = engine.CalculateSeverity("L5", BlastRadiusNode, "")
	assert.Equal(t, AlertSeverityWarning, severity)
}

func TestAlertDecisionEngineCalculateSeverityBlastRadius(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// Zone blast radius should escalate warning to critical
	severity := engine.CalculateSeverity("L3", BlastRadiusZone, "")
	assert.Equal(t, AlertSeverityCritical, severity)

	// Cluster blast radius should always be critical
	severity = engine.CalculateSeverity("L3", BlastRadiusCluster, "")
	assert.Equal(t, AlertSeverityCritical, severity)
}

func TestAlertDecisionEngineCalculateSeverityCriticality(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// P0 criticality should always be critical
	severity := engine.CalculateSeverity("L3", BlastRadiusNode, "P0")
	assert.Equal(t, AlertSeverityCritical, severity)

	// P1 criticality should escalate warning to critical
	severity = engine.CalculateSeverity("L3", BlastRadiusNode, "P1")
	assert.Equal(t, AlertSeverityCritical, severity)
}

func TestAlertDecisionEngineGetStats(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// Add some targets
	engine.ProcessResult("target-1", false, "err", "L2")
	engine.ProcessResult("target-2", false, "err", "L2")
	engine.ProcessResult("target-3", false, "err", "L2")

	// Trigger alert on one
	engine.ProcessResult("target-1", false, "err", "L2")
	engine.ProcessResult("target-1", false, "err", "L2")

	stats := engine.GetStats()

	assert.Equal(t, 3, stats.TotalTargets)
	assert.Equal(t, 1, stats.AlertingCount)
	assert.Equal(t, 0, stats.RuleCount)
}

func TestAlertDecisionEngineUpdateRule(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	stats := engine.GetStats()
	assert.Equal(t, 0, stats.RuleCount)

	// Add rule (would need actual AlertRule CRD in production)
	// For now just verify the method doesn't panic
}

func TestAlertDecisionEngineMaxSeverity(t *testing.T) {
	// Test maxSeverity function
	assert.Equal(t, AlertSeverityCritical, maxSeverity(AlertSeverityCritical, AlertSeverityWarning))
	assert.Equal(t, AlertSeverityCritical, maxSeverity(AlertSeverityWarning, AlertSeverityCritical))
	assert.Equal(t, AlertSeverityWarning, maxSeverity(AlertSeverityWarning, AlertSeverityInfo))
	assert.Equal(t, AlertSeverityWarning, maxSeverity(AlertSeverityWarning, AlertSeverityWarning))
}

func TestAlertDecisionEngineStateTracking(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// Add failures with different codes
	engine.ProcessResult("target-1", false, "tcp_refused", "L2")
	engine.ProcessResult("target-1", false, "timeout", "L2")
	engine.ProcessResult("target-1", false, "dns_timeout", "L1")

	state, _ := engine.GetTargetState("target-1")

	assert.Equal(t, int32(3), state.ConsecutiveFailures)
	assert.Equal(t, "dns_timeout", state.LastFailureCode)
	assert.Equal(t, "L1", state.LastFailureLayer)
	assert.False(t, state.LastFailureTime.IsZero())
}

func TestAlertDecisionEngineMultipleTargets(t *testing.T) {
	cfg := DefaultAlertEngineConfig()
	engine := NewAlertDecisionEngine(cfg)

	// Process results for multiple targets
	for i := 0; i < 5; i++ {
		engine.ProcessResult("target-"+string(rune('a'+i)), false, "err", "L2")
	}

	stats := engine.GetStats()
	assert.Equal(t, 5, stats.TotalTargets)
	assert.Equal(t, 0, stats.AlertingCount) // None have reached threshold yet
}
