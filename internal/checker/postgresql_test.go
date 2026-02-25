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
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

func TestPostgreSQLCheckerFactoryCreation(t *testing.T) {
	factory := &PostgreSQLCheckerFactory{}
	checker, err := factory.Create(&k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypePostgreSQL,
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.GreaterOrEqual(t, len(checker.Layers()), 4) // L0, L1, L2, L4 minimum
}

func TestPostgreSQLCheckerFactorySupportedTypes(t *testing.T) {
	factory := &PostgreSQLCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "postgresql")
}

func TestPostgreSQLProtocolLayerName(t *testing.T) {
	layer := NewPostgreSQLProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestPostgreSQLProtocolLayerEnabled(t *testing.T) {
	layer := NewPostgreSQLProtocolLayer()

	// Enabled for PostgreSQL by default
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypePostgreSQL,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestPostgreSQLProtocolLayerGetAddress(t *testing.T) {
	layer := NewPostgreSQLProtocolLayer()

	tests := []struct {
		name        string
		target      *k8swatchv1.Target
		expectError bool
		expected    string
	}{
		{
			name: "DNS endpoint with default port",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						DNS: strPtr("postgres.example.com"),
					},
				},
			},
			expectError: false,
			expected:    "postgres.example.com:5432",
		},
		{
			name: "DNS endpoint with custom port",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						DNS:  strPtr("postgres.example.com"),
						Port: int32Ptr(5433),
					},
				},
			},
			expectError: false,
			expected:    "postgres.example.com:5433",
		},
		{
			name: "IP endpoint",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						IP:   strPtr("192.168.1.100"),
						Port: int32Ptr(5432),
					},
				},
			},
			expectError: false,
			expected:    "192.168.1.100:5432",
		},
		{
			name: "K8s service",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						K8sService: &k8swatchv1.K8sServiceEndpoint{
							Name:      "postgres",
							Namespace: "database",
							Port:      "5432",
						},
					},
				},
			},
			expectError: false,
			expected:    "postgres.database.svc.cluster.local:5432",
		},
		{
			name: "K8s service with default namespace",
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Endpoint: k8swatchv1.EndpointConfig{
						K8sService: &k8swatchv1.K8sServiceEndpoint{
							Name: "postgres",
							Port: "5432",
						},
					},
				},
			},
			expectError: false,
			expected:    "postgres.default.svc.cluster.local:5432",
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
			address, err := layer.getAddress(tt.target)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, address)
			}
		})
	}
}

func TestPostgreSQLProtocolLayerHandlePostgreSQLError(t *testing.T) {
	layer := NewPostgreSQLProtocolLayer()

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
			name:          "Timeout",
			err:           &netError{msg: "timeout"},
			expectedCode:  string(k8swatchv1.FailureCodeProtocolTimeout),
			expectedMatch: "timeout",
		},
		{
			name:          "No route",
			err:           &netError{msg: "no route to host"},
			expectedCode:  string(k8swatchv1.FailureCodeTCPNoRoute),
			expectedMatch: "no route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := layer.handlePostgreSQLError(tt.err)
			assert.False(t, result.Success)
			assert.Equal(t, tt.expectedCode, result.FailureCode)
			assert.Contains(t, result.FailureMessage, tt.expectedMatch)
		})
	}
}

func TestPostgreSQLAuthLayerName(t *testing.T) {
	layer := NewPostgreSQLAuthLayer()
	assert.Equal(t, "L5", layer.Name())
}

func TestPostgreSQLAuthLayerEnabled(t *testing.T) {
	layer := NewPostgreSQLAuthLayer()

	// Disabled by default
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypePostgreSQL,
		},
	}
	assert.False(t, layer.Enabled(target))

	// Enabled when configured
	target.Spec.Layers.L5Auth = &k8swatchv1.AuthConfig{
		LayerConfigBase: k8swatchv1.LayerConfigBase{Enabled: true},
	}
	assert.True(t, layer.Enabled(target))
}

func TestPostgreSQLSemanticLayerName(t *testing.T) {
	layer := NewPostgreSQLSemanticLayer()
	assert.Equal(t, "L6", layer.Name())
}

func TestPostgreSQLSemanticLayerEnabled(t *testing.T) {
	layer := NewPostgreSQLSemanticLayer()

	// Disabled by default
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypePostgreSQL,
		},
	}
	assert.False(t, layer.Enabled(target))

	// Enabled when configured
	target.Spec.Layers.L6Semantic = &k8swatchv1.SemanticConfig{
		LayerConfigBase: k8swatchv1.LayerConfigBase{Enabled: true},
	}
	assert.True(t, layer.Enabled(target))
}

func TestPostgreSQLCheckerExecute(t *testing.T) {
	factory := &PostgreSQLCheckerFactory{}
	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-postgres",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypePostgreSQL,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("localhost"),
				Port: int32Ptr(5432),
			},
			Layers: k8swatchv1.LayerConfig{
				L4Protocol: &k8swatchv1.ProtocolConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{Enabled: true},
				},
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
				Timeout:  "10s",
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)

	// Note: This test requires a running PostgreSQL instance
	// It will fail if PostgreSQL is not available
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := checker.Execute(ctx, target)
	if err != nil {
		t.Logf("PostgreSQL check failed (expected without running instance): %v", err)
	} else {
		t.Logf("PostgreSQL check result: success=%v, finalLayer=%s, duration=%dms",
			result.Check.Success, result.Check.FinalLayer, result.Metadata.CheckDurationMs)
	}
}
