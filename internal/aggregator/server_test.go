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
	"github.com/stretchr/testify/require"

	pb "github.com/k8swatch/k8s-monitor/internal/pb"
)

// mockResultHandler is a mock result handler for testing
type mockResultHandler struct {
	handleFunc func(ctx context.Context, result *pb.SubmitResultRequest) error
}

func (m *mockResultHandler) HandleResult(ctx context.Context, result *pb.SubmitResultRequest) error {
	if m.handleFunc != nil {
		return m.handleFunc(ctx, result)
	}
	return nil
}

func TestServerConfigDefaults(t *testing.T) {
	cfg := DefaultServerConfig()

	assert.Equal(t, 1000, cfg.MaxQueueSize)
	assert.Equal(t, 30*time.Second, cfg.ProcessTimeout)
}

func TestServerCreation(t *testing.T) {
	cfg := DefaultServerConfig()
	handler := &mockResultHandler{}

	server := NewServer(cfg, handler)

	assert.NotNil(t, server)
	assert.Equal(t, cfg, server.config)
	assert.NotNil(t, server.resultHandler)
}

func TestServerCreationNilConfig(t *testing.T) {
	handler := &mockResultHandler{}

	server := NewServer(nil, handler)

	assert.NotNil(t, server)
	assert.Equal(t, 1000, server.config.MaxQueueSize)
}

func TestServerSubmitResultValid(t *testing.T) {
	handler := &mockResultHandler{
		handleFunc: func(ctx context.Context, result *pb.SubmitResultRequest) error {
			return nil
		},
	}

	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName: "test-node",
		},
		Target: &pb.TargetInfo{
			Name: "test-target",
			Type: "http",
		},
		Check: &pb.CheckInfo{
			Success: true,
		},
	}

	ctx := context.Background()
	resp, err := server.SubmitResult(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Accepted)
	assert.Empty(t, resp.Error)
}

func TestServerSubmitResultNilRequest(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	ctx := context.Background()
	resp, err := server.SubmitResult(ctx, nil)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Accepted)
	assert.Contains(t, resp.Error, "request is nil")
}

func TestServerSubmitResultMissingResultId(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	req := &pb.SubmitResultRequest{
		Agent: &pb.AgentInfo{
			NodeName: "test-node",
		},
		Target: &pb.TargetInfo{
			Name: "test-target",
		},
		Check: &pb.CheckInfo{
			Success: true,
		},
	}

	ctx := context.Background()
	resp, err := server.SubmitResult(ctx, req)

	require.NoError(t, err)
	assert.False(t, resp.Accepted)
	assert.Contains(t, resp.Error, "resultId is required")
}

func TestServerSubmitResultMissingAgent(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Target: &pb.TargetInfo{
			Name: "test-target",
		},
		Check: &pb.CheckInfo{
			Success: true,
		},
	}

	ctx := context.Background()
	resp, err := server.SubmitResult(ctx, req)

	require.NoError(t, err)
	assert.False(t, resp.Accepted)
	assert.Contains(t, resp.Error, "agent info is required")
}

func TestServerSubmitResultMissingTarget(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName: "test-node",
		},
		Check: &pb.CheckInfo{
			Success: true,
		},
	}

	ctx := context.Background()
	resp, err := server.SubmitResult(ctx, req)

	require.NoError(t, err)
	assert.False(t, resp.Accepted)
	assert.Contains(t, resp.Error, "target info is required")
}

func TestServerSubmitResultMissingCheck(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName: "test-node",
		},
		Target: &pb.TargetInfo{
			Name: "test-target",
		},
	}

	ctx := context.Background()
	resp, err := server.SubmitResult(ctx, req)

	require.NoError(t, err)
	assert.False(t, resp.Accepted)
	assert.Contains(t, resp.Error, "check info is required")
}

func TestServerSubmitResultHandlerError(t *testing.T) {
	handler := &mockResultHandler{
		handleFunc: func(ctx context.Context, result *pb.SubmitResultRequest) error {
			return assert.AnError
		},
	}

	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName: "test-node",
		},
		Target: &pb.TargetInfo{
			Name: "test-target",
		},
		Check: &pb.CheckInfo{
			Success: true,
		},
	}

	ctx := context.Background()
	resp, err := server.SubmitResult(ctx, req)

	require.NoError(t, err)
	assert.False(t, resp.Accepted)
	assert.Contains(t, resp.Error, "failed to process result")
}

func TestServerHealthCheck(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	// Submit a result to increment counter
	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName: "test-node",
		},
		Target: &pb.TargetInfo{
			Name: "test-target",
		},
		Check: &pb.CheckInfo{
			Success: true,
		},
	}

	ctx := context.Background()
	_, _ = server.SubmitResult(ctx, req)

	// Check health
	status := server.HealthCheck()

	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, "1.0.0", status.Version)
	assert.GreaterOrEqual(t, status.UptimeSeconds, int64(0))
	assert.Equal(t, int64(1), status.ResultsProcessed)
}

func TestServerGetStats(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	stats := server.GetStats()

	assert.GreaterOrEqual(t, stats.UptimeSeconds, int64(0))
	assert.Equal(t, int64(0), stats.ResultsReceived)
	assert.Equal(t, int64(0), stats.ResultsRejected)
}

func TestServerSubmitResultMetrics(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	// Submit successful result
	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent:    &pb.AgentInfo{NodeName: "test-node"},
		Target:   &pb.TargetInfo{Name: "test-target"},
		Check:    &pb.CheckInfo{Success: true},
	}

	ctx := context.Background()
	_, _ = server.SubmitResult(ctx, req)

	stats := server.GetStats()
	assert.Equal(t, int64(1), stats.ResultsReceived)
	assert.Equal(t, int64(0), stats.ResultsRejected)

	// Submit invalid result
	_, _ = server.SubmitResult(ctx, nil)

	stats = server.GetStats()
	assert.Equal(t, int64(1), stats.ResultsReceived)
	assert.Equal(t, int64(1), stats.ResultsRejected)
}

func TestServerValidateRequestComplete(t *testing.T) {
	handler := &mockResultHandler{}
	cfg := DefaultServerConfig()
	server := NewServer(cfg, handler)

	// Complete valid request
	req := &pb.SubmitResultRequest{
		ResultId: "test-123",
		Agent: &pb.AgentInfo{
			NodeName: "test-node",
			NodeZone: "zone-a",
		},
		Target: &pb.TargetInfo{
			Name:      "test-target",
			Namespace: "default",
			Type:      "http",
		},
		Check: &pb.CheckInfo{
			Success:      true,
			FinalLayer:   "L4",
			FailureCode:  "",
			FailureLayer: "",
		},
	}

	ctx := context.Background()
	resp, err := server.SubmitResult(ctx, req)

	require.NoError(t, err)
	assert.True(t, resp.Accepted)
}
