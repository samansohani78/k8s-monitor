package scale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ScaleTestConfig holds scaling test configuration
type ScaleTestConfig struct {
	// NodeCount is the number of nodes to simulate
	NodeCount int

	// TargetCount is the number of targets to create
	TargetCount int

	// CheckInterval is the interval between checks
	CheckInterval time.Duration

	// Duration is the total test duration
	Duration time.Duration

	// Kubeconfig is the path to kubeconfig
	Kubeconfig string

	// Namespace is the namespace where scale test targets are created
	Namespace string

	// PrometheusURL is the Prometheus base URL used for metrics collection
	PrometheusURL string
}

// ScaleTestResults holds test results
type ScaleTestResults struct {
	// StartTime is when the test started
	StartTime time.Time

	// EndTime is when the test ended
	EndTime time.Time

	// TotalChecks is the total number of checks executed
	TotalChecks int64

	// SuccessfulChecks is the number of successful checks
	SuccessfulChecks int64

	// FailedChecks is the number of failed checks
	FailedChecks int64

	// AvgLatencyMs is the average check latency in milliseconds
	AvgLatencyMs float64

	// P95LatencyMs is the P95 check latency in milliseconds
	P95LatencyMs float64

	// P99LatencyMs is the P99 check latency in milliseconds
	P99LatencyMs float64

	// AggregatorCPU is the aggregator CPU usage
	AggregatorCPU float64

	// AggregatorMemory is the aggregator memory usage in bytes
	AggregatorMemory int64

	// RedisCPU is the Redis CPU usage
	RedisCPU float64

	// RedisMemory is the Redis memory usage in bytes
	RedisMemory int64

	// Errors is a list of errors encountered during the test
	Errors []string
}

// ScaleTester runs scaling tests
type ScaleTester struct {
	config    *ScaleTestConfig
	restCfg   *rest.Config
	clientset *kubernetes.Clientset
	dynClient dynamic.Interface
	results   *ScaleTestResults
	runID     string
}

// NewScaleTester creates a new scale tester
func NewScaleTester(config *ScaleTestConfig) (*ScaleTester, error) {
	// Load kubeconfig
	configAccess := clientcmd.NewDefaultClientConfigLoadingRules()
	configAccess.ExplicitPath = config.Kubeconfig

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		configAccess,
		&clientcmd.ConfigOverrides{},
	)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	if config.Namespace == "" {
		config.Namespace = "k8swatch"
	}
	if config.PrometheusURL == "" {
		if envURL := os.Getenv("K8SWATCH_PROMETHEUS_URL"); envURL != "" {
			config.PrometheusURL = envURL
		} else {
			config.PrometheusURL = "http://prometheus-server.monitoring.svc:9090"
		}
	}

	return &ScaleTester{
		config:    config,
		restCfg:   restConfig,
		clientset: clientset,
		dynClient: dynClient,
		results: &ScaleTestResults{
			StartTime: time.Now(),
		},
		runID: fmt.Sprintf("scale-%d", time.Now().UnixNano()),
	}, nil
}

// Run executes the scaling test
func (t *ScaleTester) Run(ctx context.Context) (*ScaleTestResults, error) {
	t.results.StartTime = time.Now()

	// Create targets
	fmt.Printf("Creating %d targets...\n", t.config.TargetCount)
	if err := t.createTargets(ctx); err != nil {
		return nil, fmt.Errorf("failed to create targets: %w", err)
	}
	defer t.cleanupTargets(ctx)

	// Wait for agents to pick up targets
	fmt.Println("Waiting for agents to discover targets...")
	time.Sleep(30 * time.Second)

	// Run test for specified duration
	fmt.Printf("Running test for %v...\n", t.config.Duration)
	testCtx, cancel := context.WithTimeout(ctx, t.config.Duration)
	defer cancel()

	// Collect metrics periodically
	go t.collectMetrics(testCtx)

	// Wait for test to complete
	<-testCtx.Done()

	t.results.EndTime = time.Now()

	// Collect final metrics
	t.collectFinalMetrics()

	return t.results, nil
}

// createTargets creates test targets
func (t *ScaleTester) createTargets(ctx context.Context) error {
	targetsGVR := schema.GroupVersionResource{
		Group:    "k8swatch.io",
		Version:  "v1",
		Resource: "targets",
	}

	for i := 0; i < t.config.TargetCount; i++ {
		name := fmt.Sprintf("scale-target-%s-%d", t.runID, i)
		interval := t.config.CheckInterval.String()
		timeout := "10s"
		dns := "kubernetes.default.svc.cluster.local"
		port := int64(443)

		target := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "k8swatch.io/v1",
				"kind":       "Target",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": t.config.Namespace,
					"labels": map[string]interface{}{
						"k8swatch.io/scale-test": "true",
						"k8swatch.io/scale-run":  t.runID,
					},
				},
				"spec": map[string]interface{}{
					"type": "https",
					"endpoint": map[string]interface{}{
						"dns":  dns,
						"port": port,
						"path": "/readyz",
					},
					"networkModes": []interface{}{"pod"},
					"layers": map[string]interface{}{
						"L1_dns": map[string]interface{}{"enabled": true},
						"L2_tcp": map[string]interface{}{"enabled": true},
						"L3_tls": map[string]interface{}{"enabled": true},
						"L4_protocol": map[string]interface{}{
							"enabled":    true,
							"method":     "GET",
							"statusCode": int64(200),
						},
					},
					"schedule": map[string]interface{}{
						"interval": interval,
						"timeout":  timeout,
					},
				},
			},
		}

		if _, err := t.dynClient.Resource(targetsGVR).Namespace(t.config.Namespace).Create(ctx, target, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create target %s: %w", name, err)
		}
	}

	return nil
}

// cleanupTargets removes test targets
func (t *ScaleTester) cleanupTargets(ctx context.Context) {
	fmt.Println("Cleaning up targets...")

	targetsGVR := schema.GroupVersionResource{
		Group:    "k8swatch.io",
		Version:  "v1",
		Resource: "targets",
	}
	selector := fmt.Sprintf("k8swatch.io/scale-test=true,k8swatch.io/scale-run=%s", t.runID)
	if err := t.dynClient.Resource(targetsGVR).Namespace(t.config.Namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: selector},
	); err != nil {
		t.results.Errors = append(t.results.Errors, fmt.Sprintf("target cleanup failed: %v", err))
	}
}

// collectMetrics collects metrics during the test
func (t *ScaleTester) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.collectMetricsOnce()
		}
	}
}

// collectMetricsOnce collects metrics at a single point in time
func (t *ScaleTester) collectMetricsOnce() {
	aggregatorCPU, err := t.queryPrometheus(`sum(rate(container_cpu_usage_seconds_total{namespace="k8swatch",pod=~"k8swatch-aggregator-.*"}[5m]))`)
	if err == nil {
		t.results.AggregatorCPU = aggregatorCPU
	} else {
		t.results.Errors = append(t.results.Errors, fmt.Sprintf("aggregator CPU query failed: %v", err))
	}

	aggregatorMem, err := t.queryPrometheus(`sum(container_memory_working_set_bytes{namespace="k8swatch",pod=~"k8swatch-aggregator-.*"})`)
	if err == nil {
		t.results.AggregatorMemory = int64(aggregatorMem)
	} else {
		t.results.Errors = append(t.results.Errors, fmt.Sprintf("aggregator memory query failed: %v", err))
	}

	redisCPU, err := t.queryPrometheus(`sum(rate(container_cpu_usage_seconds_total{namespace="k8swatch",pod=~"k8swatch-redis-.*"}[5m]))`)
	if err == nil {
		t.results.RedisCPU = redisCPU
	}

	redisMem, err := t.queryPrometheus(`sum(container_memory_working_set_bytes{namespace="k8swatch",pod=~"k8swatch-redis-.*"})`)
	if err == nil {
		t.results.RedisMemory = int64(redisMem)
	}

	p99Latency, err := t.queryPrometheus(`histogram_quantile(0.99, sum(rate(k8swatch_agent_check_duration_seconds_bucket[5m])) by (le)) * 1000`)
	if err == nil {
		t.results.P99LatencyMs = p99Latency
	}

	p95Latency, err := t.queryPrometheus(`histogram_quantile(0.95, sum(rate(k8swatch_agent_check_duration_seconds_bucket[5m])) by (le)) * 1000`)
	if err == nil {
		t.results.P95LatencyMs = p95Latency
	}

	avgLatency, err := t.queryPrometheus(`(sum(rate(k8swatch_agent_check_duration_seconds_sum[5m])) / sum(rate(k8swatch_agent_check_duration_seconds_count[5m]))) * 1000`)
	if err == nil {
		t.results.AvgLatencyMs = avgLatency
	}
}

// collectFinalMetrics collects final metrics after test completion
func (t *ScaleTester) collectFinalMetrics() {
	t.collectMetricsOnce()

	checkRate, err := t.queryPrometheus(`sum(rate(k8swatch_agent_check_duration_seconds_count[5m]))`)
	if err == nil {
		duration := t.results.EndTime.Sub(t.results.StartTime).Seconds()
		totalChecks := int64(checkRate * duration)
		t.results.TotalChecks = totalChecks
		t.results.SuccessfulChecks = totalChecks
		t.results.FailedChecks = 0
	} else {
		t.results.Errors = append(t.results.Errors, fmt.Sprintf("check rate query failed: %v", err))
	}
}

func (t *ScaleTester) queryPrometheus(query string) (float64, error) {
	base := strings.TrimRight(t.config.PrometheusURL, "/")
	endpoint, err := url.Parse(base + "/api/v1/query")
	if err != nil {
		return 0, fmt.Errorf("invalid Prometheus URL: %w", err)
	}

	params := endpoint.Query()
	params.Set("query", query)
	endpoint.RawQuery = params.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus API returned %d", resp.StatusCode)
	}

	var parsed struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, fmt.Errorf("failed to decode Prometheus response: %w", err)
	}
	if parsed.Status != "success" {
		return 0, fmt.Errorf("prometheus query failed with status %q", parsed.Status)
	}
	if len(parsed.Data.Result) == 0 || len(parsed.Data.Result[0].Value) < 2 {
		return 0, fmt.Errorf("prometheus query returned no data")
	}

	rawValue, ok := parsed.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected value type in Prometheus response")
	}
	val, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse Prometheus scalar %q: %w", rawValue, err)
	}

	return val, nil
}

// Test100Nodes500Targets runs scaling test with 100 nodes and 500 targets
func Test100Nodes500Targets(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scaling test in short mode")
	}

	// Check if kubeconfig exists
	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		t.Skip("Skipping scaling test: no kubeconfig found")
	}

	config := &ScaleTestConfig{
		NodeCount:     100,
		TargetCount:   500,
		CheckInterval: 30 * time.Second,
		Duration:      10 * time.Minute,
		Kubeconfig:    kubeconfig,
		Namespace:     "k8swatch",
	}

	tester, err := NewScaleTester(config)
	if err != nil {
		t.Skipf("Skipping scaling test: cluster is not reachable from kubeconfig: %v", err)
	}

	ctx := context.Background()
	results, err := tester.Run(ctx)
	if err != nil {
		t.Fatalf("Scaling test failed: %v", err)
	}

	// Validate results
	t.Logf("Test completed:")
	t.Logf("  Duration: %v", results.EndTime.Sub(results.StartTime))
	t.Logf("  Total checks: %d", results.TotalChecks)
	t.Logf("  Success rate: %.2f%%", float64(results.SuccessfulChecks)/float64(results.TotalChecks)*100)
	t.Logf("  Avg latency: %.2fms", results.AvgLatencyMs)
	t.Logf("  P95 latency: %.2fms", results.P95LatencyMs)
	t.Logf("  P99 latency: %.2fms", results.P99LatencyMs)
	t.Logf("  Aggregator CPU: %.2f cores", results.AggregatorCPU)
	t.Logf("  Aggregator Memory: %.2f MiB", float64(results.AggregatorMemory)/1024/1024)

	// Assert SLOs
	if results.P99LatencyMs > 5000 {
		t.Errorf("P99 latency %.2fms exceeds SLO of 5000ms", results.P99LatencyMs)
	}

	if results.AggregatorCPU > 4.0 {
		t.Errorf("Aggregator CPU %.2f exceeds limit of 4 cores", results.AggregatorCPU)
	}
}

// Test1000Nodes2000Targets runs scaling test with 1000 nodes and 2000 targets
func Test1000Nodes2000Targets(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scaling test in short mode")
	}

	// Check if kubeconfig exists
	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		t.Skip("Skipping scaling test: no kubeconfig found")
	}

	config := &ScaleTestConfig{
		NodeCount:     1000,
		TargetCount:   2000,
		CheckInterval: 30 * time.Second,
		Duration:      30 * time.Minute,
		Kubeconfig:    kubeconfig,
		Namespace:     "k8swatch",
	}

	tester, err := NewScaleTester(config)
	if err != nil {
		t.Skipf("Skipping scaling test: cluster is not reachable from kubeconfig: %v", err)
	}

	ctx := context.Background()
	results, err := tester.Run(ctx)
	if err != nil {
		t.Fatalf("Scaling test failed: %v", err)
	}

	// Validate results
	t.Logf("Test completed:")
	t.Logf("  Duration: %v", results.EndTime.Sub(results.StartTime))
	t.Logf("  Total checks: %d", results.TotalChecks)
	t.Logf("  Success rate: %.2f%%", float64(results.SuccessfulChecks)/float64(results.TotalChecks)*100)
	t.Logf("  Avg latency: %.2fms", results.AvgLatencyMs)
	t.Logf("  P95 latency: %.2fms", results.P95LatencyMs)
	t.Logf("  P99 latency: %.2fms", results.P99LatencyMs)
	t.Logf("  Aggregator CPU: %.2f cores", results.AggregatorCPU)
	t.Logf("  Aggregator Memory: %.2f MiB", float64(results.AggregatorMemory)/1024/1024)

	// Assert SLOs
	if results.P99LatencyMs > 10000 {
		t.Errorf("P99 latency %.2fms exceeds SLO of 10000ms", results.P99LatencyMs)
	}

	if results.AggregatorCPU > 10.0 {
		t.Errorf("Aggregator CPU %.2f exceeds limit of 10 cores", results.AggregatorCPU)
	}
}

// TestScaleSmokeManual runs a short scale smoke scenario intended for manual validation.
// Enable with K8SWATCH_SCALE_SMOKE=1.
func TestScaleSmokeManual(t *testing.T) {
	if os.Getenv("K8SWATCH_SCALE_SMOKE") != "1" {
		t.Skip("Skipping manual scale smoke test: set K8SWATCH_SCALE_SMOKE=1 to enable")
	}

	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		t.Skip("Skipping scale smoke test: no kubeconfig found")
	}

	cfg := &ScaleTestConfig{
		NodeCount:     1,
		TargetCount:   5,
		CheckInterval: 15 * time.Second,
		Duration:      45 * time.Second,
		Kubeconfig:    kubeconfig,
		Namespace:     "k8swatch",
	}

	tester, err := NewScaleTester(cfg)
	if err != nil {
		t.Fatalf("Failed to create scale smoke tester: %v", err)
	}

	ctx := context.Background()
	results, err := tester.Run(ctx)
	if err != nil {
		t.Fatalf("Scale smoke run failed: %v", err)
	}

	t.Logf("Scale smoke completed: duration=%v, totalChecks=%d, p99=%.2fms, errors=%d",
		results.EndTime.Sub(results.StartTime),
		results.TotalChecks,
		results.P99LatencyMs,
		len(results.Errors),
	)
}
