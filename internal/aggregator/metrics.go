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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all aggregator metrics
type Metrics struct {
	ResultsReceivedTotal   *prometheus.CounterVec
	ResultsInvalidTotal    *prometheus.CounterVec
	AlertsFiredTotal       *prometheus.CounterVec
	BlastRadius            *prometheus.GaugeVec
	StateSize              *prometheus.GaugeVec
	RedisOperationsTotal   *prometheus.CounterVec
	ResultsProcessedTotal  prometheus.Counter
	CorrelationEventsTotal *prometheus.CounterVec
	ProcessingDuration     *prometheus.HistogramVec
}

// NewMetrics creates and registers aggregator metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		ResultsReceivedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_aggregator_results_received_total",
				Help: "Total number of results received from agents",
			},
			[]string{"source_agent", "status"},
		),
		ResultsInvalidTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_aggregator_results_invalid_total",
				Help: "Total number of invalid results rejected",
			},
			[]string{"reason"},
		),
		AlertsFiredTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_aggregator_alerts_fired_total",
				Help: "Total number of alerts fired",
			},
			[]string{"severity", "target_category"},
		),
		BlastRadius: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "k8swatch_aggregator_blast_radius",
				Help: "Current blast radius classification per target",
			},
			[]string{"target", "classification"},
		),
		StateSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "k8swatch_aggregator_state_size",
				Help: "Current state size by type",
			},
			[]string{"type"},
		),
		RedisOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_aggregator_redis_operations_total",
				Help: "Total number of Redis operations",
			},
			[]string{"operation", "status"},
		),
		ResultsProcessedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "k8swatch_aggregator_results_processed_total",
				Help: "Total number of results processed successfully",
			},
		),
		CorrelationEventsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_aggregator_correlation_events_total",
				Help: "Total number of correlation events detected",
			},
			[]string{"pattern"},
		),
		ProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "k8swatch_aggregator_processing_duration_seconds",
				Help:    "Duration of result processing",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
	}
	return m
}

// RecordResultReceived records a received result
func (m *Metrics) RecordResultReceived(sourceAgent, status string) {
	m.ResultsReceivedTotal.WithLabelValues(sourceAgent, status).Inc()
}

// RecordResultInvalid records an invalid result
func (m *Metrics) RecordResultInvalid(reason string) {
	m.ResultsInvalidTotal.WithLabelValues(reason).Inc()
}

// RecordAlertFired records a fired alert
func (m *Metrics) RecordAlertFired(severity, targetCategory string) {
	m.AlertsFiredTotal.WithLabelValues(severity, targetCategory).Inc()
}

// SetBlastRadius sets blast radius for a target
func (m *Metrics) SetBlastRadius(target, classification string) {
	m.BlastRadius.WithLabelValues(target, classification).Set(1)
}

// ClearBlastRadius clears blast radius for a target
func (m *Metrics) ClearBlastRadius(target string) {
	m.BlastRadius.WithLabelValues(target, "node").Set(0)
	m.BlastRadius.WithLabelValues(target, "zone").Set(0)
	m.BlastRadius.WithLabelValues(target, "cluster").Set(0)
}

// SetStateSize sets state size for a type
func (m *Metrics) SetStateSize(stateType string, size float64) {
	m.StateSize.WithLabelValues(stateType).Set(size)
}

// RecordRedisOperation records a Redis operation
func (m *Metrics) RecordRedisOperation(operation, status string) {
	m.RedisOperationsTotal.WithLabelValues(operation, status).Inc()
}

// RecordResultProcessed records a processed result
func (m *Metrics) RecordResultProcessed() {
	m.ResultsProcessedTotal.Inc()
}

// RecordCorrelationEvent records a correlation event
func (m *Metrics) RecordCorrelationEvent(pattern string) {
	m.CorrelationEventsTotal.WithLabelValues(pattern).Inc()
}

// RecordProcessingDuration records processing duration
func (m *Metrics) RecordProcessingDuration(operation string, durationSeconds float64) {
	m.ProcessingDuration.WithLabelValues(operation).Observe(durationSeconds)
}
