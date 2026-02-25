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

package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

func TestConfigLoaderCreation(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Test with default config
	loader := ConfigLoaderWithClient(fakeClient, "default")
	assert.NotNil(t, loader)
	assert.Equal(t, "default", loader.namespace)
}

func TestConfigLoaderWithCustomConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &ConfigLoaderConfig{
		Namespace:     "k8swatch",
		LabelSelector: "app=test",
	}

	_, err := NewConfigLoader(fakeClient, cfg)
	assert.NoError(t, err)
}

func TestConfigLoaderWithInvalidLabelSelector(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &ConfigLoaderConfig{
		Namespace:     "k8swatch",
		LabelSelector: "invalid===selector",
	}

	_, err := NewConfigLoader(fakeClient, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse label selector")
}

func TestConfigLoaderLoadTargets(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	// Create test targets
	targets := []k8swatchv1.Target{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-target-1",
				Namespace: "k8swatch",
			},
			Spec: k8swatchv1.TargetSpec{
				Type: k8swatchv1.TargetTypeDNS,
				Endpoint: k8swatchv1.EndpointConfig{
					DNS: strPtr("google.com"),
				},
				Schedule: k8swatchv1.ScheduleConfig{
					Interval: "30s",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-target-2",
				Namespace: "k8swatch",
			},
			Spec: k8swatchv1.TargetSpec{
				Type: k8swatchv1.TargetTypeHTTP,
				Endpoint: k8swatchv1.EndpointConfig{
					DNS: strPtr("example.com"),
				},
				Schedule: k8swatchv1.ScheduleConfig{
					Interval: "60s",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&targets[0], &targets[1]).Build()
	loader := ConfigLoaderWithClient(fakeClient, "k8swatch")

	ctx := context.Background()
	loadedTargets, configVersion, err := loader.LoadTargets(ctx)

	require.NoError(t, err)
	assert.Len(t, loadedTargets, 2)
	assert.NotEmpty(t, configVersion)
}

func TestConfigLoaderLoadTargetsEmpty(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	loader := ConfigLoaderWithClient(fakeClient, "k8swatch")

	ctx := context.Background()
	loadedTargets, configVersion, err := loader.LoadTargets(ctx)

	require.NoError(t, err)
	assert.Len(t, loadedTargets, 0)
	assert.Equal(t, "empty", configVersion)
}

func TestValidateTargetValid(t *testing.T) {
	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
				Timeout:  "10s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.NoError(t, err)
}

func TestValidateTargetUnsupportedType(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: "unsupported-type",
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported target type")
}

func TestValidateTargetNoEndpoint(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				// No endpoint configured
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "one of k8sService, dns, or ip must be specified")
}

func TestValidateTargetMultipleEndpoints(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
				IP:  strPtr("8.8.8.8"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one of k8sService, dns, or ip can be specified")
}

func TestValidateTargetInvalidInterval(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "invalid",
			},
		},
	}

	err := ValidateTarget(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid schedule.interval")
}

func TestValidateTargetMissingInterval(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				// No interval
			},
		},
	}

	err := ValidateTarget(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schedule.interval is required")
}

func TestValidateTargetInvalidTimeout(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
				Timeout:  "invalid",
			},
		},
	}

	err := ValidateTarget(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid schedule.timeout")
}

func TestValidateTargetK8sService(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKubernetes,
			Endpoint: k8swatchv1.EndpointConfig{
				K8sService: &k8swatchv1.K8sServiceEndpoint{
					Name:      "kubernetes",
					Namespace: "default",
					Port:      "443",
				},
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.NoError(t, err)
}

func TestValidateTargetK8sServiceMissingName(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKubernetes,
			Endpoint: k8swatchv1.EndpointConfig{
				K8sService: &k8swatchv1.K8sServiceEndpoint{
					Name: "",
					Port: "443",
				},
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "k8sService.name is required")
}

func TestValidateTargetIP(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("192.168.1.1"),
				Port: int32Ptr(80),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.NoError(t, err)
}

func TestValidateTargetNetworkModes(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			NetworkModes: []k8swatchv1.NetworkMode{
				k8swatchv1.NetworkModePod,
				k8swatchv1.NetworkModeHost,
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.NoError(t, err)
	// Should default to pod network mode if not specified
	assert.Len(t, target.Spec.NetworkModes, 2)
}

func TestValidateTargetDefaultNetworkModes(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	err := ValidateTarget(target)
	assert.NoError(t, err)
	// Should default to pod network mode
	assert.Len(t, target.Spec.NetworkModes, 1)
	assert.Equal(t, k8swatchv1.NetworkModePod, target.Spec.NetworkModes[0])
}

// Helper functions are in agent_test.go

// TestConfigLoaderGenerateConfigVersion tests the config version generation
func TestConfigLoaderGenerateConfigVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	targets := []k8swatchv1.Target{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "target-1",
				ResourceVersion: "100",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "target-2",
				ResourceVersion: "200",
			},
		},
	}

	loader := &ConfigLoader{
		namespace: "default",
	}

	version := loader.generateConfigVersion(targets)
	assert.Equal(t, "v2-200", version)
}

func TestConfigLoaderGenerateConfigVersionEmpty(t *testing.T) {
	loader := &ConfigLoader{
		namespace: "default",
	}

	version := loader.generateConfigVersion([]k8swatchv1.Target{})
	assert.Equal(t, "empty", version)
}

// TestConfigLoaderLoadTarget tests loading a single target
func TestConfigLoaderLoadTarget(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "single-target",
			Namespace: "k8swatch",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeDNS,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(target).Build()
	loader := ConfigLoaderWithClient(fakeClient, "k8swatch")

	ctx := context.Background()
	loaded, err := loader.LoadTarget(ctx, "single-target")

	require.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Equal(t, "single-target", loaded.Name)
	assert.Equal(t, k8swatchv1.TargetTypeDNS, loaded.Spec.Type)
}

func TestConfigLoaderLoadTargetNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	loader := ConfigLoaderWithClient(fakeClient, "k8swatch")

	ctx := context.Background()
	_, err := loader.LoadTarget(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
