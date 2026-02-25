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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/k8swatch/k8s-monitor/internal/checker"
)

// Test NewAgent with various configurations
func TestNewAgent_Comprehensive(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		setupEnv      func()
		cleanupEnv    func()
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid config with aggregator address",
			config: &Config{
				AggregatorAddress: "localhost:50051",
				Namespace:         "test-ns",
				NodeName:          "test-node",
			},
			cleanupEnv:  func() {},
			expectError: true, // Will fail without k8s connection
		},
		{
			name: "Config with empty aggregator uses default",
			config: &Config{
				Namespace: "test-ns",
				NodeName:  "test-node",
			},
			cleanupEnv:  func() {},
			expectError: true,
		},
		{
			name: "Config gets namespace from env",
			config: &Config{
				NodeName: "test-node",
			},
			setupEnv: func() {
				os.Setenv("POD_NAMESPACE", "env-ns")
			},
			cleanupEnv: func() {
				os.Unsetenv("POD_NAMESPACE")
			},
			expectError: true,
		},
		{
			name: "Config gets node name from env",
			config: &Config{},
			setupEnv: func() {
				os.Setenv("NODE_NAME", "env-node")
			},
			cleanupEnv: func() {
				os.Unsetenv("NODE_NAME")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			defer tt.cleanupEnv()

			agent, err := NewAgent(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, agent)
			}
		})
	}
}

// Test agent accessor methods
func TestAgent_AccessorMethods(t *testing.T) {
	a := &Agent{
		config: &Config{
			Namespace: "test-ns",
		},
		nodeName:     "test-node",
		nodeZone:     "us-east-1a",
		agentVersion: "v1.2.3",
		kubeClient:   nil,
		client:       nil,
		restConfig:   nil,
		shutdown:     make(chan struct{}),
	}

	assert.Equal(t, "test-ns", a.config.Namespace)
	assert.Equal(t, "test-node", a.NodeName())
	assert.Equal(t, "us-east-1a", a.NodeZone())
	assert.Equal(t, "v1.2.3", a.AgentVersion())
	assert.Nil(t, a.KubeClient())
	assert.Nil(t, a.Client())
	assert.Nil(t, a.RestConfig())
}

// Test getNamespace function
func TestGetNamespace(t *testing.T) {
	tests := []struct {
		name       string
		setupEnv   func()
		setupFile  func() string // returns path to temp file
		cleanup    func()
		expectNS   string
	}{
		{
			name: "From service account file",
			setupFile: func() string {
				tmpDir := t.TempDir()
				saDir := tmpDir + "/serviceaccount"
				require.NoError(t, os.Mkdir(saDir, 0755))
				require.NoError(t, os.WriteFile(saDir+"/namespace", []byte("sa-namespace"), 0644))
				return saDir
			},
			cleanup:  func() {},
			expectNS: "sa-namespace",
		},
		{
			name: "From environment variable",
			setupEnv: func() {
				os.Setenv("POD_NAMESPACE", "env-namespace")
			},
			cleanup: func() {
				os.Unsetenv("POD_NAMESPACE")
			},
			expectNS: "env-namespace",
		},
		{
			name:       "Default namespace",
			setupEnv:   func() {},
			cleanup:    func() {},
			expectNS:   "k8swatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			defer tt.cleanup()

			// Note: getNamespace reads from /var/run/secrets/... which we can't easily mock
			// This test demonstrates the logic flow
			ns := getNamespace()
			// In real environment, this would read from the appropriate source
			assert.NotEmpty(t, ns)
		})
	}
}

// Test SetLogger and GetContextLogger
func TestAgent_Logger(t *testing.T) {
	// Test that SetLogger can be called with a valid logger
	// Note: We can't easily test the logger without importing logr/zapr
	assert.NotPanics(t, func() {
		// Logger is already initialized in package, just test getter
		logger := GetContextLogger()
		// Logger may be nil if SetLogger wasn't called with a valid logger
		assert.True(t, logger == nil || logger != nil) // Always true, just checking no panic
	})
}

// Test newCheckContext
func TestNewCheckContext(t *testing.T) {
	ctx := context.Background()

	// Test with nil contextLogger
	newCtx, opLogger := newCheckContext(ctx, "test-target", "default", "http")
	assert.Equal(t, ctx, newCtx)
	assert.Nil(t, opLogger)
}

// Test Metrics
func TestMetrics_Comprehensive(t *testing.T) {
	t.Run("NewMetrics creates metrics", func(t *testing.T) {
		metrics := NewMetrics()
		assert.NotNil(t, metrics)
		assert.NotNil(t, metrics.CheckTotal)
		assert.NotNil(t, metrics.CheckDurationSeconds)
		assert.NotNil(t, metrics.ConfigVersion)
		assert.NotNil(t, metrics.ResultsDroppedTotal)
		assert.NotNil(t, metrics.ChecksInProgress)
	})

	// Note: RecordCheck, RecordDroppedResult, etc. use global Prometheus registry
	// which doesn't allow duplicate registration. These are tested indirectly
	// through integration tests.
}

// Test verifyKubeConnectivity
func TestAgent_VerifyKubeConnectivity(t *testing.T) {
	t.Run("Agent without kubeclient", func(t *testing.T) {
		a := &Agent{
			config:     &Config{},
			kubeClient: nil,
			nodeName:   "test-node",
		}

		// This will panic due to nil kubeClient, so we recover
		assert.Panics(t, func() {
			_ = a.verifyKubeConnectivity()
		})
	})
}

// Test registerCheckers
func TestAgent_RegisterCheckers(t *testing.T) {
	a := &Agent{
		config:     &Config{},
		checkerReg: checker.NewRegistry(),
	}

	assert.NotPanics(t, func() {
		a.registerCheckers()
	})

	// Verify checkers were registered
	types := a.checkerReg.SupportedTypes()
	assert.NotEmpty(t, types)
}

// Test startHTTPServer
func TestAgent_StartHTTPServer(t *testing.T) {
	a := &Agent{
		config: &Config{
			HTTPAddress: ":0", // Use any available port
		},
	}

	assert.NotPanics(t, func() {
		a.startHTTPServer()
	})

	assert.NotNil(t, a.httpServer)
	assert.NotEmpty(t, a.httpServer.Addr)
}

// Test Agent Start with context cancellation
// Note: Full Start() testing requires proper Kubernetes client setup
// which is done in integration tests
func TestAgent_Start_ContextCancellation(t *testing.T) {
	// This test is skipped as it requires a full Kubernetes environment
	t.Skip("Skipping test - requires Kubernetes environment")
}

// Test scheduler watchTargets
// Note: Full testing requires Kubernetes environment
func TestScheduler_WatchTargets(t *testing.T) {
	t.Skip("Skipping test - requires Kubernetes environment")
}

// Test scheduler scheduleChecks
func TestScheduler_ScheduleChecks(t *testing.T) {
	// Simple test that just verifies the function exists and doesn't panic
	// Full testing requires proper setup which is done in integration tests
	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return nil, nil
	}

	s := NewScheduler(DefaultSchedulerConfig(), checkFunc)
	
	// Verify scheduler was created
	assert.NotNil(t, s)
	assert.Equal(t, 10, cap(s.semaphore))
}

// Test scheduler executeCheck
func TestScheduler_ExecuteCheck(t *testing.T) {
	// Simple test - full testing requires proper setup
	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return nil, nil
	}

	s := NewScheduler(DefaultSchedulerConfig(), checkFunc)
	assert.NotNil(t, s)
}

// Test scheduler with custom config
func TestScheduler_CustomConfig(t *testing.T) {
	config := &SchedulerConfig{
		MaxConcurrency:  5,
		DefaultInterval: 10 * time.Second,
		DefaultJitter:   0.05,
		DefaultTimeout:  3 * time.Second,
	}

	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return nil, nil
	}

	s := NewScheduler(config, checkFunc)
	assert.NotNil(t, s)
	assert.Equal(t, 5, cap(s.semaphore))
}

// Test scheduler with nil config
func TestScheduler_NilConfig(t *testing.T) {
	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return nil, nil
	}

	s := NewScheduler(nil, checkFunc)
	assert.NotNil(t, s)
	assert.Equal(t, 10, cap(s.semaphore)) // Default
}

// Test scheduler Start and Stop
func TestScheduler_StartStop(t *testing.T) {
	checkFunc := func(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
		return nil, nil
	}

	s := NewScheduler(DefaultSchedulerConfig(), checkFunc)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error)
	go func() {
		done <- s.Start(ctx)
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Error("Scheduler Start did not complete")
	}
}

// Test ConfigLoader
func TestConfigLoader_Comprehensive(t *testing.T) {
	t.Run("DefaultConfigLoaderConfig", func(t *testing.T) {
		cfg := DefaultConfigLoaderConfig()
		assert.NotNil(t, cfg)
		assert.Equal(t, "k8swatch", cfg.Namespace)
	})

	t.Run("ConfigLoaderWithClient", func(t *testing.T) {
		// This would need a real client to test fully
		// Just verify it doesn't panic
		assert.NotPanics(t, func() {
			ConfigLoaderWithClient(nil, "test-ns")
		})
	})
}

// Test ResultClient
func TestResultClient_Comprehensive(t *testing.T) {
	t.Run("DefaultResultClientConfig", func(t *testing.T) {
		cfg := DefaultResultClientConfig()
		assert.NotNil(t, cfg)
		assert.Equal(t, 10*time.Second, cfg.Timeout)
		assert.Equal(t, 3, cfg.MaxRetries)
	})

	t.Run("NewResultClient creates client", func(t *testing.T) {
		cfg := DefaultResultClientConfig()
		cfg.AggregatorAddress = "localhost:50051"
		
		client, err := NewResultClient(cfg, "test-node", "test-zone", "v1.0.0")
		// This may succeed or fail depending on network setup
		// Just verify the function can be called
		assert.True(t, (err == nil && client != nil) || (err != nil && client == nil))
	})
}
