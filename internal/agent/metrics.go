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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all agent metrics
type Metrics struct {
	CheckTotal           *prometheus.CounterVec
	CheckDurationSeconds *prometheus.HistogramVec
	ConfigVersion        prometheus.Gauge
	ResultsDroppedTotal  *prometheus.CounterVec
	ChecksInProgress     prometheus.Gauge
}

// NewMetrics creates and registers agent metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		CheckTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_agent_check_total",
				Help: "Total number of checks executed by the agent",
			},
			[]string{"target", "namespace", "type", "layer", "network_mode", "status"},
		),
		CheckDurationSeconds: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "k8swatch_agent_check_duration_seconds",
				Help:    "Duration of checks executed by the agent",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"target", "namespace", "type", "layer", "network_mode"},
		),
		ConfigVersion: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "k8swatch_agent_config_version",
				Help: "Current configuration version",
			},
		),
		ResultsDroppedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_agent_results_dropped_total",
				Help: "Total number of results dropped due to transmission failure",
			},
			[]string{"reason"},
		),
		ChecksInProgress: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "k8swatch_agent_checks_in_progress",
				Help: "Number of checks currently in progress",
			},
		),
	}
	return m
}

// RecordCheck records metrics for a completed check
func (m *Metrics) RecordCheck(target, namespace, targetType, layer, networkMode, status string, durationSeconds float64) {
	m.CheckTotal.WithLabelValues(target, namespace, targetType, layer, networkMode, status).Inc()
	m.CheckDurationSeconds.WithLabelValues(target, namespace, targetType, layer, networkMode).Observe(durationSeconds)
}

// RecordDroppedResult records a dropped result
func (m *Metrics) RecordDroppedResult(reason string) {
	m.ResultsDroppedTotal.WithLabelValues(reason).Inc()
}

// SetConfigVersion sets the current config version
func (m *Metrics) SetConfigVersion(version float64) {
	m.ConfigVersion.Set(version)
}

// BeginCheck increments checks in progress
func (m *Metrics) BeginCheck() {
	m.ChecksInProgress.Inc()
}

// EndCheck decrements checks in progress
func (m *Metrics) EndCheck() {
	m.ChecksInProgress.Dec()
}
