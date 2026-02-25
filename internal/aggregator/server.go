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
	"fmt"
	"sync"
	"time"

	pb "github.com/k8swatch/k8s-monitor/internal/pb"
)

// ServerConfig holds aggregator server configuration
type ServerConfig struct {
	MaxQueueSize   int
	ProcessTimeout time.Duration
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		MaxQueueSize:   1000,
		ProcessTimeout: 30 * time.Second,
	}
}

// Server is the aggregator server (simplified version)
type Server struct {
	config          *ServerConfig
	resultHandler   ResultHandler
	mu              sync.RWMutex
	startTime       time.Time
	resultsReceived int64
	resultsRejected int64
}

// ResultHandler handles submitted results
type ResultHandler interface {
	HandleResult(ctx context.Context, result *pb.SubmitResultRequest) error
}

// NewServer creates new aggregator server
func NewServer(config *ServerConfig, handler ResultHandler) *Server {
	if config == nil {
		config = DefaultServerConfig()
	}

	return &Server{
		config:        config,
		resultHandler: handler,
		startTime:     time.Now(),
	}
}

// SubmitResult submits single check result
func (s *Server) SubmitResult(ctx context.Context, req *pb.SubmitResultRequest) (*pb.SubmitResultResponse, error) {
	startTime := time.Now()

	// Check for nil request first
	if req == nil {
		s.mu.Lock()
		s.resultsRejected++
		s.mu.Unlock()
		return &pb.SubmitResultResponse{
			Accepted: false,
			Error:    "request is nil",
		}, nil
	}

	if err := s.validateRequest(req); err != nil {
		s.mu.Lock()
		s.resultsRejected++
		s.mu.Unlock()

		log.Info("Result validation failed", "resultId", req.ResultId, "error", err)

		return &pb.SubmitResultResponse{
			Accepted: false,
			Error:    err.Error(),
		}, nil
	}

	if s.resultHandler != nil {
		if err := s.resultHandler.HandleResult(ctx, req); err != nil {
			s.mu.Lock()
			s.resultsRejected++
			s.mu.Unlock()

			log.Error(err, "Failed to handle result", "resultId", req.ResultId)

			return &pb.SubmitResultResponse{
				Accepted: false,
				Error:    fmt.Sprintf("failed to process result: %v", err),
			}, nil
		}
	}

	s.mu.Lock()
	s.resultsReceived++
	s.mu.Unlock()

	log.Info("Result accepted",
		"resultId", req.ResultId,
		"target", req.Target.Name,
		"success", req.Check.Success,
		"durationMs", time.Since(startTime).Milliseconds(),
	)

	return &pb.SubmitResultResponse{Accepted: true}, nil
}

// validateRequest validates submit result request
func (s *Server) validateRequest(req *pb.SubmitResultRequest) error {
	if req.ResultId == "" {
		return fmt.Errorf("resultId is required")
	}
	if req.Agent == nil {
		return fmt.Errorf("agent info is required")
	}
	if req.Agent.NodeName == "" {
		return fmt.Errorf("agent nodeName is required")
	}
	if req.Target == nil {
		return fmt.Errorf("target info is required")
	}
	if req.Target.Name == "" {
		return fmt.Errorf("target name is required")
	}
	if req.Check == nil {
		return fmt.Errorf("check info is required")
	}
	return nil
}

// GetStats returns server statistics
func (s *Server) GetStats() ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ServerStats{
		StartTime:       s.startTime,
		UptimeSeconds:   int64(time.Since(s.startTime).Seconds()),
		ResultsReceived: s.resultsReceived,
		ResultsRejected: s.resultsRejected,
	}
}

// ServerStats contains server statistics
type ServerStats struct {
	StartTime       time.Time
	UptimeSeconds   int64
	ResultsReceived int64
	ResultsRejected int64
}

// HealthStatus represents health check response
type HealthStatus struct {
	Status           string
	Version          string
	UptimeSeconds    int64
	ResultsProcessed int64
}

// HealthCheck returns health status
func (s *Server) HealthCheck() HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return HealthStatus{
		Status:           "healthy",
		Version:          "1.0.0",
		UptimeSeconds:    int64(time.Since(s.startTime).Seconds()),
		ResultsProcessed: s.resultsReceived,
	}
}
