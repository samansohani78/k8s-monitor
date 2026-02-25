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
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/k8swatch/k8s-monitor/internal/checker"
)

// Config holds the agent configuration
type Config struct {
	// Kubeconfig is the path to kubeconfig file (empty for in-cluster)
	Kubeconfig string

	// AggregatorAddress is the gRPC address of the aggregator
	AggregatorAddress string

	// HTTPAddress is the HTTP server address for metrics and health
	HTTPAddress string

	// Namespace is the namespace where the agent runs
	Namespace string

	// NodeName is the name of the node where the agent runs
	NodeName string

	// AgentVersion is the version of the agent
	AgentVersion string

	// Verbose enables verbose logging
	Verbose bool
}

// Agent represents the K8sWatch monitoring agent
type Agent struct {
	config       *Config
	kubeClient   kubernetes.Interface
	client       client.Client
	restConfig   *rest.Config
	nodeName     string
	nodeZone     string
	agentVersion string

	// Components
	configLoader *ConfigLoader
	scheduler    *Scheduler
	resultClient *ResultClient
	checkerReg   *checker.Registry
	httpServer   *http.Server

	// State
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// NewAgent creates a new agent instance
func NewAgent(cfg *Config) (*Agent, error) {
	if cfg.AggregatorAddress == "" {
		cfg.AggregatorAddress = "k8swatch-aggregator.k8swatch.svc:50051"
	}

	if cfg.Namespace == "" {
		cfg.Namespace = getNamespace()
	}

	if cfg.NodeName == "" {
		cfg.NodeName = os.Getenv("NODE_NAME")
	}

	// Get kubeconfig
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

	// Create Kubernetes clientset
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Create controller-runtime client with our scheme
	scheme := runtime.NewScheme()
	_ = k8swatchv1.AddToScheme(scheme)

	ctrlClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create controller-runtime client: %w", err)
	}

	// Get node info
	nodeName := cfg.NodeName
	nodeZone := ""

	if nodeName != "" {
		node, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err == nil {
			nodeZone = node.Labels["topology.kubernetes.io/zone"]
			if nodeZone == "" {
				nodeZone = node.Labels["failure-domain.beta.kubernetes.io/zone"]
			}
		}
	}

	return &Agent{
		config:       cfg,
		kubeClient:   kubeClient,
		client:       ctrlClient,
		restConfig:   restConfig,
		nodeName:     nodeName,
		nodeZone:     nodeZone,
		agentVersion: cfg.AgentVersion,
		shutdown:     make(chan struct{}),
	}, nil
}

// Start starts the agent
func (a *Agent) Start(ctx context.Context) error {
	log.Info("Starting K8sWatch Agent",
		"version", a.agentVersion,
		"node", a.nodeName,
		"zone", a.nodeZone,
		"namespace", a.config.Namespace,
		"aggregator", a.config.AggregatorAddress,
	)

	// Verify connectivity to Kubernetes API
	if err := a.verifyKubeConnectivity(); err != nil {
		return fmt.Errorf("kubernetes API connectivity check failed: %w", err)
	}

	log.Info("Kubernetes API connectivity verified")

	// Initialize checker registry
	a.checkerReg = checker.NewRegistry()
	a.registerCheckers()

	// Initialize result client
	var err error
	a.resultClient, err = NewResultClient(
		DefaultResultClientConfig(),
		a.nodeName,
		a.nodeZone,
		a.agentVersion,
	)
	if err != nil {
		return fmt.Errorf("failed to create result client: %w", err)
	}
	defer a.resultClient.Close()

	// Initialize config loader
	a.configLoader = ConfigLoaderWithClient(a.client, a.config.Namespace)

	// Create check function
	checkFunc := a.executeCheck

	// Initialize scheduler
	a.scheduler = NewScheduler(DefaultSchedulerConfig(), checkFunc)

	// Start scheduler in a goroutine
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.scheduler.Start(ctx); err != nil {
			log.Error(err, "Scheduler failed")
		}
	}()

	// Start config refresh loop
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.refreshConfigLoop(ctx)
	}()

	// Start HTTP server for metrics and health
	a.startHTTPServer()

	// Wait for shutdown signal
	<-a.shutdown

	log.Info("Agent shutdown initiated, waiting for in-flight checks...")

	// Wait for all goroutines to complete (with timeout)
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("Agent shutdown complete")
	case <-time.After(30 * time.Second):
		log.Info("Agent shutdown timeout, forcing exit")
	}

	return nil
}

// Stop stops the agent gracefully
func (a *Agent) Stop() error {
	log.Info("Stopping agent...")
	close(a.shutdown)
	return nil
}

// startHTTPServer starts the HTTP server for metrics and health endpoints
func (a *Agent) startHTTPServer() {
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Ready endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	// Metrics endpoint
	mux.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)

	// Determine HTTP address from config or default
	httpAddress := a.config.HTTPAddress
	if httpAddress == "" {
		httpAddress = ":8080"
	}

	a.httpServer = &http.Server{
		Addr:              httpAddress,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start HTTP server in goroutine
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		log.Info("HTTP server started", "address", httpAddress)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, "HTTP server failed")
		}
	}()
}

// KubeClient returns the Kubernetes clientset
func (a *Agent) KubeClient() kubernetes.Interface {
	return a.kubeClient
}

// Client returns the controller-runtime client
func (a *Agent) Client() client.Client {
	return a.client
}

// RestConfig returns the REST config
func (a *Agent) RestConfig() *rest.Config {
	return a.restConfig
}

// NodeName returns the node name
func (a *Agent) NodeName() string {
	return a.nodeName
}

// NodeZone returns the node zone
func (a *Agent) NodeZone() string {
	return a.nodeZone
}

// AgentVersion returns the agent version
func (a *Agent) AgentVersion() string {
	return a.agentVersion
}

// verifyKubeConnectivity verifies connectivity to the Kubernetes API server
func (a *Agent) verifyKubeConnectivity() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to get the node
	_, err := a.kubeClient.CoreV1().Nodes().Get(ctx, a.nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	return nil
}

// registerCheckers registers all available checkers
// Currently only L0 node sanity checker is implemented
func (a *Agent) registerCheckers() {
	// Register L0 node sanity checker for all target types that support it
	l0Checker := checker.NewNodeSanityChecker(checker.DefaultNodeSanityConfig())

	// For Phase 1, we register a basic checker that only does L0
	// In Phase 2, we'll register full checkers for each target type
	targetTypes := []string{
		"network", "dns", "http", "https", "kubernetes",
		"redis", "postgresql", "mysql", "mssql", "mongodb", "clickhouse",
		"elasticsearch", "opensearch", "minio",
		"kafka", "rabbitmq",
		"keycloak", "nginx",
		"internal-canary", "external-http", "node-egress", "node-to-node",
	}

	for _, t := range targetTypes {
		a.checkerReg.Register(&basicCheckerFactory{l0Checker: l0Checker}, t)
	}

	log.Info("Checker registry initialized", "supportedTypes", len(targetTypes))
}

// basicCheckerFactory creates basic checkers with L0 support
type basicCheckerFactory struct {
	l0Checker *checker.NodeSanityChecker
}

func (f *basicCheckerFactory) Create(target *k8swatchv1.Target) (checker.Checker, error) {
	layers := []checker.Layer{f.l0Checker}
	return checker.NewBaseChecker(string(target.Spec.Type), layers), nil
}

func (f *basicCheckerFactory) SupportedTypes() []string {
	return []string{"all"}
}

// executeCheck executes a health check for a target
func (a *Agent) executeCheck(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	// Create context with correlation ID for this check
	ctx, opLog := newCheckContext(ctx, target.Name, target.Namespace, string(target.Spec.Type))

	// Validate target
	if err := ValidateTarget(target); err != nil {
		err = fmt.Errorf("invalid target configuration: %w", err)
		if opLog != nil {
			opLog.EndWithError(err)
		}
		return nil, err
	}

	// Create checker for target
	chk, err := a.checkerReg.Create(target)
	if err != nil {
		err = fmt.Errorf("failed to create checker: %w", err)
		if opLog != nil {
			opLog.EndWithError(err)
		}
		return nil, err
	}

	// Execute check
	result, err := chk.Execute(ctx, target)
	if err != nil {
		if opLog != nil {
			opLog.EndWithError(err,
				"target", target.Name,
				"namespace", target.Namespace,
			)
		}
		return nil, err
	}

	// Send result to aggregator
	if a.resultClient != nil {
		if err := a.resultClient.SubmitResult(ctx, result); err != nil {
			if opLog != nil {
				opLog.EndWithError(err,
					"resultId", result.ResultID,
					"target", result.Target.Name,
					"phase", "transmission",
				)
			}
			// Note: We don't return error here - the check itself succeeded
			// The result transmission failure is logged but not propagated
		} else {
			if opLog != nil {
				opLog.End(
					"resultId", result.ResultID,
					"success", result.Check.Success,
					"finalLayer", result.Check.FinalLayer,
				)
			}
		}
	} else {
		if opLog != nil {
			opLog.End(
				"success", result.Check.Success,
				"finalLayer", result.Check.FinalLayer,
			)
		}
	}

	return result, nil
}

// refreshConfigLoop periodically refreshes the configuration
// Stateless design: fetch fresh config each interval, no caching
func (a *Agent) refreshConfigLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.shutdown:
			return
		case <-ticker.C:
			targets, configVersion, err := a.configLoader.LoadTargets(ctx)
			if err != nil {
				log.Error(err, "Failed to load targets")
				continue
			}

			// Update scheduler with new targets
			a.scheduler.UpdateTargets(targets)

			log.Info("Configuration refreshed",
				"targetCount", len(targets),
				"configVersion", configVersion,
			)
		}
	}
}
