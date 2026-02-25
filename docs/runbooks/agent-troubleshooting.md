# Runbook: Agent Troubleshooting

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** SRE Team

---

## Overview

This runbook provides step-by-step instructions for troubleshooting K8sWatch agent issues.

---

## Symptoms

| Symptom | Severity | Likely Cause |
|---------|----------|--------------|
| Agent not reporting | Critical | Pod crashed, network issue |
| Gaps in monitoring | High | Agent restarting, check failures |
| High check latency | Medium | Node resource pressure |
| Results dropped | Medium | Aggregator unreachable |

---

## Investigation Steps

### Step 1: Check DaemonSet Status

```bash
# Check overall DaemonSet health
kubectl get daemonset -n k8swatch k8swatch-agent

# Expected output:
# NAME             DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE
# k8swatch-agent   10        10        10      10           10

# Check if DESIRED == CURRENT == READY
# If not, investigate missing agents
```

**Troubleshooting:**

| Issue | Command | Solution |
|-------|---------|----------|
| DESIRED != CURRENT | `kubectl get nodes` | Check if nodes are ready |
| CURRENT != READY | `kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent` | Check agent pod status |
| UP-TO-DATE < DESIRED | `kubectl rollout status daemonset/k8swatch-agent -n k8swatch` | Wait for rollout or check image pull issues |

### Step 2: Check Agent Pods

```bash
# List all agent pods with node assignment
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent -o wide

# Check for pods not in Running state
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent --field-selector=status.phase!=Running
```

**Pod Status Troubleshooting:**

| Status | Command | Solution |
|--------|---------|----------|
| Pending | `kubectl describe pod -n k8swatch <pod>` | Check resource quotas, node capacity |
| CrashLoopBackOff | `kubectl logs -n k8swatch <pod> --previous` | Check application errors |
| ImagePullBackOff | `kubectl describe pod -n k8swatch <pod>` | Check image name, registry credentials |
| Evicted | `kubectl get events -n k8swatch --field-selector involvedObject.name=<pod>` | Check node resource pressure |

### Step 3: Review Agent Logs

```bash
# Check logs for specific agent
kubectl logs -n k8swatch <agent-pod> --tail=100

# Stream logs in real-time
kubectl logs -n k8swatch <agent-pod> -f

# Search for errors
kubectl logs -n k8swatch <agent-pod> | grep -i error

# Check for specific target issues
kubectl logs -n k8swatch <agent-pod> | grep "target=<target-name>"
```

**Common Log Patterns:**

| Log Pattern | Meaning | Action |
|-------------|---------|--------|
| `failed to connect to aggregator` | Network issue | Check aggregator service, network policies |
| `config fetch failed` | RBAC or K8s API issue | Check ServiceAccount, ClusterRole |
| `check timeout` | Target unreachable | Check target health, network connectivity |
| `result dropped` | Aggregator unreachable | Check aggregator availability |
| `certificate expired` | TLS issue | Check certificate expiry, renew if needed |

### Step 4: Check Agent on Specific Node

```bash
# Find agent pod on specific node
NODE_NAME="node-1"
AGENT_POD=$(kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent \
  -o jsonpath="{.items[?(@.spec.nodeName=='$NODE_NAME')].metadata.name}")

# Check pod status
kubectl get pod -n k8swatch $AGENT_POD

# Describe pod for events
kubectl describe pod -n k8swatch $AGENT_POD

# Check logs
kubectl logs -n k8swatch $AGENT_POD
```

### Step 5: Test Agent Connectivity

```bash
# Test connectivity to aggregator from agent
kubectl exec -n k8swatch <agent-pod> -- \
  curl -v k8swatch-aggregator.k8swatch.svc:50051

# Test DNS resolution
kubectl exec -n k8swatch <agent-pod> -- \
  nslookup k8swatch-aggregator.k8swatch.svc

# Test K8s API connectivity
kubectl exec -n k8swatch <agent-pod> -- \
  curl -s https://kubernetes.default.svc/healthz
```

**Expected Results:**

| Test | Expected | If Fails |
|------|----------|----------|
| Aggregator curl | Connection established | Check aggregator service, network policies |
| DNS lookup | Resolves to ClusterIP | Check CoreDNS, kube-dns |
| K8s API | Returns OK | Check ServiceAccount, RBAC |

### Step 6: Check Resource Usage

```bash
# Check agent resource usage
kubectl top pods -n k8swatch -l app.kubernetes.io/component=agent

# Check node resources
kubectl top node <node-name>

# Check for resource limits
kubectl describe pod -n k8swatch <agent-pod> | grep -A 5 "Limits"
```

**Resource Thresholds:**

| Resource | Warning | Critical | Action |
|----------|---------|----------|--------|
| CPU | > 150m | > 180m | Increase limits, check for loops |
| Memory | > 200Mi | > 240Mi | Increase limits, check for leaks |

### Step 7: Restart Agent

```bash
# Delete agent pod (will be recreated by DaemonSet)
kubectl delete pod -n k8swatch <agent-pod>

# Wait for new pod to be ready
kubectl wait -n k8swatch -l app.kubernetes.io/component=agent \
  --for=condition=Ready pod/<agent-pod> --timeout=120s

# Verify new pod is running
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent
```

---

## Common Issues

### Issue 1: Agent Not Reporting After Node Restart

**Symptoms:**
- Node restarted
- Agent pod not recreated

**Solution:**
```bash
# Check if node is ready
kubectl get node <node-name>

# If node NotReady, investigate node issue
kubectl describe node <node-name>

# Force delete stuck agent pod
kubectl delete pod -n k8swatch <agent-pod> --grace-period=0 --force

# DaemonSet will recreate pod
```

### Issue 2: High Check Latency

**Symptoms:**
- P99 latency > 5s
- Checks timing out

**Solution:**
```bash
# Check node resource pressure
kubectl top node <node-name>

# Check for FD exhaustion
kubectl exec -n k8swatch <agent-pod> -- \
  cat /host/proc/sys/fs/file-nr

# Check conntrack pressure
kubectl exec -n k8swatch <agent-pod> -- \
  cat /host/proc/sys/net/netfilter/nf_conntrack_count

# If resources exhausted, restart agent
kubectl delete pod -n k8swatch <agent-pod>
```

### Issue 3: Results Dropped

**Symptoms:**
- Logs show "result dropped"
- Aggregator not receiving results

**Solution:**
```bash
# Check aggregator availability
kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator

# Test connectivity to aggregator
kubectl exec -n k8swatch <agent-pod> -- \
  curl -v k8swatch-aggregator.k8swatch.svc:50051

# If aggregator down, scale up
kubectl scale deployment -n k8swatch k8swatch-aggregator --replicas=3
```

### Issue 4: Certificate Errors

**Symptoms:**
- TLS handshake failures
- Certificate expired errors

**Solution:**
```bash
# Check certificate expiry
kubectl get secret k8swatch-agent-tls -n k8swatch \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | \
  openssl x509 -noout -dates

# If expired, renew certificate
kubectl delete secret k8swatch-agent-tls -n k8swatch
kubectl delete certificate k8swatch-agent-cert -n k8swatch

# Wait for cert-manager to renew
kubectl wait certificate k8swatch-agent-cert -n k8swatch \
  --for=condition=Ready --timeout=120s

# Restart agent to pick up new cert
kubectl rollout restart daemonset/k8swatch-agent -n k8swatch
```

---

## Metrics to Monitor

### Agent Health Metrics

```promql
# Agent check rate
sum(rate(k8swatch_agent_check_total{namespace="k8swatch"}[5m])) by (node)

# Agent check latency (P99)
histogram_quantile(0.99, sum(rate(k8swatch_agent_check_duration_seconds_bucket[5m])) by (le))

# Result drop rate
sum(rate(k8swatch_agent_results_dropped_total[5m])) by (node)

# Agent pod ready
kube_pod_status_ready{pod=~"k8swatch-agent-.*", condition="true"}
```

### Alert Thresholds

| Metric | Warning | Critical |
|--------|---------|----------|
| Check rate | < 0.5/s per node | < 0.1/s per node |
| P99 latency | > 5s | > 10s |
| Result drop rate | > 0.1/s | > 1/s |
| Pod not ready | > 5m | > 10m |

---

## Escalation

| Condition | Escalate To |
|-----------|-------------|
| Single agent down > 30m | On-call SRE |
| Multiple agents down | Platform Team |
| Cluster-wide agent failure | SRE Lead + Platform Lead |

---

## Quick Reference

```bash
# Check all agents
kubectl get daemonset -n k8swatch k8swatch-agent

# Find agent on node
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent -o wide | grep <node>

# Check agent logs
kubectl logs -n k8swatch <agent-pod> --tail=100

# Restart agent
kubectl delete pod -n k8swatch <agent-pod>

# Test aggregator connectivity
kubectl exec -n k8swatch <agent-pod> -- curl -v k8swatch-aggregator.k8swatch.svc:50051
```

---

**Related Runbooks:**
- [Alert Investigation](investigate-alert.md)
- [Aggregator Issues](aggregator-issues.md)
- [System Issues](system-issues.md)
