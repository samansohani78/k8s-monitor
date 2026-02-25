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
	"sync"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// AlertEngineConfig holds alert engine configuration
type AlertEngineConfig struct {
	// DefaultConsecutiveFailures is default failures before alerting
	DefaultConsecutiveFailures int32
	// DefaultRecoverySuccesses is default successes before resolving
	DefaultRecoverySuccesses int32
}

// DefaultAlertEngineConfig returns default alert engine configuration
func DefaultAlertEngineConfig() *AlertEngineConfig {
	return &AlertEngineConfig{
		DefaultConsecutiveFailures: 3,
		DefaultRecoverySuccesses:   2,
	}
}

// AlertDecisionEngine makes alerting decisions based on results and rules
type AlertDecisionEngine struct {
	config        *AlertEngineConfig
	targetStates  map[string]*AlertTargetState
	rules         map[string]*k8swatchv1.AlertRule
	mu            sync.RWMutex
	alertCallback AlertCallback
}

// AlertTargetState tracks state for alerting decisions
type AlertTargetState struct {
	TargetKey            string
	ConsecutiveFailures  int32
	ConsecutiveSuccesses int32
	LastFailureTime      time.Time
	LastSuccessTime      time.Time
	LastFailureCode      string
	LastFailureLayer     string
	IsAlerting           bool
	AlertStartTime       time.Time
}

// AlertCallback is called when alert state changes
type AlertCallback func(target string, isAlerting bool, state *AlertTargetState)

// NewAlertDecisionEngine creates new alert decision engine
func NewAlertDecisionEngine(config *AlertEngineConfig) *AlertDecisionEngine {
	if config == nil {
		config = DefaultAlertEngineConfig()
	}

	return &AlertDecisionEngine{
		config:       config,
		targetStates: make(map[string]*AlertTargetState),
		rules:        make(map[string]*k8swatchv1.AlertRule),
	}
}

// SetAlertCallback sets callback for alert state changes
func (a *AlertDecisionEngine) SetAlertCallback(callback AlertCallback) {
	a.alertCallback = callback
}

// ProcessResult processes a check result and makes alerting decision
func (a *AlertDecisionEngine) ProcessResult(targetKey string, success bool, failureCode, failureLayer string) AlertDecision {
	a.mu.Lock()
	defer a.mu.Unlock()

	state, exists := a.targetStates[targetKey]
	if !exists {
		state = &AlertTargetState{
			TargetKey: targetKey,
		}
		a.targetStates[targetKey] = state
	}

	now := time.Now()

	if success {
		state.ConsecutiveSuccesses++
		state.ConsecutiveFailures = 0
		state.LastSuccessTime = now

		// Check if we should resolve alert
		if state.IsAlerting {
			threshold := a.getRecoveryThreshold(targetKey)
			if state.ConsecutiveSuccesses >= threshold {
				state.IsAlerting = false
				if a.alertCallback != nil {
					a.alertCallback(targetKey, false, state)
				}
				return AlertDecision{
					TargetKey:     targetKey,
					ShouldAlert:   false,
					ShouldResolve: true,
					Reason:        "Recovery threshold met",
				}
			}
		}

		return AlertDecision{
			TargetKey:     targetKey,
			ShouldAlert:   false,
			ShouldResolve: false,
		}
	}

	// Failure
	state.ConsecutiveFailures++
	state.ConsecutiveSuccesses = 0
	state.LastFailureTime = now
	state.LastFailureCode = failureCode
	state.LastFailureLayer = failureLayer

	// Check if we should alert
	threshold := a.getFailureThreshold(targetKey)
	if state.ConsecutiveFailures >= threshold && !state.IsAlerting {
		state.IsAlerting = true
		state.AlertStartTime = now
		if a.alertCallback != nil {
			a.alertCallback(targetKey, true, state)
		}
		return AlertDecision{
			TargetKey:     targetKey,
			ShouldAlert:   true,
			ShouldResolve: false,
			Reason:        "Failure threshold met",
		}
	}

	return AlertDecision{
		TargetKey:     targetKey,
		ShouldAlert:   false,
		ShouldResolve: false,
	}
}

// getFailureThreshold gets failure threshold from rules or default
func (a *AlertDecisionEngine) getFailureThreshold(targetKey string) int32 {
	// Check matching rules
	for _, rule := range a.rules {
		if a.matchesTarget(rule, targetKey) {
			if rule.Spec.Trigger.ConsecutiveFailures > 0 {
				return rule.Spec.Trigger.ConsecutiveFailures
			}
		}
	}
	return a.config.DefaultConsecutiveFailures
}

// getRecoveryThreshold gets recovery threshold from rules or default
func (a *AlertDecisionEngine) getRecoveryThreshold(targetKey string) int32 {
	// Check matching rules
	for _, rule := range a.rules {
		if a.matchesTarget(rule, targetKey) {
			if rule.Spec.Recovery.ConsecutiveSuccesses > 0 {
				return rule.Spec.Recovery.ConsecutiveSuccesses
			}
		}
	}
	return a.config.DefaultRecoverySuccesses
}

// matchesTarget checks if a rule matches a target
func (a *AlertDecisionEngine) matchesTarget(rule *k8swatchv1.AlertRule, targetKey string) bool {
	// Simple matching - in production would check selectors
	return true
}

// UpdateRule updates an alert rule
func (a *AlertDecisionEngine) UpdateRule(rule *k8swatchv1.AlertRule) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.rules[rule.Name] = rule
}

// RemoveRule removes an alert rule
func (a *AlertDecisionEngine) RemoveRule(ruleName string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.rules, ruleName)
}

// GetTargetState returns state for a target
func (a *AlertDecisionEngine) GetTargetState(targetKey string) (*AlertTargetState, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	state, exists := a.targetStates[targetKey]
	if !exists {
		return nil, false
	}

	// Return copy
	copy := &AlertTargetState{
		TargetKey:            state.TargetKey,
		ConsecutiveFailures:  state.ConsecutiveFailures,
		ConsecutiveSuccesses: state.ConsecutiveSuccesses,
		LastFailureTime:      state.LastFailureTime,
		LastSuccessTime:      state.LastSuccessTime,
		LastFailureCode:      state.LastFailureCode,
		LastFailureLayer:     state.LastFailureLayer,
		IsAlerting:           state.IsAlerting,
		AlertStartTime:       state.AlertStartTime,
	}

	return copy, true
}

// CalculateSeverity calculates alert severity based on layer, blast radius, and criticality
func (a *AlertDecisionEngine) CalculateSeverity(failureLayer string, blastRadius BlastRadiusType, criticality string) AlertSeverity {
	// Base severity from failure layer
	baseSeverity := a.getSeverityFromLayer(failureLayer)

	// Adjust based on blast radius
	switch blastRadius {
	case BlastRadiusCluster:
		baseSeverity = maxSeverity(baseSeverity, AlertSeverityCritical)
	case BlastRadiusZone:
		if baseSeverity == AlertSeverityWarning {
			baseSeverity = AlertSeverityCritical
		}
	}

	// Adjust based on criticality
	switch criticality {
	case "P0":
		baseSeverity = AlertSeverityCritical
	case "P1":
		if baseSeverity == AlertSeverityWarning {
			baseSeverity = AlertSeverityCritical
		}
	}

	return baseSeverity
}

// getSeverityFromLayer gets base severity from failure layer
func (a *AlertDecisionEngine) getSeverityFromLayer(layer string) AlertSeverity {
	switch layer {
	case "L1": // DNS
		return AlertSeverityCritical
	case "L2": // Network
		return AlertSeverityCritical
	case "L3": // TLS
		return AlertSeverityWarning
	case "L5": // Auth
		return AlertSeverityWarning
	default:
		return AlertSeverityWarning
	}
}

// GetStats returns alert engine statistics
func (a *AlertDecisionEngine) GetStats() AlertEngineStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	alertingCount := 0
	for _, state := range a.targetStates {
		if state.IsAlerting {
			alertingCount++
		}
	}

	return AlertEngineStats{
		TotalTargets:  len(a.targetStates),
		AlertingCount: alertingCount,
		RuleCount:     len(a.rules),
	}
}

// AlertDecision represents an alerting decision
type AlertDecision struct {
	TargetKey     string
	ShouldAlert   bool
	ShouldResolve bool
	Reason        string
}

// AlertSeverity represents alert severity level
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertEngineStats contains alert engine statistics
type AlertEngineStats struct {
	TotalTargets  int
	AlertingCount int
	RuleCount     int
}

// maxSeverity returns the more severe of two severities
func maxSeverity(a, b AlertSeverity) AlertSeverity {
	order := map[AlertSeverity]int{
		AlertSeverityInfo:     0,
		AlertSeverityWarning:  1,
		AlertSeverityCritical: 2,
	}

	if order[a] > order[b] {
		return a
	}
	return b
}
