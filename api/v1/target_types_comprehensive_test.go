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

// Helper functions
func strPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}

// Test TargetType constants
func TestTargetTypeConstants(t *testing.T) {
	assert.Equal(t, TargetType("network"), TargetTypeNetwork)
	assert.Equal(t, TargetType("dns"), TargetTypeDNS)
	assert.Equal(t, TargetType("http"), TargetTypeHTTP)
	assert.Equal(t, TargetType("https"), TargetTypeHTTPS)
	assert.Equal(t, TargetType("kubernetes"), TargetTypeKubernetes)
	assert.Equal(t, TargetType("redis"), TargetTypeRedis)
	assert.Equal(t, TargetType("postgresql"), TargetTypePostgreSQL)
	assert.Equal(t, TargetType("mysql"), TargetTypeMySQL)
	assert.Equal(t, TargetType("mssql"), TargetTypeMSSQL)
	assert.Equal(t, TargetType("mongodb"), TargetTypeMongoDB)
	assert.Equal(t, TargetType("clickhouse"), TargetTypeClickHouse)
	assert.Equal(t, TargetType("elasticsearch"), TargetTypeElasticsearch)
	assert.Equal(t, TargetType("opensearch"), TargetTypeOpenSearch)
	assert.Equal(t, TargetType("minio"), TargetTypeMinIO)
	assert.Equal(t, TargetType("kafka"), TargetTypeKafka)
	assert.Equal(t, TargetType("rabbitmq"), TargetTypeRabbitMQ)
	assert.Equal(t, TargetType("keycloak"), TargetTypeKeycloak)
	assert.Equal(t, TargetType("nginx"), TargetTypeNginx)
	assert.Equal(t, TargetType("internal-canary"), TargetTypeInternalCanary)
	assert.Equal(t, TargetType("external-http"), TargetTypeExternalHTTP)
	assert.Equal(t, TargetType("node-egress"), TargetTypeNodeEgress)
	assert.Equal(t, TargetType("node-to-node"), TargetTypeNodeToNode)
}

// Test NetworkMode constants
func TestNetworkModeConstants(t *testing.T) {
	assert.Equal(t, NetworkMode("pod"), NetworkModePod)
	assert.Equal(t, NetworkMode("host"), NetworkModeHost)
}

// Test Target struct
func TestTarget_StructFields(t *testing.T) {
	target := Target{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Target",
			APIVersion: "k8swatch.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: TargetSpec{
			Type: TargetTypeHTTP,
		},
	}

	assert.Equal(t, "Target", target.Kind)
	assert.Equal(t, "k8swatch.io/v1", target.APIVersion)
	assert.Equal(t, "test-target", target.Name)
	assert.Equal(t, "default", target.Namespace)
	assert.Equal(t, "test", target.Labels["app"])
	assert.Equal(t, TargetTypeHTTP, target.Spec.Type)
}

// Test TargetSpec with all fields
func TestTargetSpec_Complete(t *testing.T) {
	spec := TargetSpec{
		Type:         TargetTypeHTTPS,
		NetworkModes: []NetworkMode{NetworkModePod, NetworkModeHost},
		Endpoint: EndpointConfig{
			DNS:  strPtr("example.com"),
			Port: int32Ptr(443),
			Path: strPtr("/health"),
		},
		Layers: LayerConfig{
			L3TLS: &TLSConfig{
				LayerConfigBase: LayerConfigBase{
					Enabled: true,
				},
			},
		},
		Schedule: ScheduleConfig{
			Interval: "30s",
			Timeout:  "10s",
			Retries:  3,
		},
		Alerting: AlertingConfig{
			CriticalityOverride: "P1",
			SuppressAlerts:      false,
		},
		Tags: map[string]string{
			"env":  "prod",
			"team": "sre",
		},
	}

	assert.Equal(t, TargetTypeHTTPS, spec.Type)
	assert.Len(t, spec.NetworkModes, 2)
	assert.Equal(t, "example.com", *spec.Endpoint.DNS)
	assert.Equal(t, int32(443), *spec.Endpoint.Port)
	assert.Equal(t, "/health", *spec.Endpoint.Path)
	assert.NotNil(t, spec.Layers.L3TLS)
	assert.Equal(t, "30s", spec.Schedule.Interval)
	assert.Equal(t, "10s", spec.Schedule.Timeout)
	assert.Equal(t, int32(3), spec.Schedule.Retries)
	assert.Equal(t, "P1", spec.Alerting.CriticalityOverride)
	assert.Equal(t, 2, len(spec.Tags))
}

// Test TargetSpec minimal
func TestTargetSpec_Minimal(t *testing.T) {
	spec := TargetSpec{
		Type: TargetTypeNetwork,
		Endpoint: EndpointConfig{
			DNS: strPtr("example.com"),
		},
	}

	assert.Equal(t, TargetTypeNetwork, spec.Type)
	assert.NotNil(t, spec.Endpoint.DNS)
	assert.Nil(t, spec.NetworkModes)
	assert.Nil(t, spec.Layers.L3TLS)
	assert.Equal(t, "", spec.Schedule.Interval)
	assert.Empty(t, spec.Tags)
}

// Test EndpointConfig with K8sService
func TestEndpointConfig_WithK8sService(t *testing.T) {
	config := EndpointConfig{
		K8sService: &K8sServiceEndpoint{
			Name:      "my-service",
			Namespace: "default",
			Port:      "8080",
		},
	}

	assert.NotNil(t, config.K8sService)
	assert.Equal(t, "my-service", config.K8sService.Name)
	assert.Equal(t, "default", config.K8sService.Namespace)
	assert.Equal(t, "8080", config.K8sService.Port)
}

// Test EndpointConfig with IP
func TestEndpointConfig_WithIP(t *testing.T) {
	config := EndpointConfig{
		IP:   strPtr("192.168.1.1"),
		Port: int32Ptr(8080),
	}

	assert.Equal(t, "192.168.1.1", *config.IP)
	assert.Equal(t, int32(8080), *config.Port)
}

// Test EndpointConfig with DNS
func TestEndpointConfig_WithDNS(t *testing.T) {
	config := EndpointConfig{
		DNS:  strPtr("example.com"),
		Port: int32Ptr(443),
		Path: strPtr("/api/health"),
	}

	assert.Equal(t, "example.com", *config.DNS)
	assert.Equal(t, int32(443), *config.Port)
	assert.Equal(t, "/api/health", *config.Path)
}

// Test K8sServiceEndpoint
func TestK8sServiceEndpoint(t *testing.T) {
	endpoint := K8sServiceEndpoint{
		Name:      "test-service",
		Namespace: "monitoring",
		Port:      "https",
	}

	assert.Equal(t, "test-service", endpoint.Name)
	assert.Equal(t, "monitoring", endpoint.Namespace)
	assert.Equal(t, "https", endpoint.Port)
}

// Test ScheduleConfig
func TestScheduleConfig(t *testing.T) {
	config := ScheduleConfig{
		Interval:       "60s",
		Timeout:        "15s",
		Retries:        5,
		RetryBackoff:   "2s",
	}

	assert.Equal(t, "60s", config.Interval)
	assert.Equal(t, "15s", config.Timeout)
	assert.Equal(t, int32(5), config.Retries)
	assert.Equal(t, "2s", config.RetryBackoff)
}

// Test ScheduleConfig defaults
func TestScheduleConfig_Defaults(t *testing.T) {
	config := ScheduleConfig{}

	assert.Equal(t, "", config.Interval)
	assert.Equal(t, "", config.Timeout)
	assert.Equal(t, int32(0), config.Retries)
	assert.Equal(t, "", config.RetryBackoff)
}

// Test AlertingConfig
func TestAlertingConfig(t *testing.T) {
	config := AlertingConfig{
		CriticalityOverride: "P0",
		NotificationChannels: []string{"slack", "pagerduty"},
		SuppressAlerts:      true,
	}

	assert.Equal(t, "P0", config.CriticalityOverride)
	assert.Len(t, config.NotificationChannels, 2)
	assert.True(t, config.SuppressAlerts)
}

// Test AlertingConfig with CustomThresholds
func TestAlertingConfig_WithCustomThresholds(t *testing.T) {
	config := AlertingConfig{
		CustomThresholds: &CustomThresholds{
			ConsecutiveFailures: 5,
			RecoverySuccesses:   3,
		},
	}

	assert.NotNil(t, config.CustomThresholds)
	assert.Equal(t, int32(5), config.CustomThresholds.ConsecutiveFailures)
	assert.Equal(t, int32(3), config.CustomThresholds.RecoverySuccesses)
}

// Test CustomThresholds
func TestCustomThresholds(t *testing.T) {
	thresholds := CustomThresholds{
		ConsecutiveFailures: 10,
		RecoverySuccesses:   5,
	}

	assert.Equal(t, int32(10), thresholds.ConsecutiveFailures)
	assert.Equal(t, int32(5), thresholds.RecoverySuccesses)
}

// Test LayerConfig with L3TLS
func TestLayerConfig_WithL3TLS(t *testing.T) {
	config := LayerConfig{
		L3TLS: &TLSConfig{
			LayerConfigBase: LayerConfigBase{
				Enabled: true,
				Timeout: "10s",
			},
			ValidationMode:     "strict",
			InsecureSkipVerify: false,
		},
	}

	assert.NotNil(t, config.L3TLS)
	assert.True(t, config.L3TLS.Enabled)
	assert.Equal(t, "10s", config.L3TLS.Timeout)
	assert.Equal(t, "strict", config.L3TLS.ValidationMode)
	assert.False(t, config.L3TLS.InsecureSkipVerify)
}

// Test LayerConfig with L4Protocol
func TestLayerConfig_WithL4Protocol(t *testing.T) {
	statusCode := int32(200)
	config := LayerConfig{
		L4Protocol: &ProtocolConfig{
			LayerConfigBase: LayerConfigBase{
				Enabled: true,
			},
			HealthQuery:      "/health",
			Method:           "GET",
			StatusCode:       &statusCode,
			ExpectedResponse: "ok",
		},
	}

	assert.NotNil(t, config.L4Protocol)
	assert.True(t, config.L4Protocol.Enabled)
	assert.Equal(t, "/health", config.L4Protocol.HealthQuery)
	assert.Equal(t, "GET", config.L4Protocol.Method)
	assert.NotNil(t, config.L4Protocol.StatusCode)
	assert.Equal(t, int32(200), *config.L4Protocol.StatusCode)
}

// Test LayerConfig with L5Auth
func TestLayerConfig_WithL5Auth(t *testing.T) {
	config := LayerConfig{
		L5Auth: &AuthConfig{
			LayerConfigBase: LayerConfigBase{
				Enabled: true,
			},
			Token: "bearer-token",
		},
	}

	assert.NotNil(t, config.L5Auth)
	assert.True(t, config.L5Auth.Enabled)
	assert.Equal(t, "bearer-token", config.L5Auth.Token)
}

// Test LayerConfig with L6Semantic
func TestLayerConfig_WithL6Semantic(t *testing.T) {
	config := LayerConfig{
		L6Semantic: &SemanticConfig{
			LayerConfigBase: LayerConfigBase{
				Enabled: true,
			},
			ExpectedContent: "success",
		},
	}

	assert.NotNil(t, config.L6Semantic)
	assert.True(t, config.L6Semantic.Enabled)
	assert.Equal(t, "success", config.L6Semantic.ExpectedContent)
}

// Test LayerConfig with L1DNS
func TestLayerConfig_WithL1DNS(t *testing.T) {
	config := LayerConfig{
		L1DNS: &LayerConfigBase{
			Enabled: true,
			Timeout: "5s",
		},
	}

	assert.NotNil(t, config.L1DNS)
	assert.True(t, config.L1DNS.Enabled)
	assert.Equal(t, "5s", config.L1DNS.Timeout)
}

// Test LayerConfig with L2TCP
func TestLayerConfig_WithL2TCP(t *testing.T) {
	config := LayerConfig{
		L2TCP: &LayerConfigBase{
			Enabled: true,
		},
	}

	assert.NotNil(t, config.L2TCP)
	assert.True(t, config.L2TCP.Enabled)
}

// Test TLSConfig
func TestTLSConfig(t *testing.T) {
	config := TLSConfig{
		LayerConfigBase: LayerConfigBase{
			Enabled: true,
		},
		ValidationMode:     "permissive",
		InsecureSkipVerify: true,
		CABundleRef: &SecretKeyRef{
			SecretName: "ca-bundle",
			Key:        "ca.crt",
		},
		ClientCertRef: &TLSCertRef{
			SecretName: "client-cert",
			CertKey:    "tls.crt",
			KeyKey:     "tls.key",
		},
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "permissive", config.ValidationMode)
	assert.True(t, config.InsecureSkipVerify)
	assert.NotNil(t, config.CABundleRef)
	assert.Equal(t, "ca-bundle", config.CABundleRef.SecretName)
	assert.NotNil(t, config.ClientCertRef)
	assert.Equal(t, "client-cert", config.ClientCertRef.SecretName)
}

// Test TLSConfig minimal
func TestTLSConfig_Minimal(t *testing.T) {
	config := TLSConfig{}

	assert.False(t, config.Enabled)
	assert.Equal(t, "", config.ValidationMode)
	assert.False(t, config.InsecureSkipVerify)
	assert.Nil(t, config.CABundleRef)
	assert.Nil(t, config.ClientCertRef)
}

// Test ProtocolConfig
func TestProtocolConfig(t *testing.T) {
	statusCode := int32(201)
	config := ProtocolConfig{
		LayerConfigBase: LayerConfigBase{
			Enabled: true,
		},
		HealthQuery:      "/api/health",
		Method:           "POST",
		StatusCode:       &statusCode,
		Body:             `{"test": "data"}`,
		Headers:          map[string]string{"Content-Type": "application/json"},
		ExpectedResponse: "success",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "/api/health", config.HealthQuery)
	assert.Equal(t, "POST", config.Method)
	assert.NotNil(t, config.StatusCode)
	assert.Equal(t, int32(201), *config.StatusCode)
	assert.Equal(t, `{"test": "data"}`, config.Body)
	assert.Equal(t, "success", config.ExpectedResponse)
}

// Test AuthConfig
func TestAuthConfig(t *testing.T) {
	config := AuthConfig{
		LayerConfigBase: LayerConfigBase{
			Enabled: true,
		},
		Token: "test-token",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "test-token", config.Token)
}

// Test SemanticConfig
func TestSemanticConfig(t *testing.T) {
	config := SemanticConfig{
		LayerConfigBase: LayerConfigBase{
			Enabled: true,
		},
		ExpectedContent: "success",
		JSONPath:        "$.status",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "success", config.ExpectedContent)
	assert.Equal(t, "$.status", config.JSONPath)
}

// Test LayerConfigBase
func TestLayerConfigBase(t *testing.T) {
	config := LayerConfigBase{
		Enabled: true,
		Timeout: "30s",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "30s", config.Timeout)
}

// Test SecretKeyRef
func TestSecretKeyRef(t *testing.T) {
	ref := SecretKeyRef{
		SecretName: "my-secret",
		Key:        "password",
	}

	assert.Equal(t, "my-secret", ref.SecretName)
	assert.Equal(t, "password", ref.Key)
}

// Test TLSCertRef
func TestTLSCertRef(t *testing.T) {
	ref := TLSCertRef{
		SecretName: "tls-secret",
		CertKey:    "cert.pem",
		KeyKey:     "key.pem",
	}

	assert.Equal(t, "tls-secret", ref.SecretName)
	assert.Equal(t, "cert.pem", ref.CertKey)
	assert.Equal(t, "key.pem", ref.KeyKey)
}

// Test NodeSanityConfig
func TestNodeSanityConfig(t *testing.T) {
	config := NodeSanityConfig{
		LayerConfigBase: LayerConfigBase{
			Enabled: true,
		},
		ClockSkew: &ClockSkewConfig{
			Enabled:   true,
			Threshold: "5s",
			NTPServer: "pool.ntp.org",
		},
		FileDescriptors: &ThresholdConfig{
			Enabled:          true,
			WarningThreshold: 80,
			CriticalThreshold: 95,
		},
	}

	assert.True(t, config.Enabled)
	assert.NotNil(t, config.ClockSkew)
	assert.True(t, config.ClockSkew.Enabled)
	assert.Equal(t, "5s", config.ClockSkew.Threshold)
	assert.NotNil(t, config.FileDescriptors)
	assert.Equal(t, int32(80), config.FileDescriptors.WarningThreshold)
}

// Test ClockSkewConfig
func TestClockSkewConfig(t *testing.T) {
	config := ClockSkewConfig{
		Enabled:   true,
		Threshold: "10s",
		NTPServer: "time.google.com",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "10s", config.Threshold)
	assert.Equal(t, "time.google.com", config.NTPServer)
}

// Test ThresholdConfig
func TestThresholdConfig(t *testing.T) {
	config := ThresholdConfig{
		Enabled:          true,
		WarningThreshold: 75,
		CriticalThreshold: 90,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, int32(75), config.WarningThreshold)
	assert.Equal(t, int32(90), config.CriticalThreshold)
}

// Test TargetStatus
func TestTargetStatus(t *testing.T) {
	now := metav1.Now()
	status := TargetStatus{
		ObservedGeneration:  1,
		LastCheckTime:       &now,
		ConsecutiveFailures: 3,
		Conditions: []TargetCondition{
			{
				Type:   TargetConditionHealthy,
				Status: metav1.ConditionTrue,
				Reason: "CheckPassed",
			},
		},
	}

	assert.Equal(t, int64(1), status.ObservedGeneration)
	assert.NotNil(t, status.LastCheckTime)
	assert.Equal(t, int32(3), status.ConsecutiveFailures)
	assert.Len(t, status.Conditions, 1)
}

// Test CheckStatus
func TestCheckStatus(t *testing.T) {
	checkStatus := CheckStatus{
		Success:        true,
		FailureLayer:   "",
		FailureCode:    "",
		FailureMessage: "",
		LatencyMs:      50,
	}

	assert.True(t, checkStatus.Success)
	assert.Empty(t, checkStatus.FailureLayer)
	assert.Equal(t, int64(50), checkStatus.LatencyMs)
}

// Test CheckStatus with failure
func TestCheckStatus_WithFailure(t *testing.T) {
	checkStatus := CheckStatus{
		Success:        false,
		FailureLayer:   "L2",
		FailureCode:    "tcp_refused",
		FailureMessage: "Connection refused",
		LatencyMs:      100,
	}

	assert.False(t, checkStatus.Success)
	assert.Equal(t, "L2", checkStatus.FailureLayer)
	assert.Equal(t, "tcp_refused", checkStatus.FailureCode)
	assert.Equal(t, "Connection refused", checkStatus.FailureMessage)
}

// Test TargetCondition
func TestTargetCondition(t *testing.T) {
	now := metav1.Now()
	condition := TargetCondition{
		Type:               TargetConditionHealthy,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "HealthCheckPassed",
		Message:            "All health checks passed",
	}

	assert.Equal(t, TargetConditionHealthy, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "HealthCheckPassed", condition.Reason)
}

// Test TargetConditionType constants
func TestTargetConditionTypeConstants(t *testing.T) {
	assert.NotEmpty(t, string(TargetConditionHealthy))
	assert.NotEmpty(t, string(TargetConditionDegraded))
	assert.NotEmpty(t, string(TargetConditionUnhealthy))
}

// Test Target with Status
func TestTarget_WithStatus(t *testing.T) {
	now := metav1.Now()
	target := Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-target",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: TargetSpec{
			Type: TargetTypeHTTP,
		},
		Status: TargetStatus{
			ObservedGeneration:  1,
			LastCheckTime:       &now,
			ConsecutiveFailures: 0,
		},
	}

	assert.Equal(t, int64(1), target.Generation)
	assert.Equal(t, int64(1), target.Status.ObservedGeneration)
	assert.Equal(t, int32(0), target.Status.ConsecutiveFailures)
}

// Test Target with Tags
func TestTarget_WithTags(t *testing.T) {
	target := Target{
		Spec: TargetSpec{
			Type: TargetTypeHTTP,
			Tags: map[string]string{
				"env":     "production",
				"team":    "platform",
				"critical": "true",
			},
		},
	}

	assert.Equal(t, 3, len(target.Spec.Tags))
	assert.Equal(t, "production", target.Spec.Tags["env"])
	assert.Equal(t, "platform", target.Spec.Tags["team"])
	assert.Equal(t, "true", target.Spec.Tags["critical"])
}

// Test Target with NetworkModes
func TestTarget_WithNetworkModes(t *testing.T) {
	target := Target{
		Spec: TargetSpec{
			Type:         TargetTypeHTTP,
			NetworkModes: []NetworkMode{NetworkModePod, NetworkModeHost},
		},
	}

	assert.Len(t, target.Spec.NetworkModes, 2)
	assert.Contains(t, target.Spec.NetworkModes, NetworkModePod)
	assert.Contains(t, target.Spec.NetworkModes, NetworkModeHost)
}

// Test Target with Multiple Layers
func TestTarget_WithMultipleLayers(t *testing.T) {
	target := Target{
		Spec: TargetSpec{
			Type: TargetTypeHTTPS,
			Layers: LayerConfig{
				L1DNS: &LayerConfigBase{
					Enabled: true,
				},
				L2TCP: &LayerConfigBase{
					Enabled: true,
				},
				L3TLS: &TLSConfig{
					LayerConfigBase: LayerConfigBase{
						Enabled: true,
					},
				},
				L4Protocol: &ProtocolConfig{
					LayerConfigBase: LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	assert.NotNil(t, target.Spec.Layers.L1DNS)
	assert.NotNil(t, target.Spec.Layers.L2TCP)
	assert.NotNil(t, target.Spec.Layers.L3TLS)
	assert.NotNil(t, target.Spec.Layers.L4Protocol)
}

// Test Target with Empty Tags
func TestTarget_WithEmptyTags(t *testing.T) {
	target := Target{
		Spec: TargetSpec{
			Type: TargetTypeHTTP,
			Tags: map[string]string{},
		},
	}

	assert.Empty(t, target.Spec.Tags)
}

// Test Target with Nil Tags
func TestTarget_WithNilTags(t *testing.T) {
	target := Target{
		Spec: TargetSpec{
			Type: TargetTypeHTTP,
		},
	}

	assert.Nil(t, target.Spec.Tags)
}

// Test Target with All TargetTypes
func TestTarget_AllTargetTypes(t *testing.T) {
	targetTypes := []TargetType{
		TargetTypeNetwork,
		TargetTypeDNS,
		TargetTypeHTTP,
		TargetTypeHTTPS,
		TargetTypeKubernetes,
		TargetTypeRedis,
		TargetTypePostgreSQL,
		TargetTypeMySQL,
		TargetTypeMSSQL,
		TargetTypeMongoDB,
		TargetTypeClickHouse,
		TargetTypeElasticsearch,
		TargetTypeOpenSearch,
		TargetTypeMinIO,
		TargetTypeKafka,
		TargetTypeRabbitMQ,
		TargetTypeKeycloak,
		TargetTypeNginx,
		TargetTypeInternalCanary,
		TargetTypeExternalHTTP,
		TargetTypeNodeEgress,
		TargetTypeNodeToNode,
	}

	for _, targetType := range targetTypes {
		t.Run(string(targetType), func(t *testing.T) {
			target := Target{
				Spec: TargetSpec{
					Type: targetType,
					Endpoint: EndpointConfig{
						DNS: strPtr("example.com"),
					},
				},
			}
			assert.Equal(t, targetType, target.Spec.Type)
		})
	}
}

// Test Target DeepCopy (generated code is excluded, but we can test the concept)
func TestTarget_Copy(t *testing.T) {
	original := Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: TargetSpec{
			Type: TargetTypeHTTP,
			Tags: map[string]string{"key": "value"},
		},
	}

	// Manual copy (simulating DeepCopy)
	copied := Target{
		ObjectMeta: original.ObjectMeta,
		Spec:       original.Spec,
	}

	assert.Equal(t, original.Name, copied.Name)
	assert.Equal(t, original.Spec.Type, copied.Spec.Type)
	// Note: Tags would be shared without proper DeepCopy
	assert.Equal(t, original.Spec.Tags, copied.Spec.Tags)
}

// Test TargetSpec DeepCopy concept
func TestTargetSpec_Copy(t *testing.T) {
	original := TargetSpec{
		Type: TargetTypeHTTP,
		Tags: map[string]string{"env": "prod"},
		Endpoint: EndpointConfig{
			DNS: strPtr("example.com"),
		},
	}

	copied := TargetSpec{
		Type:     original.Type,
		Tags:     original.Tags,
		Endpoint: original.Endpoint,
	}

	assert.Equal(t, original.Type, copied.Type)
	assert.Equal(t, original.Tags, copied.Tags)
	assert.Equal(t, *original.Endpoint.DNS, *copied.Endpoint.DNS)
}
