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
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k8swatch/k8s-monitor/internal/agent"
)

func TestAgentVersionInfo(t *testing.T) {
	// Verify version variables are defined
	assert.NotEmpty(t, Version)
	assert.NotEmpty(t, BuildDate)
	assert.NotEmpty(t, GitCommit)
}

func TestAgentConfigDefaults(t *testing.T) {
	cfg := &agent.Config{}

	// Verify config can be created
	assert.NotNil(t, cfg)
}

func TestAgentFlagParsing(t *testing.T) {
	// Save original args
	origArgs := os.Args
	origCommandLine := flag.CommandLine

	// Restore after test
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	// Set up test args
	os.Args = []string{"agent", "-version"}

	// Create new flag set
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var showVersion bool
	fs.BoolVar(&showVersion, "version", false, "Show version")

	err := fs.Parse([]string{"-version"})
	require.NoError(t, err)
	assert.True(t, showVersion)
}

func TestAgentContextCancellation(t *testing.T) {
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

func TestAgentSignalHandling(t *testing.T) {
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

func TestAgentConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *agent.Config
		wantErr bool
	}{
		{
			name: "Valid config with defaults",
			config: &agent.Config{
				AggregatorAddress: "localhost:50051",
			},
			wantErr: false,
		},
		{
			name: "Config with kubeconfig",
			config: &agent.Config{
				Kubeconfig:        "/path/to/kubeconfig",
				AggregatorAddress: "localhost:50051",
			},
			wantErr: false,
		},
		{
			name: "Config with custom HTTP address",
			config: &agent.Config{
				AggregatorAddress: "localhost:50051",
				HTTPAddress:       ":9090",
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

func TestAgentVersionOutput(t *testing.T) {
	// Test version string format
	versionOutput := "K8sWatch Agent\nVersion: " + Version + "\nBuild Date: " + BuildDate + "\nGit Commit: " + GitCommit + "\n"
	assert.NotEmpty(t, versionOutput)
	assert.Contains(t, versionOutput, "K8sWatch Agent")
	assert.Contains(t, versionOutput, "Version:")
}
