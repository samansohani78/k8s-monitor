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

// Test CheckResult struct
func TestCheckResult_StructFields(t *testing.T) {
	timestamp := metav1.NewTime(time.Now())
	result := CheckResult{
		ResultID:  "result-123",
		Timestamp: timestamp,
		Agent: AgentInfo{
			NodeName:     "node-1",
			NodeZone:     "us-east-1a",
			NetworkMode:  NetworkModePod,
			AgentVersion: "1.0.0",
			PodName:      "agent-abc123",
		},
		Target: TargetInfo{
			Name:      "test-target",
			Namespace: "default",
			Type:      TargetTypeHTTP,
			Endpoint:  "http://example.com",
			Labels:    map[string]string{"env": "prod"},
		},
		Check: CheckInfo{
			LayersEnabled:  []string{"L0", "L1", "L2"},
			FinalLayer:     "L2",
			Success:        true,
			FailureLayer:   "",
			FailureCode:    "",
			FailureMessage: "",
		},
		Latencies: map[string]LayerLatency{
			"L0": {DurationMs: 10, Success: true},
			"L1": {DurationMs: 25, Success: true},
			"L2": {DurationMs: 50, Success: true},
		},
		Metadata: CheckMetadata{
			CheckDurationMs: 100,
			AttemptNumber:   1,
			ConfigVersion:   "v1",
			Error:           "",
		},
	}

	assert.Equal(t, "result-123", result.ResultID)
	assert.Equal(t, timestamp, result.Timestamp)
	assert.Equal(t, "node-1", result.Agent.NodeName)
	assert.Equal(t, "us-east-1a", result.Agent.NodeZone)
	assert.Equal(t, NetworkModePod, result.Agent.NetworkMode)
	assert.Equal(t, "1.0.0", result.Agent.AgentVersion)
	assert.Equal(t, "agent-abc123", result.Agent.PodName)
	assert.Equal(t, "test-target", result.Target.Name)
	assert.Equal(t, "default", result.Target.Namespace)
	assert.Equal(t, TargetTypeHTTP, result.Target.Type)
	assert.Equal(t, "http://example.com", result.Target.Endpoint)
	assert.Equal(t, "prod", result.Target.Labels["env"])
	assert.Equal(t, []string{"L0", "L1", "L2"}, result.Check.LayersEnabled)
	assert.Equal(t, "L2", result.Check.FinalLayer)
	assert.True(t, result.Check.Success)
	assert.Equal(t, 3, len(result.Latencies))
	assert.Equal(t, int64(100), result.Metadata.CheckDurationMs)
	assert.Equal(t, int32(1), result.Metadata.AttemptNumber)
}

// Test CheckResult with failure
func TestCheckResult_WithFailure(t *testing.T) {
	result := CheckResult{
		ResultID: "result-fail-456",
		Check: CheckInfo{
			LayersEnabled:  []string{"L0", "L1", "L2", "L3"},
			FinalLayer:     "L3",
			Success:        false,
			FailureLayer:   "L3",
			FailureCode:    string(FailureCodeTLSCertExpired),
			FailureMessage: "Certificate has expired",
		},
		Latencies: map[string]LayerLatency{
			"L0": {DurationMs: 5, Success: true},
			"L1": {DurationMs: 20, Success: true},
			"L2": {DurationMs: 40, Success: true},
			"L3": {DurationMs: 0, Success: false},
		},
	}

	assert.False(t, result.Check.Success)
	assert.Equal(t, "L3", result.Check.FailureLayer)
	assert.Equal(t, "tls_expired", result.Check.FailureCode)
	assert.Equal(t, "Certificate has expired", result.Check.FailureMessage)
}

// Test AgentInfo struct
func TestAgentInfo_StructFields(t *testing.T) {
	agent := AgentInfo{
		NodeName:     "worker-node-1",
		NodeZone:     "us-west-2b",
		NetworkMode:  NetworkModeHost,
		AgentVersion: "2.0.0",
		PodName:      "agent-xyz789",
	}

	assert.Equal(t, "worker-node-1", agent.NodeName)
	assert.Equal(t, "us-west-2b", agent.NodeZone)
	assert.Equal(t, NetworkModeHost, agent.NetworkMode)
	assert.Equal(t, "2.0.0", agent.AgentVersion)
	assert.Equal(t, "agent-xyz789", agent.PodName)
}

// Test TargetInfo struct
func TestTargetInfo_StructFields(t *testing.T) {
	target := TargetInfo{
		Name:      "postgres-primary",
		Namespace: "database",
		Type:      TargetTypePostgreSQL,
		Endpoint:  "postgres.default.svc:5432",
		Labels: map[string]string{
			"app":     "postgres",
			"tier":    "database",
			"version": "15.0",
		},
	}

	assert.Equal(t, "postgres-primary", target.Name)
	assert.Equal(t, "database", target.Namespace)
	assert.Equal(t, TargetTypePostgreSQL, target.Type)
	assert.Equal(t, "postgres.default.svc:5432", target.Endpoint)
	assert.Equal(t, "postgres", target.Labels["app"])
	assert.Equal(t, "database", target.Labels["tier"])
	assert.Equal(t, "15.0", target.Labels["version"])
}

// Test CheckInfo struct
func TestCheckInfo_StructFields(t *testing.T) {
	check := CheckInfo{
		LayersEnabled:  []string{"L0", "L1", "L2", "L3", "L4"},
		FinalLayer:     "L4",
		Success:        true,
		FailureLayer:   "",
		FailureCode:    "",
		FailureMessage: "",
	}

	assert.Equal(t, 5, len(check.LayersEnabled))
	assert.Equal(t, "L4", check.FinalLayer)
	assert.True(t, check.Success)
	assert.Empty(t, check.FailureLayer)
	assert.Empty(t, check.FailureCode)
	assert.Empty(t, check.FailureMessage)
}

// Test LayerLatency struct
func TestLayerLatency_StructFields(t *testing.T) {
	latency := LayerLatency{
		DurationMs: 150,
		Success:    true,
	}

	assert.Equal(t, int64(150), latency.DurationMs)
	assert.True(t, latency.Success)
}

// Test CheckMetadata struct
func TestCheckMetadata_StructFields(t *testing.T) {
	metadata := CheckMetadata{
		CheckDurationMs: 500,
		AttemptNumber:   3,
		ConfigVersion:   "v2",
		Error:           "connection timeout",
	}

	assert.Equal(t, int64(500), metadata.CheckDurationMs)
	assert.Equal(t, int32(3), metadata.AttemptNumber)
	assert.Equal(t, "v2", metadata.ConfigVersion)
	assert.Equal(t, "connection timeout", metadata.Error)
}

// Test FailureCode constants - L0 Node Sanity
func TestFailureCodeConstants_L0(t *testing.T) {
	assert.Equal(t, FailureCode("clock_skew"), FailureCodeClockSkew)
	assert.Equal(t, FailureCode("fd_exhausted"), FailureCodeFDExhausted)
	assert.Equal(t, FailureCode("ephemeral_ports_low"), FailureCodeEphemeralPortsLow)
	assert.Equal(t, FailureCode("conntrack_pressure"), FailureCodeConntrackPressure)
}

// Test FailureCode constants - L1 DNS
func TestFailureCodeConstants_L1(t *testing.T) {
	assert.Equal(t, FailureCode("dns_timeout"), FailureCodeDNSTimeout)
	assert.Equal(t, FailureCode("dns_nxdomain"), FailureCodeDNSNXDomain)
	assert.Equal(t, FailureCode("dns_servfail"), FailureCodeDNSServFail)
	assert.Equal(t, FailureCode("dns_refused"), FailureCodeDNSRefused)
	assert.Equal(t, FailureCode("dns_no_servers"), FailureCodeDNSNoServers)
}

// Test FailureCode constants - L2 TCP
func TestFailureCodeConstants_L2(t *testing.T) {
	assert.Equal(t, FailureCode("tcp_timeout"), FailureCodeTCPTimeout)
	assert.Equal(t, FailureCode("tcp_refused"), FailureCodeTCPRefused)
	assert.Equal(t, FailureCode("tcp_reset"), FailureCodeTCPReset)
	assert.Equal(t, FailureCode("tcp_no_route"), FailureCodeTCPNoRoute)
	assert.Equal(t, FailureCode("mtu_exceeded"), FailureCodeMTUExceeded)
}

// Test FailureCode constants - L3 TLS
func TestFailureCodeConstants_L3(t *testing.T) {
	assert.Equal(t, FailureCode("tls_expired"), FailureCodeTLSCertExpired)
	assert.Equal(t, FailureCode("tls_not_yet_valid"), FailureCodeTLSCertNotYetValid)
	assert.Equal(t, FailureCode("tls_wrong_host"), FailureCodeTLSWrongHost)
	assert.Equal(t, FailureCode("tls_untrusted_issuer"), FailureCodeTLSUntrustedIssuer)
	assert.Equal(t, FailureCode("tls_handshake_failed"), FailureCodeTLSHandshakeFailed)
	assert.Equal(t, FailureCode("tls_cert_revoked"), FailureCodeTLSCertRevoked)
}

// Test FailureCode constants - L4 Protocol
func TestFailureCodeConstants_L4(t *testing.T) {
	assert.Equal(t, FailureCode("protocol_timeout"), FailureCodeProtocolTimeout)
	assert.Equal(t, FailureCode("protocol_error"), FailureCodeProtocolError)
	assert.Equal(t, FailureCode("protocol_unexpected_response"), FailureCodeProtocolUnexpectedResp)
}

// Test FailureCode constants - L5 Auth
func TestFailureCodeConstants_L5(t *testing.T) {
	assert.Equal(t, FailureCode("auth_failed"), FailureCodeAuthFailed)
	assert.Equal(t, FailureCode("auth_timeout"), FailureCodeAuthTimeout)
	assert.Equal(t, FailureCode("auth_unauthorized"), FailureCodeAuthUnauthorized)
	assert.Equal(t, FailureCode("credentials_expired"), FailureCodeCredentialsExpired)
}

// Test FailureCode constants - L6 Semantic
func TestFailureCodeConstants_L6(t *testing.T) {
	assert.Equal(t, FailureCode("semantic_failed"), FailureCodeSemanticFailed)
	assert.Equal(t, FailureCode("semantic_timeout"), FailureCodeSemanticTimeout)
	assert.Equal(t, FailureCode("semantic_unexpected"), FailureCodeSemanticUnexpected)
}

// Test FailureCode constants - Generic
func TestFailureCodeConstants_Generic(t *testing.T) {
	assert.Equal(t, FailureCode("unknown"), FailureCodeUnknown)
	assert.Equal(t, FailureCode("timeout"), FailureCodeTimeout)
	assert.Equal(t, FailureCode("internal_error"), FailureCodeInternalError)
	assert.Equal(t, FailureCode("config_error"), FailureCodeConfigError)
}

// Test AlertEvent struct
func TestAlertEvent_StructFields(t *testing.T) {
	firedAt := metav1.NewTime(time.Now())
	event := AlertEvent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AlertEvent",
			APIVersion: "k8swatch.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alert-event-123",
			Namespace: "default",
		},
		Spec: AlertEventSpec{
			AlertID:  "alert-123",
			Rule:     "high-error-rate",
			Target:   TargetRef{Name: "api-gateway", Namespace: "default", Type: TargetTypeHTTP},
			Severity: "critical",
			Status:   "firing",
			FiredAt:  firedAt,
			BlastRadius: BlastRadiusCluster,
			AffectedNodes: []string{"node-1", "node-2", "node-3"},
			Evidence: AlertEvidence{
				FailedChecks: []CheckResultRef{
					{
						ResultID:     "result-1",
						Timestamp:    firedAt,
						Node:         "node-1",
						FailureLayer: "L2",
						FailureCode:  "tcp_refused",
					},
				},
				ConsecutiveFailures: 5,
				AffectedNodeCount:   3,
				AffectedZoneCount:   1,
			},
			Annotations: map[string]string{
				"summary": "API Gateway unreachable from multiple nodes",
			},
		},
		Status: AlertEventStatus{
			ObservedGeneration:  1,
			NotificationsSent:   2,
			LastNotificationAt:  &firedAt,
			AcknowledgedBy:      "",
			AcknowledgedAt:      nil,
			SilencedBy:          "",
			SilencedAt:          nil,
			SilenceReason:       "",
		},
	}

	assert.Equal(t, "AlertEvent", event.Kind)
	assert.Equal(t, "k8swatch.io/v1", event.APIVersion)
	assert.Equal(t, "alert-event-123", event.Name)
	assert.Equal(t, "alert-123", event.Spec.AlertID)
	assert.Equal(t, "high-error-rate", event.Spec.Rule)
	assert.Equal(t, "api-gateway", event.Spec.Target.Name)
	assert.Equal(t, "critical", event.Spec.Severity)
	assert.Equal(t, "firing", event.Spec.Status)
	assert.Equal(t, BlastRadiusCluster, event.Spec.BlastRadius)
	assert.Equal(t, 3, len(event.Spec.AffectedNodes))
	assert.Equal(t, int32(5), event.Spec.Evidence.ConsecutiveFailures)
	assert.Equal(t, int32(3), event.Spec.Evidence.AffectedNodeCount)
	assert.Equal(t, int32(1), event.Spec.Evidence.AffectedZoneCount)
	assert.Equal(t, int32(2), event.Status.NotificationsSent)
}

// Test AlertEvent with resolved status
func TestAlertEvent_Resolved(t *testing.T) {
	firedAt := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	resolvedAt := metav1.NewTime(time.Now())
	acknowledgedAt := metav1.NewTime(time.Now().Add(-5 * time.Minute))

	event := AlertEvent{
		Spec: AlertEventSpec{
			AlertID:    "alert-456",
			Status:     "acknowledged",
			FiredAt:    firedAt,
			ResolvedAt: &resolvedAt,
		},
		Status: AlertEventStatus{
			AcknowledgedBy: "oncall-engineer",
			AcknowledgedAt: &acknowledgedAt,
		},
	}

	assert.Equal(t, "acknowledged", event.Spec.Status)
	assert.NotNil(t, event.Spec.ResolvedAt)
	assert.Equal(t, "oncall-engineer", event.Status.AcknowledgedBy)
	assert.NotNil(t, event.Status.AcknowledgedAt)
}

// Test AlertEvent with silenced status
func TestAlertEvent_Silenced(t *testing.T) {
	firedAt := metav1.NewTime(time.Now())
	silencedAt := metav1.NewTime(time.Now())

	event := AlertEvent{
		Spec: AlertEventSpec{
			AlertID:  "alert-789",
			Status:   "silenced",
			FiredAt:  firedAt,
			Severity: "warning",
		},
		Status: AlertEventStatus{
			SilencedBy:    "admin",
			SilencedAt:    &silencedAt,
			SilenceReason: "Scheduled maintenance window",
		},
	}

	assert.Equal(t, "silenced", event.Spec.Status)
	assert.Equal(t, "warning", event.Spec.Severity)
	assert.Equal(t, "admin", event.Status.SilencedBy)
	assert.Equal(t, "Scheduled maintenance window", event.Status.SilenceReason)
}

// Test TargetRef struct
func TestTargetRef_StructFields(t *testing.T) {
	ref := TargetRef{
		Name:      "redis-cache",
		Namespace: "cache",
		Type:      TargetTypeRedis,
	}

	assert.Equal(t, "redis-cache", ref.Name)
	assert.Equal(t, "cache", ref.Namespace)
	assert.Equal(t, TargetTypeRedis, ref.Type)
}

// Test AlertEvidence struct
func TestAlertEvidence_StructFields(t *testing.T) {
	timestamp := metav1.NewTime(time.Now())
	evidence := AlertEvidence{
		FailedChecks: []CheckResultRef{
			{
				ResultID:     "result-abc",
				Timestamp:    timestamp,
				Node:         "node-1",
				FailureLayer: "L1",
				FailureCode:  "dns_timeout",
			},
			{
				ResultID:     "result-def",
				Timestamp:    timestamp,
				Node:         "node-2",
				FailureLayer: "L1",
				FailureCode:  "dns_timeout",
			},
		},
		ConsecutiveFailures: 10,
		AffectedNodeCount:   5,
		AffectedZoneCount:   2,
	}

	assert.Equal(t, 2, len(evidence.FailedChecks))
	assert.Equal(t, "result-abc", evidence.FailedChecks[0].ResultID)
	assert.Equal(t, "node-1", evidence.FailedChecks[0].Node)
	assert.Equal(t, "dns_timeout", evidence.FailedChecks[0].FailureCode)
	assert.Equal(t, int32(10), evidence.ConsecutiveFailures)
	assert.Equal(t, int32(5), evidence.AffectedNodeCount)
	assert.Equal(t, int32(2), evidence.AffectedZoneCount)
}

// Test CheckResultRef struct
func TestCheckResultRef_StructFields(t *testing.T) {
	timestamp := metav1.NewTime(time.Now())
	ref := CheckResultRef{
		ResultID:     "result-xyz",
		Timestamp:    timestamp,
		Node:         "worker-3",
		FailureLayer: "L4",
		FailureCode:  "protocol_timeout",
	}

	assert.Equal(t, "result-xyz", ref.ResultID)
	assert.Equal(t, "worker-3", ref.Node)
	assert.Equal(t, "L4", ref.FailureLayer)
	assert.Equal(t, "protocol_timeout", ref.FailureCode)
}

// Test AlertEventList struct
func TestAlertEventList_StructFields(t *testing.T) {
	list := AlertEventList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AlertEventList",
			APIVersion: "k8swatch.io/v1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "12345",
			Continue:        "",
		},
		Items: []AlertEvent{
			{
				Spec: AlertEventSpec{
					AlertID:  "alert-1",
					Severity: "critical",
				},
			},
			{
				Spec: AlertEventSpec{
					AlertID:  "alert-2",
					Severity: "warning",
				},
			},
		},
	}

	assert.Equal(t, "AlertEventList", list.Kind)
	assert.Equal(t, "k8swatch.io/v1", list.APIVersion)
	assert.Equal(t, "12345", list.ResourceVersion)
	assert.Equal(t, 2, len(list.Items))
	assert.Equal(t, "alert-1", list.Items[0].Spec.AlertID)
	assert.Equal(t, "alert-2", list.Items[1].Spec.AlertID)
}


// Test AlertEventStatus with all fields populated
func TestAlertEventStatus_Complete(t *testing.T) {
	now := metav1.NewTime(time.Now())
	status := AlertEventStatus{
		ObservedGeneration:  5,
		NotificationsSent:   10,
		LastNotificationAt:  &now,
		AcknowledgedBy:      "engineer-1",
		AcknowledgedAt:      &now,
		SilencedBy:          "",
		SilencedAt:          nil,
		SilenceReason:       "",
	}

	assert.Equal(t, int64(5), status.ObservedGeneration)
	assert.Equal(t, int32(10), status.NotificationsSent)
	assert.NotNil(t, status.LastNotificationAt)
	assert.Equal(t, "engineer-1", status.AcknowledgedBy)
	assert.NotNil(t, status.AcknowledgedAt)
	assert.Empty(t, status.SilencedBy)
	assert.Nil(t, status.SilencedAt)
	assert.Empty(t, status.SilenceReason)
}

// Test CheckMetadata with error
func TestCheckMetadata_WithError(t *testing.T) {
	metadata := CheckMetadata{
		CheckDurationMs: 30000,
		AttemptNumber:   5,
		ConfigVersion:   "v1",
		Error:           "context deadline exceeded",
	}

	assert.Equal(t, int64(30000), metadata.CheckDurationMs)
	assert.Equal(t, int32(5), metadata.AttemptNumber)
	assert.Equal(t, "context deadline exceeded", metadata.Error)
}

// Test LayerLatency with failure
func TestLayerLatency_WithFailure(t *testing.T) {
	latency := LayerLatency{
		DurationMs: 0,
		Success:    false,
	}

	assert.Equal(t, int64(0), latency.DurationMs)
	assert.False(t, latency.Success)
}

// Test CheckInfo with all layers
func TestCheckInfo_AllLayers(t *testing.T) {
	check := CheckInfo{
		LayersEnabled:  []string{"L0", "L1", "L2", "L3", "L4", "L5", "L6"},
		FinalLayer:     "L6",
		Success:        true,
		FailureLayer:   "",
		FailureCode:    "",
		FailureMessage: "",
	}

	assert.Equal(t, 7, len(check.LayersEnabled))
	assert.Equal(t, "L6", check.FinalLayer)
	assert.True(t, check.Success)
}

// Test AgentInfo without optional fields
func TestAgentInfo_Minimal(t *testing.T) {
	agent := AgentInfo{
		NodeName:     "node-minimal",
		NetworkMode:  NetworkModePod,
		AgentVersion: "1.0.0",
	}

	assert.Equal(t, "node-minimal", agent.NodeName)
	assert.Empty(t, agent.NodeZone)
	assert.Equal(t, NetworkModePod, agent.NetworkMode)
	assert.Equal(t, "1.0.0", agent.AgentVersion)
	assert.Empty(t, agent.PodName)
}

// Test TargetInfo without optional fields
func TestTargetInfo_Minimal(t *testing.T) {
	target := TargetInfo{
		Name:      "minimal-target",
		Namespace: "default",
		Type:      TargetTypeNetwork,
	}

	assert.Equal(t, "minimal-target", target.Name)
	assert.Equal(t, "default", target.Namespace)
	assert.Equal(t, TargetTypeNetwork, target.Type)
	assert.Empty(t, target.Endpoint)
	assert.Nil(t, target.Labels)
}

// Test AlertEvidence without failed checks
func TestAlertEvidence_Minimal(t *testing.T) {
	evidence := AlertEvidence{
		ConsecutiveFailures: 3,
		AffectedNodeCount:   1,
		AffectedZoneCount:   1,
	}

	assert.Nil(t, evidence.FailedChecks)
	assert.Equal(t, int32(3), evidence.ConsecutiveFailures)
	assert.Equal(t, int32(1), evidence.AffectedNodeCount)
	assert.Equal(t, int32(1), evidence.AffectedZoneCount)
}

// Test CheckResultRef without optional fields
func TestCheckResultRef_Minimal(t *testing.T) {
	ref := CheckResultRef{
		ResultID:  "result-minimal",
		Timestamp: metav1.Time{},
		Node:      "node-minimal",
	}

	assert.Equal(t, "result-minimal", ref.ResultID)
	assert.Equal(t, "node-minimal", ref.Node)
	assert.Empty(t, ref.FailureLayer)
	assert.Empty(t, ref.FailureCode)
}
