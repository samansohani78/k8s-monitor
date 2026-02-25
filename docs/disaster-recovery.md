# K8sWatch Disaster Recovery Guide

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** SRE Team

---

## Overview

This document describes disaster recovery procedures for K8sWatch, including backup requirements, recovery procedures, and expected recovery times.

---

## Disaster Scenarios

| Scenario | Severity | RTO | RPO |
|----------|----------|-----|-----|
| Single agent pod failure | Low | 1 min | 0 checks |
| Aggregator pod failure | Medium | 1 min | 0 results |
| Redis data loss | High | 5 min | 5 minutes |
| Complete namespace loss | Critical | 15 min | 0 config |
| Complete cluster loss | Critical | 30 min | 0 config |

**RTO (Recovery Time Objective):** Time to restore service  
**RPO (Recovery Point Objective):** Maximum data loss

---

## Backup Requirements

### What to Backup

| Resource | Backup Method | Frequency | Retention |
|----------|---------------|-----------|-----------|
| **CRDs (Targets, AlertRules)** | GitOps (Git repository) | On change | Permanent |
| **ConfigMaps** | GitOps (Git repository) | On change | Permanent |
| **Secrets** | External secrets manager / Sealed Secrets | On change | Permanent |
| **Redis state** | Redis BGSAVE / Redis replication | Every 5 min | 1 hour |
| **Alert history** | PostgreSQL (if used) | Continuous | 90 days |
| **TLS certificates** | cert-manager (automatic) | Automatic | Until expiry |

### What NOT to Backup

| Resource | Reason |
|----------|--------|
| Agent state | Stateless, rebuilds automatically |
| Aggregator in-memory state | Rebuilds from incoming results |
| Check results | Sent to aggregator, not stored long-term |
| Pod state | Recreated by Deployments/DaemonSets |

---

## Recovery Procedures

### Scenario 1: Agent Pod Failure

**Impact:** Monitoring gaps on single node

**Automatic Recovery:**
- Kubernetes restarts failed pod automatically
- DaemonSet ensures pod is recreated

**Manual Recovery (if automatic fails):**

```bash
# Delete stuck pod
kubectl delete pod -n k8swatch <agent-pod> --force --grace-period=0

# DaemonSet will recreate automatically
kubectl get pods -n k8swatch -l app.kubernetes.io/component=agent
```

**RTO:** 1-2 minutes  
**RPO:** 0-1 checks (30 seconds)

### Scenario 2: Aggregator Pod Failure

**Impact:** Result ingestion paused, no alert correlation

**Automatic Recovery:**
- Kubernetes restarts failed pod
- HPA maintains replica count
- Redis preserves state

**Manual Recovery:**

```bash
# Check pod status
kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator

# Restart deployment
kubectl rollout restart deployment/k8swatch-aggregator -n k8swatch

# Monitor rollout
kubectl rollout status deployment/k8swatch-aggregator -n k8swatch
```

**RTO:** 1-2 minutes  
**RPO:** 0 results (Redis backup)

### Scenario 3: Redis Data Loss

**Impact:** Loss of in-flight alert state, correlation data

**Recovery:**

```bash
# 1. Check Redis status
kubectl get pods -n k8swatch -l app.kubernetes.io/name=redis

# 2. If Redis pod failed, restart
kubectl delete pod -n k8swatch <redis-pod>

# 3. If data lost, aggregator will rebuild state from incoming results
# No manual intervention needed

# 4. Monitor aggregator logs
kubectl logs -n k8swatch -l app.kubernetes.io/component=aggregator -f
```

**Expected Behavior:**
- Aggregator detects Redis connection loss
- Aggregator retries connection
- When Redis available, state rebuilds from new results
- Brief alert correlation gap (1-2 minutes)

**RTO:** 5 minutes  
**RPO:** 5 minutes of alert state

### Scenario 4: Complete Namespace Loss

**Impact:** All K8sWatch components lost

**Recovery:**

```bash
# 1. Recreate namespace
kubectl create namespace k8swatch
kubectl label namespace k8swatch pod-security.kubernetes.io/enforce=baseline

# 2. Apply CRDs
kubectl apply -f https://github.com/k8swatch/k8s-monitor/releases/download/<version>/crds.yaml

# 3. Apply RBAC
kubectl apply -f https://github.com/k8swatch/k8s-monitor/releases/download/<version>/rbac.yaml

# 4. Apply secrets (from backup)
kubectl apply -f secrets-backup.yaml

# 5. Apply ConfigMaps (from GitOps)
kubectl apply -f configmaps.yaml

# 6. Apply K8sWatch components
kubectl apply -f https://github.com/k8swatch/k8s-monitor/releases/download/<version>/manifests.yaml

# 7. Monitor rollout
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=k8swatch \
  -n k8swatch \
  --timeout=300s

# 8. Reapply targets and alert rules (from GitOps)
kubectl apply -f targets-backup.yaml
kubectl apply -f alertrules-backup.yaml
```

**RTO:** 15 minutes  
**RPO:** 0 (GitOps backup)

### Scenario 5: Complete Cluster Loss

**Impact:** Entire Kubernetes cluster lost

**Recovery:**

```bash
# 1. Provision new Kubernetes cluster
# (Procedure varies by provider)

# 2. Install prerequisites
# - cert-manager
# - Prometheus (optional)
# - Grafana (optional)

# 3. Install K8sWatch
kubectl create namespace k8swatch
kubectl apply -f https://github.com/k8swatch/k8s-monitor/releases/download/<version>/crds.yaml
kubectl apply -f https://github.com/k8swatch/k8s-monitor/releases/download/<version>/manifests.yaml

# 4. Restore configuration from GitOps
kubectl apply -f gitops-repo/targets/
kubectl apply -f gitops-repo/alertrules/

# 5. Restore secrets from external secrets manager
# (Procedure varies by secrets manager)

# 6. Verify installation
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=k8swatch \
  -n k8swatch \
  --timeout=300s
```

**RTO:** 30 minutes  
**RPO:** 0 (GitOps backup)

---

## Backup Procedures

### Export Current Configuration

```bash
# Export targets
kubectl get targets.k8swatch.io -A -o yaml > targets-backup-$(date +%Y%m%d).yaml

# Export alert rules
kubectl get alertrules.k8swatch.io -A -o yaml > alertrules-backup-$(date +%Y%m%d).yaml

# Export ConfigMaps
kubectl get configmaps -n k8swatch -o yaml > configmaps-backup-$(date +%Y%m%d).yaml

# Export secrets (careful with sensitive data!)
kubectl get secrets -n k8swatch -o yaml > secrets-backup-$(date +%Y%m%d).yaml
# WARNING: This exports secrets in plain text. Use external secrets manager in production.
```

### Automate Backups with Velero

```yaml
# Install Velero for cluster backups
# https://velero.io/docs/

# Create backup schedule
velero schedule create k8swatch-daily \
  --schedule="0 2 * * *" \
  --include-namespaces k8swatch \
  --ttl 72h

# Create backup on demand
velero backup create k8swatch-manual --include-namespaces k8swatch
```

### Redis Backup

```bash
# Trigger manual backup
kubectl exec -n k8swatch <redis-pod> -- redis-cli BGSAVE

# Check backup status
kubectl exec -n k8swatch <redis-pod> -- redis-cli LASTSAVE

# Configure automatic backups (Redis config)
# save 900 1
# save 300 10
# save 60 10000
```

---

## Testing Disaster Recovery

### Monthly DR Drill

**Objective:** Verify recovery procedures work

**Steps:**

1. **Schedule drill** (off-peak hours)
2. **Notify stakeholders**
3. **Execute failure scenario** (e.g., delete aggregator pods)
4. **Measure recovery time**
5. **Verify data integrity**
6. **Document results**
7. **Update procedures if needed**

**Drill Checklist:**

- [ ] Backup verified recent
- [ ] Recovery procedure documented
- [ ] Team trained on procedure
- [ ] RTO met
- [ ] RPO met
- [ ] No data corruption
- [ ] Monitoring restored
- [ ] Alerting restored

### Quarterly Full DR Test

**Objective:** Full cluster recovery test

**Steps:**

1. **Provision test cluster** (kind/k3d)
2. **Restore from backup**
3. **Verify all components**
4. **Execute test checks**
5. **Verify alerts fire**
6. **Document results**

---

## Monitoring Recovery

### Key Metrics During Recovery

```promql
# Component availability
kube_pod_status_ready{namespace="k8swatch"}

# Check execution rate
sum(rate(k8swatch_agent_check_total[5m]))

# Result ingestion rate
sum(rate(k8swatch_aggregator_results_received_total[5m]))

# Alert firing rate
sum(rate(k8swatch_alertmanager_alerts_fired_total[5m]))
```

### Recovery Verification Checklist

- [ ] All agent pods running (DaemonSet ready)
- [ ] All aggregator pods running (Deployment ready)
- [ ] All alertmanager pods running (Deployment ready)
- [ ] Redis pod running and accepting connections
- [ ] Checks executing (logs show "check completed")
- [ ] Results ingested (logs show "result accepted")
- [ ] Alerts firing (logs show "alert fired")
- [ ] Metrics exporting (Prometheus scraping)
- [ ] Dashboards showing data (Grafana)

---

## Contact Information

| Role | Contact | Escalation |
|------|---------|------------|
| On-call SRE | oncall-sre@example.com | PagerDuty: k8swatch-critical |
| Platform Team | platform@example.com | Slack: #platform-support |
| SRE Lead | sre-lead@example.com | Phone: +1-XXX-XXX-XXXX |

---

## Appendix: Recovery Time Estimates

| Component | Recovery Time | Notes |
|-----------|---------------|-------|
| Agent pod | 30-60 seconds | DaemonSet auto-restart |
| Aggregator pod | 30-60 seconds | Deployment auto-restart |
| AlertManager pod | 30-60 seconds | Deployment auto-restart |
| Redis pod | 1-2 minutes | State rebuild from persistence |
| Full namespace | 10-15 minutes | Manual intervention required |
| Full cluster | 20-30 minutes | New cluster provisioning |

---

**Document Owner:** SRE Team  
**Review Date:** 2026-02-21  
**Next Review:** 2026-05-21 (Quarterly)
