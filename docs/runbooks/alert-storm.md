# Runbook: Alert Storm Handling

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** SRE Team

---

## Overview

This runbook provides step-by-step instructions for handling alert storms - situations where hundreds of alerts fire simultaneously.

---

## What is an Alert Storm?

An alert storm occurs when a single underlying issue triggers a large volume of alerts, potentially:
- Overwhelming notification channels
- Causing alert fatigue
- Hiding the root cause
- Slowing down incident response

---

## Detection

### Indicators

| Indicator | Threshold | Metric |
|-----------|-----------|--------|
| High alert volume | > 10 alerts/5m | `sum(rate(k8swatch_alertmanager_alerts_fired_total[5m]))` |
| Many targets failing | > 30% of targets | `sum(k8swatch_agent_check_total{status="failure"}) / sum(k8swatch_agent_check_total)` |
| Cluster-wide blast radius | All/most nodes affected | Check blast radius in alert |
| Single failure layer | > 80% same layer | Check failure_layer distribution |

### Grafana Dashboard

**Open:** `K8sWatch - Alerting Metrics`

**Check Panels:**
- Alerts fired over time (spike indicates storm)
- Top failing targets (many targets = widespread issue)
- Failure code distribution (single code = common cause)

---

## Investigation Steps

### Step 1: Identify Root Cause Pattern

```bash
# Check blast radius from alerts
# Look for pattern in alert notifications:
# - All nodes failing same target = Target outage
# - Single node failing all targets = Node issue
# - Zone-level failures = Zone outage
# - Cluster-wide DNS failures = DNS outage
```

**Pattern Recognition:**

| Pattern | Likely Cause | Action |
|---------|--------------|--------|
| All nodes, single target | Target service down | Contact target owner |
| Single node, all targets | Node network issue | Check node, restart agent |
| Zone-level, multiple targets | Zone outage | Check zone infrastructure |
| Cluster-wide, DNS layer | CoreDNS failure | Check CoreDNS pods |
| Cluster-wide, network layer | CNI failure | Check CNI components |

### Step 2: Check Failure Layer Distribution

**Open:** `K8sWatch - Cluster Health Overview`

**Check:** Failure code distribution panel

| Failure Layer | Meaning | Action |
|---------------|---------|--------|
| L1 (DNS) | DNS resolution failing | Check CoreDNS |
| L2 (TCP) | Network connectivity | Check CNI, network policies |
| L3 (TLS) | Certificate issues | Check cert expiry |
| L4/L5 | Application/Auth issues | Check target service |
| L6 | Application logic | Check target application |

### Step 3: Identify Affected Scope

```bash
# Count affected nodes
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent \
  -o jsonpath='{.items[*].spec.nodeName}' | tr ' ' '\n' | sort -u | wc -l

# Check which nodes are reporting failures
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "status=failure" | grep -o "node=[^ ]*" | sort | uniq -c | sort -rn
```

### Step 4: Check Recent Changes

```bash
# Check recent deployments
kubectl get deployments -A --sort-by='.metadata.creationTimestamp' | tail -20

# Check recent node changes
kubectl get events -A --field-selector reason=NodeNotReady --sort-by='.lastTimestamp'

# Check recent config changes
kubectl get configmaps -n k8swatch -o jsonpath='{.items[*].metadata.creationTimestamp}'
```

---

## Mitigation Actions

### Action 1: Apply Silencing (If Overwhelming)

**When:** Alert volume is preventing effective response

```bash
# Option 1: Silence by namespace (if namespace-specific)
curl -X POST http://k8swatch-alertmanager.k8swatch.svc:8080/api/v1/silence \
  -H "Content-Type: application/json" \
  -d '{
    "matchers": [{"name": "namespace", "value": "affected-namespace"}],
    "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "endsAt": "'$(date -u -d '+30 minutes' +%Y-%m-%dT%H:%M:%SZ)'",
    "createdBy": "on-call-sre",
    "comment": "Alert storm mitigation - investigating root cause"
  }'

# Option 2: Silence by target category (if category-wide)
curl -X POST http://k8swatch-alertmanager.k8swatch.svc:8080/api/v1/silence \
  -H "Content-Type: application/json" \
  -d '{
    "matchers": [{"name": "target_category", "value": "database"}],
    "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "endsAt": "'$(date -u -d '+30 minutes' +%Y-%m-%dT%H:%M:%SZ)'",
    "createdBy": "on-call-sre",
    "comment": "Database alert storm - investigating"
  }'

# Option 3: Silence all K8sWatch alerts (last resort)
curl -X POST http://k8swatch-alertmanager.k8swatch.svc:8080/api/v1/silence \
  -H "Content-Type: application/json" \
  -d '{
    "matchers": [{"name": "alertname", "value": ".+", "isRegex": true}],
    "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "endsAt": "'$(date -u -d '+30 minutes' +%Y-%m-%dT%H:%M:%SZ)'",
    "createdBy": "on-call-sre",
    "comment": "Emergency silence - investigating alert storm"
  }'
```

**Important:** 
- Set silence duration to 30 minutes initially
- Extend if needed
- Always include comment explaining reason

### Action 2: Escalate Immediately

**When:** Blast radius is cluster-wide or zone-level

**Escalation Template:**

```
Subject: [ALERT STORM] K8sWatch - <Blast Radius> - <Failure Layer>

Alert Storm Detected:
- Start Time: <time>
- Blast Radius: <node|zone|cluster>
- Failure Layer: <L1|L2|L3|L4|L5|L6>
- Dominant Failure Code: <code>
- Alerts Fired: <count> in last 5m
- Targets Affected: <count>/<total>

Pattern Analysis:
- <All nodes failing single target OR Single node failing all targets>
- <Failure layer analysis>

Silencing Applied:
- <Yes/No>
- <Scope: namespace|category|all>
- <Duration: 30m>

Immediate Actions Needed:
- <Check CoreDNS / Check CNI / Contact target owner>

Request:
<What you need from escalatee>
```

### Action 3: Focus on Root Cause

**Don't:** Investigate individual alerts

**Do:** Identify and fix the underlying issue

| Root Cause | Investigation |
|------------|---------------|
| DNS outage | Check CoreDNS pods, DNS resolution |
| Network outage | Check CNI, node networking |
| Target outage | Check target service, contact owner |
| Certificate expiry | Check cert expiry, renew |

---

## Common Alert Storm Scenarios

### Scenario 1: CoreDNS Failure

**Pattern:**
- All nodes affected
- L1 (DNS) failures
- Failure code: `dns_timeout` or `dns_servfail`

**Investigation:**
```bash
# Check CoreDNS pods
kubectl get pods -n kube-system -l k8s-app=kube-dns

# Check CoreDNS logs
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=100

# Test DNS resolution
kubectl run -it --rm dns-test --image=busybox:1.36 --restart=Never -- \
  nslookup kubernetes.default
```

**Resolution:**
```bash
# Restart CoreDNS
kubectl rollout restart deployment/coredns -n kube-system

# Wait for pods to be ready
kubectl wait -n kube-system -l k8s-app=kube-dns \
  --for=condition=Ready pod --timeout=120s
```

### Scenario 2: CNI Failure

**Pattern:**
- All nodes or zone-level affected
- L2 (TCP) failures
- Failure code: `tcp_timeout` or `tcp_refused`

**Investigation:**
```bash
# Check CNI pods
kubectl get pods -n kube-system -l app=calico-node  # or your CNI

# Check node networking
kubectl get nodes
kubectl describe node <node-name> | grep -A 10 "Conditions"
```

**Resolution:**
```bash
# Restart CNI pods
kubectl rollout restart daemonset/calico-node -n kube-system

# Or contact network team if CNI issue
```

### Scenario 3: Target Service Outage

**Pattern:**
- All nodes failing single target
- L4/L5/L6 failures
- Specific to one service

**Investigation:**
```bash
# Check target service
kubectl get pods -n <namespace> -l app=<target-app>

# Check service endpoints
kubectl get endpoints -n <namespace> <target-service>
```

**Resolution:**
```bash
# Contact target service owner
# Or restart target service if you have access
kubectl rollout restart deployment/<target> -n <namespace>
```

### Scenario 4: Certificate Expiry

**Pattern:**
- All nodes failing HTTPS/TLS targets
- L3 (TLS) failures
- Failure code: `tls_expired` or `tls_invalid`

**Investigation:**
```bash
# Check certificate expiry
kubectl get secrets -A -o jsonpath='{range .items[*]}{.metadata.name}: {.data.tls\.crt}{"\n"}{end}' \
  | base64 -d | openssl x509 -noout -dates 2>/dev/null
```

**Resolution:**
```bash
# Renew expired certificates
# Or use cert-manager for automatic renewal
```

---

## Post-Storm Actions

### 1. Remove Silences

```bash
# List active silences
curl http://k8swatch-alertmanager.k8swatch.svc:8080/api/v1/silences

# Remove silence by ID
curl -X DELETE http://k8swatch-alertmanager.k8swatch.svc:8080/api/v1/silence/<silence-id>
```

### 2. Verify Alerting Resumed

```bash
# Check alerts firing (should return to normal)
# Grafana: K8sWatch - Alerting Metrics

# Check notification delivery
kubectl logs -n k8swatch -l app.kubernetes.io/component=alertmanager \
  | grep "notification sent" | tail -20
```

### 3. Document Learnings

**Update this runbook with:**
- What pattern was observed
- What root cause was
- How long resolution took
- What could be improved

### 4. Schedule Post-Mortem

**When:**
- Alert storm lasted > 30 minutes
- Affected production services
- Revealed systemic issues

**Topics:**
- Why did alert storm occur?
- Could correlation have prevented it?
- Are silencing rules appropriate?
- What monitoring gaps exist?

---

## Prevention

### Improve Correlation

- Ensure blast radius classification is accurate
- Tune correlation thresholds
- Group related alerts

### Tune Alerting Rules

```yaml
# Example: Increase consecutive failures threshold for non-critical targets
apiVersion: k8swatch.io/v1
kind: AlertRule
metadata:
  name: p2-database-alerts
spec:
  trigger:
    consecutiveFailures: 5  # Increased from 3
    timeWindow: 5m
```

### Add Storm Detection

```yaml
# Prometheus rule for storm detection
- alert: K8sWatchAlertStormDetected
  expr: sum(rate(k8swatch_alertmanager_alerts_fired_total[5m])) > 10
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Alert storm detected - {{ $value }} alerts/5m"
```

---

## Quick Reference

```bash
# Check alert volume
# Grafana: K8sWatch - Alerting Metrics

# Apply 30m silence
curl -X POST http://k8swatch-alertmanager.k8swatch.svc:8080/api/v1/silence \
  -H "Content-Type: application/json" \
  -d '{"matchers": [{"name": "alertname", "value": ".+", "isRegex": true}], "startsAt": "NOW", "endsAt": "NOW+30m", "createdBy": "on-call", "comment": "Storm mitigation"}'

# Check failure pattern
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "failure_code" | sort | uniq -c | sort -rn

# Escalate immediately if cluster-wide
```

---

**Related Runbooks:**
- [Alert Investigation](investigate-alert.md)
- [Aggregator Issues](aggregator-issues.md)
- [System Issues](system-issues.md)
