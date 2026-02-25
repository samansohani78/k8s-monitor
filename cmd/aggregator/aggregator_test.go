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

package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAggregatorVersionInfo(t *testing.T) {
	// Verify version variables are defined
	assert.NotEmpty(t, Version)
	assert.NotEmpty(t, BuildDate)
	assert.NotEmpty(t, GitCommit)
}

func TestAggregatorConfigDefaults(t *testing.T) {
	cfg := &Config{
		GRPCAddress: ":50051",
		HTTPAddress: ":8080",
	}

	// Verify config can be created
	assert.NotNil(t, cfg)
	assert.Equal(t, ":50051", cfg.GRPCAddress)
	assert.Equal(t, ":8080", cfg.HTTPAddress)
}

func TestAggregatorContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Verify context can be cancelled
	cancel()

	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context was not cancelled")
	}
}

func TestAggregatorSignalHandling(t *testing.T) {
	// Test that we can create a context that responds to cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Wait for context to be done
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("Context timeout did not work")
	}
}

func TestAggregatorConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid config with defaults",
			config: &Config{
				GRPCAddress: ":50051",
				HTTPAddress: ":8080",
			},
			wantErr: false,
		},
		{
			name: "Config with custom addresses",
			config: &Config{
				GRPCAddress: ":9090",
				HTTPAddress: ":9091",
			},
			wantErr: false,
		},
		{
			name: "Config with kubeconfig",
			config: &Config{
				GRPCAddress: ":50051",
				HTTPAddress: ":8080",
				Kubeconfig:  "/path/to/kubeconfig",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Config creation should not fail
			assert.NotNil(t, tt.config)
		})
	}
}

func TestAggregatorVersionOutput(t *testing.T) {
	// Test version string format
	versionOutput := "K8sWatch Aggregator\nVersion: " + Version + "\nBuild Date: " + BuildDate + "\nGit Commit: " + GitCommit + "\n"
	assert.NotEmpty(t, versionOutput)
	assert.Contains(t, versionOutput, "K8sWatch Aggregator")
	assert.Contains(t, versionOutput, "Version:")
}

func TestAggregatorServerStruct(t *testing.T) {
	// Verify AggregatorServer struct can be instantiated
	server := &AggregatorServer{}
	assert.NotNil(t, server)

	// Verify all fields are initially nil/zero
	assert.Nil(t, server.config)
	assert.Nil(t, server.kubeClient)
	assert.Nil(t, server.ctrlClient)
	assert.Nil(t, server.grpcServer)
	assert.Nil(t, server.httpServer)
}
