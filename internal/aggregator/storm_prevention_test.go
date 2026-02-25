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
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStormPreventionConfigDefaults(t *testing.T) {
	cfg := DefaultStormPreventionConfig()

	assert.Equal(t, []string{"namespace", "failureLayer"}, cfg.GroupBy)
	assert.Equal(t, int32(3), cfg.MaxAlertsPerGroup)
	assert.Equal(t, 5*time.Minute, cfg.CooldownPeriod)
	assert.Equal(t, 15*time.Minute, cfg.SuppressionWindow)
}

func TestAlertStormPreventerCreation(t *testing.T) {
	cfg := DefaultStormPreventionConfig()
	preventer := NewAlertStormPreventer(cfg)

	assert.NotNil(t, preventer)
	assert.NotNil(t, preventer.alertGroups)
	assert.NotNil(t, preventer.parentChild)
}

func TestAlertStormPreventerCreationNilConfig(t *testing.T) {
	preventer := NewAlertStormPreventer(nil)

	assert.NotNil(t, preventer)
	assert.Equal(t, 5*time.Minute, preventer.config.CooldownPeriod)
}

func TestAlertStormPreventerShouldSendAlertFirst(t *testing.T) {
	cfg := DefaultStormPreventionConfig()
	preventer := NewAlertStormPreventer(cfg)

	shouldSend, reason := preventer.ShouldSendAlert("target-1", "default", "L2")

	assert.True(t, shouldSend)
	assert.Empty(t, reason)
}

func TestAlertStormPreventerShouldSendAlertCooldown(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy:           []string{"namespace"},
		MaxAlertsPerGroup: 10,
		CooldownPeriod:    1 * time.Second,
		SuppressionWindow: 5 * time.Minute,
	}
	preventer := NewAlertStormPreventer(cfg)

	// First alert - should send
	shouldSend, _ := preventer.ShouldSendAlert("target-1", "default", "L2")
	assert.True(t, shouldSend)

	// Second alert immediately - should be in cooldown
	shouldSend, reason := preventer.ShouldSendAlert("target-2", "default", "L2")
	assert.False(t, shouldSend)
	assert.Contains(t, reason, "cooldown")

	// Wait for cooldown to expire
	time.Sleep(1100 * time.Millisecond)

	// Third alert - should send again
	shouldSend, _ = preventer.ShouldSendAlert("target-3", "default", "L2")
	assert.True(t, shouldSend)
}

func TestAlertStormPreventerShouldSendAlertMaxReached(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy:           []string{"namespace"},
		MaxAlertsPerGroup: 3,
		CooldownPeriod:    100 * time.Millisecond,
		SuppressionWindow: 5 * time.Minute,
	}
	preventer := NewAlertStormPreventer(cfg)

	// Send max alerts
	for i := 0; i < 3; i++ {
		shouldSend, _ := preventer.ShouldSendAlert("target-"+string(rune('a'+i)), "default", "L2")
		assert.True(t, shouldSend)
		// Wait for cooldown
		time.Sleep(110 * time.Millisecond)
	}

	// Fourth alert - should be suppressed
	shouldSend, reason := preventer.ShouldSendAlert("target-d", "default", "L2")
	assert.False(t, shouldSend)
	assert.Contains(t, reason, "Max alerts")
}

func TestAlertStormPreventerGroupKey(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy: []string{"namespace", "failureLayer"},
	}
	preventer := NewAlertStormPreventer(cfg)

	key := preventer.makeGroupKey("default", "L2")
	assert.Equal(t, "default/L2", key)

	// Test with single field
	cfg.GroupBy = []string{"namespace"}
	key = preventer.makeGroupKey("default", "L2")
	assert.Equal(t, "default", key)

	// Test with empty
	cfg.GroupBy = []string{}
	key = preventer.makeGroupKey("default", "L2")
	assert.Equal(t, "default", key)
}

func TestAlertStormPreventerAddParentChildRule(t *testing.T) {
	cfg := DefaultStormPreventionConfig()
	preventer := NewAlertStormPreventer(cfg)

	preventer.AddParentChildRule("cluster.*", "node.*")

	assert.Len(t, preventer.parentChild, 1)
	assert.Equal(t, "cluster.*", preventer.parentChild[0].Parent)
	assert.Equal(t, "node.*", preventer.parentChild[0].Child)
}

func TestAlertStormPreventerIsSuppressedByParent(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy:           []string{"namespace", "failureLayer"},
		MaxAlertsPerGroup: 10,
		CooldownPeriod:    100 * time.Millisecond,
		SuppressionWindow: 5 * time.Minute,
	}
	preventer := NewAlertStormPreventer(cfg)

	// Add parent-child rule
	preventer.AddParentChildRule("L1", "L2")

	// No parent alert active - should not be suppressed
	suppressed, reason := preventer.IsSuppressedByParent("target-1", "L2")
	assert.False(t, suppressed)
	assert.Empty(t, reason)
}

func TestAlertStormPreventerMatchesPattern(t *testing.T) {
	cfg := DefaultStormPreventionConfig()
	preventer := NewAlertStormPreventer(cfg)

	// Exact match
	assert.True(t, preventer.matchesPattern("L1", "L1"))
	assert.False(t, preventer.matchesPattern("L1", "L2"))

	// Wildcard
	assert.True(t, preventer.matchesPattern("L1", "*"))
	assert.True(t, preventer.matchesPattern("anything", "*"))

	// Prefix wildcard
	assert.True(t, preventer.matchesPattern("cluster-wide", "cluster*"))
	assert.True(t, preventer.matchesPattern("cluster", "cluster*"))
	assert.False(t, preventer.matchesPattern("node", "cluster*"))
}

func TestAlertStormPreventerGetGroupStats(t *testing.T) {
	cfg := DefaultStormPreventionConfig()
	preventer := NewAlertStormPreventer(cfg)

	// No group yet
	stats := preventer.GetGroupStats("default", "L2")
	assert.Nil(t, stats)

	// Create group
	preventer.ShouldSendAlert("target-1", "default", "L2")

	stats = preventer.GetGroupStats("default", "L2")
	assert.NotNil(t, stats)
	assert.Equal(t, int32(1), stats.AlertCount)
	assert.Equal(t, 1, stats.TargetCount)
}

func TestAlertStormPreventerCleanupExpiredGroups(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy:           []string{"namespace"},
		MaxAlertsPerGroup: 10,
		CooldownPeriod:    100 * time.Millisecond,
		SuppressionWindow: 200 * time.Millisecond,
	}
	preventer := NewAlertStormPreventer(cfg)

	// Create group
	preventer.ShouldSendAlert("target-1", "default", "L2")

	// Wait for expiration
	time.Sleep(250 * time.Millisecond)

	// Cleanup
	expired := preventer.CleanupExpiredGroups()
	assert.Equal(t, 1, expired)

	// Group should be gone
	stats := preventer.GetGroupStats("default", "L2")
	assert.Nil(t, stats)
}

func TestAlertStormPreventerGetStats(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy:           []string{"namespace"},
		MaxAlertsPerGroup: 5,
		CooldownPeriod:    50 * time.Millisecond,
		SuppressionWindow: 5 * time.Minute,
	}
	preventer := NewAlertStormPreventer(cfg)

	// Send some alerts
	for i := 0; i < 3; i++ {
		preventer.ShouldSendAlert("target-"+string(rune('a'+i)), "default", "L2")
		time.Sleep(55 * time.Millisecond)
	}

	stats := preventer.GetStats()

	assert.Equal(t, 1, stats.TotalGroups)
	assert.Equal(t, int32(3), stats.TotalAlerts)
	assert.Equal(t, 0, stats.SuppressedGroups)
}

func TestAlertStormPreventerResetGroup(t *testing.T) {
	cfg := DefaultStormPreventionConfig()
	preventer := NewAlertStormPreventer(cfg)

	// Create group
	preventer.ShouldSendAlert("target-1", "default", "L2")

	stats := preventer.GetGroupStats("default", "L2")
	assert.NotNil(t, stats)

	// Reset group
	preventer.ResetGroup("default", "L2")

	stats = preventer.GetGroupStats("default", "L2")
	assert.Nil(t, stats)
}

func TestAlertStormPreventerMultipleGroups(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy:           []string{"namespace"},
		MaxAlertsPerGroup: 2,
		CooldownPeriod:    50 * time.Millisecond,
		SuppressionWindow: 5 * time.Minute,
	}
	preventer := NewAlertStormPreventer(cfg)

	// Send alerts to different namespaces
	preventer.ShouldSendAlert("target-1", "ns-1", "L2")
	preventer.ShouldSendAlert("target-2", "ns-2", "L2")

	stats := preventer.GetStats()
	assert.Equal(t, 2, stats.TotalGroups)
	assert.Equal(t, int32(2), stats.TotalAlerts)
}

func TestAlertStormPreventerSuppressionWindow(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy:           []string{"namespace"},
		MaxAlertsPerGroup: 2,
		CooldownPeriod:    50 * time.Millisecond,
		SuppressionWindow: 200 * time.Millisecond,
	}
	preventer := NewAlertStormPreventer(cfg)

	// Send max alerts
	preventer.ShouldSendAlert("target-1", "default", "L2")
	time.Sleep(55 * time.Millisecond)
	preventer.ShouldSendAlert("target-2", "default", "L2")

	// Should be suppressed now
	time.Sleep(55 * time.Millisecond)
	shouldSend, _ := preventer.ShouldSendAlert("target-3", "default", "L2")
	assert.False(t, shouldSend)

	// Wait for suppression window to expire
	time.Sleep(200 * time.Millisecond)

	// Should be able to send again
	shouldSend, _ = preventer.ShouldSendAlert("target-4", "default", "L2")
	assert.True(t, shouldSend)
}

func TestAlertStormPreventerGrouping(t *testing.T) {
	cfg := &StormPreventionConfig{
		GroupBy:           []string{"failureLayer"},
		MaxAlertsPerGroup: 2,
		CooldownPeriod:    50 * time.Millisecond,
		SuppressionWindow: 5 * time.Minute,
	}
	preventer := NewAlertStormPreventer(cfg)

	// Send alerts with same failure layer - should be grouped
	preventer.ShouldSendAlert("target-1", "ns-1", "L2")
	time.Sleep(55 * time.Millisecond)
	preventer.ShouldSendAlert("target-2", "ns-2", "L2")

	// Third alert with same layer should be suppressed
	time.Sleep(55 * time.Millisecond)
	shouldSend, _ := preventer.ShouldSendAlert("target-3", "ns-3", "L2")
	assert.False(t, shouldSend)

	// Alert with different layer should not be suppressed
	shouldSend, _ = preventer.ShouldSendAlert("target-4", "ns-4", "L1")
	assert.True(t, shouldSend)

	stats := preventer.GetStats()
	assert.Equal(t, 2, stats.TotalGroups) // L1 and L2 groups
}
