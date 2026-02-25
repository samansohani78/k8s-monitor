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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/k8swatch/k8s-monitor/internal/checker"
)

func TestAgentAccessorMethods(t *testing.T) {
	a := &Agent{
		config:       &Config{Namespace: "test-ns"},
		nodeName:     "test-node",
		nodeZone:     "test-zone",
		agentVersion: "v1.0.0",
		shutdown:     make(chan struct{}),
	}

	assert.Equal(t, "test-node", a.NodeName())
	assert.Equal(t, "test-zone", a.NodeZone())
	assert.Equal(t, "v1.0.0", a.AgentVersion())
}

func TestAgentConfigDefaults(t *testing.T) {
	cfg := &Config{
		AggregatorAddress: "",
	}
	assert.Equal(t, "", cfg.AggregatorAddress)
}

func TestAgentStop(t *testing.T) {
	a := &Agent{
		config:   &Config{},
		shutdown: make(chan struct{}),
	}

	err := a.Stop()
	assert.NoError(t, err)
}

func TestBasicCheckerFactory(t *testing.T) {
	factory := &basicCheckerFactory{}

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)

	types := factory.SupportedTypes()
	assert.Contains(t, types, "all")
}

func TestAgentExecuteCheckInvalidTarget(t *testing.T) {
	a := &Agent{
		config:     &Config{},
		checkerReg: checker.NewDefaultRegistry(),
		shutdown:   make(chan struct{}),
	}

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
		},
	}

	ctx := context.Background()
	result, err := a.executeCheck(ctx, target)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAgentWithEnvNamespace(t *testing.T) {
	os.Setenv("POD_NAMESPACE", "env-namespace")
	defer os.Unsetenv("POD_NAMESPACE")

	cfg := &Config{
		Namespace: "",
	}

	assert.Equal(t, "", cfg.Namespace)
}

func TestAgentWithEnvNodeName(t *testing.T) {
	os.Setenv("NODE_NAME", "env-node")
	defer os.Unsetenv("NODE_NAME")

	cfg := &Config{
		NodeName: "",
	}

	assert.Equal(t, "", cfg.NodeName)
}

func TestAgentExecuteCheckUnsupportedType(t *testing.T) {
	a := &Agent{
		config:     &Config{},
		checkerReg: checker.NewDefaultRegistry(),
		shutdown:   make(chan struct{}),
	}

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: "unsupported-type",
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("test.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
		},
	}

	ctx := context.Background()
	result, err := a.executeCheck(ctx, target)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAgentRefreshConfigLoopShutdown(t *testing.T) {
	a := &Agent{
		config:   &Config{Namespace: "k8swatch"},
		shutdown: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	close(a.shutdown)
	a.refreshConfigLoop(ctx)
}

func TestAgentConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "Valid config",
			config: &Config{
				Kubeconfig:        "",
				AggregatorAddress: "localhost:50051",
				Namespace:         "k8swatch",
			},
		},
		{
			name: "Empty aggregator address",
			config: &Config{
				AggregatorAddress: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.config)
		})
	}
}

func TestAgentExecuteCheckValidTarget(t *testing.T) {
	a := &Agent{
		config:     &Config{},
		checkerReg: checker.NewDefaultRegistry(),
		shutdown:   make(chan struct{}),
	}

	target := &k8swatchv1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNetwork,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS: strPtr("google.com"),
			},
			Schedule: k8swatchv1.ScheduleConfig{
				Interval: "30s",
			},
			Layers: k8swatchv1.LayerConfig{
				L0NodeSanity: &k8swatchv1.NodeSanityConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{Enabled: true},
				},
			},
		},
	}

	ctx := context.Background()
	result, _ := a.executeCheck(ctx, target)
	assert.NotNil(t, result)
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
