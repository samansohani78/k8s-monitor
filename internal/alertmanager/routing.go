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

// Router routes alerts to notification channels
type Router struct {
	config     *RouterConfig
	channels   map[string]NotificationChannel
	rules      []RoutingRule
	mu         sync.RWMutex
	escalator  *Escalator
	lastAlerts map[string]time.Time // For cooldown
}

// RouterConfig holds router configuration
type RouterConfig struct {
	// DefaultCooldown is the minimum time between notifications for same alert
	DefaultCooldown time.Duration
	// MaxRetries is the maximum retries for failed notifications
	MaxRetries int
	// RetryBackoff is the backoff between retries
	RetryBackoff time.Duration
}

// DefaultRouterConfig returns the default router configuration
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		DefaultCooldown: 5 * time.Minute,
		MaxRetries:      3,
		RetryBackoff:    10 * time.Second,
	}
}

// NotificationChannel is the interface for notification channels
type NotificationChannel interface {
	// Name returns the channel name
	Name() string
	// Send sends a notification
	Send(ctx context.Context, alert *Alert) error
	// Close closes the channel
	Close() error
}

// RoutingRule defines a routing rule
type RoutingRule struct {
	// Name is the rule name
	Name string
	// Match defines matching criteria
	Match RouteMatch
	// Channels is the list of channel names to notify
	Channels []string
	// SeverityOverrides allows overriding severity per channel
	SeverityOverrides map[string]AlertSeverity
	// Continue matching after this rule
	Continue bool
}

// RouteMatch defines matching criteria for routing
type RouteMatch struct {
	// Severities matches alert severities
	Severities []AlertSeverity
	// Namespaces matches target namespaces
	Namespaces []string
	// Teams matches team labels
	Teams []string
	// BlastRadius matches blast radius
	BlastRadius []string
	// TargetTypes matches target types
	TargetTypes []string
}

// NewRouter creates a new router
func NewRouter(config *RouterConfig) *Router {
	if config == nil {
		config = DefaultRouterConfig()
	}

	r := &Router{
		config:     config,
		channels:   make(map[string]NotificationChannel),
		rules:      make([]RoutingRule, 0),
		escalator:  NewEscalator(DefaultEscalatorConfig()),
		lastAlerts: make(map[string]time.Time),
	}

	return r
}

// RegisterChannel registers a notification channel
func (r *Router) RegisterChannel(channel NotificationChannel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[channel.Name()] = channel
}

// AddRule adds a routing rule
func (r *Router) AddRule(rule RoutingRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = append(r.rules, rule)
}

// Route routes an alert to appropriate channels
func (r *Router) Route(ctx context.Context, alert *Alert) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check cooldown
	cooldownKey := fmt.Sprintf("%s:%s:%s", alert.Target.Namespace, alert.Target.Name, alert.FailureCode)
	if r.isInCooldown(cooldownKey) {
		return nil // Skip due to cooldown
	}

	// Find matching rules
	matchedRules := r.findMatchingRules(alert)
	if len(matchedRules) == 0 {
		// Use default routing
		return r.routeDefault(ctx, alert)
	}

	// Route to channels from matching rules
	for _, rule := range matchedRules {
		for _, channelName := range rule.Channels {
			channel, exists := r.channels[channelName]
			if !exists {
				continue
			}

			if err := r.sendWithRetry(ctx, channel, alert); err != nil {
				fmt.Printf("Failed to send notification via %s: %v\n", channelName, err)
			}
		}

		if !rule.Continue {
			break
		}
	}

	// Update cooldown
	r.lastAlerts[cooldownKey] = time.Now()

	return nil
}

// Escalate escalates an alert
func (r *Router) Escalate(ctx context.Context, alert *Alert, level int) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.escalator == nil {
		return nil
	}

	return r.escalator.Escalate(ctx, alert, level)
}

// findMatchingRules finds rules that match the alert
func (r *Router) findMatchingRules(alert *Alert) []RoutingRule {
	var matched []RoutingRule

	for _, rule := range r.rules {
		if r.matchesRule(alert, rule) {
			matched = append(matched, rule)
		}
	}

	return matched
}

// matchesRule checks if an alert matches a rule
func (r *Router) matchesRule(alert *Alert, rule RoutingRule) bool {
	match := rule.Match

	// Check severities
	if len(match.Severities) > 0 {
		found := false
		for _, s := range match.Severities {
			if alert.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check namespaces
	if len(match.Namespaces) > 0 {
		found := false
		for _, ns := range match.Namespaces {
			if alert.Target.Namespace == ns {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check blast radius
	if len(match.BlastRadius) > 0 {
		found := false
		for _, br := range match.BlastRadius {
			if alert.BlastRadius == br {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check team labels
	if len(match.Teams) > 0 && alert.Labels != nil {
		team, exists := alert.Labels["team"]
		if !exists {
			return false
		}
		found := false
		for _, t := range match.Teams {
			if team == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// routeDefault routes an alert using default routing
func (r *Router) routeDefault(ctx context.Context, alert *Alert) error {
	// Default routing based on severity
	var channelNames []string

	switch alert.Severity {
	case AlertSeverityCritical:
		channelNames = []string{"pagerduty", "slack"}
	case AlertSeverityWarning:
		channelNames = []string{"slack"}
	case AlertSeverityInfo:
		channelNames = []string{"slack"}
	default:
		channelNames = []string{"slack"}
	}

	for _, channelName := range channelNames {
		channel, exists := r.channels[channelName]
		if !exists {
			continue
		}

		if err := r.sendWithRetry(ctx, channel, alert); err != nil {
			fmt.Printf("Failed to send notification via %s: %v\n", channelName, err)
		}
	}

	return nil
}

// sendWithRetry sends a notification with retry
func (r *Router) sendWithRetry(ctx context.Context, channel NotificationChannel, alert *Alert) error {
	var lastErr error

	for attempt := 0; attempt < r.config.MaxRetries; attempt++ {
		if err := channel.Send(ctx, alert); err != nil {
			lastErr = err
			time.Sleep(r.config.RetryBackoff)
			continue
		}
		return nil
	}

	return fmt.Errorf("failed after %d attempts: %w", r.config.MaxRetries, lastErr)
}

// isInCooldown checks if an alert is in cooldown period
func (r *Router) isInCooldown(key string) bool {
	lastSent, exists := r.lastAlerts[key]
	if !exists {
		return false
	}

	return time.Since(lastSent) < r.config.DefaultCooldown
}

// Close closes the router and all channels
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, channel := range r.channels {
		if err := channel.Close(); err != nil {
			fmt.Printf("Failed to close channel %s: %v\n", channel.Name(), err)
		}
	}

	return nil
}

// GetEscalator returns the escalator
func (r *Router) GetEscalator() *Escalator {
	return r.escalator
}
