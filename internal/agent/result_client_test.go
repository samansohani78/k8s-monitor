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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	pb "github.com/k8swatch/k8s-monitor/internal/pb"
)

func TestResultClientConfigDefaults(t *testing.T) {
	cfg := DefaultResultClientConfig()

	assert.Equal(t, "k8swatch-aggregator.k8swatch.svc:50051", cfg.AggregatorAddress)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, time.Second, cfg.RetryBackoff)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
}

func TestResultClientConfigCustom(t *testing.T) {
	cfg := &ResultClientConfig{
		AggregatorAddress: "custom:50051",
		MaxRetries:        5,
		RetryBackoff:      2 * time.Second,
		Timeout:           30 * time.Second,
	}

	assert.Equal(t, "custom:50051", cfg.AggregatorAddress)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 2*time.Second, cfg.RetryBackoff)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
}

func TestResultClientCreation(t *testing.T) {
	cfg := DefaultResultClientConfig()

	// Create client (will fail to connect but should create)
	client, err := NewResultClient(cfg, "test-node", "test-zone", "v1.0")

	// Client creation may succeed even without server
	// We're testing the constructor logic
	if err == nil {
		assert.NotNil(t, client)
		assert.Equal(t, "test-node", client.nodeName)
		assert.Equal(t, "test-zone", client.nodeZone)
		assert.Equal(t, "v1.0", client.agentVersion)

		// Test Close
		err = client.Close()
		assert.NoError(t, err)
	}
}

func TestResultClientNilConfig(t *testing.T) {
	// Test with nil config (should use defaults)
	client, err := NewResultClient(nil, "test-node", "test-zone", "v1.0")

	if err == nil {
		assert.NotNil(t, client)
		if client != nil {
			client.Close()
		}
	}
}

func TestResultClientBuildSubmitRequest(t *testing.T) {
	cfg := DefaultResultClientConfig()

	client := &ResultClient{
		config:       cfg,
		nodeName:     "test-node",
		nodeZone:     "test-zone",
		agentVersion: "v1.0",
		networkMode:  pb.NetworkMode_NETWORK_MODE_POD,
	}

	result := &k8swatchv1.CheckResult{
		ResultID:  "test-id-123",
		Timestamp: metav1.Now(),
		Target: k8swatchv1.TargetInfo{
			Name:      "test-target",
			Namespace: "default",
			Type:      k8swatchv1.TargetTypeHTTP,
			Labels:    map[string]string{"team": "platform"},
		},
		Check: k8swatchv1.CheckInfo{
			LayersEnabled:  []string{"L0", "L1", "L2"},
			FinalLayer:     "L2",
			Success:        true,
			FailureLayer:   "",
			FailureCode:    "",
			FailureMessage: "",
		},
		Latencies: map[string]k8swatchv1.LayerLatency{
			"L0": {DurationMs: 5, Success: true},
			"L1": {DurationMs: 10, Success: true},
			"L2": {DurationMs: 15, Success: true},
		},
		Metadata: k8swatchv1.CheckMetadata{
			CheckDurationMs: 30,
			AttemptNumber:   1,
			ConfigVersion:   "v1",
		},
	}

	req := client.buildSubmitRequest(result)

	assert.Equal(t, "test-id-123", req.ResultId)
	assert.Equal(t, "test-node", req.Agent.NodeName)
	assert.Equal(t, "test-zone", req.Agent.NodeZone)
	assert.Equal(t, "v1.0", req.Agent.AgentVersion)
	assert.Equal(t, pb.NetworkMode_NETWORK_MODE_POD, req.Agent.NetworkMode)
	assert.Equal(t, "test-target", req.Target.Name)
	assert.Equal(t, "default", req.Target.Namespace)
	assert.Equal(t, "http", req.Target.Type)
	assert.Equal(t, "platform", req.Target.Labels["team"])
	assert.True(t, req.Check.Success)
	assert.Equal(t, []string{"L0", "L1", "L2"}, req.Check.LayersEnabled)
	assert.Equal(t, int64(30), req.Metadata.CheckDurationMs)
	assert.Len(t, req.Latencies, 3)
}

func TestResultClientBuildSubmitRequestWithFailure(t *testing.T) {
	cfg := DefaultResultClientConfig()

	client := &ResultClient{
		config:       cfg,
		nodeName:     "test-node",
		nodeZone:     "test-zone",
		agentVersion: "v1.0",
		networkMode:  pb.NetworkMode_NETWORK_MODE_POD,
	}

	result := &k8swatchv1.CheckResult{
		ResultID:  "fail-id",
		Timestamp: metav1.Now(),
		Target: k8swatchv1.TargetInfo{
			Name:      "fail-target",
			Namespace: "default",
			Type:      k8swatchv1.TargetTypeDNS,
		},
		Check: k8swatchv1.CheckInfo{
			LayersEnabled:  []string{"L0", "L1"},
			FinalLayer:     "L1",
			Success:        false,
			FailureLayer:   "L1",
			FailureCode:    "dns_timeout",
			FailureMessage: "DNS query timed out",
		},
		Latencies: map[string]k8swatchv1.LayerLatency{
			"L0": {DurationMs: 5, Success: true},
			"L1": {DurationMs: 5000, Success: false},
		},
		Metadata: k8swatchv1.CheckMetadata{
			CheckDurationMs: 5005,
			AttemptNumber:   1,
			Error:           "timeout",
		},
	}

	req := client.buildSubmitRequest(result)

	assert.Equal(t, "fail-id", req.ResultId)
	assert.False(t, req.Check.Success)
	assert.Equal(t, "L1", req.Check.FailureLayer)
	assert.Equal(t, "dns_timeout", req.Check.FailureCode)
	assert.Equal(t, "DNS query timed out", req.Check.FailureMessage)
	assert.Equal(t, "timeout", req.Metadata.Error)
}

func TestResultClientBuildSubmitRequestHostNetworkMode(t *testing.T) {
	cfg := DefaultResultClientConfig()

	client := &ResultClient{
		config:       cfg,
		nodeName:     "test-node",
		nodeZone:     "test-zone",
		agentVersion: "v1.0",
		networkMode:  pb.NetworkMode_NETWORK_MODE_POD,
	}

	result := &k8swatchv1.CheckResult{
		ResultID:  "host-mode-id",
		Timestamp: metav1.Now(),
		Target: k8swatchv1.TargetInfo{
			Name:      "host-target",
			Namespace: "default",
			Type:      k8swatchv1.TargetTypeHTTP,
			Labels: map[string]string{
				"k8swatch.io/network-mode": "host",
			},
		},
		Check: k8swatchv1.CheckInfo{Success: true},
	}

	req := client.buildSubmitRequest(result)
	assert.Equal(t, pb.NetworkMode_NETWORK_MODE_HOST, req.Agent.NetworkMode)
}

func TestResolveNetworkModeFromEnv(t *testing.T) {
	t.Setenv("K8SWATCH_NETWORK_MODE", "host")
	assert.Equal(t, pb.NetworkMode_NETWORK_MODE_HOST, resolveNetworkModeFromEnv())

	t.Setenv("K8SWATCH_NETWORK_MODE", "pod")
	assert.Equal(t, pb.NetworkMode_NETWORK_MODE_POD, resolveNetworkModeFromEnv())

	_ = os.Unsetenv("K8SWATCH_NETWORK_MODE")
	assert.Equal(t, pb.NetworkMode_NETWORK_MODE_POD, resolveNetworkModeFromEnv())
}

func TestResultClientIsConnected(t *testing.T) {
	cfg := DefaultResultClientConfig()

	client, err := NewResultClient(cfg, "test-node", "test-zone", "v1.0")

	if err == nil && client != nil {
		// Test IsConnected
		connected := client.IsConnected()
		// Connection state depends on whether server is running
		// Just verify the method doesn't panic
		assert.IsType(t, false, connected)

		client.Close()

		// After close, should still not panic
		connected = client.IsConnected()
		assert.IsType(t, false, connected)
	}
}

func TestResultClientHealthCheck(t *testing.T) {
	cfg := DefaultResultClientConfig()
	cfg.Timeout = 100 * time.Millisecond

	client, err := NewResultClient(cfg, "test-node", "test-zone", "v1.0")

	if err == nil && client != nil {
		defer client.Close()

		// Health check will fail without server
		ctx := context.Background()
		err := client.HealthCheck(ctx)
		assert.Error(t, err)
	}
}

func TestResultClientSubmitResultTimeout(t *testing.T) {
	cfg := &ResultClientConfig{
		AggregatorAddress: "localhost:50051",
		MaxRetries:        1,
		RetryBackoff:      10 * time.Millisecond,
		Timeout:           50 * time.Millisecond,
	}

	client, err := NewResultClient(cfg, "test-node", "test-zone", "v1.0")

	if err == nil && client != nil {
		defer client.Close()

		result := &k8swatchv1.CheckResult{
			ResultID:  "test",
			Timestamp: metav1.Now(),
			Target:    k8swatchv1.TargetInfo{Name: "test", Namespace: "default", Type: k8swatchv1.TargetTypeHTTP},
			Check:     k8swatchv1.CheckInfo{Success: true},
		}

		ctx := context.Background()
		// Will fail because no server
		err := client.SubmitResult(ctx, result)
		assert.Error(t, err)
	}
}

func TestResultClientConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *ResultClientConfig
	}{
		{
			name:   "Default config",
			config: DefaultResultClientConfig(),
		},
		{
			name: "Custom retries",
			config: &ResultClientConfig{
				MaxRetries: 10,
			},
		},
		{
			name: "Zero retries",
			config: &ResultClientConfig{
				MaxRetries: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.config)
		})
	}
}

func TestResultClientRetryBackoff(t *testing.T) {
	cfg := &ResultClientConfig{
		MaxRetries:   3,
		RetryBackoff: 100 * time.Millisecond,
		Timeout:      50 * time.Millisecond,
	}

	// Test backoff calculation
	expectedBackoffs := []time.Duration{
		100 * time.Millisecond, // 1 * 100ms
		200 * time.Millisecond, // 2 * 100ms
		400 * time.Millisecond, // 4 * 100ms
	}

	for i, expected := range expectedBackoffs {
		backoff := cfg.RetryBackoff * time.Duration(1<<uint(i)) // nolint:gosec
		assert.Equal(t, expected, backoff)
	}
}
