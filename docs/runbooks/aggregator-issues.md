# Runbook: Aggregator Issues

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** SRE Team

---

## Overview

This runbook provides step-by-step instructions for troubleshooting K8sWatch aggregator issues.

---

## Symptoms

| Symptom | Severity | Likely Cause |
|---------|----------|--------------|
| Alerts not firing | Critical | Aggregator down, correlation failure |
| Results not processed | Critical | gRPC server down, Redis unreachable |
| High ingestion latency | High | Resource pressure, Redis slow |
| Duplicate alerts | Medium | State corruption, Redis failover |

---

## Investigation Steps

### Step 1: Check Aggregator Deployment

```bash
# Check deployment status
kubectl get deployment -n k8swatch k8swatch-aggregator

# Expected output:
# NAME                  READY   UP-TO-DATE   AVAILABLE   AGE
# k8swatch-aggregator   3/3     3            3           7d

# Check if READY == UP-TO-DATE == AVAILABLE
# If not, investigate missing replicas
```

**Troubleshooting:**

| Issue | Command | Solution |
|-------|---------|----------|
| READY < replicas | `kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator` | Check pod status |
| UP-TO-DATE < READY | `kubectl rollout status deployment/k8swatch-aggregator -n k8swatch` | Wait for rollout |
| AVAILABLE < READY | `kubectl describe deployment -n k8swatch k8swatch-aggregator` | Check readiness probe |

### Step 2: Check Aggregator Pods

```bash
# List all aggregator pods
kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator -o wide

# Check for pods not in Running state
kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator \
  --field-selector=status.phase!=Running
```

**Pod Status Troubleshooting:**

| Status | Command | Solution |
|--------|---------|----------|
| Pending | `kubectl describe pod -n k8swatch <pod>` | Check resource quotas |
| CrashLoopBackOff | `kubectl logs -n k8swatch <pod> --previous` | Check application errors |
| OOMKilled | `kubectl describe pod -n k8swatch <pod> \| grep -A 5 "State"` | Increase memory limits |

### Step 3: Review Aggregator Logs

```bash
# Check logs for all aggregator pods
kubectl logs -n k8swatch -l app.kubernetes.io/component=aggregator --tail=50

# Stream logs from specific pod
kubectl logs -n k8swatch <aggregator-pod> -f

# Search for errors
kubectl logs -n k8swatch <aggregator-pod> | grep -i error

# Check for Redis issues
kubectl logs -n k8swatch <aggregator-pod> | grep -i redis
```

**Common Log Patterns:**

| Log Pattern | Meaning | Action |
|-------------|---------|--------|
| `failed to connect to Redis` | Redis unreachable | Check Redis pod, network |
| `result validation failed` | Invalid result format | Check agent version compatibility |
| `correlation error` | State tracking issue | Check Redis connectivity |
| `alert fired` | Normal operation | Verify alert reached AlertManager |
| `blast radius calculation failed` | Topology data missing | Check node API access |

### Step 4: Check Redis Connectivity

```bash
# Check Redis pod
kubectl get pods -n k8swatch -l app.kubernetes.io/name=redis

# Test Redis connectivity from aggregator
AGGREGATOR_POD=$(kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator \
  -o jsonpath='{.items[0].metadata.name}')

kubectl exec -n k8swatch $AGGREGATOR_POD -- redis-cli -h k8swatch-redis ping

# Expected: PONG
```

**Redis Troubleshooting:**

| Issue | Command | Solution |
|-------|---------|----------|
| Redis pod not running | `kubectl get pods -n k8swatch -l app.kubernetes.io/name=redis` | Restart Redis |
| Connection refused | `kubectl exec -n k8swatch <redis-pod> -- redis-cli ping` | Check Redis service |
| Timeout | `kubectl top pods -n k8swatch -l app.kubernetes.io/name=redis` | Check Redis resource usage |

### Step 5: Check Result Ingestion Metrics

```bash
# Open Grafana → Explore and run:

# Results received rate
sum(rate(k8swatch_aggregator_results_received_total{namespace="k8swatch"}[5m]))

# Results rejected rate
sum(rate(k8swatch_aggregator_results_invalid_total{namespace="k8swatch"}[5m]))

# Redis operation errors
sum(rate(k8swatch_aggregator_redis_operations_total{namespace="k8swatch", status="error"}[5m]))
```

**Metric Thresholds:**

| Metric | Normal | Warning | Critical |
|--------|--------|---------|----------|
| Results received | > 1/s | < 0.5/s | < 0.1/s |
| Results rejected | < 0.1/s | > 0.5/s | > 1/s |
| Redis errors | 0 | > 0.1/s | > 1/s |

### Step 6: Check gRPC Service

```bash
# Test gRPC endpoint from agent pod
AGENT_POD=$(kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent \
  -o jsonpath='{.items[0].metadata.name}')

kubectl exec -n k8swatch $AGENT_POD -- \
  curl -v k8swatch-aggregator.k8swatch.svc:50051

# Expected: Connection established
```

### Step 7: Check for Stuck State

```bash
# Check Redis keys (if accessible)
kubectl exec -n k8swatch <redis-pod> -- redis-cli keys '*'

# Check state size via metrics
# In Grafana: k8swatch_aggregator_state_size
```

**If state is corrupted:**

```bash
# WARNING: This clears all aggregator state
# Only do this if correlation is broken

kubectl exec -n k8swatch <redis-pod> -- redis-cli FLUSHDB

# Restart aggregator pods
kubectl rollout restart deployment/k8swatch-aggregator -n k8swatch

# State will rebuild from incoming results
```

### Step 8: Scale Aggregator

```bash
# Check current scale
kubectl get deployment -n k8swatch k8swatch-aggregator

# Scale up if under pressure
kubectl scale deployment -n k8swatch k8swatch-aggregator --replicas=5

# Monitor CPU/memory after scaling
kubectl top pods -n k8swatch -l app.kubernetes.io/component=aggregator
```

---

## Common Issues

### Issue 1: Redis Connection Failures

**Symptoms:**
- Logs show "failed to connect to Redis"
- Results not being processed

**Solution:**
```bash
# Check Redis pod status
kubectl get pods -n k8swatch -l app.kubernetes.io/name=redis

# If Redis pod crashed, check logs
kubectl logs -n k8swatch <redis-pod>

# Restart Redis
kubectl delete pod -n k8swatch <redis-pod>

# Verify connectivity restored
kubectl exec -n k8swatch <aggregator-pod> -- redis-cli -h k8swatch-redis ping
```

### Issue 2: High Result Rejection Rate

**Symptoms:**
- Metrics show high `results_invalid_total`
- Logs show validation errors

**Solution:**
```bash
# Check validation errors in logs
kubectl logs -n k8swatch <aggregator-pod> | grep "validation failed"

# Common causes:
# 1. Agent version mismatch
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent \
  -o jsonpath='{.items[*].spec.containers[*].image}'

# 2. Corrupted results
# Check agent logs for submission errors
kubectl logs -n k8swatch <agent-pod> | grep "SubmitResult"

# Fix: Ensure agent and aggregator versions are compatible
```

### Issue 3: Correlation Failures

**Symptoms:**
- Blast radius calculation fails
- Alerts not correlating properly

**Solution:**
```bash
# Check Redis state
kubectl exec -n k8swatch <redis-pod> -- redis-cli info keyspace

# Check aggregator logs for correlation errors
kubectl logs -n k8swatch <aggregator-pod> | grep "correlation"

# If state corrupted, flush and restart (see Step 7)
```

### Issue 4: Aggregator OOM

**Symptoms:**
- Pod restarts with OOMKilled
- Memory usage increasing

**Solution:**
```bash
# Check current memory limits
kubectl describe pod -n k8swatch <aggregator-pod> | grep -A 5 "Limits"

# Increase memory limits temporarily
kubectl patch deployment k8swatch-aggregator -n k8swatch --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value": "1Gi"}]'

# Long-term: Deploy with updated values
```

---

## Metrics to Monitor

### Aggregator Health Metrics

```promql
# Results received rate
sum(rate(k8swatch_aggregator_results_received_total[5m]))

# Results rejected rate
sum(rate(k8swatch_aggregator_results_invalid_total[5m]))

# Redis errors
sum(rate(k8swatch_aggregator_redis_operations_total{status="error"}[5m]))

# Aggregator pod ready
kube_pod_status_ready{pod=~"k8swatch-aggregator-.*", condition="true"}

# Aggregator CPU usage
sum(rate(container_cpu_usage_seconds_total{pod=~"k8swatch-aggregator-.*"}[5m])) by (pod)

# Aggregator memory usage
sum(container_memory_working_set_bytes{pod=~"k8swatch-aggregator-.*"}) by (pod)
```

### Alert Thresholds

| Metric | Warning | Critical |
|--------|---------|----------|
| Results received | < 0.5/s | < 0.1/s |
| Results rejected | > 0.5/s | > 1/s |
| Redis errors | > 0.1/s | > 1/s |
| Pod not ready | > 5m | > 10m |
| CPU usage | > 80% | > 95% |
| Memory usage | > 80% | > 95% |

---

## Escalation

| Condition | Escalate To |
|-----------|-------------|
| Single aggregator pod down | On-call SRE |
| All aggregators down | Platform Team |
| Redis data loss | SRE Lead + Database Team |
| Correlation broken > 1h | Engineering Lead |

---

## Quick Reference

```bash
# Check aggregator status
kubectl get deployment -n k8swatch k8swatch-aggregator

# Check aggregator pods
kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator

# Check aggregator logs
kubectl logs -n k8swatch <aggregator-pod> --tail=100

# Test Redis connectivity
kubectl exec -n k8swatch <aggregator-pod> -- redis-cli -h k8swatch-redis ping

# Restart aggregator
kubectl rollout restart deployment/k8swatch-aggregator -n k8swatch

# Scale aggregator
kubectl scale deployment -n k8swatch k8swatch-aggregator --replicas=5
```

---

**Related Runbooks:**
- [Alert Investigation](investigate-alert.md)
- [Agent Troubleshooting](agent-troubleshooting.md)
- [Alert Storm](alert-storm.md)
