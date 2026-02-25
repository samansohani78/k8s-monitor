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

// Package main is the entry point for the K8sWatch agent.
// The agent runs as a DaemonSet on each node and executes health checks.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"

	"github.com/k8swatch/k8s-monitor/internal/agent"
)

var (
	Version   = "0.1.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Command-line flags
	var (
		kubeconfig  string
		verbose     bool
		showVersion bool
		aggregator  string
		httpAddress string
	)

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.StringVar(&aggregator, "aggregator", "k8swatch-aggregator.k8swatch.svc:50051", "Aggregator gRPC address")
	flag.StringVar(&httpAddress, "http-address", ":8080", "HTTP server address for metrics and health")
	flag.Parse()

	if showVersion {
		fmt.Printf("K8sWatch Agent\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Date: %s\n", BuildDate)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Set up logger
	zapConfig := zap.NewProductionConfig()
	if verbose {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	zapLogger, err := zapConfig.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	ctrlLog := zapr.NewLogger(zapLogger)

	// Create agent configuration
	cfg := &agent.Config{
		Kubeconfig:        kubeconfig,
		AggregatorAddress: aggregator,
		HTTPAddress:       httpAddress,
		AgentVersion:      Version,
		Verbose:           verbose,
	}

	// Create agent
	a, err := agent.NewAgent(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	// Set logger for agent package
	agent.SetLogger(ctrlLog)

	// Set up signal handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
		cancel()
	}()

	// Start agent
	fmt.Printf("Starting K8sWatch Agent %s...\n", Version)
	fmt.Printf("Node: %s\n", a.NodeName())
	fmt.Printf("Zone: %s\n", a.NodeZone())
	fmt.Printf("Aggregator: %s\n", cfg.AggregatorAddress)
	fmt.Printf("HTTP Address: %s\n", cfg.HTTPAddress)

	if err := a.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Agent failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Agent stopped")
}
