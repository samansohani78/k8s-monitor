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
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	pb "github.com/k8swatch/k8s-monitor/internal/pb"
	tlsconfig "github.com/k8swatch/k8s-monitor/internal/tls"
)

// ResultClientConfig holds the result client configuration
type ResultClientConfig struct {
	// AggregatorAddress is the gRPC address of the aggregator
	AggregatorAddress string

	// MaxRetries is the maximum number of retries on failure
	MaxRetries int

	// RetryBackoff is the base backoff duration between retries
	RetryBackoff time.Duration

	// Timeout is the timeout for each result submission
	Timeout time.Duration

	// TLS configuration for mTLS
	TLS *tlsconfig.TLSConfig
}

// DefaultResultClientConfig returns the default result client configuration
func DefaultResultClientConfig() *ResultClientConfig {
	return &ResultClientConfig{
		AggregatorAddress: "k8swatch-aggregator.k8swatch.svc:50051",
		MaxRetries:        3,
		RetryBackoff:      time.Second,
		Timeout:           10 * time.Second,
	}
}

// ResultClient sends check results to the aggregator via gRPC
type ResultClient struct {
	config       *ResultClientConfig
	conn         *grpc.ClientConn
	client       pb.ResultServiceClient
	nodeName     string
	nodeZone     string
	agentVersion string
	networkMode  pb.NetworkMode
}

// NewResultClient creates a new result client
func NewResultClient(config *ResultClientConfig, nodeName, nodeZone, agentVersion string) (*ResultClient, error) {
	if config == nil {
		config = DefaultResultClientConfig()
	}

	// Create gRPC credentials
	var creds credentials.TransportCredentials
	if config.TLS != nil {
		// Load mTLS configuration
		tlsConfig, err := tlsconfig.LoadClientTLSConfig(config.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		creds = credentials.NewTLS(tlsConfig)
		log.Info("mTLS enabled for aggregator connection")
	} else {
		// Use insecure credentials (development only)
		creds = insecure.NewCredentials()
		log.Info("Using insecure connection to aggregator (development mode)")
	}

	// Create gRPC connection
	conn, err := grpc.NewClient(config.AggregatorAddress,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	client := pb.NewResultServiceClient(conn)

	return &ResultClient{
		config:       config,
		conn:         conn,
		client:       client,
		nodeName:     nodeName,
		nodeZone:     nodeZone,
		agentVersion: agentVersion,
		networkMode:  resolveNetworkModeFromEnv(),
	}, nil
}

// Close closes the gRPC connection
func (c *ResultClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SubmitResult submits a check result to the aggregator with retry
func (c *ResultClient) SubmitResult(ctx context.Context, result *k8swatchv1.CheckResult) error {
	var lastErr error

	for attempt := 0; attempt < c.config.MaxRetries; attempt++ {
		if err := c.submitResultWithTimeout(ctx, result); err != nil {
			lastErr = err
			log.Error(err, "Result submission failed, retrying",
				"attempt", attempt+1,
				"maxRetries", c.config.MaxRetries,
				"resultId", result.ResultID,
			)

			// Backoff before retry
			backoff := c.config.RetryBackoff * time.Duration(1<<uint(attempt)) // nolint:gosec // attempt is bounded by MaxRetries (3), no overflow risk
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		// Success
		return nil
	}

	return fmt.Errorf("failed to submit result after %d attempts: %w", c.config.MaxRetries, lastErr)
}

// submitResultWithTimeout submits a result with timeout
func (c *ResultClient) submitResultWithTimeout(ctx context.Context, result *k8swatchv1.CheckResult) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	req := c.buildSubmitRequest(result)

	_, err := c.client.SubmitResult(timeoutCtx, req)
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	log.Info("Result submitted successfully",
		"resultId", result.ResultID,
		"target", result.Target.Name,
		"success", result.Check.Success,
	)

	return nil
}

// buildSubmitRequest builds a gRPC submit request from a check result
func (c *ResultClient) buildSubmitRequest(result *k8swatchv1.CheckResult) *pb.SubmitResultRequest {
	req := &pb.SubmitResultRequest{
		ResultId:  result.ResultID,
		Timestamp: timestamppb.New(result.Timestamp.Time),
		Agent: &pb.AgentInfo{
			NodeName:     c.nodeName,
			NodeZone:     c.nodeZone,
			NetworkMode:  c.resolveNetworkModeFromResult(result),
			AgentVersion: c.agentVersion,
		},
		Target: &pb.TargetInfo{
			Name:      result.Target.Name,
			Namespace: result.Target.Namespace,
			Type:      string(result.Target.Type),
			Labels:    result.Target.Labels,
		},
		Check: &pb.CheckInfo{
			LayersEnabled:  result.Check.LayersEnabled,
			FinalLayer:     result.Check.FinalLayer,
			Success:        result.Check.Success,
			FailureLayer:   result.Check.FailureLayer,
			FailureCode:    result.Check.FailureCode,
			FailureMessage: result.Check.FailureMessage,
		},
		Latencies: make(map[string]*pb.LayerLatency),
		Metadata: &pb.CheckMetadata{
			CheckDurationMs: result.Metadata.CheckDurationMs,
			AttemptNumber:   result.Metadata.AttemptNumber,
			ConfigVersion:   result.Metadata.ConfigVersion,
			Error:           result.Metadata.Error,
		},
	}

	// Convert latencies
	for layer, latency := range result.Latencies {
		req.Latencies[layer] = &pb.LayerLatency{
			DurationMs: latency.DurationMs,
			Success:    latency.Success,
		}
	}

	return req
}

func resolveNetworkModeFromEnv() pb.NetworkMode {
	switch strings.ToLower(os.Getenv("K8SWATCH_NETWORK_MODE")) {
	case "host":
		return pb.NetworkMode_NETWORK_MODE_HOST
	default:
		return pb.NetworkMode_NETWORK_MODE_POD
	}
}

func (c *ResultClient) resolveNetworkModeFromResult(result *k8swatchv1.CheckResult) pb.NetworkMode {
	if result != nil {
		for _, key := range []string{"k8swatch.io/network-mode", "network-mode", "network_mode"} {
			if result.Target.Labels[key] == "host" {
				return pb.NetworkMode_NETWORK_MODE_HOST
			}
		}
	}
	return c.networkMode
}

// HealthCheck checks the health of the aggregator
func (c *ResultClient) HealthCheck(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.HealthCheck(timeoutCtx, &pb.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// IsConnected returns true if the client is connected to the aggregator
func (c *ResultClient) IsConnected() bool {
	return c.conn != nil && c.conn.GetState().String() != "Shutdown"
}
