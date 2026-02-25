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

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestTLSLayer_handleTLSError(t *testing.T) {
	layer := NewTLSLayer()

	// The handleTLSError function uses contains() which checks for substring
	// We need to use the actual error messages that would come from TLS errors
	tests := []struct {
		name           string
		expectedCode   string
	}{
		{
			name:            "generic TLS error",
			expectedCode:    string(k8swatchv1.FailureCodeTLSHandshakeFailed),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := layer.handleTLSError(assert.AnError)
			assert.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Equal(t, tt.expectedCode, result.FailureCode)
		})
	}
}

func TestTLSLayer_handleTCPError(t *testing.T) {
	layer := NewTLSLayer()

	tests := []struct {
		name         string
		expectedCode string
	}{
		{
			name:         "generic error",
			expectedCode: string(k8swatchv1.FailureCodeTCPTimeout),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := layer.handleTCPError(assert.AnError)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedCode, result.FailureCode)
		})
	}
}

func TestTLSLayer_getTLSConfig(t *testing.T) {
	layer := NewTLSLayer()

	// Test with default config
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTPS,
		},
	}

	config, err := layer.getTLSConfig(target)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.False(t, config.InsecureSkipVerify)
}

func TestHTTPProtocolLayer_Name(t *testing.T) {
	layer := NewHTTPProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestHTTPProtocolLayer_Enabled(t *testing.T) {
	layer := NewHTTPProtocolLayer()

	// HTTP target
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}
	assert.True(t, layer.Enabled(target))

	// HTTPS target
	target2 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTPS,
		},
	}
	assert.True(t, layer.Enabled(target2))

	// Explicit L4 config
	target3 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
			Layers: k8swatchv1.LayerConfig{
				L4Protocol: &k8swatchv1.ProtocolConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}
	assert.True(t, layer.Enabled(target3))

	// Disabled
	target4 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}
	assert.False(t, layer.Enabled(target4))
}

func TestHTTPProtocolLayer_getExpectedStatusCode(t *testing.T) {
	layer := NewHTTPProtocolLayer()

	// Default for HTTP
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}
	assert.Equal(t, 200, layer.getExpectedStatusCode(target))

	// Default for HTTPS
	target2 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTPS,
		},
	}
	assert.Equal(t, 200, layer.getExpectedStatusCode(target2))
}

func TestHTTPProtocolLayer_isStatusCodeExpected(t *testing.T) {
	layer := NewHTTPProtocolLayer()

	// Exact match when expected > 0
	assert.True(t, layer.isStatusCodeExpected(200, 200))
	assert.False(t, layer.isStatusCodeExpected(201, 200))

	// Default: accept 2xx and 3xx when expected = 0
	assert.True(t, layer.isStatusCodeExpected(200, 0))
	assert.True(t, layer.isStatusCodeExpected(201, 0))
	assert.True(t, layer.isStatusCodeExpected(301, 0))
	assert.False(t, layer.isStatusCodeExpected(404, 0))
	assert.False(t, layer.isStatusCodeExpected(500, 0))
}

func TestHTTPAuthLayer_Name(t *testing.T) {
	layer := NewHTTPAuthLayer()
	assert.Equal(t, "L5", layer.Name())
}

func TestHTTPAuthLayer_Enabled(t *testing.T) {
	layer := NewHTTPAuthLayer()

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Layers: k8swatchv1.LayerConfig{
				L5Auth: &k8swatchv1.AuthConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}
	assert.True(t, layer.Enabled(target))

	target2 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Layers: k8swatchv1.LayerConfig{
				L5Auth: &k8swatchv1.AuthConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: false,
					},
				},
			},
		},
	}
	assert.False(t, layer.Enabled(target2))
}

func TestHTTPAuthLayer_Check_NoConfig(t *testing.T) {
	layer := NewHTTPAuthLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L5Auth: &k8swatchv1.AuthConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeConfigError), result.FailureCode)
}

func TestHTTPSemanticLayer_Name(t *testing.T) {
	layer := NewHTTPSemanticLayer()
	assert.Equal(t, "L6", layer.Name())
}

func TestHTTPSemanticLayer_Enabled(t *testing.T) {
	layer := NewHTTPSemanticLayer()

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Layers: k8swatchv1.LayerConfig{
				L6Semantic: &k8swatchv1.SemanticConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}
	assert.True(t, layer.Enabled(target))

	target2 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Layers: k8swatchv1.LayerConfig{
				L6Semantic: &k8swatchv1.SemanticConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: false,
					},
				},
			},
		},
	}
	assert.False(t, layer.Enabled(target2))
}

func TestHTTPSemanticLayer_Check_NoConfig(t *testing.T) {
	layer := NewHTTPSemanticLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L6Semantic: &k8swatchv1.SemanticConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeConfigError), result.FailureCode)
}

func TestHTTPProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewHTTPProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Endpoint: k8swatchv1.EndpointConfig{},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeConfigError), result.FailureCode)
}

func TestHTTPProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewHTTPProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(9999),
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
}

func TestHTTPProtocolLayer_Check_WithTLS(t *testing.T) {
	layer := NewHTTPProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTPS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("example.com"),
				Port: int32Ptr(443),
			},
			Layers: k8swatchv1.LayerConfig{
				L3TLS: &k8swatchv1.TLSConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestHTTPProtocolLayer_Check_WithCustomHeaders(t *testing.T) {
	layer := NewHTTPProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("example.com"),
				Port: int32Ptr(80),
			},
			Layers: k8swatchv1.LayerConfig{
				L4Protocol: &k8swatchv1.ProtocolConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
					Headers: map[string]string{
						"X-Custom-Header": "test",
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestHTTPProtocolLayer_Check_WithContextCancellation(t *testing.T) {
	layer := NewHTTPProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("example.com"),
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestHTTPProtocolLayer_buildHTTPRequest(t *testing.T) {
	layer := NewHTTPProtocolLayer()

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("example.com"),
			},
			Layers: k8swatchv1.LayerConfig{
				L4Protocol: &k8swatchv1.ProtocolConfig{
					Method: "GET",
				},
			},
		},
	}

	req, err := layer.buildHTTPRequest(target)
	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, "GET", req.Method)
}

func TestHTTPProtocolLayer_buildHTTPRequest_WithBody(t *testing.T) {
	layer := NewHTTPProtocolLayer()

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("example.com"),
			},
			Layers: k8swatchv1.LayerConfig{
				L4Protocol: &k8swatchv1.ProtocolConfig{
					Method: "POST",
					Body:   `{"test": "data"}`,
				},
			},
		},
	}

	req, err := layer.buildHTTPRequest(target)
	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, "POST", req.Method)
}

func TestHTTPProtocolLayer_buildHTTPRequest_CustomPath(t *testing.T) {
	layer := NewHTTPProtocolLayer()

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("example.com"),
				Path: strPtr("/health"),
			},
		},
	}

	req, err := layer.buildHTTPRequest(target)
	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Contains(t, req.URL.Path, "/health")
}

func TestHTTPProtocolLayer_buildHTTPRequest_K8sService(t *testing.T) {
	layer := NewHTTPProtocolLayer()

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				K8sService: &k8swatchv1.K8sServiceEndpoint{
					Name:      "test-service",
					Namespace: "default",
					Port:      "80",
				},
			},
		},
	}

	req, err := layer.buildHTTPRequest(target)
	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Contains(t, req.URL.Host, "test-service.default.svc")
}

func TestTLSLayer_Enabled(t *testing.T) {
	layer := NewTLSLayer()

	// HTTPS target
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTPS,
		},
	}
	assert.True(t, layer.Enabled(target))

	// HTTP target
	target2 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}
	assert.False(t, layer.Enabled(target2))

	// Explicit L3 config
	target3 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L3TLS: &k8swatchv1.TLSConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}
	assert.True(t, layer.Enabled(target3))
}

func TestTLSLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewTLSLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTPS,
			Endpoint: k8swatchv1.EndpointConfig{},
			Layers: k8swatchv1.LayerConfig{
				L3TLS: &k8swatchv1.TLSConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeConfigError), result.FailureCode)
}

func TestTLSLayer_Check_TLSConfig(t *testing.T) {
	layer := NewTLSLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTPS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("example.com"),
				Port: int32Ptr(443),
			},
			Layers: k8swatchv1.LayerConfig{
				L3TLS: &k8swatchv1.TLSConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
					InsecureSkipVerify: true,
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestHTTPProtocolLayer_handleHTTPError(t *testing.T) {
	layer := NewHTTPProtocolLayer()

	// Test with a generic error
	result := layer.handleHTTPError(assert.AnError)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
}

func TestHTTPAuthLayer_handleHTTPError(t *testing.T) {
	layer := NewHTTPAuthLayer()

	result := layer.handleHTTPError(assert.AnError)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
}

func TestHTTPSemanticLayer_handleHTTPError(t *testing.T) {
	layer := NewHTTPSemanticLayer()

	result := layer.handleHTTPError(assert.AnError)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
}
