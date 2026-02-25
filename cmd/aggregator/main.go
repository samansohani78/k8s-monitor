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

// Package main is the entry point for the K8sWatch aggregator.
// The aggregator collects results from agents and correlates failures.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/k8swatch/k8s-monitor/internal/aggregator"
	pb "github.com/k8swatch/k8s-monitor/internal/pb"
)

var (
	Version   = "0.1.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// AggregatorServer holds all aggregator components
type AggregatorServer struct {
	config        *Config
	kubeClient    kubernetes.Interface
	ctrlClient    client.Client
	grpcServer    *grpc.Server
	httpServer    *http.Server
	server        *aggregator.Server
	processor     *aggregator.StreamProcessor
	topology      *aggregator.TopologyAnalyzer
	correlation   *aggregator.CorrelationEngine
	alertEngine   *aggregator.AlertDecisionEngine
	healthServer  *health.Server
	resultHandler *ResultHandler
	shutdown      chan struct{}
}

// Config holds aggregator configuration
type Config struct {
	GRPCAddress string
	HTTPAddress string
	Kubeconfig  string
	Verbose     bool
}

func main() {
	// Command-line flags
	var (
		kubeconfig  string
		verbose     bool
		showVersion bool
		grpcAddress string
		httpAddress string
	)

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.StringVar(&grpcAddress, "grpc-address", ":50051", "gRPC server address")
	flag.StringVar(&httpAddress, "http-address", ":8080", "HTTP server address")
	flag.Parse()

	if showVersion {
		fmt.Printf("K8sWatch Aggregator\n")
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
	aggregator.SetLogger(ctrlLog)

	// Create configuration
	cfg := &Config{
		GRPCAddress: grpcAddress,
		HTTPAddress: httpAddress,
		Kubeconfig:  kubeconfig,
		Verbose:     verbose,
	}

	// Create aggregator
	agg, err := NewAggregator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create aggregator: %v\n", err)
		os.Exit(1)
	}

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

	// Start aggregator
	fmt.Printf("Starting K8sWatch Aggregator %s...\n", Version)
	fmt.Printf("gRPC Address: %s\n", cfg.GRPCAddress)
	fmt.Printf("HTTP Address: %s\n", cfg.HTTPAddress)

	if err := agg.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Aggregator failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Aggregator stopped")
}

// NewAggregator creates a new aggregator instance
func NewAggregator(cfg *Config) (*AggregatorServer, error) {
	// Create Kubernetes clients
	var restConfig *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		restConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create kubeconfig: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Create controller-runtime client
	scheme := k8swatchv1.GetScheme()
	ctrlClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create controller-runtime client: %w", err)
	}

	// Create components
	processor := aggregator.NewStreamProcessor(aggregator.DefaultProcessorConfig())
	topology := aggregator.NewTopologyAnalyzer(aggregator.DefaultTopologyConfig())
	correlation := aggregator.NewCorrelationEngine(aggregator.DefaultCorrelationConfig())
	alertEngine := aggregator.NewAlertDecisionEngine(aggregator.DefaultAlertEngineConfig())

	// Create result handler
	resultHandler := NewResultHandler(processor, correlation, alertEngine, topology)

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxConcurrentStreams(100),
	)

	// Create aggregator server
	server := aggregator.NewServer(aggregator.DefaultServerConfig(), resultHandler)

	// Register services
	pb.RegisterResultServiceServer(grpcServer, &gRPCServerImpl{
		server: server,
	})

	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Enable reflection for grpc_cli
	reflection.Register(grpcServer)

	// Create HTTP server for metrics and health
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &AggregatorServer{
		config:        cfg,
		kubeClient:    kubeClient,
		ctrlClient:    ctrlClient,
		grpcServer:    grpcServer,
		httpServer:    httpServer,
		server:        server,
		processor:     processor,
		topology:      topology,
		correlation:   correlation,
		alertEngine:   alertEngine,
		healthServer:  healthServer,
		resultHandler: resultHandler,
		shutdown:      make(chan struct{}),
	}, nil
}

// Start starts the aggregator
func (a *AggregatorServer) Start(ctx context.Context) error {
	// Start gRPC server
	grpcListener, err := net.Listen("tcp", a.config.GRPCAddress)
	if err != nil {
		return fmt.Errorf("failed to create gRPC listener: %w", err)
	}

	go func() {
		fmt.Printf("gRPC server listening on %s\n", a.config.GRPCAddress)
		if err := a.grpcServer.Serve(grpcListener); err != nil {
			fmt.Fprintf(os.Stderr, "gRPC server failed: %v\n", err)
		}
	}()

	// Start HTTP server
	go func() {
		fmt.Printf("HTTP server listening on %s\n", a.config.HTTPAddress)
		if err := a.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server failed: %v\n", err)
		}
	}()

	// Start topology watcher (watch nodes)
	go a.watchNodes(ctx)

	// Start state cleanup
	go a.cleanupLoop(ctx)

	// Print stats
	go a.statsLoop(ctx)

	// Wait for shutdown
	<-ctx.Done()

	// Graceful shutdown
	fmt.Println("Shutting down aggregator...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "HTTP server shutdown error: %v\n", err)
	}

	a.grpcServer.GracefulStop()

	fmt.Println("Aggregator shutdown complete")
	return nil
}

// watchNodes watches Kubernetes nodes and updates topology
func (a *AggregatorServer) watchNodes(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial load
	if err := a.loadNodes(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load nodes: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.loadNodes(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load nodes: %v\n", err)
			}
		}
	}
}

// loadNodes loads all nodes and updates topology
func (a *AggregatorServer) loadNodes(ctx context.Context) error {
	nodeList, err := a.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, node := range nodeList.Items {
		a.topology.UpdateNode(&node)
	}

	fmt.Printf("Topology updated: %d nodes, %d zones\n",
		a.topology.GetTotalNodes(),
		a.topology.GetTotalZones(),
	)

	return nil
}

// cleanupLoop periodically cleans up expired state
func (a *AggregatorServer) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expired := a.processor.CleanupExpiredStates()
			if expired > 0 {
				fmt.Printf("Cleaned up %d expired states\n", expired)
			}
		}
	}
}

// statsLoop periodically prints statistics
func (a *AggregatorServer) statsLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := a.server.GetStats()
			procStats := a.processor.GetStats()
			alertStats := a.alertEngine.GetStats()

			fmt.Printf("Stats - Results: %d received, %d rejected | Targets: %d total, %d healthy, %d unhealthy | Alerts: %d active\n",
				stats.ResultsReceived,
				stats.ResultsRejected,
				procStats.TotalTargets,
				procStats.HealthyCount,
				procStats.UnhealthyCount,
				alertStats.AlertingCount,
			)
		}
	}
}

// gRPCServerImpl implements the gRPC ResultService
type gRPCServerImpl struct {
	pb.UnimplementedResultServiceServer
	server *aggregator.Server
}

func (g *gRPCServerImpl) SubmitResult(ctx context.Context, req *pb.SubmitResultRequest) (*pb.SubmitResultResponse, error) {
	return g.server.SubmitResult(ctx, req)
}

func (g *gRPCServerImpl) SubmitResults(stream pb.ResultService_SubmitResultsServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		resp, err := g.server.SubmitResult(stream.Context(), req)
		if err != nil {
			return err
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (g *gRPCServerImpl) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	status := g.server.HealthCheck()
	return &pb.HealthCheckResponse{
		Status:           status.Status,
		Version:          status.Version,
		UptimeSeconds:    status.UptimeSeconds,
		ResultsProcessed: status.ResultsProcessed,
	}, nil
}

// ResultHandler handles submitted results
type ResultHandler struct {
	processor   *aggregator.StreamProcessor
	correlation *aggregator.CorrelationEngine
	alertEngine *aggregator.AlertDecisionEngine
	topology    *aggregator.TopologyAnalyzer
}

// NewResultHandler creates a new result handler
func NewResultHandler(
	processor *aggregator.StreamProcessor,
	correlation *aggregator.CorrelationEngine,
	alertEngine *aggregator.AlertDecisionEngine,
	topology *aggregator.TopologyAnalyzer,
) *ResultHandler {
	return &ResultHandler{
		processor:   processor,
		correlation: correlation,
		alertEngine: alertEngine,
		topology:    topology,
	}
}

// HandleResult processes a submitted result
func (h *ResultHandler) HandleResult(ctx context.Context, result *pb.SubmitResultRequest) error {
	// Process result and update state
	if err := h.processor.ProcessResult(ctx, result); err != nil {
		return fmt.Errorf("failed to process result: %w", err)
	}

	// Record failure for correlation if check failed
	if !result.Check.Success {
		targetKey := result.Target.Namespace + "/" + result.Target.Name
		h.correlation.RecordFailure(targetKey, result)
	}

	// Make alerting decision
	targetKey := result.Target.Namespace + "/" + result.Target.Name
	decision := h.alertEngine.ProcessResult(
		targetKey,
		result.Check.Success,
		result.Check.FailureCode,
		result.Check.FailureLayer,
	)

	// Handle alert state changes
	if decision.ShouldAlert {
		fmt.Printf("ALERT: Target %s - %s\n", targetKey, decision.Reason)
		// TODO: Create AlertEvent CR
	}

	if decision.ShouldResolve {
		fmt.Printf("RESOLVED: Target %s - %s\n", targetKey, decision.Reason)
		// TODO: Update AlertEvent CR
	}

	return nil
}
