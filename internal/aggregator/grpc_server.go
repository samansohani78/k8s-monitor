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
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	pb "github.com/k8swatch/k8s-monitor/internal/pb"
	tlsconfig "github.com/k8swatch/k8s-monitor/internal/tls"
)

// GRPCServerConfig holds gRPC server configuration
type GRPCServerConfig struct {
	// Address is the listen address
	Address string

	// TLS configuration for mTLS
	TLS *tlsconfig.TLSConfig

	// MaxConcurrentStreams is the maximum number of concurrent streams
	MaxConcurrentStreams uint32

	// Keepalive configuration
	Keepalive *KeepaliveConfig
}

// KeepaliveConfig holds keepalive configuration
type KeepaliveConfig struct {
	// ServerParameters
	MaxConnectionIdle     time.Duration
	MaxConnectionAge      time.Duration
	MaxConnectionAgeGrace time.Duration
	Time                  time.Duration
	Timeout               time.Duration

	// EnforcementPolicy
	MinTime             time.Duration
	PermitWithoutStream bool
}

// DefaultGRPCServerConfig returns default gRPC server configuration
func DefaultGRPCServerConfig() *GRPCServerConfig {
	return &GRPCServerConfig{
		Address:              ":50051",
		MaxConcurrentStreams: 100,
		Keepalive: &KeepaliveConfig{
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:      10 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Minute,
			Timeout:               20 * time.Second,
			MinTime:               5 * time.Minute,
			PermitWithoutStream:   false,
		},
	}
}

// GRPCServer is the gRPC server wrapper
type GRPCServer struct {
	config     *GRPCServerConfig
	server     *grpc.Server
	handler    *Server
	listener   net.Listener
	mu         sync.RWMutex
	isServing  bool
	shutdownCh chan struct{}
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(config *GRPCServerConfig, handler *Server) (*GRPCServer, error) {
	if config == nil {
		config = DefaultGRPCServerConfig()
	}

	// Create gRPC server options
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(config.MaxConcurrentStreams),
	}

	// Add TLS credentials if configured
	if config.TLS != nil {
		tlsConfig, err := tlsconfig.LoadServerTLSConfig(config.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
		log.Info("mTLS enabled for gRPC server")
	} else {
		log.Info("gRPC server running without TLS (development mode)")
	}

	// Add keepalive parameters
	if config.Keepalive != nil {
		opts = append(opts, grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     config.Keepalive.MaxConnectionIdle,
			MaxConnectionAge:      config.Keepalive.MaxConnectionAge,
			MaxConnectionAgeGrace: config.Keepalive.MaxConnectionAgeGrace,
			Time:                  config.Keepalive.Time,
			Timeout:               config.Keepalive.Timeout,
		}))

		opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             config.Keepalive.MinTime,
			PermitWithoutStream: config.Keepalive.PermitWithoutStream,
		}))
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(opts...)

	// Create adapter wrapper
	adapter := &grpcServerAdapter{server: handler}

	// Register result service
	pb.RegisterResultServiceServer(grpcServer, adapter)

	// Create listener
	listener, err := net.Listen("tcp", config.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	return &GRPCServer{
		config:     config,
		server:     grpcServer,
		handler:    handler,
		listener:   listener,
		shutdownCh: make(chan struct{}),
	}, nil
}

// Start starts the gRPC server
func (s *GRPCServer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isServing {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}
	s.isServing = true
	s.mu.Unlock()

	log.Info("Starting gRPC server", "address", s.listener.Addr())

	// Start server in goroutine
	go func() {
		if err := s.server.Serve(s.listener); err != nil {
			select {
			case <-s.shutdownCh:
				// Expected shutdown
			default:
				log.Error(err, "gRPC server error")
			}
		}
	}()

	return nil
}

// Stop stops the gRPC server gracefully
func (s *GRPCServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isServing {
		return nil
	}

	log.Info("Stopping gRPC server")

	close(s.shutdownCh)

	// Graceful stop with timeout
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// Force stop
		s.server.Stop()
		return ctx.Err()
	}
}

// IsRunning returns true if the server is running
func (s *GRPCServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isServing
}

// GetAddress returns the listen address
func (s *GRPCServer) GetAddress() string {
	return s.listener.Addr().String()
}

// GetStats returns server statistics
func (s *GRPCServer) GetStats() ServerStats {
	return s.handler.GetStats()
}

// grpcServerAdapter adapts Server to pb.ResultServiceServer interface
type grpcServerAdapter struct {
	server *Server
}

// SubmitResult implements pb.ResultServiceServer
func (a *grpcServerAdapter) SubmitResult(ctx context.Context, req *pb.SubmitResultRequest) (*pb.SubmitResultResponse, error) {
	return a.server.SubmitResult(ctx, req)
}

// SubmitResults implements pb.ResultServiceServer (streaming)
func (a *grpcServerAdapter) SubmitResults(stream pb.ResultService_SubmitResultsServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		resp, err := a.server.SubmitResult(stream.Context(), req)
		if err != nil {
			return err
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

// HealthCheck implements pb.ResultServiceServer
func (a *grpcServerAdapter) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	status := a.server.HealthCheck()
	return &pb.HealthCheckResponse{
		Status:  status.Status,
		Version: status.Version,
	}, nil
}
