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

package checker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

func TestNetworkCheckerCreation(t *testing.T) {
	factory := &NetworkCheckerFactory{}
	checker, err := factory.Create(&k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 3) // L0, L1, L2
}

func TestNetworkCheckerFactorySupportedTypes(t *testing.T) {
	factory := &NetworkCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "network")
}

func TestDNSLayerName(t *testing.T) {
	layer := NewDNSLayer()
	assert.Equal(t, "L1", layer.Name())
}

func TestDNSLayerEnabled(t *testing.T) {
	layer := NewDNSLayer()

	// Disabled by default
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}
	assert.False(t, layer.Enabled(target))

	// Enabled when configured
	target.Spec.Layers.L1DNS = &k8swatchv1.LayerConfigBase{Enabled: true}
	assert.True(t, layer.Enabled(target))
}

func TestDNSLayerGetHostname(t *testing.T) {
	layer := NewDNSLayer()

	tests := []struct {
		name     string
		target   *k8swatchv1.Target
		expected string
	}{
		{
			name: "DNS endpoint",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						DNS: strPtr("google.com"),
					},
				},
			},
			expected: "google.com",
		},
		{
			name: "K8s service endpoint",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						K8sService: &k8swatchv1.K8sServiceEndpoint{
							Name:      "kubernetes",
							Namespace: "default",
							Port:      "443",
						},
					},
				},
			},
			expected: "kubernetes.default.svc.cluster.local",
		},
		{
			name: "K8s service with empty namespace",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						K8sService: &k8swatchv1.K8sServiceEndpoint{
							Name: "kubernetes",
							Port: "443",
						},
					},
				},
			},
			expected: "kubernetes.default.svc.cluster.local",
		},
		{
			name: "IP endpoint",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						IP:   strPtr("8.8.8.8"),
						Port: int32Ptr(53),
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := layer.getHostname(tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDNSLayerCheckWithDNS(t *testing.T) {
	layer := NewDNSLayer()

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dns",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Layers: k8swatchv1.LayerConfig{
				L1DNS: &k8swatchv1.LayerConfigBase{
					Enabled: true,
				},
			},
		},
	}

	ctx := context.Background()
	result, err := layer.Check(ctx, target)

	assert.NoError(t, err)
	// Note: This may fail in environments without DNS
	// In production, use cluster DNS
	t.Logf("DNS check result: success=%v, duration=%dms", result.Success, result.DurationMs)
}

func TestDNSLayerHandleDNSError(t *testing.T) {
	layer := NewDNSLayer()

	tests := []struct {
		name          string
		err           error
		expectedCode  string
		expectedMatch string
	}{
		{
			name:          "NXDOMAIN",
			err:           &netError{msg: "no such host"},
			expectedCode:  string(k8swatchv1.FailureCodeDNSNXDomain),
			expectedMatch: "NXDOMAIN",
		},
		{
			name:          "SERVFAIL",
			err:           &netError{msg: "server misbehaving"},
			expectedCode:  string(k8swatchv1.FailureCodeDNSServFail),
			expectedMatch: "SERVFAIL",
		},
		{
			name:          "Connection refused",
			err:           &netError{msg: "connection refused"},
			expectedCode:  string(k8swatchv1.FailureCodeDNSRefused),
			expectedMatch: "refused",
		},
		{
			name:          "Timeout",
			err:           &netError{msg: "timeout"},
			expectedCode:  string(k8swatchv1.FailureCodeDNSTimeout),
			expectedMatch: "timeout",
		},
		{
			name:          "No servers",
			err:           &netError{msg: "no servers"},
			expectedCode:  string(k8swatchv1.FailureCodeDNSNoServers),
			expectedMatch: "No DNS servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := layer.handleDNSError(tt.err)
			assert.False(t, result.Success)
			assert.Equal(t, tt.expectedCode, result.FailureCode)
			assert.Contains(t, result.FailureMessage, tt.expectedMatch)
		})
	}
}

func TestTCPLayerName(t *testing.T) {
	layer := NewTCPLayer()
	assert.Equal(t, "L2", layer.Name())
}

func TestTCPLayerEnabled(t *testing.T) {
	layer := NewTCPLayer()

	// Disabled by default
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}
	assert.False(t, layer.Enabled(target))

	// Enabled when configured
	target.Spec.Layers.L2TCP = &k8swatchv1.LayerConfigBase{Enabled: true}
	assert.True(t, layer.Enabled(target))
}

func TestTCPLayerGetTargetAddress(t *testing.T) {
	layer := NewTCPLayer()

	tests := []struct {
		name        string
		target      *k8swatchv1.Target
		expectError bool
		expected    string
	}{
		{
			name: "DNS endpoint with port",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						DNS:  strPtr("example.com"),
						Port: int32Ptr(8080),
					},
				},
			},
			expectError: false,
			expected:    "example.com:8080",
		},
		{
			name: "IP endpoint with port",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						IP:   strPtr("192.168.1.1"),
						Port: int32Ptr(443),
					},
				},
			},
			expectError: false,
			expected:    "192.168.1.1:443",
		},
		{
			name: "K8s service endpoint",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						K8sService: &k8swatchv1.K8sServiceEndpoint{
							Name:      "my-service",
							Namespace: "my-ns",
							Port:      "80",
						},
					},
				},
			},
			expectError: false,
			expected:    "my-service.my-ns.svc.cluster.local:80",
		},
		{
			name: "No endpoint",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, err := layer.getTargetAddress(tt.target)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, address)
			}
		})
	}
}

func TestTCPLayerGetPort(t *testing.T) {
	layer := NewTCPLayer()

	tests := []struct {
		name     string
		target   *k8swatchv1.Target
		expected int
	}{
		{
			name: "Explicit port",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						Port: int32Ptr(9999),
					},
				},
			},
			expected: 9999,
		},
		{
			name: "HTTP default",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeHTTP,
				},
			},
			expected: 80,
		},
		{
			name: "HTTPS default",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeHTTPS,
				},
			},
			expected: 443,
		},
		{
			name: "PostgreSQL default",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypePostgreSQL,
				},
			},
			expected: 5432,
		},
		{
			name: "Redis default",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeRedis,
				},
			},
			expected: 6379,
		},
		{
			name: "MongoDB default",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeMongoDB,
				},
			},
			expected: 27017,
		},
		{
			name: "Kafka default",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeKafka,
				},
			},
			expected: 9092,
		},
		{
			name: "Unknown default",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: "unknown",
				},
			},
			expected: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := layer.getPort(tt.target)
			assert.Equal(t, tt.expected, port)
		})
	}
}

func TestTCPLayerHandleTCPError(t *testing.T) {
	layer := NewTCPLayer()

	tests := []struct {
		name          string
		err           error
		expectedCode  string
		expectedMatch string
	}{
		{
			name:          "Connection refused",
			err:           &netError{msg: "connection refused"},
			expectedCode:  string(k8swatchv1.FailureCodeTCPRefused),
			expectedMatch: "refused",
		},
		{
			name:          "No route to host",
			err:           &netError{msg: "no route to host"},
			expectedCode:  string(k8swatchv1.FailureCodeTCPNoRoute),
			expectedMatch: "no route",
		},
		{
			name:          "Network unreachable",
			err:           &netError{msg: "network is unreachable"},
			expectedCode:  string(k8swatchv1.FailureCodeTCPNoRoute),
			expectedMatch: "unreachable",
		},
		{
			name:          "Timeout",
			err:           &netError{msg: "timeout"},
			expectedCode:  string(k8swatchv1.FailureCodeTCPTimeout),
			expectedMatch: "timeout",
		},
		{
			name:          "I/O timeout",
			err:           &netError{msg: "i/o timeout"},
			expectedCode:  string(k8swatchv1.FailureCodeTCPTimeout),
			expectedMatch: "timeout",
		},
		{
			name:          "Connection reset",
			err:           &netError{msg: "connection reset by peer"},
			expectedCode:  string(k8swatchv1.FailureCodeTCPReset),
			expectedMatch: "reset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := layer.handleTCPError(tt.err)
			assert.False(t, result.Success)
			assert.Equal(t, tt.expectedCode, result.FailureCode)
			assert.Contains(t, result.FailureMessage, tt.expectedMatch)
		})
	}
}

func TestResolvePort(t *testing.T) {
	tests := []struct {
		name        string
		portStr     string
		defaultPort int
		expected    int
		expectError bool
	}{
		{
			name:        "Numeric port",
			portStr:     "8080",
			defaultPort: 80,
			expected:    8080,
			expectError: false,
		},
		{
			name:        "Named port uses default",
			portStr:     "http",
			defaultPort: 80,
			expected:    80,
			expectError: false,
		},
		{
			name:        "Empty uses default",
			portStr:     "",
			defaultPort: 443,
			expected:    443,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := resolvePort(tt.portStr, tt.defaultPort)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, port)
			}
		})
	}
}

// netError is a mock error for testing
type netError struct {
	msg string
}

func (e *netError) Error() string   { return e.msg }
func (e *netError) Timeout() bool   { return false }
func (e *netError) Temporary() bool { return false }

// Helper functions
func strPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
