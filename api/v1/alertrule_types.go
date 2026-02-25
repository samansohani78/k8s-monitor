// Package v1 contains API Schema definitions for the k8swatch.io v1 API group
// +kubebuilder:object:generate=true
// +groupName=k8swatch.io
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Severity",type=string,JSONPath=`.spec.severity.base`
// +kubebuilder:printcolumn:name="Consecutive Failures",type=integer,JSONPath=`.spec.trigger.consecutiveFailures`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AlertRule represents an alerting rule configuration
type AlertRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlertRuleSpec   `json:"spec,omitempty"`
	Status AlertRuleStatus `json:"status,omitempty"`
}

// AlertRuleSpec defines the desired state of an AlertRule
type AlertRuleSpec struct {
	// TargetSelector selects which targets this rule applies to
	TargetSelector TargetSelector `json:"targetSelector,omitempty"`

	// Trigger defines the conditions that trigger an alert
	Trigger TriggerConfig `json:"trigger"`

	// Severity defines how alert severity is calculated
	Severity SeverityConfig `json:"severity"`

	// Recovery defines conditions for alert resolution
	Recovery RecoveryConfig `json:"recovery,omitempty"`

	// StormPrevention configures alert storm prevention
	StormPrevention StormPreventionConfig `json:"stormPrevention,omitempty"`

	// NotificationChannels overrides default notification channels
	NotificationChannels []string `json:"notificationChannels,omitempty"`
}

// TargetSelector selects targets for an alert rule
type TargetSelector struct {
	// Names is a list of target names to match
	Names []string `json:"names,omitempty"`

	// Namespace is the namespace to match
	Namespace string `json:"namespace,omitempty"`

	// Labels are labels to match on targets
	Labels map[string]string `json:"labels,omitempty"`

	// Category is the target category to match
	// +kubebuilder:validation:Enum=core;database;search;messaging;identity;proxy;synthetic
	Category string `json:"category,omitempty"`

	// Criticality is the target criticality to match
	// +kubebuilder:validation:Enum=P0;P1;P2;P3
	Criticality string `json:"criticality,omitempty"`

	// Type is the target type to match
	Type TargetType `json:"type,omitempty"`
}

// TriggerConfig defines the conditions that trigger an alert
type TriggerConfig struct {
	// ConsecutiveFailures is the number of consecutive failures before alerting
	ConsecutiveFailures int32 `json:"consecutiveFailures"`

	// AffectedNodes defines constraints on affected nodes
	AffectedNodes *AffectedNodesConfig `json:"affectedNodes,omitempty"`

	// BlastRadius specifies which blast radius levels trigger alerts
	// +kubebuilder:validation:MinItems=1
	BlastRadius []BlastRadiusType `json:"blastRadius,omitempty"`

	// FailureLayers specifies which failure layers trigger alerts
	FailureLayers []string `json:"failureLayers,omitempty"`

	// TimeWindow is the time window for evaluating triggers (e.g., "5m")
	TimeWindow string `json:"timeWindow,omitempty"`
}

// AffectedNodesConfig defines constraints on affected nodes
type AffectedNodesConfig struct {
	// Min is the minimum number of affected nodes
	Min *int32 `json:"min,omitempty"`

	// Max is the maximum number of affected nodes
	Max *int32 `json:"max,omitempty"`

	// MaxPercent is the maximum percentage of nodes affected (0-100)
	MaxPercent *int32 `json:"maxPercent,omitempty"`
}

// BlastRadiusType represents the scope of a failure
type BlastRadiusType string

const (
	// BlastRadiusNode indicates a single-node failure
	BlastRadiusNode BlastRadiusType = "node"
	// BlastRadiusZone indicates a zone-level failure
	BlastRadiusZone BlastRadiusType = "zone"
	// BlastRadiusCluster indicates a cluster-wide failure
	BlastRadiusCluster BlastRadiusType = "cluster"
)

// SeverityConfig defines how alert severity is calculated
type SeverityConfig struct {
	// Base is the base severity level
	// +kubebuilder:validation:Enum=info;warning;critical
	Base string `json:"base"`

	// Overrides define severity overrides based on conditions
	Overrides []SeverityOverride `json:"overrides,omitempty"`
}

// SeverityOverride defines a severity override based on conditions
type SeverityOverride struct {
	// Condition is a CEL expression for the override condition
	Condition string `json:"condition"`

	// Severity is the severity to apply when condition is met
	// +kubebuilder:validation:Enum=info;warning;critical
	Severity string `json:"severity"`
}

// RecoveryConfig defines conditions for alert resolution
type RecoveryConfig struct {
	// ConsecutiveSuccesses is the number of consecutive successes before resolving
	ConsecutiveSuccesses int32 `json:"consecutiveSuccesses"`

	// SustainedPeriod is the minimum sustained healthy period (e.g., "60s")
	SustainedPeriod string `json:"sustainedPeriod,omitempty"`

	// AutoResolve enables automatic alert resolution
	AutoResolve bool `json:"autoResolve"`
}

// StormPreventionConfig configures alert storm prevention
type StormPreventionConfig struct {
	// GroupBy specifies fields to group alerts by
	GroupBy []string `json:"groupBy,omitempty"`

	// MaxAlertsPerGroup is the maximum alerts per group before suppression
	MaxAlertsPerGroup int32 `json:"maxAlertsPerGroup,omitempty"`

	// CooldownPeriod is the minimum time between same alerts (e.g., "5m")
	CooldownPeriod string `json:"cooldownPeriod,omitempty"`

	// SuppressionWindow is the total suppression window (e.g., "10m")
	SuppressionWindow string `json:"suppressionWindow,omitempty"`

	// ParentChildRules defines parent-child suppression relationships
	ParentChildRules []ParentChildRule `json:"parentChildRules,omitempty"`
}

// ParentChildRule defines a parent-child alert suppression relationship
type ParentChildRule struct {
	// Parent is the parent alert pattern
	Parent string `json:"parent"`

	// Child is the child alert pattern to suppress
	Child string `json:"child"`
}

// AlertRuleStatus defines the observed state of an AlertRule
type AlertRuleStatus struct {
	// ObservedGeneration is the last observed generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ActiveAlerts is the number of currently active alerts
	ActiveAlerts int32 `json:"activeAlerts,omitempty"`

	// LastTriggered is the time the rule last triggered an alert
	LastTriggered *metav1.Time `json:"lastTriggered,omitempty"`

	// Conditions represent the current conditions of the rule
	Conditions []AlertRuleCondition `json:"conditions,omitempty"`
}

// AlertRuleCondition represents a condition of an AlertRule
type AlertRuleCondition struct {
	// Type is the type of condition
	Type AlertRuleConditionType `json:"type"`

	// Status is the status of the condition
	Status metav1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Reason is a one-word reason for the condition
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message
	Message string `json:"message,omitempty"`
}

// AlertRuleConditionType is a type of AlertRule condition
type AlertRuleConditionType string

const (
	// AlertRuleConditionActive indicates the rule is actively monitoring
	AlertRuleConditionActive AlertRuleConditionType = "Active"
	// AlertRuleConditionFiring indicates the rule is currently firing
	AlertRuleConditionFiring AlertRuleConditionType = "Firing"
)

// +kubebuilder:object:root=true

// AlertRuleList contains a list of AlertRule
type AlertRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AlertRule{}, &AlertRuleList{})
}
