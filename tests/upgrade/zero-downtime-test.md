# Zero-Downtime Upgrade Test Procedure

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** SRE Team

---

## Overview

This document describes the procedure for testing zero-downtime upgrades of K8sWatch components.

---

## Prerequisites

- [ ] K8sWatch v1.0 deployed and healthy
- [ ] New version (v1.1) manifests ready
- [ ] Monitoring dashboards open (Grafana)
- [ ] Log streaming enabled (kubectl logs -f)
- [ ] Backup completed (GitOps repo, secrets)

---

## Test Scenarios

### Scenario 1: Agent DaemonSet Upgrade

**Objective:** Verify agent upgrade rolls out one node at a time with no monitoring gaps.

**Procedure:**

```bash
# 1. Record baseline metrics
BASELINE_CHECKS=$(kubectl get --raw '/apis/metrics.k8s.io/v1beta1/pods' | \
  jq '.items[] | select(.metadata.namespace=="k8swatch") | .usage.cpu')

# 2. Start upgrade
kubectl set image daemonset/k8swatch-agent -n k8swatch \
  agent=k8swatch/agent:v1.1

# 3. Monitor rollout
kubectl rollout status daemonset/k8swatch-agent -n k8swatch --timeout=600s

# 4. Verify no gaps in monitoring
# Check Grafana: K8sWatch - System Health
# Look for gaps in "Check Rate" panel

# 5. Verify all pods ready
kubectl get daemonset -n k8swatch k8swatch-agent
# Expected: DESIRED == CURRENT == READY == UP-TO-DATE
```

**Success Criteria:**
- [ ] Rollout completes without errors
- [ ] All agent pods ready within 10 minutes
- [ ] No gaps in check execution (> 95% success rate maintained)
- [ ] No spike in result drop rate
- [ ] CPU/memory within expected bounds

**Rollback:**
```bash
kubectl rollout undo daemonset/k8swatch-agent -n k8swatch
```

---

### Scenario 2: Aggregator Deployment Upgrade

**Objective:** Verify aggregator upgrade with zero downtime and no result loss.

**Procedure:**

```bash
# 1. Record baseline
BASELINE_RESULTS=$(curl -s http://k8swatch-aggregator.k8swatch:8080/metrics | \
  grep k8swatch_aggregator_results_received_total | awk '{sum+=$1} END {print sum}')

# 2. Start upgrade
kubectl set image deployment/k8swatch-aggregator -n k8swatch \
  aggregator=k8swatch/aggregator:v1.1

# 3. Monitor rollout
kubectl rollout status deployment/k8swatch-aggregator -n k8swatch --timeout=300s

# 4. Watch pod transitions
kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator -w

# 5. Verify continuous result ingestion
# Check Grafana: K8sWatch - System Health
# "Results Received" should have no gaps

# 6. Verify Redis state intact
kubectl exec -n k8swatch <redis-pod> -- redis-cli INFO keyspace
```

**Success Criteria:**
- [ ] Rollout completes without errors
- [ ] At least 2 pods ready at all times (PDB enforced)
- [ ] No gaps in result ingestion
- [ ] Redis state intact (no data loss)
- [ ] Alert correlation continues working

**Rollback:**
```bash
kubectl rollout undo deployment/k8swatch-aggregator -n k8swatch
```

---

### Scenario 3: AlertManager Deployment Upgrade

**Objective:** Verify alertmanager upgrade with no duplicate notifications.

**Procedure:**

```bash
# 1. Start upgrade
kubectl set image deployment/k8swatch-alertmanager -n k8swatch \
  alertmanager=k8swatch/alertmanager:v1.1

# 2. Monitor rollout
kubectl rollout status deployment/k8swatch-alertmanager -n k8swatch --timeout=300s

# 3. Watch for duplicate notifications
# Check Slack/PagerDuty for duplicate alerts during upgrade

# 4. Verify notification delivery
kubectl logs -n k8swatch -l app.kubernetes.io/component=alertmanager \
  | grep "notification sent" | tail -20
```

**Success Criteria:**
- [ ] Rollout completes without errors
- [ ] No duplicate notifications sent
- [ ] No missed notifications
- [ ] Alert state preserved

**Rollback:**
```bash
kubectl rollout undo deployment/k8swatch-alertmanager -n k8swatch
```

---

### Scenario 4: Full System Upgrade

**Objective:** Verify complete system upgrade (all components) with zero downtime.

**Procedure:**

```bash
# 1. Apply all new manifests
kubectl apply -f https://github.com/k8swatch/k8s-monitor/releases/download/v1.1/manifests.yaml

# 2. Monitor all rollouts
kubectl rollout status daemonset/k8swatch-agent -n k8swatch --timeout=600s
kubectl rollout status deployment/k8swatch-aggregator -n k8swatch --timeout=300s
kubectl rollout status deployment/k8swatch-alertmanager -n k8swatch --timeout=300s

# 3. Verify all components healthy
kubectl get all -n k8swatch

# 4. Run smoke test
cat <<EOF | kubectl apply -f -
apiVersion: k8swatch.io/v1
kind: Target
metadata:
  name: upgrade-smoke-test
  namespace: k8swatch
spec:
  type: http
  endpoint:
    dns: https://httpbin.org/get
  schedule:
    interval: 30s
EOF

# 5. Verify smoke test executes
sleep 60
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "upgrade-smoke-test" | tail -5

# 6. Clean up
kubectl delete target upgrade-smoke-test -n k8swatch
```

**Success Criteria:**
- [ ] All rollouts complete without errors
- [ ] All pods ready within 10 minutes
- [ ] No monitoring gaps (> 95% success rate)
- [ ] No duplicate alerts
- [ ] Smoke test executes successfully

**Rollback:**
```bash
kubectl apply -f https://github.com/k8swatch/k8s-monitor/releases/download/v1.0/manifests.yaml
kubectl rollout undo daemonset/k8swatch-agent -n k8swatch
kubectl rollout undo deployment/k8swatch-aggregator -n k8swatch
kubectl rollout undo deployment/k8swatch-alertmanager -n k8swatch
```

---

## Monitoring During Upgrade

### Grafana Dashboards

**Open Before Upgrade:**
1. K8sWatch - System Health
2. K8sWatch - Cluster Health Overview
3. K8sWatch - Alerting Metrics

**Watch For:**
- Gaps in "Check Rate" panel
- Spikes in "Result Drop Rate"
- Drops in "Ready Pods"
- Alert notification failures

### Prometheus Queries

```promql
# Check execution rate (should be continuous)
sum(rate(k8swatch_agent_check_total[1m]))

# Result ingestion rate (should be continuous)
sum(rate(k8swatch_aggregator_results_received_total[1m]))

# Pod readiness
count(kube_pod_status_ready{namespace="k8swatch", condition="true"})

# Error rate (should not spike)
sum(rate(k8swatch_agent_results_dropped_total[1m]))
```

---

## Expected Downtime

| Component | Expected Downtime | Actual | Status |
|-----------|-------------------|--------|--------|
| Agent (per node) | 15-30 seconds | [FILL] | [FILL] |
| Aggregator | 0 seconds (rolling) | [FILL] | [FILL] |
| AlertManager | 0 seconds (rolling) | [FILL] | [FILL] |
| Full System | < 1 minute total | [FILL] | [FILL] |

---

## Test Results Template

### Upgrade Test: [VERSION]

**Date:** [DATE]  
**Tester:** [NAME]  
**From Version:** v[X.X]  
**To Version:** v[Y.Y]

| Scenario | Duration | Success | Notes |
|----------|----------|---------|-------|
| Agent Upgrade | [XX]m | [✅/❌] | [NOTES] |
| Aggregator Upgrade | [XX]m | [✅/❌] | [NOTES] |
| AlertManager Upgrade | [XX]m | [✅/❌] | [NOTES] |
| Full System Upgrade | [XX]m | [✅/❌] | [NOTES] |

### Issues Encountered

| Issue | Severity | Resolution |
|-------|----------|------------|
| [ISSUE] | [Low/Med/High] | [RESOLUTION] |

### Recommendations

1. [RECOMMENDATION 1]
2. [RECOMMENDATION 2]

---

## Sign-off

- [ ] All scenarios passed
- [ ] No data loss observed
- [ ] No duplicate alerts
- [ ] Monitoring gaps < 1%
- [ ] Rollback tested (if needed)

**Approved for Production:** [YES/NO]  
**Approved By:** [NAME]  
**Date:** [DATE]

---

**Next Steps:**
- [ ] Apply to production
- [ ] Monitor for 24 hours
- [ ] Document any production-specific issues
