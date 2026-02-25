# K8sWatch Upgrade Guide

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** SRE Team

---

## Overview

This document describes the procedures for upgrading K8sWatch components with zero downtime.

---

## Pre-Upgrade Checklist

### 1. Verify Current Health

```bash
# Check all components are healthy
kubectl get daemonset -n k8swatch
kubectl get deployment -n k8swatch
kubectl get pods -n k8swatch

# Check no critical alerts are firing
# Grafana: K8sWatch - Alerting Metrics
```

**Expected:**
- All daemonset pods ready
- All deployment replicas available
- No critical alerts firing

### 2. Check Backup Status

```bash
# Verify Redis backup (if configured)
kubectl exec -n k8swatch <redis-pod> -- redis-cli BGSAVE

# Check last backup time
kubectl exec -n k8swatch <redis-pod> -- redis-cli LASTSAVE
```

### 3. Review Release Notes

- Check for breaking changes
- Review new features
- Note any required configuration changes

### 4. Notify Stakeholders

Send notification to:
- SRE team
- Platform team
- Dependent service owners

**Template:**
```
Subject: [NOTICE] K8sWatch Upgrade Scheduled for <date>

Hi Team,

We will be upgrading K8sWatch from v<old> to v<new> on <date> at <time>.

Expected Impact:
- No downtime expected
- Brief monitoring gaps possible during agent rollout (5-10 minutes)

Rollback Plan:
- If issues occur, we will rollback to v<old> within 15 minutes

Contact: <on-call-sre>
```

---

## Upgrade Procedures

### Method 1: Helm Upgrade (Recommended)

**Prerequisites:**
- Helm 3.10+
- K8sWatch installed via Helm

**Steps:**

```bash
# 1. Update Helm repository
helm repo update k8swatch

# 2. Review changes
helm diff upgrade k8swatch k8swatch/k8swatch \
  -n k8swatch \
  -f values.yaml \
  --version <new-version>

# 3. Perform upgrade
helm upgrade k8swatch k8swatch/k8swatch \
  -n k8swatch \
  -f values.yaml \
  --version <new-version> \
  --atomic \
  --timeout 10m

# 4. Monitor rollout
helm status k8swatch -n k8swatch

# 5. Verify components
kubectl rollout status daemonset/k8swatch-agent -n k8swatch
kubectl rollout status deployment/k8swatch-aggregator -n k8swatch
kubectl rollout status deployment/k8swatch-alertmanager -n k8swatch
```

**Rollback:**

```bash
# Rollback to previous version
helm rollback k8swatch -n k8swatch

# Or rollback to specific revision
helm rollback k8swatch <revision> -n k8swatch
```

### Method 2: Manifest Upgrade

**Prerequisites:**
- K8sWatch installed via kubectl apply
- kubectl 1.25+

**Steps:**

```bash
# 1. Download new manifests
curl -LO https://github.com/k8swatch/k8s-monitor/releases/download/<version>/manifests.yaml

# 2. Review changes
git diff --no-index manifests-old.yaml manifests.yaml || true

# 3. Apply new manifests
kubectl apply -f manifests.yaml

# 4. Monitor rollout
kubectl rollout status daemonset/k8swatch-agent -n k8swatch
kubectl rollout status deployment/k8swatch-aggregator -n k8swatch
kubectl rollout status deployment/k8swatch-alertmanager -n k8swatch

# 5. Verify pods are ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=k8swatch \
  -n k8swatch \
  --timeout=120s
```

**Rollback:**

```bash
# Download old manifests
curl -LO https://github.com/k8swatch/k8s-monitor/releases/download/<old-version>/manifests.yaml

# Apply old manifests
kubectl apply -f manifests.yaml

# Force rollback
kubectl rollout undo daemonset/k8swatch-agent -n k8swatch
kubectl rollout undo deployment/k8swatch-aggregator -n k8swatch
kubectl rollout undo deployment/k8swatch-alertmanager -n k8swatch
```

### Method 3: Kustomize Upgrade

**Prerequisites:**
- Kustomize 5.0+
- K8sWatch deployed via Kustomize

**Steps:**

```bash
# 1. Update base version in kustomization.yaml
# Edit kustomization.yaml:
# bases:
# - github.com/k8swatch/k8s-monitor/deploy/default?ref=<new-version>

# 2. Build and apply
kustomize build overlays/production | kubectl apply -f -

# 3. Monitor rollout
kubectl rollout status daemonset/k8swatch-agent -n k8swatch
kubectl rollout status deployment/k8swatch-aggregator -n k8swatch
kubectl rollout status deployment/k8swatch-alertmanager -n k8swatch
```

**Rollback:**

```bash
# Revert kustomization.yaml to old version
# Then rebuild and apply
kustomize build overlays/production | kubectl apply -f -
```

---

## Component-Specific Procedures

### Agent DaemonSet Upgrade

**Strategy:** RollingUpdate with maxUnavailable: 1

```bash
# Monitor agent rollout
watch kubectl get daemonset -n k8swatch k8swatch-agent

# Check agent pods
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent

# Verify all nodes covered
kubectl get daemonset k8swatch-agent -n k8swatch \
  -o jsonpath='{.status.numberReady}/{.status.desiredNumberScheduled}'
```

**Expected Duration:** 5-10 minutes for 100 nodes

**Monitoring Gaps:**
- Each node has ~15 second gap during agent restart
- No cluster-wide gaps expected

### Aggregator Deployment Upgrade

**Strategy:** RollingUpdate with maxSurge: 1, maxUnavailable: 0

```bash
# Monitor aggregator rollout
watch kubectl get deployment -n k8swatch k8swatch-aggregator

# Check aggregator pods
kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator

# Verify HPA is still active
kubectl get hpa -n k8swatch k8swatch-aggregator
```

**Expected Duration:** 2-5 minutes

**Zero Downtime:**
- Old pods remain ready until new pods are ready
- No result loss expected

### AlertManager Deployment Upgrade

**Strategy:** RollingUpdate with maxSurge: 1, maxUnavailable: 0

```bash
# Monitor alertmanager rollout
watch kubectl get deployment -n k8swatch k8swatch-alertmanager

# Check alertmanager pods
kubectl get pods -n k8swatch -l app.kubernetes.io/component=alertmanager

# Verify no alerts lost
# Grafana: K8sWatch - Alerting Metrics
```

**Expected Duration:** 2-5 minutes

**Zero Downtime:**
- In-flight alerts preserved
- No duplicate notifications expected

---

## CRD Upgrades

### Minor Version (v1.0 → v1.1)

**No action required** - CRDs are backward compatible

### Major Version (v1 → v2)

```bash
# 1. Export existing resources
kubectl get targets.k8swatch.io -A -o yaml > targets-backup.yaml
kubectl get alertrules.k8swatch.io -A -o yaml > alertrules-backup.yaml

# 2. Apply new CRDs
kubectl apply -f config/crd/bases/

# 3. Verify CRD conversion
kubectl get targets.k8swatch.io -A

# 4. If conversion fails, apply conversion webhook
kubectl apply -f config/crd/conversion-webhook.yaml
```

---

## Post-Upgrade Verification

### 1. Verify Component Health

```bash
# Check all components
kubectl get all -n k8swatch

# Expected: All pods Running, all deployments available
```

### 2. Verify Check Execution

```bash
# Check agent logs for successful checks
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "check completed" | tail -10

# Check aggregator logs for result ingestion
kubectl logs -n k8swatch -l app.kubernetes.io/component=aggregator \
  | grep "result accepted" | tail -10
```

### 3. Verify Alerting

```bash
# Check alertmanager logs
kubectl logs -n k8swatch -l app.kubernetes.io/component=alertmanager \
  | grep "alert processed" | tail -10

# Verify metrics are being exported
curl http://k8swatch-aggregator.k8swatch.svc:8080/metrics | head -20
```

### 4. Verify Dashboards

**Grafana:**
- Open `K8sWatch - Cluster Health Overview`
- Verify data is flowing
- Check for any gaps during upgrade

### 5. Run Smoke Tests

```bash
# Create a test target
cat <<EOF | kubectl apply -f -
apiVersion: k8swatch.io/v1
kind: Target
metadata:
  name: smoke-test-http
  namespace: k8swatch
spec:
  type: http
  endpoint:
    dns: https://httpbin.org/get
  schedule:
    interval: 30s
EOF

# Wait for check
sleep 60

# Check logs for smoke test
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "smoke-test-http" | tail -5

# Clean up
kubectl delete target smoke-test-http -n k8swatch
```

---

## Troubleshooting

### Issue 1: Agent Rollout Stuck

**Symptoms:**
- DaemonSet shows `DESIRED != CURRENT`
- Pods stuck in `Terminating`

**Solution:**
```bash
# Check for finalizers
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent \
  -o jsonpath='{.items[*].metadata.finalizers}'

# Force delete stuck pod
kubectl delete pod -n k8swatch <pod> --grace-period=0 --force

# Check node health
kubectl get nodes
```

### Issue 2: Aggregator Not Starting

**Symptoms:**
- New aggregator pods CrashLoopBackOff
- Logs show connection errors

**Solution:**
```bash
# Check Redis connectivity
kubectl logs -n k8swatch <aggregator-pod>

# Verify Redis is running
kubectl get pods -n k8swatch -l app.kubernetes.io/name=redis

# Check Redis connection string
kubectl get deployment k8swatch-aggregator -n k8swatch \
  -o jsonpath='{.spec.template.spec.containers[*].env[?(@.name=="REDIS_URL")].value}'
```

### Issue 3: Alerts Not Firing After Upgrade

**Symptoms:**
- Checks executing but alerts not firing

**Solution:**
```bash
# Check aggregator logs for correlation errors
kubectl logs -n k8swatch -l app.kubernetes.io/component=aggregator \
  | grep "correlation"

# Check alertmanager logs
kubectl logs -n k8swatch -l app.kubernetes.io/component=alertmanager \
  | grep "alert"

# Restart alertmanager
kubectl rollout restart deployment/k8swatch-alertmanager -n k8swatch
```

### Issue 4: Metrics Missing After Upgrade

**Symptoms:**
- Prometheus showing gaps
- Metrics endpoint not responding

**Solution:**
```bash
# Check metrics endpoint
kubectl exec -n k8swatch <pod> -- curl localhost:8080/metrics

# Verify ServiceMonitor
kubectl get servicemonitor -n k8swatch

# Restart Prometheus (if self-hosted)
kubectl rollout restart statefulset/prometheus -n monitoring
```

---

## Version-Specific Notes

### v1.0 → v1.1

**Breaking Changes:** None

**New Features:**
- mTLS support
- Enhanced security scanning

**Upgrade Time:** 10-15 minutes

**Special Instructions:**
- Generate TLS certificates before upgrade
- Apply TLS secrets first

### v1.1 → v1.2

**Breaking Changes:** None

**New Features:**
- Redis cluster support
- Leader election

**Upgrade Time:** 15-20 minutes

**Special Instructions:**
- Migrate Redis data if upgrading from single instance
- Test leader election failover

---

## Rollback Procedures

### When to Rollback

Rollback if:
- Critical functionality broken
- Data loss detected
- Performance degradation > 50%
- Upgrade duration exceeds 2x expected time

### Rollback Steps

```bash
# 1. Stop the upgrade
# Helm:
helm rollback k8swatch -n k8swatch

# Manifest:
kubectl apply -f manifests-old.yaml

# 2. Force rollback if needed
kubectl rollout undo daemonset/k8swatch-agent -n k8swatch
kubectl rollout undo deployment/k8swatch-aggregator -n k8swatch
kubectl rollout undo deployment/k8swatch-alertmanager -n k8swatch

# 3. Verify rollback
kubectl get all -n k8swatch

# 4. Investigate failure
# Review logs, events, metrics
```

---

## Maintenance Mode

For major upgrades, consider enabling maintenance mode:

```bash
# Silence all K8sWatch alerts
curl -X POST http://k8swatch-alertmanager.k8swatch.svc:8080/api/v1/silence \
  -H "Content-Type: application/json" \
  -d '{
    "matchers": [{"name": "alertname", "value": ".+", "isRegex": true}],
    "startsAt": "NOW",
    "endsAt": "NOW+1h",
    "createdBy": "upgrade-procedure",
    "comment": "K8sWatch upgrade maintenance"
  }'

# Remember to remove silence after upgrade
```

---

**Document Owner:** SRE Team  
**Review Date:** 2026-02-21  
**Next Review:** 2026-05-21 (Quarterly)
