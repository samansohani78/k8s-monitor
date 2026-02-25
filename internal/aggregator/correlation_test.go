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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	pb "github.com/k8swatch/k8s-monitor/internal/pb"
)

func TestCorrelationConfigDefaults(t *testing.T) {
	cfg := DefaultCorrelationConfig()

	assert.Equal(t, 60*time.Second, cfg.TimeWindow)
	assert.Equal(t, 2, cfg.MinNodesForPattern)
}

func TestCorrelationEngineCreation(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.failureWindow)
	assert.Equal(t, 0, len(engine.failureWindow))
}

func TestCorrelationEngineCreationNilConfig(t *testing.T) {
	engine := NewCorrelationEngine(nil)

	assert.NotNil(t, engine)
	assert.Equal(t, 60*time.Second, engine.config.TimeWindow)
}

func TestCorrelationEngineRecordFailure(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	result := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName:    "node-1",
			NetworkMode: pb.NetworkMode_NETWORK_MODE_POD,
		},
		Target: &pb.TargetInfo{
			Name: "test-target",
		},
		Check: &pb.CheckInfo{
			Success:      false,
			FailureCode:  "tcp_refused",
			FailureLayer: "L2",
		},
	}

	engine.RecordFailure("test-target", result)

	count := engine.GetFailureCount("test-target")
	assert.Equal(t, 1, count)
}

func TestCorrelationEngineRecordSuccess(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	result := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName: "node-1",
		},
		Target: &pb.TargetInfo{
			Name: "test-target",
		},
		Check: &pb.CheckInfo{
			Success: true,
		},
	}

	engine.RecordFailure("test-target", result)

	count := engine.GetFailureCount("test-target")
	assert.Equal(t, 0, count)
}

func TestCorrelationEngineGetAffectedNodes(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Add failures from multiple nodes
	for i := 0; i < 3; i++ {
		result := &pb.SubmitResultRequest{
			ResultId: "test",
			Agent: &pb.AgentInfo{
				NodeName: "node-" + string(rune('a'+i)),
			},
			Target: &pb.TargetInfo{Name: "target"},
			Check:  &pb.CheckInfo{Success: false},
		}
		engine.RecordFailure("target", result)
	}

	nodes := engine.GetAffectedNodes("target")
	assert.Len(t, nodes, 3)
}

func TestCorrelationEngineDetectPatternTargetOutage(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Add failures with same failure code from multiple nodes
	for i := 0; i < 3; i++ {
		result := &pb.SubmitResultRequest{
			ResultId: "test",
			Agent: &pb.AgentInfo{
				NodeName: "node-" + string(rune('a'+i)),
			},
			Target: &pb.TargetInfo{Name: "target"},
			Check: &pb.CheckInfo{
				Success:      false,
				FailureCode:  "tcp_refused",
				FailureLayer: "L2",
			},
		}
		engine.RecordFailure("target", result)
	}

	pattern := engine.DetectPattern("target", nil)
	assert.Equal(t, PatternTargetOutage, pattern)
}

func TestCorrelationEngineDetectPatternCNIIssue(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Add pod-network only failures with DIFFERENT failure codes (to avoid target_outage)
	codes := []string{"timeout", "connection_reset"}
	for i, code := range codes {
		result := &pb.SubmitResultRequest{
			ResultId: "test",
			Agent: &pb.AgentInfo{
				NodeName:    "node-" + string(rune('a'+i)),
				NetworkMode: pb.NetworkMode_NETWORK_MODE_POD,
			},
			Target: &pb.TargetInfo{Name: "target"},
			Check:  &pb.CheckInfo{Success: false, FailureCode: code},
		}
		engine.RecordFailure("target", result)
	}

	pattern := engine.DetectPattern("target", nil)
	assert.Equal(t, PatternCNIIssue, pattern)
}

func TestCorrelationEngineDetectPatternNodeRoutingIssue(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Add host-network only failures with DIFFERENT failure codes
	codes := []string{"timeout", "connection_reset"}
	for i, code := range codes {
		result := &pb.SubmitResultRequest{
			ResultId: "test",
			Agent: &pb.AgentInfo{
				NodeName:    "node-" + string(rune('a'+i)),
				NetworkMode: pb.NetworkMode_NETWORK_MODE_HOST,
			},
			Target: &pb.TargetInfo{Name: "target"},
			Check:  &pb.CheckInfo{Success: false, FailureCode: code},
		}
		engine.RecordFailure("target", result)
	}

	pattern := engine.DetectPattern("target", nil)
	assert.Equal(t, PatternNodeRoutingIssue, pattern)
}

func TestCorrelationEngineDetectPatternNodeIssue(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Single node with multiple failures
	for i := 0; i < 3; i++ {
		result := &pb.SubmitResultRequest{
			ResultId: "test-" + string(rune('0'+i)),
			Agent: &pb.AgentInfo{
				NodeName: "node-1",
			},
			Target: &pb.TargetInfo{Name: "target-" + string(rune('0'+i))},
			Check:  &pb.CheckInfo{Success: false},
		}
		engine.RecordFailure("target-"+string(rune('0'+i)), result)
	}

	// Each target has single node failure
	pattern := engine.DetectPattern("target-0", nil)
	assert.Equal(t, PatternUnknown, pattern)
}

func TestCorrelationEngineGenerateReport(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Add failures
	for i := 0; i < 2; i++ {
		result := &pb.SubmitResultRequest{
			ResultId: "test",
			Agent: &pb.AgentInfo{
				NodeName: "node-" + string(rune('a'+i)),
			},
			Target: &pb.TargetInfo{Name: "target"},
			Check: &pb.CheckInfo{
				Success:      false,
				FailureCode:  "tcp_refused",
				FailureLayer: "L2",
			},
		}
		engine.RecordFailure("target", result)
	}

	report := engine.GenerateReport("target", nil)

	assert.NotNil(t, report)
	assert.Equal(t, "target", report.Target)
	assert.Equal(t, "L2", report.FailureLayer)
	assert.Len(t, report.AffectedNodes, 2)
	assert.Equal(t, true, report.Ongoing)
}

func TestCorrelationEngineGenerateReportNoEvents(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	report := engine.GenerateReport("nonexistent", nil)

	assert.Nil(t, report)
}

func TestCorrelationEngineClearTarget(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Add failure
	result := &pb.SubmitResultRequest{
		ResultId: "test",
		Agent:    &pb.AgentInfo{NodeName: "node-1"},
		Target:   &pb.TargetInfo{Name: "target"},
		Check:    &pb.CheckInfo{Success: false},
	}
	engine.RecordFailure("target", result)

	assert.Equal(t, 1, engine.GetFailureCount("target"))

	// Clear target
	engine.ClearTarget("target")

	assert.Equal(t, 0, engine.GetFailureCount("target"))
}

func TestCorrelationEngineGetStats(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Add failures to multiple targets
	for i := 0; i < 3; i++ {
		result := &pb.SubmitResultRequest{
			ResultId: "test",
			Agent:    &pb.AgentInfo{NodeName: "node-1"},
			Target:   &pb.TargetInfo{Name: "target-" + string(rune('0'+i))},
			Check:    &pb.CheckInfo{Success: false},
		}
		engine.RecordFailure("target-"+string(rune('0'+i)), result)
	}

	stats := engine.GetStats()

	assert.Equal(t, 3, stats.TotalTargets)
	assert.Equal(t, 3, stats.TotalEvents)
	assert.Equal(t, 60, stats.WindowSeconds)
}

func TestCorrelationEngineCleanupOldEvents(t *testing.T) {
	cfg := &CorrelationConfig{
		TimeWindow:         100 * time.Millisecond,
		MinNodesForPattern: 2,
	}
	engine := NewCorrelationEngine(cfg)

	// Add failure
	result := &pb.SubmitResultRequest{
		ResultId: "test",
		Agent:    &pb.AgentInfo{NodeName: "node-1"},
		Target:   &pb.TargetInfo{Name: "target"},
		Check:    &pb.CheckInfo{Success: false},
	}
	engine.RecordFailure("target", result)

	assert.Equal(t, 1, engine.GetFailureCount("target"))

	// Wait for event to expire
	time.Sleep(150 * time.Millisecond)

	// Add new event to trigger cleanup
	engine.RecordFailure("target", result)

	// Old event should be cleaned up
	count := engine.GetFailureCount("target")
	assert.Equal(t, 1, count)
}

func TestCorrelationEngineMultipleFailureCodes(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	engine := NewCorrelationEngine(cfg)

	// Add failures with different failure codes
	codes := []string{"tcp_refused", "timeout", "dns_timeout"}
	for i, code := range codes {
		result := &pb.SubmitResultRequest{
			ResultId: "test",
			Agent: &pb.AgentInfo{
				NodeName: "node-" + string(rune('a'+i)),
			},
			Target: &pb.TargetInfo{Name: "target"},
			Check: &pb.CheckInfo{
				Success:      false,
				FailureCode:  code,
				FailureLayer: "L2",
			},
		}
		engine.RecordFailure("target", result)
	}

	pattern := engine.DetectPattern("target", nil)
	// Should not be target outage because failure codes differ
	assert.NotEqual(t, PatternTargetOutage, pattern)
}
