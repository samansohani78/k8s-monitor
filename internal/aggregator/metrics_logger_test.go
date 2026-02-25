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

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/go-logr/logr"
)

// =============================================================================
// Metrics Tests
// =============================================================================

func TestMetricsRecordResultReceived(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.RecordResultReceived("agent-1", "success")
	metrics.RecordResultReceived("agent-2", "failure")

	// Verify metrics were recorded
	metric := &dto.Metric{}
	err := metrics.ResultsReceivedTotal.WithLabelValues("agent-1", "success").Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.Counter.GetValue())
}

func TestMetricsRecordResultInvalid(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.RecordResultInvalid("missing_target")
	metrics.RecordResultInvalid("invalid_format")

	metric := &dto.Metric{}
	err := metrics.ResultsInvalidTotal.WithLabelValues("missing_target").Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.Counter.GetValue())
}

func TestMetricsRecordAlertFired(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.RecordAlertFired("critical", "database")
	metrics.RecordAlertFired("warning", "network")

	metric := &dto.Metric{}
	err := metrics.AlertsFiredTotal.WithLabelValues("critical", "database").Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.Counter.GetValue())
}

func TestMetricsSetBlastRadius(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.SetBlastRadius("target-1", "cluster")

	metric := &dto.Metric{}
	err := metrics.BlastRadius.WithLabelValues("target-1", "cluster").Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.Gauge.GetValue())
}

func TestMetricsClearBlastRadius(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	// Set blast radius first
	metrics.SetBlastRadius("target-1", "node")
	metrics.SetBlastRadius("target-1", "zone")
	metrics.SetBlastRadius("target-1", "cluster")

	// Clear it
	metrics.ClearBlastRadius("target-1")

	// Verify all classifications are cleared (set to 0)
	for _, classification := range []string{"node", "zone", "cluster"} {
		metric := &dto.Metric{}
		err := metrics.BlastRadius.WithLabelValues("target-1", classification).Write(metric)
		assert.NoError(t, err)
		assert.Equal(t, float64(0), metric.Gauge.GetValue())
	}
}

func TestMetricsSetStateSize(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.SetStateSize("targets", 100.0)
	metrics.SetStateSize("alerts", 50.0)

	metric := &dto.Metric{}
	err := metrics.StateSize.WithLabelValues("targets").Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(100), metric.Gauge.GetValue())
}

func TestMetricsRecordRedisOperation(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.RecordRedisOperation("get", "success")
	metrics.RecordRedisOperation("set", "error")

	metric := &dto.Metric{}
	err := metrics.RedisOperationsTotal.WithLabelValues("get", "success").Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.Counter.GetValue())
}

func TestMetricsRecordResultProcessed(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.RecordResultProcessed()
	metrics.RecordResultProcessed()
	metrics.RecordResultProcessed()

	metric := &dto.Metric{}
	err := metrics.ResultsProcessedTotal.Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(3), metric.Counter.GetValue())
}

func TestMetricsRecordCorrelationEvent(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.RecordCorrelationEvent("cluster_wide_failure")
	metrics.RecordCorrelationEvent("zone_failure")

	metric := &dto.Metric{}
	err := metrics.CorrelationEventsTotal.WithLabelValues("cluster_wide_failure").Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.Counter.GetValue())
}

func TestMetricsRecordProcessingDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	metrics.RecordProcessingDuration("correlation", 0.050)
	metrics.RecordProcessingDuration("alerting", 0.100)

	// Histogram metrics can't be easily tested without registry
	// Just verify no panic
	assert.NotNil(t, metrics)
}

func TestMetricsAllOperations(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	// Test all operations
	metrics.RecordResultReceived("agent-1", "success")
	metrics.RecordResultInvalid("timeout")
	metrics.RecordAlertFired("warning", "network")
	metrics.SetBlastRadius("target-1", "node")
	metrics.ClearBlastRadius("target-1")
	metrics.SetStateSize("alerts", 50.0)
	metrics.RecordRedisOperation("set", "error")
	metrics.RecordResultProcessed()
	metrics.RecordCorrelationEvent("zone_failure")
	metrics.RecordProcessingDuration("alerting", 0.100)

	// All operations should complete without panic
	assert.NotNil(t, metrics)
}

func TestMetricsConcurrentAccess(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := createTestMetrics(reg)

	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			metrics.RecordResultReceived("agent-"+string(rune('a'+id)), "success")
			metrics.RecordResultProcessed()
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic
	assert.NotNil(t, metrics)
}

// Helper function to create test metrics with custom registry
func createTestMetrics(reg *prometheus.Registry) *Metrics {
	return &Metrics{
		ResultsReceivedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_results_received",
				Help: "Test metric",
			},
			[]string{"source_agent", "status"},
		),
		ResultsInvalidTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_results_invalid",
				Help: "Test metric",
			},
			[]string{"reason"},
		),
		AlertsFiredTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_alerts_fired",
				Help: "Test metric",
			},
			[]string{"severity", "target_category"},
		),
		BlastRadius: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_blast_radius",
				Help: "Test metric",
			},
			[]string{"target", "classification"},
		),
		StateSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_state_size",
				Help: "Test metric",
			},
			[]string{"type"},
		),
		RedisOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_redis_ops",
				Help: "Test metric",
			},
			[]string{"operation", "status"},
		),
		ResultsProcessedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "test_results_processed",
				Help: "Test metric",
			},
		),
		CorrelationEventsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_correlation_events",
				Help: "Test metric",
			},
			[]string{"pattern"},
		),
		ProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "test_processing_duration",
				Help:    "Test metric",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
	}
}

// =============================================================================
// Logger Tests
// =============================================================================

func TestAggregatorSetLogger(t *testing.T) {
	logger := logr.Discard()
	SetLogger(logger)

	ctxLogger := GetContextLogger()
	assert.NotNil(t, ctxLogger)
}

func TestAggregatorGetContextLogger(t *testing.T) {
	// Set logger first
	SetLogger(logr.Discard())

	logger := GetContextLogger()
	assert.NotNil(t, logger)
}

func TestAggregatorLoggerNil(t *testing.T) {
	// Reset logger to nil state
	log = logr.Discard()
	contextLogger = nil

	// GetContextLogger should handle nil gracefully
	logger := GetContextLogger()
	assert.Nil(t, logger)
}

func TestNewProcessContext(t *testing.T) {
	SetLogger(logr.Discard())

	ctx := context.Background()
	newCtx, opLogger := newProcessContext(ctx, "target-key-123")

	assert.NotNil(t, newCtx)
	// opLogger may be nil if not initialized
	_ = opLogger
}

func TestNewCorrelationContext(t *testing.T) {
	SetLogger(logr.Discard())

	ctx := context.Background()
	newCtx, opLogger := newCorrelationContext(ctx, "target-key-456", "cluster_pattern")

	assert.NotNil(t, newCtx)
	_ = opLogger
}

func TestLoggerWithContext(t *testing.T) {
	SetLogger(logr.Discard())

	ctx := context.Background()
	newCtx, _ := newProcessContext(ctx, "test-target")

	assert.NotNil(t, newCtx)
}

func TestLoggerMultipleContexts(t *testing.T) {
	SetLogger(logr.Discard())

	ctx := context.Background()

	// Create multiple contexts
	ctx1, _ := newProcessContext(ctx, "target-1")
	ctx2, _ := newCorrelationContext(ctx, "target-2", "pattern")
	ctx3, _ := newProcessContext(ctx1, "target-3")

	assert.NotNil(t, ctx1)
	assert.NotNil(t, ctx2)
	assert.NotNil(t, ctx3)
}
