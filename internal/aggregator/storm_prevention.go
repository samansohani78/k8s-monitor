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
)

// StormPreventionConfig holds alert storm prevention configuration
type StormPreventionConfig struct {
	// GroupBy fields for grouping alerts
	GroupBy []string
	// MaxAlertsPerGroup is maximum alerts per group before suppression
	MaxAlertsPerGroup int32
	// CooldownPeriod is minimum time between same alerts
	CooldownPeriod time.Duration
	// SuppressionWindow is total suppression window
	SuppressionWindow time.Duration
}

// DefaultStormPreventionConfig returns default storm prevention configuration
func DefaultStormPreventionConfig() *StormPreventionConfig {
	return &StormPreventionConfig{
		GroupBy:           []string{"namespace", "failureLayer"},
		MaxAlertsPerGroup: 3,
		CooldownPeriod:    5 * time.Minute,
		SuppressionWindow: 15 * time.Minute,
	}
}

// AlertStormPreventer prevents alert storms through deduplication and suppression
type AlertStormPreventer struct {
	config      *StormPreventionConfig
	alertGroups map[string]*AlertGroup
	parentChild []ParentChildRule
	mu          sync.RWMutex
}

// AlertGroup represents a group of related alerts
type AlertGroup struct {
	Key            string
	AlertCount     int32
	FirstAlertTime time.Time
	LastAlertTime  time.Time
	LastSentTime   time.Time
	Suppressed     bool
	AlertTargets   []string
}

// ParentChildRule defines parent-child alert suppression relationship
type ParentChildRule struct {
	Parent string
	Child  string
}

// NewAlertStormPreventer creates new alert storm preventer
func NewAlertStormPreventer(config *StormPreventionConfig) *AlertStormPreventer {
	if config == nil {
		config = DefaultStormPreventionConfig()
	}

	return &AlertStormPreventer{
		config:      config,
		alertGroups: make(map[string]*AlertGroup),
		parentChild: make([]ParentChildRule, 0),
	}
}

// ShouldSendAlert checks if an alert should be sent
func (s *AlertStormPreventer) ShouldSendAlert(target, namespace, failureLayer string) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create group key
	groupKey := s.makeGroupKey(namespace, failureLayer)

	// Get or create group
	group, exists := s.alertGroups[groupKey]
	if !exists {
		group = &AlertGroup{
			Key:            groupKey,
			AlertCount:     0,
			FirstAlertTime: time.Now(),
			AlertTargets:   make([]string, 0),
		}
		s.alertGroups[groupKey] = group
	}

	now := time.Now()

	// Check if group is suppressed
	if group.Suppressed {
		// Check if suppression window has expired
		if now.Sub(group.FirstAlertTime) > s.config.SuppressionWindow {
			// Reset group
			group.AlertCount = 0
			group.FirstAlertTime = now
			group.Suppressed = false
			group.AlertTargets = make([]string, 0)
		} else {
			return false, "Alert group suppressed"
		}
	}

	// Check cooldown
	if !group.LastSentTime.IsZero() {
		if now.Sub(group.LastSentTime) < s.config.CooldownPeriod {
			return false, "Alert in cooldown period"
		}
	}

	// Check max alerts per group
	if group.AlertCount >= s.config.MaxAlertsPerGroup {
		group.Suppressed = true
		return false, "Max alerts per group reached"
	}

	// Update group
	group.AlertCount++
	group.LastAlertTime = now
	group.LastSentTime = now
	group.AlertTargets = append(group.AlertTargets, target)

	return true, ""
}

// makeGroupKey creates group key from fields
func (s *AlertStormPreventer) makeGroupKey(namespace, failureLayer string) string {
	key := ""
	for _, field := range s.config.GroupBy {
		switch field {
		case "namespace":
			if key != "" {
				key += "/"
			}
			key += namespace
		case "failureLayer":
			if key != "" {
				key += "/"
			}
			key += failureLayer
		}
	}
	if key == "" {
		key = "default"
	}
	return key
}

// AddParentChildRule adds a parent-child suppression rule
func (s *AlertStormPreventer) AddParentChildRule(parent, child string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.parentChild = append(s.parentChild, ParentChildRule{
		Parent: parent,
		Child:  child,
	})
}

// IsSuppressedByParent checks if alert is suppressed by parent alert
func (s *AlertStormPreventer) IsSuppressedByParent(target, failureLayer string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, rule := range s.parentChild {
		// Check if parent pattern matches
		if s.matchesPattern(failureLayer, rule.Parent) {
			// Check if parent alert is active
			parentKey := s.makeGroupKey("cluster", rule.Parent)
			if parentGroup, exists := s.alertGroups[parentKey]; exists {
				if !parentGroup.Suppressed && parentGroup.AlertCount > 0 {
					return true, "Suppressed by parent alert: " + rule.Parent
				}
			}
		}
	}

	return false, ""
}

// matchesPattern checks if value matches pattern (simple prefix match)
func (s *AlertStormPreventer) matchesPattern(value, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(value) >= len(prefix) && value[:len(prefix)] == prefix
	}
	return value == pattern
}

// GetGroupStats returns statistics for an alert group
func (s *AlertStormPreventer) GetGroupStats(namespace, failureLayer string) *AlertGroupStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groupKey := s.makeGroupKey(namespace, failureLayer)
	group, exists := s.alertGroups[groupKey]
	if !exists {
		return nil
	}

	return &AlertGroupStats{
		Key:            group.Key,
		AlertCount:     group.AlertCount,
		FirstAlertTime: group.FirstAlertTime,
		LastAlertTime:  group.LastAlertTime,
		Suppressed:     group.Suppressed,
		TargetCount:    len(group.AlertTargets),
	}
}

// CleanupExpiredGroups removes expired alert groups
func (s *AlertStormPreventer) CleanupExpiredGroups() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	expired := 0
	now := time.Now()

	for key, group := range s.alertGroups {
		// Remove groups that haven't had alerts in suppression window
		if now.Sub(group.LastAlertTime) > s.config.SuppressionWindow {
			delete(s.alertGroups, key)
			expired++
		}
	}

	if expired > 0 {
		log.Info("Cleaned up expired alert groups", "count", expired)
	}

	return expired
}

// GetStats returns storm preventer statistics
func (s *AlertStormPreventer) GetStats() StormPreventionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalGroups := len(s.alertGroups)
	suppressedGroups := 0
	totalAlerts := int32(0)

	for _, group := range s.alertGroups {
		totalAlerts += group.AlertCount
		if group.Suppressed {
			suppressedGroups++
		}
	}

	return StormPreventionStats{
		TotalGroups:      totalGroups,
		SuppressedGroups: suppressedGroups,
		TotalAlerts:      totalAlerts,
		ParentChildRules: len(s.parentChild),
	}
}

// ResetGroup resets an alert group
func (s *AlertStormPreventer) ResetGroup(namespace, failureLayer string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	groupKey := s.makeGroupKey(namespace, failureLayer)
	delete(s.alertGroups, groupKey)
}

// AlertGroupStats contains alert group statistics
type AlertGroupStats struct {
	Key            string
	AlertCount     int32
	FirstAlertTime time.Time
	LastAlertTime  time.Time
	Suppressed     bool
	TargetCount    int
}

// StormPreventionStats contains storm prevention statistics
type StormPreventionStats struct {
	TotalGroups      int
	SuppressedGroups int
	TotalAlerts      int32
	ParentChildRules int
}
