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

package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// =============================================================================
// BlastRadiusType Tests
// =============================================================================

func TestBlastRadiusTypeValues(t *testing.T) {
	assert.Equal(t, BlastRadiusNode, BlastRadiusType("node"))
	assert.Equal(t, BlastRadiusZone, BlastRadiusType("zone"))
	assert.Equal(t, BlastRadiusCluster, BlastRadiusType("cluster"))
}

// =============================================================================
// AlertRuleConditionType Tests
// =============================================================================

func TestAlertRuleConditionTypeValues(t *testing.T) {
	assert.Equal(t, AlertRuleConditionActive, AlertRuleConditionType("Active"))
	assert.Equal(t, AlertRuleConditionFiring, AlertRuleConditionType("Firing"))
}

// =============================================================================
// TargetSelector Tests
// =============================================================================

func TestTargetSelectorDeepCopy(t *testing.T) {
	original := TargetSelector{
		Names:       []string{"target1", "target2"},
		Namespace:   "default",
		Labels:      map[string]string{"app": "test"},
		Category:    "database",
		Criticality: "P0",
		Type:        TargetTypePostgreSQL,
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.Names, copy.Names)
	assert.Equal(t, original.Namespace, copy.Namespace)
	assert.Equal(t, original.Labels, copy.Labels)
	assert.Equal(t, original.Category, copy.Category)
	assert.Equal(t, original.Criticality, copy.Criticality)
	assert.Equal(t, original.Type, copy.Type)

	// Verify deep copy of maps and slices
	copy.Labels["app"] = "modified"
	assert.NotEqual(t, original.Labels, copy.Labels)

	copy.Names[0] = "modified"
	assert.NotEqual(t, original.Names, copy.Names)
}

func TestTargetSelectorEmpty(t *testing.T) {
	selector := TargetSelector{}

	assert.Nil(t, selector.Names)
	assert.Empty(t, selector.Namespace)
	assert.Nil(t, selector.Labels)
	assert.Empty(t, selector.Category)
	assert.Empty(t, selector.Criticality)
	assert.Empty(t, selector.Type)
}

func TestTargetSelectorWithLabels(t *testing.T) {
	selector := TargetSelector{
		Labels: map[string]string{
			"app":     "postgres",
			"tier":    "database",
			"version": "14",
		},
	}

	assert.Len(t, selector.Labels, 3)
	assert.Equal(t, "postgres", selector.Labels["app"])
}

// =============================================================================
// TriggerConfig Tests
// =============================================================================

func TestTriggerConfigDeepCopy(t *testing.T) {
	minNodes := int32(2)
	maxNodes := int32(10)
	maxPercent := int32(50)

	original := TriggerConfig{
		ConsecutiveFailures: 3,
		AffectedNodes: &AffectedNodesConfig{
			Min:        &minNodes,
			Max:        &maxNodes,
			MaxPercent: &maxPercent,
		},
		BlastRadius:   []BlastRadiusType{BlastRadiusNode, BlastRadiusZone},
		FailureLayers: []string{"L1", "L2", "L3"},
		TimeWindow:    "5m",
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.ConsecutiveFailures, copy.ConsecutiveFailures)
	assert.NotNil(t, copy.AffectedNodes)
	assert.Equal(t, *original.AffectedNodes.Min, *copy.AffectedNodes.Min)
	assert.Equal(t, *original.AffectedNodes.Max, *copy.AffectedNodes.Max)
	assert.Equal(t, *original.AffectedNodes.MaxPercent, *copy.AffectedNodes.MaxPercent)
	assert.Equal(t, original.BlastRadius, copy.BlastRadius)
	assert.Equal(t, original.FailureLayers, copy.FailureLayers)
	assert.Equal(t, original.TimeWindow, copy.TimeWindow)

	// Verify deep copy
	copy.BlastRadius[0] = BlastRadiusCluster
	assert.NotEqual(t, original.BlastRadius, copy.BlastRadius)
}

func TestTriggerConfigWithoutAffectedNodes(t *testing.T) {
	cfg := TriggerConfig{
		ConsecutiveFailures: 5,
		BlastRadius:         []BlastRadiusType{BlastRadiusCluster},
		FailureLayers:       []string{"L0", "L1"},
		TimeWindow:          "10m",
	}

	assert.Nil(t, cfg.AffectedNodes)
	assert.Equal(t, int32(5), cfg.ConsecutiveFailures)
}

func TestAffectedNodesConfigDeepCopy(t *testing.T) {
	minVal := int32(1)
	maxVal := int32(100)
	maxPercent := int32(25)

	original := AffectedNodesConfig{
		Min:        &minVal,
		Max:        &maxVal,
		MaxPercent: &maxPercent,
	}

	copy := original.DeepCopy()

	assert.NotNil(t, copy.Min)
	assert.NotNil(t, copy.Max)
	assert.NotNil(t, copy.MaxPercent)
	assert.Equal(t, *original.Min, *copy.Min)
	assert.Equal(t, *original.Max, *copy.Max)
	assert.Equal(t, *original.MaxPercent, *copy.MaxPercent)

	// Verify deep copy
	*copy.Min = 5
	assert.NotEqual(t, *original.Min, *copy.Min)
}

// =============================================================================
// SeverityConfig Tests
// =============================================================================

func TestSeverityConfigDeepCopy(t *testing.T) {
	original := SeverityConfig{
		Base: "warning",
		Overrides: []SeverityOverride{
			{
				Condition: "blast_radius == 'cluster'",
				Severity:  "critical",
			},
			{
				Condition: "criticality == 'P0'",
				Severity:  "critical",
			},
		},
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.Base, copy.Base)
	assert.Len(t, copy.Overrides, 2)
	assert.Equal(t, original.Overrides[0].Condition, copy.Overrides[0].Condition)
	assert.Equal(t, original.Overrides[0].Severity, copy.Overrides[0].Severity)

	// Verify deep copy
	copy.Overrides[0].Severity = "info"
	assert.NotEqual(t, original.Overrides[0].Severity, copy.Overrides[0].Severity)
}

func TestSeverityConfigWithoutOverrides(t *testing.T) {
	cfg := SeverityConfig{
		Base: "info",
	}

	assert.Empty(t, cfg.Overrides)
	assert.Equal(t, "info", cfg.Base)
}

func TestSeverityOverrideDeepCopy(t *testing.T) {
	original := SeverityOverride{
		Condition: "consecutive_failures > 5",
		Severity:  "critical",
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.Condition, copy.Condition)
	assert.Equal(t, original.Severity, copy.Severity)
}

// =============================================================================
// RecoveryConfig Tests
// =============================================================================

func TestRecoveryConfigDeepCopy(t *testing.T) {
	original := RecoveryConfig{
		ConsecutiveSuccesses: 3,
		SustainedPeriod:      "120s",
		AutoResolve:          true,
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.ConsecutiveSuccesses, copy.ConsecutiveSuccesses)
	assert.Equal(t, original.SustainedPeriod, copy.SustainedPeriod)
	assert.Equal(t, original.AutoResolve, copy.AutoResolve)
}

func TestRecoveryConfigMinimal(t *testing.T) {
	cfg := RecoveryConfig{
		ConsecutiveSuccesses: 1,
		AutoResolve:          false,
	}

	assert.Empty(t, cfg.SustainedPeriod)
	assert.Equal(t, int32(1), cfg.ConsecutiveSuccesses)
}

// =============================================================================
// StormPreventionConfig Tests
// =============================================================================

func TestStormPreventionConfigDeepCopy(t *testing.T) {
	original := StormPreventionConfig{
		GroupBy:           []string{"target", "failure_layer"},
		MaxAlertsPerGroup: 5,
		CooldownPeriod:    "5m",
		SuppressionWindow: "15m",
		ParentChildRules: []ParentChildRule{
			{
				Parent: "node_down",
				Child:  "service_unreachable",
			},
		},
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.GroupBy, copy.GroupBy)
	assert.Equal(t, original.MaxAlertsPerGroup, copy.MaxAlertsPerGroup)
	assert.Equal(t, original.CooldownPeriod, copy.CooldownPeriod)
	assert.Equal(t, original.SuppressionWindow, copy.SuppressionWindow)
	assert.Len(t, copy.ParentChildRules, 1)
	assert.Equal(t, original.ParentChildRules[0].Parent, copy.ParentChildRules[0].Parent)
	assert.Equal(t, original.ParentChildRules[0].Child, copy.ParentChildRules[0].Child)

	// Verify deep copy
	copy.GroupBy[0] = "modified"
	assert.NotEqual(t, original.GroupBy, copy.GroupBy)
}

func TestStormPreventionConfigEmpty(t *testing.T) {
	cfg := StormPreventionConfig{}

	assert.Nil(t, cfg.GroupBy)
	assert.Equal(t, int32(0), cfg.MaxAlertsPerGroup)
	assert.Empty(t, cfg.CooldownPeriod)
	assert.Empty(t, cfg.SuppressionWindow)
	assert.Nil(t, cfg.ParentChildRules)
}

func TestParentChildRuleDeepCopy(t *testing.T) {
	original := ParentChildRule{
		Parent: "dns_failure",
		Child:  "service_resolution_failed",
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.Parent, copy.Parent)
	assert.Equal(t, original.Child, copy.Child)
}

// =============================================================================
// AlertRuleSpec Tests
// =============================================================================

func TestAlertRuleSpecDeepCopy(t *testing.T) {
	original := AlertRuleSpec{
		TargetSelector: TargetSelector{
			Names:     []string{"target1"},
			Namespace: "default",
		},
		Trigger: TriggerConfig{
			ConsecutiveFailures: 3,
		},
		Severity: SeverityConfig{
			Base: "warning",
		},
		Recovery: RecoveryConfig{
			ConsecutiveSuccesses: 2,
			AutoResolve:          true,
		},
		StormPrevention: StormPreventionConfig{
			GroupBy:        []string{"target"},
			CooldownPeriod: "5m",
		},
		NotificationChannels: []string{"slack", "pagerduty"},
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.TargetSelector.Names, copy.TargetSelector.Names)
	assert.Equal(t, original.Trigger.ConsecutiveFailures, copy.Trigger.ConsecutiveFailures)
	assert.Equal(t, original.Severity.Base, copy.Severity.Base)
	assert.Equal(t, original.Recovery.ConsecutiveSuccesses, copy.Recovery.ConsecutiveSuccesses)
	assert.Equal(t, original.StormPrevention.GroupBy, copy.StormPrevention.GroupBy)
	assert.Equal(t, original.NotificationChannels, copy.NotificationChannels)
}

func TestAlertRuleSpecMinimal(t *testing.T) {
	spec := AlertRuleSpec{
		Trigger: TriggerConfig{
			ConsecutiveFailures: 1,
		},
		Severity: SeverityConfig{
			Base: "info",
		},
	}

	assert.Empty(t, spec.TargetSelector.Names)
	assert.Empty(t, spec.NotificationChannels)
}

// =============================================================================
// AlertRuleStatus Tests
// =============================================================================

func TestAlertRuleStatusDeepCopy(t *testing.T) {
	now := metav1.Now()

	original := AlertRuleStatus{
		ObservedGeneration: 5,
		ActiveAlerts:       10,
		LastTriggered:      &now,
		Conditions: []AlertRuleCondition{
			{
				Type:    AlertRuleConditionActive,
				Status:  metav1.ConditionTrue,
				Reason:  "MonitoringActive",
				Message: "Rule is actively monitoring",
			},
		},
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.ObservedGeneration, copy.ObservedGeneration)
	assert.Equal(t, original.ActiveAlerts, copy.ActiveAlerts)
	assert.NotNil(t, copy.LastTriggered)
	assert.Equal(t, original.LastTriggered.Time, copy.LastTriggered.Time)
	assert.Len(t, copy.Conditions, 1)
	assert.Equal(t, original.Conditions[0].Type, copy.Conditions[0].Type)
	assert.Equal(t, original.Conditions[0].Status, copy.Conditions[0].Status)

	// Verify deep copy of time
	copy.LastTriggered = &metav1.Time{Time: time.Now()}
	assert.NotEqual(t, original.LastTriggered, copy.LastTriggered)
}

func TestAlertRuleStatusEmpty(t *testing.T) {
	status := AlertRuleStatus{}

	assert.Equal(t, int64(0), status.ObservedGeneration)
	assert.Equal(t, int32(0), status.ActiveAlerts)
	assert.Nil(t, status.LastTriggered)
	assert.Nil(t, status.Conditions)
}

// =============================================================================
// AlertRuleCondition Tests
// =============================================================================

func TestAlertRuleConditionDeepCopy(t *testing.T) {
	now := metav1.Now()

	original := AlertRuleCondition{
		Type:               AlertRuleConditionFiring,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "ThresholdExceeded",
		Message:            "Consecutive failures exceeded threshold",
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.Type, copy.Type)
	assert.Equal(t, original.Status, copy.Status)
	assert.Equal(t, original.LastTransitionTime, copy.LastTransitionTime)
	assert.Equal(t, original.Reason, copy.Reason)
	assert.Equal(t, original.Message, copy.Message)
}

func TestAlertRuleConditionWithoutReason(t *testing.T) {
	now := metav1.Now()

	cond := AlertRuleCondition{
		Type:               AlertRuleConditionActive,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
	}

	assert.Empty(t, cond.Reason)
	assert.Empty(t, cond.Message)
}

// =============================================================================
// AlertRule Tests
// =============================================================================

func TestAlertRuleDeepCopy(t *testing.T) {
	now := metav1.Now()

	original := AlertRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AlertRule",
			APIVersion: "k8swatch.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-rule",
			Namespace:       "default",
			ResourceVersion: "12345",
		},
		Spec: AlertRuleSpec{
			TargetSelector: TargetSelector{
				Names: []string{"target1"},
			},
			Trigger: TriggerConfig{
				ConsecutiveFailures: 3,
			},
			Severity: SeverityConfig{
				Base: "warning",
			},
		},
		Status: AlertRuleStatus{
			ObservedGeneration: 1,
			ActiveAlerts:       0,
			LastTriggered:      &now,
		},
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.Kind, copy.Kind)
	assert.Equal(t, original.APIVersion, copy.APIVersion)
	assert.Equal(t, original.Name, copy.Name)
	assert.Equal(t, original.Namespace, copy.Namespace)
	assert.Equal(t, original.Spec.TargetSelector.Names, copy.Spec.TargetSelector.Names)
	assert.Equal(t, original.Status.ObservedGeneration, copy.Status.ObservedGeneration)
}

func TestAlertRuleEmpty(t *testing.T) {
	rule := AlertRule{}

	assert.Empty(t, rule.Name)
	assert.Empty(t, rule.Namespace)
}

// =============================================================================
// AlertRuleList Tests
// =============================================================================

func TestAlertRuleListDeepCopy(t *testing.T) {
	original := AlertRuleList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AlertRuleList",
			APIVersion: "k8swatch.io/v1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "12345",
		},
		Items: []AlertRule{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rule1",
					Namespace: "default",
				},
				Spec: AlertRuleSpec{
					Trigger: TriggerConfig{
						ConsecutiveFailures: 3,
					},
					Severity: SeverityConfig{
						Base: "warning",
					},
				},
			},
		},
	}

	copy := original.DeepCopy()

	assert.Equal(t, original.Kind, copy.Kind)
	assert.Equal(t, original.APIVersion, copy.APIVersion)
	assert.Len(t, copy.Items, 1)
	assert.Equal(t, original.Items[0].Name, copy.Items[0].Name)
	assert.Equal(t, original.Items[0].Namespace, copy.Items[0].Namespace)

	// Verify deep copy
	copy.Items[0].Name = "modified"
	assert.NotEqual(t, original.Items[0].Name, copy.Items[0].Name)
}

func TestAlertRuleListEmpty(t *testing.T) {
	list := AlertRuleList{}

	assert.Empty(t, list.Items)
	assert.Empty(t, list.ResourceVersion)
}

// =============================================================================
// AlertRule Condition Tests
// =============================================================================

func TestAlertRuleConditionActiveStatus(t *testing.T) {
	now := metav1.Now()

	cond := AlertRuleCondition{
		Type:               AlertRuleConditionActive,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "MonitoringActive",
		Message:            "Rule is actively monitoring targets",
	}

	assert.Equal(t, AlertRuleConditionActive, cond.Type)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Equal(t, "MonitoringActive", cond.Reason)
}

func TestAlertRuleConditionFiringStatus(t *testing.T) {
	now := metav1.Now()

	cond := AlertRuleCondition{
		Type:               AlertRuleConditionFiring,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "ThresholdExceeded",
		Message:            "Alert threshold exceeded",
	}

	assert.Equal(t, AlertRuleConditionFiring, cond.Type)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
}

func TestAlertRuleConditionFalse(t *testing.T) {
	now := metav1.Now()

	cond := AlertRuleCondition{
		Type:               AlertRuleConditionFiring,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             "BelowThreshold",
	}

	assert.Equal(t, metav1.ConditionFalse, cond.Status)
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestCompleteAlertRule(t *testing.T) {
	minNodes := int32(2)

	rule := AlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "critical-database-alert",
			Namespace: "monitoring",
			Labels: map[string]string{
				"team":       "dba",
				"criticality": "P0",
			},
		},
		Spec: AlertRuleSpec{
			TargetSelector: TargetSelector{
				Category:    "database",
				Criticality: "P0",
			},
			Trigger: TriggerConfig{
				ConsecutiveFailures: 2,
				AffectedNodes: &AffectedNodesConfig{
					Min: &minNodes,
				},
				BlastRadius:   []BlastRadiusType{BlastRadiusZone, BlastRadiusCluster},
				FailureLayers: []string{"L0", "L1", "L2"},
				TimeWindow:    "5m",
			},
			Severity: SeverityConfig{
				Base: "warning",
				Overrides: []SeverityOverride{
					{
						Condition: "blast_radius == 'cluster'",
						Severity:  "critical",
					},
				},
			},
			Recovery: RecoveryConfig{
				ConsecutiveSuccesses: 3,
				SustainedPeriod:      "120s",
				AutoResolve:          true,
			},
			StormPrevention: StormPreventionConfig{
				GroupBy:           []string{"target", "failure_layer"},
				MaxAlertsPerGroup: 3,
				CooldownPeriod:    "10m",
				SuppressionWindow: "30m",
				ParentChildRules: []ParentChildRule{
					{
						Parent: "node_down",
						Child:  "database_unreachable",
					},
				},
			},
			NotificationChannels: []string{"pagerduty", "slack"},
		},
	}

	assert.Equal(t, "critical-database-alert", rule.Name)
	assert.Equal(t, "database", rule.Spec.TargetSelector.Category)
	assert.Equal(t, int32(2), rule.Spec.Trigger.ConsecutiveFailures)
	assert.Equal(t, "warning", rule.Spec.Severity.Base)
	assert.Len(t, rule.Spec.StormPrevention.ParentChildRules, 1)
	assert.Len(t, rule.Spec.NotificationChannels, 2)
}

func TestAlertRuleWithTimeValues(t *testing.T) {
	now := metav1.Now()
	later := metav1.NewTime(now.Add(5 * time.Minute))

	rule := AlertRule{
		Spec: AlertRuleSpec{
			Trigger: TriggerConfig{
				ConsecutiveFailures: 3,
				TimeWindow:          "5m",
			},
			Severity: SeverityConfig{
				Base: "critical",
			},
			Recovery: RecoveryConfig{
				ConsecutiveSuccesses: 2,
				SustainedPeriod:      "2m",
				AutoResolve:          true,
			},
			StormPrevention: StormPreventionConfig{
				CooldownPeriod:    "5m",
				SuppressionWindow: "15m",
			},
		},
		Status: AlertRuleStatus{
			LastTriggered: &now,
			Conditions: []AlertRuleCondition{
				{
					LastTransitionTime: later,
				},
			},
		},
	}

	assert.Equal(t, "5m", rule.Spec.Trigger.TimeWindow)
	assert.Equal(t, "2m", rule.Spec.Recovery.SustainedPeriod)
	assert.NotNil(t, rule.Status.LastTriggered)
}
