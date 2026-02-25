# LogQL Queries for K8sWatch Investigation

This document provides common LogQL queries for investigating alerts and issues in K8sWatch.

## Quick Reference

### Connection Strings
- **Loki URL:** `http://loki.observability.svc.cluster.local:3100`
- **Grafana Explore:** `/explore?orgId=1&left=["now-1h","now","loki",{"expr":"..."}]`

---

## Basic Queries

### All K8sWatch Logs (Last Hour)
```logql
{namespace="k8swatch"} |= ""
```

### Logs by Component
```logql
// Agent logs
{namespace="k8swatch", container_name="agent"} |= ""

// Aggregator logs
{namespace="k8swatch", container_name="aggregator"} |= ""

// AlertManager logs
{namespace="k8swatch", container_name="alertmanager"} |= ""
```

### Error Logs Only
```logql
{namespace="k8swatch"} |= "level=error"
```

### Logs with Specific Correlation ID
```logql
{namespace="k8swatch"} |= "correlationID=abc123-def456-ghi789"
```

---

## Target Investigation

### Show me all errors for target X
```logql
{namespace="k8swatch"} 
|= "level=error" 
|= `target="my-target"`
```

### Show all check results for a target
```logql
{namespace="k8swatch"} 
|= `operation="check.execute"` 
|= `target="my-target"`
| json
| line_format "{{.timestamp}} {{.level}} {{.message}} - duration={{.duration}} success={{.success}}"
```

### Target failure timeline
```logql
{namespace="k8swatch"} 
|= `operation="check.execute"` 
|= `target="my-target"`
|= "level=error"
| json
| line_format "{{.timestamp}} FAILED: {{.error}}"
```

---

## Node Investigation

### Show me all L2 failures in zone Y
```logql
{namespace="k8swatch"} 
|= "level=error" 
|= "failure_layer=L2"
|= `zone="us-east-1a"`
| json
| line_format "{{.timestamp}} Node={{.node}} {{.error}}"
```

### Show agent restarts
```logql
{namespace="k8swatch", container_name="agent"} 
|= "Starting K8sWatch Agent"
| json
| line_format "{{.timestamp}} Agent restarted on {{.node}}"
```

### Node-specific check failures
```logql
{namespace="k8swatch"} 
|= "level=error" 
|= `node="node-123"`
| json
| line_format "{{.timestamp}} {{.operation}} - {{.error}}"
```

### Network mode comparison for a node
```logql
// Pod network failures
{namespace="k8swatch"} 
|= `node="node-123"` 
|= `network_mode="pod"` 
|= "level=error"

// Host network failures
{namespace="k8swatch"} 
|= `node="node-123"` 
|= `network_mode="host"` 
|= "level=error"
```

---

## Alert Investigation

### All alerts for a target
```logql
{namespace="k8swatch", container_name="alertmanager"} 
|= `target="my-target"`
| json
| line_format "{{.timestamp}} {{.severity}} {{.message}}"
```

### Alert lifecycle with correlation ID
```logql
{namespace="k8swatch"} 
|= "correlationID=abc123"
| json
| line_format "{{.timestamp}} {{.component}} {{.operation}} - {{.message}}"
```

### Notification failures
```logql
{namespace="k8swatch", container_name="alertmanager"} 
|= "operation=notification.send" 
|= "level=error"
| json
| line_format "{{.timestamp}} Channel={{.channel}} Alert={{.alertID}} Error={{.error}}"
```

### Escalation events
```logql
{namespace="k8swatch", container_name="alertmanager"} 
|= "operation=escalation"
| json
| line_format "{{.timestamp}} Alert={{.alertID}} Level={{.level}}"
```

---

## Performance Investigation

### Slow checks (> 5 seconds)
```logql
{namespace="k8swatch"} 
|= `operation="check.execute"` 
|= "level=info"
| json duration
| duration > 5s
| line_format "{{.timestamp}} SLOW CHECK target={{.target}} duration={{.duration}}"
```

### Check latency by target (aggregated)
```logql
{namespace="k8swatch"} 
|= `operation="check.execute"`
| json
| stats avg(duration) as avg_duration, max(duration) as max_duration by (target)
| sort by max_duration desc
```

### Aggregator processing delays
```logql
{namespace="k8swatch", container_name="aggregator"} 
|= `operation="result.process"`
| json
| line_format "{{.timestamp}} Target={{.targetKey}} Duration={{.duration}}"
```

---

## Failure Pattern Analysis

### DNS failures across all nodes
```logql
{namespace="k8swatch"} 
|= "level=error" 
|= "failure_code=dns_timeout"
| json
| stats count() by (node, zone)
```

### TLS certificate errors
```logql
{namespace="k8swatch"} 
|= "level=error" 
|= "failure_code=tls_expired"
| json
| line_format "{{.timestamp}} Target={{.target}} Node={{.node}}"
```

### Authentication failures
```logql
{namespace="k8swatch"} 
|= "level=error" 
|= "failure_code=auth_failed"
| json
| stats count() by (target, namespace)
```

### Connection refused errors
```logql
{namespace="k8swatch"} 
|= "level=error" 
|= "failure_code=tcp_refused"
| json
| line_format "{{.timestamp}} Target={{.target}} Node={{.node}}"
```

---

## Correlation ID Tracing

### Full trace for a correlation ID
```logql
{namespace="k8swatch"} 
|= "correlationID=abc123-def456-ghi789"
| json
| sort by timestamp
| line_format "{{.timestamp}} {{.component}} {{.operation}} - {{.message}}"
```

### Cross-component correlation
```logql
// Agent check execution
{namespace="k8swatch", container_name="agent"} 
|= "correlationID=abc123"

// Aggregator processing
{namespace="k8swatch", container_name="aggregator"} 
|= "correlationID=abc123"

// AlertManager notification
{namespace="k8swatch", container_name="alertmanager"} 
|= "correlationID=abc123"
```

---

## Dashboard Links

### From Log to Dashboard
When viewing logs in Grafana, use these derived field links:

1. **Correlation ID** → Click to trace full request
2. **Target Name** → Opens Target Deep Dive dashboard
3. **Node Name** → Opens Node Health dashboard

### Custom Dashboard URLs
- **Cluster Health:** `/d/k8swatch-cluster-health`
- **Target Deep Dive:** `/d/k8swatch-target-deep-dive?var-target=my-target`
- **Node Health:** `/d/k8swatch-node-health?var-node=node-123`
- **Alerting Metrics:** `/d/k8swatch-alerting-metrics`

---

## Tips

1. **Use time range wisely:** Start with 1h, expand if needed
2. **Filter by severity first:** `|= "level=error"` reduces noise
3. **Use JSON parsing:** `| json` enables field filtering
4. **Sort by timestamp:** `| sort by timestamp` for chronological view
5. **Combine with metrics:** Use Loki logs + Prometheus metrics together
