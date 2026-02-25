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
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// AlertState represents the state of an alert
type AlertState string

const (
	// AlertStateFiring indicates the alert is actively firing
	AlertStateFiring AlertState = "firing"
	// AlertStateAcknowledged indicates the alert has been acknowledged
	AlertStateAcknowledged AlertState = "acknowledged"
	// AlertStateResolved indicates the alert has been resolved
	AlertStateResolved AlertState = "resolved"
	// AlertStateSilenced indicates the alert is silenced (maintenance)
	AlertStateSilenced AlertState = "silenced"
)

// AlertSeverity represents alert severity
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Alert represents an alert in the system
type Alert struct {
	// AlertID is a unique identifier
	AlertID string `json:"alertId"`
	// Rule is the name of the AlertRule that triggered this alert
	Rule string `json:"rule"`
	// Target is the target that triggered the alert
	Target k8swatchv1.TargetRef `json:"target"`
	// Severity is the alert severity
	Severity AlertSeverity `json:"severity"`
	// Status is the current alert status
	Status AlertState `json:"status"`
	// FiredAt is when the alert was fired
	FiredAt time.Time `json:"firedAt"`
	// ResolvedAt is when the alert was resolved (if applicable)
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	// AcknowledgedAt is when the alert was acknowledged
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	// AcknowledgedBy is the user who acknowledged the alert
	AcknowledgedBy string `json:"acknowledgedBy,omitempty"`
	// SilencedAt is when the alert was silenced
	SilencedAt *time.Time `json:"silencedAt,omitempty"`
	// SilencedBy is the user who silenced the alert
	SilencedBy string `json:"silencedBy,omitempty"`
	// SilenceReason is the reason for silencing
	SilenceReason string `json:"silenceReason,omitempty"`
	// SilenceEndsAt is when the silence expires
	SilenceEndsAt *time.Time `json:"silenceEndsAt,omitempty"`
	// FailureLayer is the layer where failure occurred
	FailureLayer string `json:"failureLayer,omitempty"`
	// FailureCode is the specific failure code
	FailureCode string `json:"failureCode,omitempty"`
	// BlastRadius is the blast radius classification
	BlastRadius string `json:"blastRadius"`
	// AffectedNodes is the list of affected node names
	AffectedNodes []string `json:"affectedNodes,omitempty"`
	// ConsecutiveFailures is the number of consecutive failures
	ConsecutiveFailures int32 `json:"consecutiveFailures"`
	// LastUpdatedAt is when the alert was last updated
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
	// NotificationCount is the number of notifications sent
	NotificationCount int32 `json:"notificationCount"`
	// LastNotificationAt is when the last notification was sent
	LastNotificationAt *time.Time `json:"lastNotificationAt,omitempty"`
	// Labels are additional labels for routing
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations are additional annotations
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AlertEvent represents a state transition event
type AlertEvent struct {
	// EventID is a unique identifier
	EventID string `json:"eventId"`
	// AlertID is the alert this event belongs to
	AlertID string `json:"alertId"`
	// EventType is the type of event
	EventType string `json:"eventType"`
	// FromState is the previous state
	FromState AlertState `json:"fromState"`
	// ToState is the new state
	ToState AlertState `json:"toState"`
	// Reason is the reason for the transition
	Reason string `json:"reason,omitempty"`
	// User is the user who triggered the event (if applicable)
	User string `json:"user,omitempty"`
	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`
}

// AlertAction represents an action to be taken on an alert
type AlertAction struct {
	// ActionID is a unique identifier
	ActionID string `json:"actionId"`
	// AlertID is the alert this action belongs to
	AlertID string `json:"alertId"`
	// ActionType is the type of action (notify, escalate, etc.)
	ActionType string `json:"actionType"`
	// Channel is the notification channel
	Channel string `json:"channel"`
	// Status is the action status (pending, sent, failed)
	Status string `json:"status"`
	// Error is the error message if failed
	Error string `json:"error,omitempty"`
	// SentAt is when the action was executed
	SentAt time.Time `json:"sentAt,omitempty"`
}

// AlertFilter is used to filter alerts in queries
type AlertFilter struct {
	// Status filters by alert status
	Status []AlertState `json:"status,omitempty"`
	// Severity filters by severity
	Severity []AlertSeverity `json:"severity,omitempty"`
	// TargetName filters by target name
	TargetName string `json:"targetName,omitempty"`
	// Namespace filters by namespace
	Namespace string `json:"namespace,omitempty"`
	// BlastRadius filters by blast radius
	BlastRadius string `json:"blastRadius,omitempty"`
	// From filters by fired after time
	From time.Time `json:"from,omitempty"`
	// To filters by fired before time
	To time.Time `json:"to,omitempty"`
	// Limit is the maximum number of alerts to return
	Limit int `json:"limit,omitempty"`
	// Cursor is the pagination cursor
	Cursor string `json:"cursor,omitempty"`
}

// AlertQueryResult is the result of an alert query
type AlertQueryResult struct {
	// Alerts is the list of alerts
	Alerts []Alert `json:"alerts"`
	// NextCursor is the cursor for the next page
	NextCursor string `json:"nextCursor,omitempty"`
	// Total is the total number of alerts matching the filter
	Total int `json:"total"`
}

// SilenceRequest represents a request to silence an alert
type SilenceRequest struct {
	// Duration is how long to silence (e.g., "1h", "30m")
	Duration string `json:"duration"`
	// Reason is the reason for silencing
	Reason string `json:"reason"`
	// User is the user requesting the silence
	User string `json:"user"`
}

// AcknowledgeRequest represents a request to acknowledge an alert
type AcknowledgeRequest struct {
	// User is the user acknowledging the alert
	User string `json:"user"`
	// Comment is an optional comment
	Comment string `json:"comment,omitempty"`
}

// ResolveRequest represents a request to resolve an alert
type ResolveRequest struct {
	// User is the user resolving the alert
	User string `json:"user"`
	// Comment is an optional comment
	Comment string `json:"comment,omitempty"`
}
