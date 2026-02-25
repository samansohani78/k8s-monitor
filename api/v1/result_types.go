// Package v1 contains API Schema definitions for the k8swatch.io v1 API group
// +kubebuilder:object:generate=true
// +groupName=k8swatch.io
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckResult represents the result of a health check executed by an agent
type CheckResult struct {
	// ResultID is a unique identifier for this result
	ResultID string `json:"resultId"`

	// Timestamp is when the check was completed
	Timestamp metav1.Time `json:"timestamp"`

	// Agent contains information about the agent that executed the check
	Agent AgentInfo `json:"agent"`

	// Target contains information about the target that was checked
	Target TargetInfo `json:"target"`

	// Check contains the check execution details
	Check CheckInfo `json:"check"`

	// Latencies contains per-layer latency measurements
	Latencies map[string]LayerLatency `json:"latencies,omitempty"`

	// Metadata contains additional check metadata
	Metadata CheckMetadata `json:"metadata,omitempty"`
}

// AgentInfo contains information about the agent that executed a check
type AgentInfo struct {
	// NodeName is the name of the node where the agent is running
	NodeName string `json:"nodeName"`

	// NodeZone is the availability zone of the node
	NodeZone string `json:"nodeZone,omitempty"`

	// NetworkMode is the network perspective used for the check
	NetworkMode NetworkMode `json:"networkMode"`

	// AgentVersion is the version of the agent
	AgentVersion string `json:"agentVersion"`

	// PodName is the name of the agent pod
	PodName string `json:"podName,omitempty"`
}

// TargetInfo contains information about the target that was checked
type TargetInfo struct {
	// Name is the target name
	Name string `json:"name"`

	// Namespace is the target namespace
	Namespace string `json:"namespace"`

	// Type is the target type
	Type TargetType `json:"type"`

	// Endpoint is the target endpoint that was checked
	Endpoint string `json:"endpoint,omitempty"`

	// Labels are the target labels
	Labels map[string]string `json:"labels,omitempty"`
}

// CheckInfo contains the check execution details
type CheckInfo struct {
	// LayersEnabled is the list of layers that were enabled for this check
	LayersEnabled []string `json:"layersEnabled"`

	// FinalLayer is the final layer that was executed
	FinalLayer string `json:"finalLayer"`

	// Success indicates if the check was successful
	Success bool `json:"success"`

	// FailureLayer is the layer where failure occurred (if any)
	FailureLayer string `json:"failureLayer,omitempty"`

	// FailureCode is the specific failure code
	FailureCode string `json:"failureCode,omitempty"`

	// FailureMessage is a human-readable failure message
	FailureMessage string `json:"failureMessage,omitempty"`
}

// LayerLatency contains latency measurements for a layer
type LayerLatency struct {
	// DurationMs is the layer duration in milliseconds
	DurationMs int64 `json:"duration_ms"`

	// Success indicates if the layer completed successfully
	Success bool `json:"success"`
}

// CheckMetadata contains additional check metadata
type CheckMetadata struct {
	// CheckDurationMs is the total check duration in milliseconds
	CheckDurationMs int64 `json:"checkDuration_ms"`

	// AttemptNumber is the retry attempt number (1-based)
	AttemptNumber int32 `json:"attemptNumber"`

	// ConfigVersion is the configuration version used for this check
	ConfigVersion string `json:"configVersion"`

	// Error is an error message if the check failed to complete
	Error string `json:"error,omitempty"`
}

// FailureCode represents a specific failure reason
type FailureCode string

// L0 Node Sanity Failure Codes
const (
	FailureCodeClockSkew         FailureCode = "clock_skew"
	FailureCodeFDExhausted       FailureCode = "fd_exhausted"
	FailureCodeEphemeralPortsLow FailureCode = "ephemeral_ports_low"
	FailureCodeConntrackPressure FailureCode = "conntrack_pressure"
)

// L1 DNS Failure Codes
const (
	FailureCodeDNSTimeout   FailureCode = "dns_timeout"
	FailureCodeDNSNXDomain  FailureCode = "dns_nxdomain"
	FailureCodeDNSServFail  FailureCode = "dns_servfail"
	FailureCodeDNSRefused   FailureCode = "dns_refused"
	FailureCodeDNSNoServers FailureCode = "dns_no_servers"
)

// L2 TCP Failure Codes
const (
	FailureCodeTCPTimeout  FailureCode = "tcp_timeout"
	FailureCodeTCPRefused  FailureCode = "tcp_refused"
	FailureCodeTCPReset    FailureCode = "tcp_reset"
	FailureCodeTCPNoRoute  FailureCode = "tcp_no_route"
	FailureCodeMTUExceeded FailureCode = "mtu_exceeded"
)

// L3 TLS Failure Codes
const (
	FailureCodeTLSCertExpired     FailureCode = "tls_expired"
	FailureCodeTLSCertNotYetValid FailureCode = "tls_not_yet_valid"
	FailureCodeTLSWrongHost       FailureCode = "tls_wrong_host"
	FailureCodeTLSUntrustedIssuer FailureCode = "tls_untrusted_issuer"
	FailureCodeTLSHandshakeFailed FailureCode = "tls_handshake_failed"
	FailureCodeTLSCertRevoked     FailureCode = "tls_cert_revoked"
)

// L4 Protocol Failure Codes
const (
	FailureCodeProtocolTimeout        FailureCode = "protocol_timeout"
	FailureCodeProtocolError          FailureCode = "protocol_error"
	FailureCodeProtocolUnexpectedResp FailureCode = "protocol_unexpected_response"
)

// L5 Auth Failure Codes
const (
	FailureCodeAuthFailed         FailureCode = "auth_failed"
	FailureCodeAuthTimeout        FailureCode = "auth_timeout"
	FailureCodeAuthUnauthorized   FailureCode = "auth_unauthorized"
	FailureCodeCredentialsExpired FailureCode = "credentials_expired"
)

// L6 Semantic Failure Codes
const (
	FailureCodeSemanticFailed     FailureCode = "semantic_failed"
	FailureCodeSemanticTimeout    FailureCode = "semantic_timeout"
	FailureCodeSemanticUnexpected FailureCode = "semantic_unexpected"
)

// Generic Failure Codes
const (
	FailureCodeUnknown       FailureCode = "unknown"
	FailureCodeTimeout       FailureCode = "timeout"
	FailureCodeInternalError FailureCode = "internal_error"
	FailureCodeConfigError   FailureCode = "config_error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AlertEvent represents an alert event generated by the aggregator
type AlertEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlertEventSpec   `json:"spec,omitempty"`
	Status AlertEventStatus `json:"status,omitempty"`
}

// AlertEventSpec defines the desired state of an AlertEvent
type AlertEventSpec struct {
	// AlertID is a unique identifier for this alert
	AlertID string `json:"alertId"`

	// Rule is the name of the AlertRule that triggered this alert
	Rule string `json:"rule"`

	// Target is the target that triggered the alert
	Target TargetRef `json:"target"`

	// Severity is the alert severity
	// +kubebuilder:validation:Enum=info;warning;critical
	Severity string `json:"severity"`

	// Status is the alert status
	// +kubebuilder:validation:Enum=firing;acknowledged;resolved;silenced
	Status string `json:"status"`

	// FiredAt is when the alert was fired
	FiredAt metav1.Time `json:"firedAt"`

	// ResolvedAt is when the alert was resolved (if applicable)
	ResolvedAt *metav1.Time `json:"resolvedAt,omitempty"`

	// FailureLayer is the layer where failure occurred
	FailureLayer string `json:"failureLayer,omitempty"`

	// FailureCode is the specific failure code
	FailureCode string `json:"failureCode,omitempty"`

	// BlastRadius is the blast radius classification
	BlastRadius BlastRadiusType `json:"blastRadius"`

	// AffectedNodes is the list of affected node names
	AffectedNodes []string `json:"affectedNodes,omitempty"`

	// Evidence contains evidence that triggered the alert
	Evidence AlertEvidence `json:"evidence,omitempty"`

	// Annotations are additional key-value metadata
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TargetRef references a Target
type TargetRef struct {
	// Name is the target name
	Name string `json:"name"`

	// Namespace is the target namespace
	Namespace string `json:"namespace"`

	// Type is the target type
	Type TargetType `json:"type"`
}

// AlertEvidence contains evidence that triggered an alert
type AlertEvidence struct {
	// FailedChecks is a list of check results that triggered the alert
	FailedChecks []CheckResultRef `json:"failedChecks,omitempty"`

	// ConsecutiveFailures is the number of consecutive failures
	ConsecutiveFailures int32 `json:"consecutiveFailures"`

	// AffectedNodeCount is the number of affected nodes
	AffectedNodeCount int32 `json:"affectedNodeCount"`

	// AffectedZoneCount is the number of affected zones
	AffectedZoneCount int32 `json:"affectedZoneCount"`
}

// CheckResultRef references a CheckResult
type CheckResultRef struct {
	// ResultID is the result ID
	ResultID string `json:"resultId"`

	// Timestamp is when the check was completed
	Timestamp metav1.Time `json:"timestamp"`

	// Node is the node that executed the check
	Node string `json:"node"`

	// FailureLayer is the layer where failure occurred
	FailureLayer string `json:"failureLayer,omitempty"`

	// FailureCode is the specific failure code
	FailureCode string `json:"failureCode,omitempty"`
}

// AlertEventStatus defines the observed state of an AlertEvent
type AlertEventStatus struct {
	// ObservedGeneration is the last observed generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// NotificationsSent is the number of notifications sent for this alert
	NotificationsSent int32 `json:"notificationsSent,omitempty"`

	// LastNotificationAt is when the last notification was sent
	LastNotificationAt *metav1.Time `json:"lastNotificationAt,omitempty"`

	// AcknowledgedBy is the user who acknowledged the alert
	AcknowledgedBy string `json:"acknowledgedBy,omitempty"`

	// AcknowledgedAt is when the alert was acknowledged
	AcknowledgedAt *metav1.Time `json:"acknowledgedAt,omitempty"`

	// SilencedBy is the user who silenced the alert
	SilencedBy string `json:"silencedBy,omitempty"`

	// SilencedAt is when the alert was silenced
	SilencedAt *metav1.Time `json:"silencedAt,omitempty"`

	// SilenceReason is the reason for silencing the alert
	SilenceReason string `json:"silenceReason,omitempty"`
}

// +kubebuilder:object:root=true

// AlertEventList contains a list of AlertEvent
type AlertEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertEvent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AlertEvent{}, &AlertEventList{})
}
