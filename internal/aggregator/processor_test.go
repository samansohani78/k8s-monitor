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

package aggregator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	pb "github.com/k8swatch/k8s-monitor/internal/pb"
)

func TestProcessorConfigDefaults(t *testing.T) {
	cfg := DefaultProcessorConfig()

	assert.Equal(t, 5*time.Minute, cfg.StateCleanupInterval)
	assert.Equal(t, 24*time.Hour, cfg.StateExpiration)
	assert.Equal(t, 10000, cfg.MaxStateSize)
}

func TestStreamProcessorCreation(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	assert.NotNil(t, processor)
	assert.NotNil(t, processor.states)
	assert.Equal(t, 0, len(processor.states))
}

func TestStreamProcessorCreationNilConfig(t *testing.T) {
	processor := NewStreamProcessor(nil)

	assert.NotNil(t, processor)
	assert.Equal(t, 5*time.Minute, processor.config.StateCleanupInterval)
}

func TestStreamProcessorProcessResult(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName:    "node-1",
			NetworkMode: pb.NetworkMode_NETWORK_MODE_POD,
		},
		Target: &pb.TargetInfo{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Check: &pb.CheckInfo{
			Success: true,
		},
	}

	ctx := context.Background()
	err := processor.ProcessResult(ctx, req)

	assert.NoError(t, err)

	state, exists := processor.GetState("default/test-target")
	assert.True(t, exists)
	assert.NotNil(t, state)
	assert.Equal(t, TargetStateHealthy, state.State)
	assert.Equal(t, 1, state.ConsecutiveSuccesses)
	assert.Equal(t, 0, state.ConsecutiveFailures)
}

func TestStreamProcessorProcessResultFailure(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName:    "node-1",
			NetworkMode: pb.NetworkMode_NETWORK_MODE_POD,
		},
		Target: &pb.TargetInfo{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Check: &pb.CheckInfo{
			Success:      false,
			FailureCode:  "tcp_refused",
			FailureLayer: "L2",
		},
	}

	ctx := context.Background()
	err := processor.ProcessResult(ctx, req)

	assert.NoError(t, err)

	state, exists := processor.GetState("default/test-target")
	assert.True(t, exists)
	assert.NotNil(t, state)
	assert.Equal(t, TargetStateUnhealthy, state.State)
	assert.Equal(t, 0, state.ConsecutiveSuccesses)
	assert.Equal(t, 1, state.ConsecutiveFailures)
	assert.Equal(t, "tcp_refused", state.LastFailureCode)
	assert.Equal(t, "L2", state.LastFailureLayer)
}

func TestStreamProcessorConsecutiveResults(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	ctx := context.Background()

	// First success
	req1 := &pb.SubmitResultRequest{
		ResultId: "test-1",
		Agent:    &pb.AgentInfo{NodeName: "node-1"},
		Target:   &pb.TargetInfo{Name: "target", Namespace: "default"},
		Check:    &pb.CheckInfo{Success: true},
	}
	_ = processor.ProcessResult(ctx, req1)

	// Second success
	req2 := &pb.SubmitResultRequest{
		ResultId: "test-2",
		Agent:    &pb.AgentInfo{NodeName: "node-1"},
		Target:   &pb.TargetInfo{Name: "target", Namespace: "default"},
		Check:    &pb.CheckInfo{Success: true},
	}
	_ = processor.ProcessResult(ctx, req2)

	state, _ := processor.GetState("default/target")
	assert.Equal(t, 2, state.ConsecutiveSuccesses)
	assert.Equal(t, 0, state.ConsecutiveFailures)

	// Then failure
	req3 := &pb.SubmitResultRequest{
		ResultId: "test-3",
		Agent:    &pb.AgentInfo{NodeName: "node-1"},
		Target:   &pb.TargetInfo{Name: "target", Namespace: "default"},
		Check:    &pb.CheckInfo{Success: false},
	}
	_ = processor.ProcessResult(ctx, req3)

	state, _ = processor.GetState("default/target")
	assert.Equal(t, 0, state.ConsecutiveSuccesses)
	assert.Equal(t, 1, state.ConsecutiveFailures)
}

func TestStreamProcessorAgentStates(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	ctx := context.Background()

	// Result from node-1
	req1 := &pb.SubmitResultRequest{
		ResultId: "test-1",
		Agent: &pb.AgentInfo{
			NodeName:    "node-1",
			NetworkMode: pb.NetworkMode_NETWORK_MODE_POD,
		},
		Target: &pb.TargetInfo{Name: "target", Namespace: "default"},
		Check:  &pb.CheckInfo{Success: true},
	}
	_ = processor.ProcessResult(ctx, req1)

	// Result from node-2
	req2 := &pb.SubmitResultRequest{
		ResultId: "test-2",
		Agent: &pb.AgentInfo{
			NodeName:    "node-2",
			NetworkMode: pb.NetworkMode_NETWORK_MODE_HOST,
		},
		Target: &pb.TargetInfo{Name: "target", Namespace: "default"},
		Check:  &pb.CheckInfo{Success: false},
	}
	_ = processor.ProcessResult(ctx, req2)

	state, _ := processor.GetState("default/target")
	assert.Equal(t, 2, len(state.AgentStates))

	// Check that agent states exist (keys may vary based on NetworkMode enum)
	assert.GreaterOrEqual(t, len(state.AgentStates), 1)
}

func TestStreamProcessorGetStateNotFound(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	state, exists := processor.GetState("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, state)
}

func TestStreamProcessorGetAllStates(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	ctx := context.Background()

	// Add multiple targets
	for i := 0; i < 3; i++ {
		req := &pb.SubmitResultRequest{
			ResultId: "test",
			Agent:    &pb.AgentInfo{NodeName: "node-1"},
			Target:   &pb.TargetInfo{Name: "target", Namespace: "default"},
			Check:    &pb.CheckInfo{Success: true},
		}
		_ = processor.ProcessResult(ctx, req)
	}

	allStates := processor.GetAllStates()
	assert.Equal(t, 1, len(allStates))
}

func TestStreamProcessorCleanupExpiredStates(t *testing.T) {
	cfg := &ProcessorConfig{
		StateCleanupInterval: 1 * time.Millisecond,
		StateExpiration:      1 * time.Millisecond,
		MaxStateSize:         100,
	}
	processor := NewStreamProcessor(cfg)

	ctx := context.Background()

	// Add state
	req := &pb.SubmitResultRequest{
		ResultId: "test",
		Agent:    &pb.AgentInfo{NodeName: "node-1"},
		Target:   &pb.TargetInfo{Name: "target", Namespace: "default"},
		Check:    &pb.CheckInfo{Success: true},
	}
	_ = processor.ProcessResult(ctx, req)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Cleanup
	expired := processor.CleanupExpiredStates()
	assert.Equal(t, 1, expired)

	// Verify state is gone
	_, exists := processor.GetState("default/target")
	assert.False(t, exists)
}

func TestStreamProcessorGetStats(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	ctx := context.Background()

	// Add healthy target
	req1 := &pb.SubmitResultRequest{
		ResultId: "test-1",
		Agent:    &pb.AgentInfo{NodeName: "node-1"},
		Target:   &pb.TargetInfo{Name: "healthy", Namespace: "default"},
		Check:    &pb.CheckInfo{Success: true},
	}
	_ = processor.ProcessResult(ctx, req1)

	// Add unhealthy target
	req2 := &pb.SubmitResultRequest{
		ResultId: "test-2",
		Agent:    &pb.AgentInfo{NodeName: "node-1"},
		Target:   &pb.TargetInfo{Name: "unhealthy", Namespace: "default"},
		Check:    &pb.CheckInfo{Success: false},
	}
	_ = processor.ProcessResult(ctx, req2)

	stats := processor.GetStats()

	assert.GreaterOrEqual(t, stats.TotalTargets, 2)
	assert.GreaterOrEqual(t, stats.HealthyCount, 1)
	assert.GreaterOrEqual(t, stats.UnhealthyCount, 1)
}

func TestStreamProcessorMakeTargetKey(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	req := &pb.SubmitResultRequest{
		Target: &pb.TargetInfo{
			Name:      "my-target",
			Namespace: "my-namespace",
		},
	}

	key := processor.makeTargetKey(req.Target)
	assert.Equal(t, "my-namespace/my-target", key)
}

func TestStreamProcessorCopyState(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	original := &TargetState{
		TargetKey:            "key",
		TargetName:           "target",
		ConsecutiveFailures:  5,
		ConsecutiveSuccesses: 3,
		State:                TargetStateUnhealthy,
		AgentStates: map[string]*AgentTargetState{
			"agent-1": {
				AgentNode:           "node-1",
				ConsecutiveFailures: 2,
			},
		},
	}

	copy := processor.copyState(original)

	// Verify values are copied
	assert.Equal(t, original.TargetKey, copy.TargetKey)
	assert.Equal(t, original.ConsecutiveFailures, copy.ConsecutiveFailures)
	assert.Equal(t, original.State, copy.State)

	// Verify it's a deep copy
	copy.ConsecutiveFailures = 999
	assert.NotEqual(t, original.ConsecutiveFailures, copy.ConsecutiveFailures)
}

func TestStreamProcessorNilCopyState(t *testing.T) {
	cfg := DefaultProcessorConfig()
	processor := NewStreamProcessor(cfg)

	copy := processor.copyState(nil)
	assert.Nil(t, copy)
}
