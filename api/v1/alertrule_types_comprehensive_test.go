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

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test AlertRule struct
func TestAlertRule_StructFields(t *testing.T) {
	rule := AlertRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AlertRule",
			APIVersion: "k8swatch.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rule",
			Namespace: "default",
		},
		Spec: AlertRuleSpec{
			Severity: SeverityConfig{
				Base: "warning",
			},
			Trigger: TriggerConfig{
				ConsecutiveFailures: 3,
			},
		},
	}

	assert.Equal(t, "AlertRule", rule.Kind)
	assert.Equal(t, "k8swatch.io/v1", rule.APIVersion)
	assert.Equal(t, "test-rule", rule.Name)
	assert.Equal(t, "default", rule.Namespace)
	assert.Equal(t, "warning", rule.Spec.Severity.Base)
	assert.Equal(t, int32(3), rule.Spec.Trigger.ConsecutiveFailures)
}

// Test AlertRuleSpec with all fields
func TestAlertRuleSpec_Complete(t *testing.T) {
	spec := AlertRuleSpec{
		TargetSelector: TargetSelector{
			Names:       []string{"target-1", "target-2"},
			Namespace:   "default",
			Labels:      map[string]string{"env": "prod"},
			Category:    "database",
			Criticality: "P1",
			Type:        TargetTypePostgreSQL,
		},
		Trigger: TriggerConfig{
			ConsecutiveFailures: 5,
			AffectedNodes: &AffectedNodesConfig{
				Min:        int32Ptr(1),
				Max:        int32Ptr(10),
				MaxPercent: int32Ptr(50),
			},
			BlastRadius:   []BlastRadiusType{BlastRadiusNode, BlastRadiusZone},
			FailureLayers: []string{"L1", "L2"},
			TimeWindow:    "5m",
		},
		Severity: SeverityConfig{
			Base: "warning",
			Overrides: []SeverityOverride{
				{
					Condition: "blastRadius == 'cluster'",
					Severity:  "critical",
				},
			},
		},
		Recovery: RecoveryConfig{
			ConsecutiveSuccesses: 3,
			SustainedPeriod:      "60s",
			AutoResolve:          true,
		},
		StormPrevention: StormPreventionConfig{
			GroupBy:           []string{"target", "severity"},
			MaxAlertsPerGroup: 5,
			CooldownPeriod:    "300s",
			SuppressionWindow: "3600s",
			ParentChildRules: []ParentChildRule{
				{
					Parent: "node-down",
					Child:  "pod-down",
				},
			},
		},
		NotificationChannels: []string{"slack", "pagerduty"},
	}

	// Test TargetSelector
	assert.Len(t, spec.TargetSelector.Names, 2)
	assert.Equal(t, "default", spec.TargetSelector.Namespace)
	assert.Equal(t, "prod", spec.TargetSelector.Labels["env"])
	assert.Equal(t, "database", spec.TargetSelector.Category)
	assert.Equal(t, "P1", spec.TargetSelector.Criticality)
	assert.Equal(t, TargetTypePostgreSQL, spec.TargetSelector.Type)

	// Test Trigger
	assert.Equal(t, int32(5), spec.Trigger.ConsecutiveFailures)
	assert.NotNil(t, spec.Trigger.AffectedNodes)
	assert.Equal(t, int32(1), *spec.Trigger.AffectedNodes.Min)
	assert.Equal(t, int32(10), *spec.Trigger.AffectedNodes.Max)
	assert.Equal(t, int32(50), *spec.Trigger.AffectedNodes.MaxPercent)
	assert.Len(t, spec.Trigger.BlastRadius, 2)
	assert.Len(t, spec.Trigger.FailureLayers, 2)
	assert.Equal(t, "5m", spec.Trigger.TimeWindow)

	// Test Severity
	assert.Equal(t, "warning", spec.Severity.Base)
	assert.Len(t, spec.Severity.Overrides, 1)
	assert.Equal(t, "blastRadius == 'cluster'", spec.Severity.Overrides[0].Condition)
	assert.Equal(t, "critical", spec.Severity.Overrides[0].Severity)

	// Test Recovery
	assert.Equal(t, int32(3), spec.Recovery.ConsecutiveSuccesses)
	assert.Equal(t, "60s", spec.Recovery.SustainedPeriod)
	assert.True(t, spec.Recovery.AutoResolve)

	// Test StormPrevention
	assert.Len(t, spec.StormPrevention.GroupBy, 2)
	assert.Equal(t, int32(5), spec.StormPrevention.MaxAlertsPerGroup)
	assert.Equal(t, "300s", spec.StormPrevention.CooldownPeriod)
	assert.Equal(t, "3600s", spec.StormPrevention.SuppressionWindow)
	assert.Len(t, spec.StormPrevention.ParentChildRules, 1)
	assert.Equal(t, "node-down", spec.StormPrevention.ParentChildRules[0].Parent)
	assert.Equal(t, "pod-down", spec.StormPrevention.ParentChildRules[0].Child)

	// Test NotificationChannels
	assert.Len(t, spec.NotificationChannels, 2)
}

// Test AlertRuleSpec minimal
func TestAlertRuleSpec_Minimal(t *testing.T) {
	spec := AlertRuleSpec{
		Trigger: TriggerConfig{
			ConsecutiveFailures: 3,
		},
		Severity: SeverityConfig{
			Base: "info",
		},
	}

	assert.Equal(t, int32(3), spec.Trigger.ConsecutiveFailures)
	assert.Equal(t, "info", spec.Severity.Base)
	assert.Nil(t, spec.Trigger.AffectedNodes)
	assert.Empty(t, spec.Trigger.BlastRadius)
	assert.Empty(t, spec.Severity.Overrides)
	assert.Equal(t, int32(0), spec.Recovery.ConsecutiveSuccesses)
	assert.False(t, spec.Recovery.AutoResolve)
}

// Test TargetSelector
func TestTargetSelector(t *testing.T) {
	selector := TargetSelector{
		Names:       []string{"target-1"},
		Namespace:   "monitoring",
		Labels:      map[string]string{"app": "test"},
		Category:    "messaging",
		Criticality: "P2",
		Type:        TargetTypeKafka,
	}

	assert.Len(t, selector.Names, 1)
	assert.Equal(t, "monitoring", selector.Namespace)
	assert.Equal(t, "test", selector.Labels["app"])
	assert.Equal(t, "messaging", selector.Category)
	assert.Equal(t, "P2", selector.Criticality)
	assert.Equal(t, TargetTypeKafka, selector.Type)
}

// Test TargetSelector empty
func TestTargetSelector_Empty(t *testing.T) {
	selector := TargetSelector{}

	assert.Empty(t, selector.Names)
	assert.Empty(t, selector.Namespace)
	assert.Empty(t, selector.Labels)
	assert.Empty(t, selector.Category)
	assert.Empty(t, selector.Criticality)
	assert.Equal(t, TargetType(""), selector.Type)
}

// Test TriggerConfig
func TestTriggerConfig(t *testing.T) {
	minVal := int32(2)
	maxVal := int32(20)
	percentVal := int32(75)

	trigger := TriggerConfig{
		ConsecutiveFailures: 5,
		AffectedNodes: &AffectedNodesConfig{
			Min:        &minVal,
			Max:        &maxVal,
			MaxPercent: &percentVal,
		},
		BlastRadius:   []BlastRadiusType{BlastRadiusCluster},
		FailureLayers: []string{"L1", "L2", "L3"},
		TimeWindow:    "10m",
	}

	assert.Equal(t, int32(5), trigger.ConsecutiveFailures)
	assert.NotNil(t, trigger.AffectedNodes)
	assert.Equal(t, int32(2), *trigger.AffectedNodes.Min)
	assert.Equal(t, int32(20), *trigger.AffectedNodes.Max)
	assert.Equal(t, int32(75), *trigger.AffectedNodes.MaxPercent)
	assert.Len(t, trigger.BlastRadius, 1)
	assert.Len(t, trigger.FailureLayers, 3)
	assert.Equal(t, "10m", trigger.TimeWindow)
}

// Test TriggerConfig minimal
func TestTriggerConfig_Minimal(t *testing.T) {
	trigger := TriggerConfig{
		ConsecutiveFailures: 3,
	}

	assert.Equal(t, int32(3), trigger.ConsecutiveFailures)
	assert.Nil(t, trigger.AffectedNodes)
	assert.Empty(t, trigger.BlastRadius)
	assert.Empty(t, trigger.FailureLayers)
	assert.Empty(t, trigger.TimeWindow)
}

// Test AffectedNodesConfig
func TestAffectedNodesConfig(t *testing.T) {
	minVal := int32(1)
	maxVal := int32(50)
	percentVal := int32(80)

	config := AffectedNodesConfig{
		Min:        &minVal,
		Max:        &maxVal,
		MaxPercent: &percentVal,
	}

	assert.NotNil(t, config.Min)
	assert.NotNil(t, config.Max)
	assert.NotNil(t, config.MaxPercent)
	assert.Equal(t, int32(1), *config.Min)
	assert.Equal(t, int32(50), *config.Max)
	assert.Equal(t, int32(80), *config.MaxPercent)
}

// Test AffectedNodesConfig nil
func TestAffectedNodesConfig_Nil(t *testing.T) {
	config := AffectedNodesConfig{}

	assert.Nil(t, config.Min)
	assert.Nil(t, config.Max)
	assert.Nil(t, config.MaxPercent)
}

// Test BlastRadiusType constants
func TestBlastRadiusTypeConstants(t *testing.T) {
	assert.Equal(t, BlastRadiusType("node"), BlastRadiusNode)
	assert.Equal(t, BlastRadiusType("zone"), BlastRadiusZone)
	assert.Equal(t, BlastRadiusType("cluster"), BlastRadiusCluster)
}

// Test SeverityConfig
func TestSeverityConfig(t *testing.T) {
	config := SeverityConfig{
		Base: "critical",
		Overrides: []SeverityOverride{
			{
				Condition: "affectedNodes > 10",
				Severity:  "critical",
			},
			{
				Condition: "blastRadius == 'zone'",
				Severity:  "warning",
			},
		},
	}

	assert.Equal(t, "critical", config.Base)
	assert.Len(t, config.Overrides, 2)
	assert.Equal(t, "affectedNodes > 10", config.Overrides[0].Condition)
	assert.Equal(t, "critical", config.Overrides[0].Severity)
}

// Test SeverityConfig minimal
func TestSeverityConfig_Minimal(t *testing.T) {
	config := SeverityConfig{
		Base: "info",
	}

	assert.Equal(t, "info", config.Base)
	assert.Empty(t, config.Overrides)
}

// Test SeverityOverride
func TestSeverityOverride(t *testing.T) {
	override := SeverityOverride{
		Condition: "blastRadius == 'cluster'",
		Severity:  "critical",
	}

	assert.Equal(t, "blastRadius == 'cluster'", override.Condition)
	assert.Equal(t, "critical", override.Severity)
}

// Test RecoveryConfig
func TestRecoveryConfig(t *testing.T) {
	config := RecoveryConfig{
		ConsecutiveSuccesses: 5,
		SustainedPeriod:      "120s",
		AutoResolve:          true,
	}

	assert.Equal(t, int32(5), config.ConsecutiveSuccesses)
	assert.Equal(t, "120s", config.SustainedPeriod)
	assert.True(t, config.AutoResolve)
}

// Test RecoveryConfig minimal
func TestRecoveryConfig_Minimal(t *testing.T) {
	config := RecoveryConfig{
		ConsecutiveSuccesses: 2,
		AutoResolve:          false,
	}

	assert.Equal(t, int32(2), config.ConsecutiveSuccesses)
	assert.Empty(t, config.SustainedPeriod)
	assert.False(t, config.AutoResolve)
}

// Test StormPreventionConfig
func TestStormPreventionConfig(t *testing.T) {
	config := StormPreventionConfig{
		GroupBy:           []string{"target", "severity", "namespace"},
		MaxAlertsPerGroup: 10,
		CooldownPeriod:    "600s",
		SuppressionWindow: "7200s",
		ParentChildRules: []ParentChildRule{
			{
				Parent: "cluster-down",
				Child:  "node-down",
			},
			{
				Parent: "node-down",
				Child:  "pod-down",
			},
		},
	}

	assert.Len(t, config.GroupBy, 3)
	assert.Equal(t, int32(10), config.MaxAlertsPerGroup)
	assert.Equal(t, "600s", config.CooldownPeriod)
	assert.Equal(t, "7200s", config.SuppressionWindow)
	assert.Len(t, config.ParentChildRules, 2)
}

// Test StormPreventionConfig minimal
func TestStormPreventionConfig_Minimal(t *testing.T) {
	config := StormPreventionConfig{}

	assert.Empty(t, config.GroupBy)
	assert.Equal(t, int32(0), config.MaxAlertsPerGroup)
	assert.Empty(t, config.CooldownPeriod)
	assert.Empty(t, config.SuppressionWindow)
	assert.Empty(t, config.ParentChildRules)
}

// Test ParentChildRule
func TestParentChildRule(t *testing.T) {
	rule := ParentChildRule{
		Parent: "parent-alert",
		Child:  "child-alert",
	}

	assert.Equal(t, "parent-alert", rule.Parent)
	assert.Equal(t, "child-alert", rule.Child)
}

// Test AlertRuleStatus
func TestAlertRuleStatus(t *testing.T) {
	now := metav1.Now()
	status := AlertRuleStatus{
		ObservedGeneration: 5,
		ActiveAlerts:       3,
		LastTriggered:      &now,
		Conditions: []AlertRuleCondition{
			{
				Type:   AlertRuleConditionActive,
				Status: metav1.ConditionTrue,
				Reason: "MonitoringActive",
			},
		},
	}

	assert.Equal(t, int64(5), status.ObservedGeneration)
	assert.Equal(t, int32(3), status.ActiveAlerts)
	assert.NotNil(t, status.LastTriggered)
	assert.Len(t, status.Conditions, 1)
	assert.Equal(t, AlertRuleConditionActive, status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, status.Conditions[0].Status)
}

// Test AlertRuleStatus minimal
func TestAlertRuleStatus_Minimal(t *testing.T) {
	status := AlertRuleStatus{}

	assert.Equal(t, int64(0), status.ObservedGeneration)
	assert.Equal(t, int32(0), status.ActiveAlerts)
	assert.Nil(t, status.LastTriggered)
	assert.Empty(t, status.Conditions)
}

// Test AlertRuleCondition
func TestAlertRuleCondition(t *testing.T) {
	now := metav1.Now()
	condition := AlertRuleCondition{
		Type:               AlertRuleConditionFiring,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "ThresholdExceeded",
		Message:            "Alert threshold exceeded",
	}

	assert.Equal(t, AlertRuleConditionFiring, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "ThresholdExceeded", condition.Reason)
	assert.Equal(t, "Alert threshold exceeded", condition.Message)
}

// Test AlertRuleConditionType constants
func TestAlertRuleConditionTypeConstants(t *testing.T) {
	assert.Equal(t, AlertRuleConditionType("Active"), AlertRuleConditionActive)
	assert.Equal(t, AlertRuleConditionType("Firing"), AlertRuleConditionFiring)
}

// Test AlertRuleList
func TestAlertRuleList(t *testing.T) {
	list := AlertRuleList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AlertRuleList",
			APIVersion: "k8swatch.io/v1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1",
		},
		Items: []AlertRule{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rule-1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rule-2",
				},
			},
		},
	}

	assert.Equal(t, "AlertRuleList", list.Kind)
	assert.Equal(t, "1", list.ResourceVersion)
	assert.Len(t, list.Items, 2)
	assert.Equal(t, "rule-1", list.Items[0].Name)
	assert.Equal(t, "rule-2", list.Items[1].Name)
}

// Test AlertRule with Status
func TestAlertRule_WithStatus(t *testing.T) {
	now := metav1.Now()
	rule := AlertRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-rule",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: AlertRuleSpec{
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

	assert.Equal(t, int64(1), rule.Generation)
	assert.Equal(t, int64(1), rule.Status.ObservedGeneration)
	assert.Equal(t, int32(0), rule.Status.ActiveAlerts)
	assert.NotNil(t, rule.Status.LastTriggered)
}

// Test AlertRule with all notification channels
func TestAlertRuleSpec_WithNotificationChannels(t *testing.T) {
	spec := AlertRuleSpec{
		Trigger: TriggerConfig{
			ConsecutiveFailures: 3,
		},
		Severity: SeverityConfig{
			Base: "warning",
		},
		NotificationChannels: []string{"slack", "pagerduty", "email", "webhook"},
	}

	assert.Len(t, spec.NotificationChannels, 4)
	assert.Contains(t, spec.NotificationChannels, "slack")
	assert.Contains(t, spec.NotificationChannels, "pagerduty")
}

// Test AlertRuleSpec without StormPrevention
func TestAlertRuleSpec_WithoutStormPrevention(t *testing.T) {
	spec := AlertRuleSpec{
		Trigger: TriggerConfig{
			ConsecutiveFailures: 3,
		},
		Severity: SeverityConfig{
			Base: "warning",
		},
	}

	assert.Empty(t, spec.StormPrevention.GroupBy)
	assert.Equal(t, int32(0), spec.StormPrevention.MaxAlertsPerGroup)
}

// Test TriggerConfig with only BlastRadius
func TestTriggerConfig_WithBlastRadius(t *testing.T) {
	trigger := TriggerConfig{
		ConsecutiveFailures: 3,
		BlastRadius:         []BlastRadiusType{BlastRadiusNode, BlastRadiusZone, BlastRadiusCluster},
	}

	assert.Equal(t, int32(3), trigger.ConsecutiveFailures)
	assert.Len(t, trigger.BlastRadius, 3)
	assert.Nil(t, trigger.AffectedNodes)
}

// Test TriggerConfig with only FailureLayers
func TestTriggerConfig_WithFailureLayers(t *testing.T) {
	trigger := TriggerConfig{
		ConsecutiveFailures: 5,
		FailureLayers:       []string{"L0", "L1", "L2", "L3", "L4", "L5", "L6"},
	}

	assert.Equal(t, int32(5), trigger.ConsecutiveFailures)
	assert.Len(t, trigger.FailureLayers, 7)
}

// Test SeverityConfig with multiple overrides
func TestSeverityConfig_MultipleOverrides(t *testing.T) {
	config := SeverityConfig{
		Base: "info",
		Overrides: []SeverityOverride{
			{
				Condition: "affectedNodes > 5",
				Severity:  "warning",
			},
			{
				Condition: "affectedNodes > 20",
				Severity:  "critical",
			},
			{
				Condition: "blastRadius == 'cluster'",
				Severity:  "critical",
			},
		},
	}

	assert.Equal(t, "info", config.Base)
	assert.Len(t, config.Overrides, 3)
}

// Test RecoveryConfig with AutoResolve
func TestRecoveryConfig_WithAutoResolve(t *testing.T) {
	config := RecoveryConfig{
		ConsecutiveSuccesses: 2,
		SustainedPeriod:      "30s",
		AutoResolve:          true,
	}

	assert.True(t, config.AutoResolve)
	assert.Equal(t, "30s", config.SustainedPeriod)
}

// Test StormPreventionConfig with ParentChildRules
func TestStormPreventionConfig_WithParentChildRules(t *testing.T) {
	config := StormPreventionConfig{
		GroupBy: []string{"target"},
		ParentChildRules: []ParentChildRule{
			{
				Parent: "network-down",
				Child:  "service-down",
			},
		},
	}

	assert.Len(t, config.ParentChildRules, 1)
	assert.Equal(t, "network-down", config.ParentChildRules[0].Parent)
	assert.Equal(t, "service-down", config.ParentChildRules[0].Child)
}

// Test TargetSelector with only Type
func TestTargetSelector_WithType(t *testing.T) {
	selector := TargetSelector{
		Type: TargetTypeHTTP,
	}

	assert.Equal(t, TargetTypeHTTP, selector.Type)
	assert.Empty(t, selector.Names)
	assert.Empty(t, selector.Namespace)
}

// Test TargetSelector with only Category
func TestTargetSelector_WithCategory(t *testing.T) {
	selector := TargetSelector{
		Category: "database",
	}

	assert.Equal(t, "database", selector.Category)
}

// Test TargetSelector with only Criticality
func TestTargetSelector_WithCriticality(t *testing.T) {
	selector := TargetSelector{
		Criticality: "P0",
	}

	assert.Equal(t, "P0", selector.Criticality)
}

// Test AlertRuleCondition with all fields
func TestAlertRuleCondition_Complete(t *testing.T) {
	now := metav1.Now()
	condition := AlertRuleCondition{
		Type:               AlertRuleConditionActive,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "Healthy",
		Message:            "All checks passing",
	}

	assert.Equal(t, AlertRuleConditionActive, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "Healthy", condition.Reason)
	assert.Equal(t, "All checks passing", condition.Message)
}

// Test AlertRuleCondition without optional fields
func TestAlertRuleCondition_Minimal(t *testing.T) {
	now := metav1.Now()
	condition := AlertRuleCondition{
		Type:               AlertRuleConditionActive,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
	}

	assert.Empty(t, condition.Reason)
	assert.Empty(t, condition.Message)
}

// Test AlertRuleList empty
func TestAlertRuleList_Empty(t *testing.T) {
	list := AlertRuleList{}

	assert.Empty(t, list.Items)
	assert.Empty(t, list.ResourceVersion)
}
