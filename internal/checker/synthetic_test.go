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

func TestInternalCanaryCheckerFactory_Create(t *testing.T) {
	factory := &InternalCanaryCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeInternalCanary,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("canary.default.svc"),
				Port: int32Ptr(80),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 4)
}

func TestInternalCanaryCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &InternalCanaryCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"internal-canary"}, types)
}

func TestInternalCanaryChecker_Execute(t *testing.T) {
	factory := &InternalCanaryCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeInternalCanary,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("canary.invalid.svc"),
				Port: int32Ptr(80),
			},
		},
	}

	checker, _ := factory.Create(target)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := checker.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// May succeed or fail depending on DNS resolution
	assert.NotNil(t, result.Check)
}

func TestExternalHTTPCheckerFactory_Create(t *testing.T) {
	factory := &ExternalHTTPCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeExternalHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 4)
}

func TestExternalHTTPCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &ExternalHTTPCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"external-http"}, types)
}

func TestExternalHTTPChecker_Execute(t *testing.T) {
	factory := &ExternalHTTPCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeExternalHTTP,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("example.com"),
			},
		},
	}

	checker, _ := factory.Create(target)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := checker.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNodeEgressCheckerFactory_Create(t *testing.T) {
	factory := &NodeEgressCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNodeEgress,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 3)
}

func TestNodeEgressCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &NodeEgressCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"node-egress"}, types)
}

func TestNodeEgressChecker_Execute(t *testing.T) {
	factory := &NodeEgressCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNodeEgress,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
		},
	}

	checker, _ := factory.Create(target)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := checker.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNodeToNodeCheckerFactory_Create(t *testing.T) {
	factory := &NodeToNodeCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNodeToNode,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("10.0.0.1"),
				Port: int32Ptr(8080),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 2)
}

func TestNodeToNodeCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &NodeToNodeCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"node-to-node"}, types)
}

func TestNodeToNodeChecker_Execute(t *testing.T) {
	factory := &NodeToNodeCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNodeToNode,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(8080),
			},
		},
	}

	checker, _ := factory.Create(target)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := checker.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Connection will be refused but that's expected
	assert.NotNil(t, result.Check)
}

func TestInternalCanaryChecker_Layers(t *testing.T) {
	factory := &InternalCanaryCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeInternalCanary,
		},
	}

	checker, _ := factory.Create(target)
	layers := checker.Layers()
	assert.Len(t, layers, 4)
	assert.Equal(t, "L0", layers[0].Name())
	assert.Equal(t, "L1", layers[1].Name())
	assert.Equal(t, "L2", layers[2].Name())
	assert.Equal(t, "L4", layers[3].Name())
}

func TestExternalHTTPChecker_Layers(t *testing.T) {
	factory := &ExternalHTTPCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeExternalHTTP,
		},
	}

	checker, _ := factory.Create(target)
	layers := checker.Layers()
	assert.Len(t, layers, 4)
	assert.Equal(t, "L1", layers[0].Name())
	assert.Equal(t, "L2", layers[1].Name())
	assert.Equal(t, "L3", layers[2].Name())
	assert.Equal(t, "L4", layers[3].Name())
}

func TestNodeEgressChecker_Layers(t *testing.T) {
	factory := &NodeEgressCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNodeEgress,
		},
	}

	checker, _ := factory.Create(target)
	layers := checker.Layers()
	assert.Len(t, layers, 3)
	assert.Equal(t, "L0", layers[0].Name())
	assert.Equal(t, "L1", layers[1].Name())
	assert.Equal(t, "L2", layers[2].Name())
}

func TestNodeToNodeChecker_Layers(t *testing.T) {
	factory := &NodeToNodeCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNodeToNode,
		},
	}

	checker, _ := factory.Create(target)
	layers := checker.Layers()
	assert.Len(t, layers, 2)
	assert.Equal(t, "L0", layers[0].Name())
	assert.Equal(t, "L2", layers[1].Name())
}

func TestSyntheticCheckers_ContextCancellation(t *testing.T) {
	tests := []struct {
		name    string
		factory CheckerFactory
		target  *k8swatchv1.Target
	}{
		{
			name:    "InternalCanary",
			factory: &InternalCanaryCheckerFactory{},
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeInternalCanary,
					Endpoint: k8swatchv1.EndpointConfig{
						DNS: strPtr("canary.invalid.svc"),
					},
				},
			},
		},
		{
			name:    "ExternalHTTP",
			factory: &ExternalHTTPCheckerFactory{},
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeExternalHTTP,
					Endpoint: k8swatchv1.EndpointConfig{
						DNS: strPtr("example.invalid.com"),
					},
				},
			},
		},
		{
			name:    "NodeEgress",
			factory: &NodeEgressCheckerFactory{},
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeNodeEgress,
					Endpoint: k8swatchv1.EndpointConfig{
						DNS: strPtr("invalid.invalid"),
					},
				},
			},
		},
		{
			name:    "NodeToNode",
			factory: &NodeToNodeCheckerFactory{},
			target: &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetTypeNodeToNode,
					Endpoint: k8swatchv1.EndpointConfig{
						IP: strPtr("10.255.255.255"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := tt.factory.Create(tt.target)
			assert.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			result, err := checker.Execute(ctx, tt.target)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}
