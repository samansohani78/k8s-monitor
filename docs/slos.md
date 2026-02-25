# K8sWatch Service Level Objectives (SLOs)

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** SRE Team

---

## Overview

This document defines the Service Level Objectives (SLOs) for K8sWatch. These SLOs measure the reliability and performance of the K8sWatch monitoring system itself.

### SLO vs SLI vs SLA

- **SLI (Service Level Indicator):** What we measure (e.g., check latency)
- **SLO (Service Level Objective):** The target we aim for (e.g., 99% of checks < 5s)
- **SLA (Service Level Agreement):** Contractual commitment to users (not covered here)

---

## SLO 1: Check Execution Latency

### Definition

99% of health checks should complete within 5 seconds.

### SLI

```promql
# P99 check latency
histogram_quantile(0.99, sum(rate(k8swatch_agent_check_duration_seconds_bucket[5m])) by (le))
```

### Target

- **SLO:** 99% of checks complete within 5 seconds
- **Threshold:** `histogram_quantile(0.99, ...) < 5s`

### Measurement

```promql
# Percentage of checks completing within 5s
sum(rate(k8swatch_agent_check_duration_seconds_bucket{le="5"}[5m]))
/
sum(rate(k8swatch_agent_check_duration_seconds_count[5m]))
* 100
```

### Alert

```yaml
# See: deploy/monitoring/k8swatch-system-alerts.yaml
- alert: K8sWatchAgentCheckLatencyHigh
  expr: histogram_quantile(0.99, sum(rate(k8swatch_agent_check_duration_seconds_bucket[5m])) by (le)) > 10
  for: 5m
  severity: warning
```

### Error Budget

- **Monthly Error Budget:** 1% of checks can exceed 5s
- **Monthly Check Volume:** ~8.6M checks (100 targets × 30s interval × 24h × 30d)
- **Allowed Failures:** ~86,000 checks can exceed 5s per month

---

## SLO 2: Result Delivery Latency

### Definition

99% of check results should be delivered to the aggregator within 10 seconds of check completion.

### SLI

```promql
# Result delivery latency (measured via aggregator ingestion timestamp - agent check timestamp)
# This requires distributed tracing or timestamp comparison
histogram_quantile(0.99, sum(rate(k8swatch_aggregator_result_delivery_seconds_bucket[5m])) by (le))
```

### Target

- **SLO:** 99% of results delivered within 10 seconds
- **Threshold:** `histogram_quantile(0.99, ...) < 10s`

### Measurement

This metric requires instrumentation to track the time difference between:
1. Agent check completion timestamp
2. Aggregator result ingestion timestamp

Implementation in `internal/agent/result_client.go`:
```go
// Add timestamp when check completes
result.CheckCompletedAt = time.Now()

// Aggregator measures delivery time
deliveryLatency := time.Since(result.CheckCompletedAt)
```

### Alert

```yaml
# Planned: add to deploy/monitoring/k8swatch-system-alerts.yaml once metric is emitted
- alert: K8sWatchResultDeliveryLatencyHigh
  expr: histogram_quantile(0.99, sum(rate(k8swatch_aggregator_result_delivery_seconds_bucket[5m])) by (le)) > 10
  for: 5m
  severity: warning
```

### Error Budget

- **Monthly Error Budget:** 1% of results can exceed 10s delivery
- **Allowed Failures:** ~86,000 results can exceed 10s per month

---

## SLO 3: Alert Firing Latency

### Definition

99% of alerts should fire within 60 seconds of failure detection.

### SLI

```promql
# Alert firing latency (time from first failure to alert fired)
# Requires instrumentation in alerting pipeline
histogram_quantile(0.99, sum(rate(k8swatch_alertmanager_alert_firing_latency_seconds_bucket[5m])) by (le))
```

### Target

- **SLO:** 99% of alerts fire within 60 seconds
- **Threshold:** `histogram_quantile(0.99, ...) < 60s`

### Measurement

This metric requires instrumentation to track:
1. Timestamp of first check failure
2. Timestamp of alert fired

Implementation in `internal/aggregator/alerting.go`:
```go
// Track first failure time
if firstFailure {
    alertState.FirstFailureTime = time.Now()
}

// When firing alert
latency := time.Since(alertState.FirstFailureTime)
```

### Alert

```yaml
# Planned: add to deploy/monitoring/k8swatch-system-alerts.yaml once metric is emitted
- alert: K8sWatchAlertFiringLatencyHigh
  expr: histogram_quantile(0.99, sum(rate(k8swatch_alertmanager_alert_firing_latency_seconds_bucket[5m])) by (le)) > 60
  for: 5m
  severity: warning
```

### Error Budget

- **Monthly Error Budget:** 1% of alerts can exceed 60s
- **Typical Alert Volume:** ~100 alerts/month (varies by environment)
- **Allowed Failures:** ~1 alert can exceed 60s per month

---

## SLO 4: System Availability

### Definition

K8sWatch aggregator and alert manager should maintain 99.9% uptime.

### SLI

```promql
# Aggregator availability
avg_over_time(kube_pod_status_ready{pod=~"k8swatch-aggregator-.*", condition="true"}[30d]) * 100

# Alert Manager availability
avg_over_time(kube_pod_status_ready{pod=~"k8swatch-alertmanager-.*", condition="true"}[30d]) * 100
```

### Target

- **SLO:** 99.9% uptime for aggregator and alert manager
- **Threshold:** `availability >= 99.9%`

### Measurement

```promql
# Monthly availability percentage
avg_over_time(kube_pod_status_ready{pod=~"k8swatch-aggregator-.*", condition="true"}[30d]) * 100
```

### Alert

```yaml
# See: deploy/monitoring/k8swatch-system-alerts.yaml
- alert: K8sWatchAggregatorPodNotReady
  expr: sum(kube_pod_status_ready{pod=~"k8swatch-aggregator-.*", condition="true"}) < 2
  for: 5m
  severity: critical

- alert: K8sWatchAlertManagerPodNotReady
  expr: sum(kube_pod_status_ready{pod=~"k8swatch-alertmanager-.*", condition="true"}) < 1
  for: 5m
  severity: critical
```

### Error Budget

- **Monthly Error Budget:** 0.1% downtime allowed
- **Monthly Minutes:** 43,200 minutes (30 days)
- **Allowed Downtime:** ~43 minutes per month

---

## SLO 5: Check Coverage

### Definition

99.9% of configured targets should be checked at their configured intervals.

### SLI

```promql
# Percentage of targets checked in last interval
sum(k8swatch_agent_check_total{status="success"}[5m]) / count(k8swatch_targets) * 100
```

### Target

- **SLO:** 99.9% of targets checked on schedule
- **Threshold:** `coverage >= 99.9%`

### Measurement

```promql
# Check coverage percentage
sum(increase(k8swatch_agent_check_total{status="success"}[5m])) by (target)
/
(expected_checks_per_5m) * 100
```

### Alert

```yaml
# See: deploy/monitoring/k8swatch-system-alerts.yaml
- alert: K8sWatchNoChecksExecuted
  expr: sum(rate(k8swatch_agent_check_total[10m])) == 0
  for: 10m
  severity: critical
```

### Error Budget

- **Monthly Error Budget:** 0.1% of targets can miss checks
- **Typical Target Count:** 100 targets
- **Allowed Misses:** ~0.1 targets can miss checks (essentially zero tolerance)

---

## SLO 6: Check Success Rate

### Definition

95% of health checks should complete successfully (not fail due to K8sWatch issues).

### SLI

```promql
# Check success rate
sum(rate(k8swatch_agent_check_total{status="success"}[5m]))
/
sum(rate(k8swatch_agent_check_total[5m])) * 100
```

### Target

- **SLO:** 95% check success rate
- **Warning Threshold:** < 95%
- **Critical Threshold:** < 80%

### Measurement

```promql
# Success rate percentage
sum(rate(k8swatch_agent_check_total{status="success"}[5m]))
/
sum(rate(k8swatch_agent_check_total[5m])) * 100
```

### Alert

```yaml
# See: deploy/monitoring/k8swatch-system-alerts.yaml
- alert: K8sWatchCheckSuccessRateDegraded
  expr: sum(rate(k8swatch_agent_check_total{status="success"}[5m])) / sum(rate(k8swatch_agent_check_total[5m])) < 0.95
  for: 5m
  severity: warning

- alert: K8sWatchCheckSuccessRateCritical
  expr: sum(rate(k8swatch_agent_check_total{status="success"}[5m])) / sum(rate(k8swatch_agent_check_total[5m])) < 0.80
  for: 5m
  severity: critical
```

### Error Budget

- **Monthly Error Budget:** 5% of checks can fail
- **Monthly Check Volume:** ~8.6M checks
- **Allowed Failures:** ~430,000 checks can fail per month

---

## Error Budget Policy

### Budget Consumption Rate

| Burn Rate | Action |
|-----------|--------|
| 1x (normal) | Continue normal operations |
| 2x | Notify team, monitor closely |
| 5x | Page on-call, investigate immediately |
| 10x | Emergency response, all hands |

### Budget Recovery

- Error budgets reset monthly
- If budget exhausted before month end:
  - Freeze non-essential changes
  - Focus on reliability improvements
  - Conduct post-mortem if SLO breached

---

## Reporting

### Weekly Reports

- SLO compliance for each metric
- Error budget consumption rate
- Top failure modes

### Monthly Reports

- SLO compliance summary
- Error budget remaining
- Trend analysis
- Recommendations for improvement

### Dashboard

**Grafana Dashboard:** `K8sWatch - SLO Tracking`

Panels:
1. Check Execution Latency (P99, P95, P50)
2. Result Delivery Latency (P99)
3. Alert Firing Latency (P99)
4. System Availability (30-day rolling)
5. Check Coverage (%)
6. Check Success Rate (%)
7. Error Budget Consumption (per SLO)

---

## Implementation Status

| SLO | SLI Implemented | Alert Created | Dashboard Panel | Status |
|-----|-----------------|---------------|-----------------|--------|
| Check Execution Latency | ✅ | ✅ | ✅ | Complete |
| Result Delivery Latency | ❌ | ❌ | ❌ | Planned |
| Alert Firing Latency | ❌ | ❌ | ❌ | Planned |
| System Availability | ✅ | ✅ | ✅ | Complete |
| Check Coverage | ⚠️ | ⚠️ | ⚠️ | Partial |
| Check Success Rate | ✅ | ✅ | ✅ | Complete |

---

## Review Cadence

- **Weekly:** SRE team reviews SLO dashboards
- **Monthly:** SLO compliance report to leadership
- **Quarterly:** SLO target review and adjustment
- **Annually:** Comprehensive SLO framework review

---

## Appendix: PromQL Queries

### All SLO Queries

```promql
# SLO 1: Check Execution Latency (P99)
histogram_quantile(0.99, sum(rate(k8swatch_agent_check_duration_seconds_bucket[5m])) by (le))

# SLO 2: Result Delivery Latency (P99) - Planned
histogram_quantile(0.99, sum(rate(k8swatch_aggregator_result_delivery_seconds_bucket[5m])) by (le))

# SLO 3: Alert Firing Latency (P99) - Planned
histogram_quantile(0.99, sum(rate(k8swatch_alertmanager_alert_firing_latency_seconds_bucket[5m])) by (le))

# SLO 4: System Availability
avg_over_time(kube_pod_status_ready{pod=~"k8swatch-aggregator-.*", condition="true"}[30d]) * 100

# SLO 5: Check Coverage
sum(increase(k8swatch_agent_check_total{status="success"}[5m])) by (target) / (expected_checks_per_5m) * 100

# SLO 6: Check Success Rate
sum(rate(k8swatch_agent_check_total{status="success"}[5m])) / sum(rate(k8swatch_agent_check_total[5m])) * 100
```

---

**Document Owner:** SRE Team  
**Review Date:** 2026-02-21  
**Next Review:** 2026-05-21 (Quarterly)
