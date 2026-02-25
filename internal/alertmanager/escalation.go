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
)

// Escalator handles alert escalation
type Escalator struct {
	config   *EscalatorConfig
	policies map[string]*EscalationPolicy
	mu       sync.RWMutex
}

// EscalatorConfig holds escalator configuration
type EscalatorConfig struct {
	// DefaultEscalationDelay is the default delay before escalation
	DefaultEscalationDelay time.Duration
	// MaxEscalationLevel is the maximum escalation level
	MaxEscalationLevel int
}

// DefaultEscalatorConfig returns the default escalator configuration
func DefaultEscalatorConfig() *EscalatorConfig {
	return &EscalatorConfig{
		DefaultEscalationDelay: 5 * time.Minute,
		MaxEscalationLevel:     3,
	}
}

// EscalationPolicy defines an escalation policy
type EscalationPolicy struct {
	// Name is the policy name
	Name string
	// Levels defines escalation levels
	Levels []EscalationLevel
}

// EscalationLevel defines a single escalation level
type EscalationLevel struct {
	// Level is the escalation level (0-based)
	Level int
	// Delay is the delay before escalating to this level
	Delay time.Duration
	// Channels is the list of channels to notify at this level
	Channels []string
	// NotifyOnCall indicates if on-call should be notified
	NotifyOnCall bool
}

// NewEscalator creates a new escalator
func NewEscalator(config *EscalatorConfig) *Escalator {
	if config == nil {
		config = DefaultEscalatorConfig()
	}

	return &Escalator{
		config:   config,
		policies: make(map[string]*EscalationPolicy),
	}
}

// AddPolicy adds an escalation policy
func (e *Escalator) AddPolicy(policy *EscalationPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policies[policy.Name] = policy
}

// GetPolicy retrieves an escalation policy by name
func (e *Escalator) GetPolicy(name string) (*EscalationPolicy, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	policy, exists := e.policies[name]
	return policy, exists
}

// Escalate escalates an alert to the next level
func (e *Escalator) Escalate(ctx context.Context, alert *Alert, level int) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if level > e.config.MaxEscalationLevel {
		return fmt.Errorf("maximum escalation level reached")
	}

	// Find matching policy
	policy := e.findPolicy(alert)
	if policy == nil {
		return nil // No policy, use default behavior
	}

	if level >= len(policy.Levels) {
		return fmt.Errorf("no escalation level %d defined", level)
	}

	escalationLevel := policy.Levels[level]

	// Notify channels for this level
	for _, channelName := range escalationLevel.Channels {
		// Channel notification would happen through the router
		fmt.Printf("Escalation level %d: Would notify channel %s for alert %s\n",
			level, channelName, alert.AlertID)
	}

	return nil
}

// GetNextEscalationTime returns when the next escalation should happen
func (e *Escalator) GetNextEscalationTime(alert *Alert, currentLevel int) time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policy := e.findPolicy(alert)
	if policy == nil || currentLevel+1 >= len(policy.Levels) {
		return time.Time{}
	}

	return alert.FiredAt.Add(policy.Levels[currentLevel+1].Delay)
}

// findPolicy finds the matching policy for an alert
func (e *Escalator) findPolicy(alert *Alert) *EscalationPolicy {
	// Try to find policy by severity
	policyName := fmt.Sprintf("default-%s", alert.Severity)
	if policy, exists := e.policies[policyName]; exists {
		return policy
	}

	// Try to find policy by blast radius
	policyName = fmt.Sprintf("blast-%s", alert.BlastRadius)
	if policy, exists := e.policies[policyName]; exists {
		return policy
	}

	// Try to find policy by target type
	policyName = fmt.Sprintf("target-%s", alert.Target.Type)
	if policy, exists := e.policies[policyName]; exists {
		return policy
	}

	// Return default policy if exists
	if policy, exists := e.policies["default"]; exists {
		return policy
	}

	return nil
}

// CreateDefaultPolicies creates default escalation policies
func (e *Escalator) CreateDefaultPolicies() {
	// Critical alerts escalate quickly
	e.AddPolicy(&EscalationPolicy{
		Name: "default-critical",
		Levels: []EscalationLevel{
			{
				Level:        0,
				Delay:        0,
				Channels:     []string{"slack"},
				NotifyOnCall: false,
			},
			{
				Level:        1,
				Delay:        5 * time.Minute,
				Channels:     []string{"pagerduty"},
				NotifyOnCall: true,
			},
			{
				Level:        2,
				Delay:        15 * time.Minute,
				Channels:     []string{"pagerduty"},
				NotifyOnCall: true,
			},
		},
	})

	// Warning alerts escalate slowly
	e.AddPolicy(&EscalationPolicy{
		Name: "default-warning",
		Levels: []EscalationLevel{
			{
				Level:        0,
				Delay:        0,
				Channels:     []string{"slack"},
				NotifyOnCall: false,
			},
			{
				Level:        1,
				Delay:        30 * time.Minute,
				Channels:     []string{"slack"},
				NotifyOnCall: false,
			},
		},
	})

	// Default policy
	e.AddPolicy(&EscalationPolicy{
		Name: "default",
		Levels: []EscalationLevel{
			{
				Level:        0,
				Delay:        0,
				Channels:     []string{"slack"},
				NotifyOnCall: false,
			},
		},
	})
}
