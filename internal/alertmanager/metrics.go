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

package alertmanager

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all alertmanager metrics
type Metrics struct {
	AlertsActive            *prometheus.GaugeVec
	AlertsFiredTotal        *prometheus.CounterVec
	AlertsResolvedTotal     *prometheus.CounterVec
	AlertsAcknowledgedTotal prometheus.Counter
	NotificationsTotal      *prometheus.CounterVec
	NotificationDuration    *prometheus.HistogramVec
	EscalationsTotal        *prometheus.CounterVec
	SilencesActive          prometheus.Gauge
	RoutingDecisionsTotal   *prometheus.CounterVec
}

// NewMetrics creates and registers alertmanager metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		AlertsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "k8swatch_alertmanager_alerts_active",
				Help: "Current number of active alerts by severity",
			},
			[]string{"severity", "status"},
		),
		AlertsFiredTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_alertmanager_alerts_fired_total",
				Help: "Total number of alerts fired",
			},
			[]string{"severity"},
		),
		AlertsResolvedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_alertmanager_alerts_resolved_total",
				Help: "Total number of alerts resolved",
			},
			[]string{"severity"},
		),
		AlertsAcknowledgedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "k8swatch_alertmanager_alerts_acknowledged_total",
				Help: "Total number of alerts acknowledged",
			},
		),
		NotificationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_alertmanager_notifications_total",
				Help: "Total number of notifications sent",
			},
			[]string{"channel", "status"},
		),
		NotificationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "k8swatch_alertmanager_notification_duration_seconds",
				Help:    "Duration of notification sending",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"channel"},
		),
		EscalationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_alertmanager_escalations_total",
				Help: "Total number of escalations",
			},
			[]string{"level"},
		),
		SilencesActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "k8swatch_alertmanager_silences_active",
				Help: "Current number of active silences",
			},
		),
		RoutingDecisionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k8swatch_alertmanager_routing_decisions_total",
				Help: "Total number of routing decisions",
			},
			[]string{"channel", "rule"},
		),
	}
	return m
}

// RecordAlertFired records a fired alert
func (m *Metrics) RecordAlertFired(severity string) {
	m.AlertsFiredTotal.WithLabelValues(severity).Inc()
	m.AlertsActive.WithLabelValues(severity, "firing").Inc()
}

// RecordAlertResolved records a resolved alert
func (m *Metrics) RecordAlertResolved(severity string) {
	m.AlertsResolvedTotal.WithLabelValues(severity).Inc()
	m.AlertsActive.WithLabelValues(severity, "firing").Dec()
}

// RecordAlertAcknowledged records an acknowledged alert
func (m *Metrics) RecordAlertAcknowledged() {
	m.AlertsAcknowledgedTotal.Inc()
	m.AlertsActive.WithLabelValues("", "acknowledged").Inc()
}

// RecordNotification records a notification
func (m *Metrics) RecordNotification(channel, status string, durationSeconds float64) {
	m.NotificationsTotal.WithLabelValues(channel, status).Inc()
	m.NotificationDuration.WithLabelValues(channel).Observe(durationSeconds)
}

// RecordEscalation records an escalation
func (m *Metrics) RecordEscalation(level string) {
	m.EscalationsTotal.WithLabelValues(level).Inc()
}

// SetSilencesActive sets the number of active silences
func (m *Metrics) SetSilencesActive(count float64) {
	m.SilencesActive.Set(count)
}

// RecordRoutingDecision records a routing decision
func (m *Metrics) RecordRoutingDecision(channel, rule string) {
	m.RoutingDecisionsTotal.WithLabelValues(channel, rule).Inc()
}

// UpdateAlertStatus updates alert status metrics
func (m *Metrics) UpdateAlertStatus(oldStatus, newStatus, severity string) {
	if oldStatus != "" {
		m.AlertsActive.WithLabelValues(severity, oldStatus).Dec()
	}
	if newStatus != "" {
		m.AlertsActive.WithLabelValues(severity, newStatus).Inc()
	}
}
