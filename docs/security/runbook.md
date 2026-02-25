# K8sWatch Security Runbook

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** Security Team + SRE

---

## Purpose

This runbook provides step-by-step procedures for security-related operations and incident response for K8sWatch.

---

## Table of Contents

1. [Credential Rotation](#credential-rotation)
2. [Certificate Management](#certificate-management)
3. [RBAC Audit](#rbac-audit)
4. [Security Incident Response](#security-incident-response)
5. [Emergency Procedures](#emergency-procedures)

---

## Credential Rotation

### Database Credentials

**Frequency:** Every 90 days or after personnel changes

#### PostgreSQL

```bash
# 1. Generate new password
NEW_PASSWORD=$(openssl rand -base64 32)
echo "New password: ${NEW_PASSWORD}"

# 2. Update PostgreSQL password
kubectl run -it --rm postgres-update --image=postgres:15 --restart=Never -- \
  psql -h <postgres-host> -U postgres -c \
  "ALTER ROLE k8swatch_reader WITH PASSWORD '${NEW_PASSWORD}';"

# 3. Update Kubernetes Secret
kubectl create secret generic postgres-health-check \
  --from-literal=username=k8swatch_reader \
  --from-literal=password="${NEW_PASSWORD}" \
  --from-literal=database=postgres \
  -n k8swatch \
  --dry-run=client -o yaml | kubectl apply -f -

# 4. Verify (wait 30 seconds for next check interval)
sleep 30
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "postgres-primary" | tail -5

# 5. Clean up
# Password stored in shell history - consider clearing
history -d $(history 1 | awk '{print $1}')
```

#### MySQL

```bash
# 1. Generate new password
NEW_PASSWORD=$(openssl rand -base64 32)

# 2. Update MySQL password
kubectl run -it --rm mysql-update --image=mysql:8 --restart=Never -- \
  mysql -h <mysql-host> -u root -p -e \
  "ALTER USER 'k8swatch_reader'@'%' IDENTIFIED BY '${NEW_PASSWORD}'; FLUSH PRIVILEGES;"

# 3. Update Kubernetes Secret
kubectl create secret generic mysql-health-check \
  --from-literal=username=k8swatch_reader \
  --from-literal=password="${NEW_PASSWORD}" \
  --from-literal=database=mysql \
  -n k8swatch \
  --dry-run=client -o yaml | kubectl apply -f -

# 4. Verify
sleep 30
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "mysql-primary" | tail -5
```

#### MongoDB

```bash
# 1. Generate new password
NEW_PASSWORD=$(openssl rand -base64 32)

# 2. Update MongoDB password
kubectl run -it --rm mongo-update --image=mongo:6 --restart=Never -- \
  mongosh -h <mongo-host> --authenticationDatabase admin -u admin -p \
  --eval '
  db.getSiblingDB("admin").setUserPassword("k8swatch_reader", "'"${NEW_PASSWORD}"'");
  '

# 3. Update Kubernetes Secret
kubectl create secret generic mongodb-health-check \
  --from-literal=username=k8swatch_reader \
  --from-literal=password="${NEW_PASSWORD}" \
  --from-literal=authSource=admin \
  -n k8swatch \
  --dry-run=client -o yaml | kubectl apply -f -

# 4. Verify
sleep 30
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "mongodb-primary" | tail -5
```

### API Keys (Elasticsearch)

```bash
# 1. Delete old API key
kubectl get secret elasticsearch-api-key -n k8swatch \
  -o jsonpath='{.data.api_key_id}' | base64 -d | \
  xargs -I {} curl -X DELETE -u elastic:<password> \
  http://<es-host>:9200/_security/api_key/{}

# 2. Create new API key
RESPONSE=$(curl -X POST -u elastic:<password> \
  http://<es-host>:9200/_security/api_key \
  -H "Content-Type: application/json" \
  -d '{
    "name": "k8swatch-healthcheck-key-'$(date +%s)'",
    "role_descriptors": {
      "k8swatch_healthcheck": {
        "cluster": ["cluster:monitor/health", "cluster:monitor/main"]
      }
    },
    "expiration": "365d"
  }')

API_KEY_ID=$(echo ${RESPONSE} | jq -r '.id')
API_KEY=$(echo ${RESPONSE} | jq -r '.api_key')

# 3. Update Kubernetes Secret
kubectl create secret generic elasticsearch-api-key \
  --from-literal=api_key_id="${API_KEY_ID}" \
  --from-literal=api_key="${API_KEY}" \
  -n k8swatch \
  --dry-run=client -o yaml | kubectl apply -f -

# 4. Verify
sleep 30
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep "elasticsearch" | tail -5
```

### Notification Credentials

```bash
# 1. Update secret (example: Slack webhook)
kubectl create secret generic k8swatch-notification-config \
  --from-literal=slack-webhook-url="https://hooks.slack.com/services/XXX/YYY/ZZZ" \
  --from-literal=pagerduty-routing-key="<new-routing-key>" \
  -n k8swatch \
  --dry-run=client -o yaml | kubectl apply -f -

# 2. Restart AlertManager to pick up changes
kubectl rollout restart deployment/k8swatch-alertmanager -n k8swatch

# 3. Verify
kubectl logs -n k8swatch -l app.kubernetes.io/component=alertmanager | tail -20
```

---

## Certificate Management

### Check Certificate Expiry

```bash
# Check all K8sWatch certificates
kubectl get secret -n k8swatch -l app.kubernetes.io/component=tls \
  -o jsonpath='{range .items[*]}{.metadata.name}: {.data.tls\.crt}{"\n"}{end}' | \
  while read -r line; do
    name=$(echo "$line" | cut -d: -f1)
    cert=$(echo "$line" | cut -d: -f2 | tr -d ' ')
    if [ -n "$cert" ]; then
      echo "=== ${name} ==="
      echo "$cert" | base64 -d | openssl x509 -noout -subject -dates
    fi
  done
```

### Force Certificate Renewal

```bash
# 1. Delete existing certificate secret
kubectl delete secret k8swatch-aggregator-tls -n k8swatch

# 2. Trigger cert-manager to reissue
kubectl delete certificate k8swatch-aggregator-cert -n k8swatch

# 3. Wait for new certificate (should take ~30 seconds)
kubectl wait certificate k8swatch-aggregator-cert -n k8swatch \
  --for=condition=Ready --timeout=120s

# 4. Restart aggregator to load new certificate
kubectl rollout restart deployment/k8swatch-aggregator -n k8swatch

# 5. Verify
kubectl get secret k8swatch-aggregator-tls -n k8swatch \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | \
  openssl x509 -noout -dates
```

### Manual Certificate Generation (Offline)

```bash
# Use the certificate generation script
cd /path/to/k8s-monitor
./deploy/tls/generate-certs.sh ./tls-certs

# Create Kubernetes secrets
kubectl create secret tls k8swatch-aggregator-tls \
  --cert=./tls-certs/aggregator.crt \
  --key=./tls-certs/aggregator.key \
  -n k8swatch

kubectl create secret tls k8swatch-agent-tls \
  --cert=./tls-certs/agent.crt \
  --key=./tls-certs/agent.key \
  -n k8swatch

kubectl create secret generic k8swatch-ca-cert \
  --from-file=ca.crt=./tls-certs/ca.crt \
  --from-file=ca.key=./tls-certs/ca.key \
  -n k8swatch

# Restart components
kubectl rollout restart -n k8swatch \
  deployment/k8swatch-aggregator \
  daemonset/k8swatch-agent
```

### Certificate Expiry Alert

**Prometheus Rule:**
```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: k8swatch-certificate-expiry
  namespace: k8swatch
spec:
  groups:
  - name: k8swatch.rules
    rules:
    - alert: K8sWatchCertificateExpiringSoon
      expr: |
        (certmanager_certificate_expiration_timestamp_seconds{namespace="k8swatch"} - time()) < (7 * 24 * 60 * 60)
      for: 1h
      labels:
        severity: warning
      annotations:
        summary: "K8sWatch certificate expiring in less than 7 days"
        description: "Certificate {{ $labels.name }} expires at {{ $value | humanizeTimestamp }}"
    
    - alert: K8sWatchCertificateExpired
      expr: |
        (certmanager_certificate_expiration_timestamp_seconds{namespace="k8swatch"} - time()) < 0
      for: 5m
      labels:
        severity: critical
      annotations:
        summary: "K8sWatch certificate has expired"
        description: "Certificate {{ $labels.name }} has expired"
```

---

## RBAC Audit

### Quarterly RBAC Review

```bash
# 1. Export current RBAC configuration
kubectl get clusterroles k8swatch-agent k8swatch-aggregator k8swatch-alertmanager \
  -o yaml > rbac-audit-$(date +%Y%m%d).yaml

kubectl get clusterrolebindings k8swatch-agent k8swatch-aggregator k8swatch-alertmanager \
  -o yaml >> rbac-audit-$(date +%Y%m%d).yaml

# 2. Review permissions
echo "=== Agent Permissions ==="
kubectl auth can-i --list --as=system:serviceaccount:k8swatch:k8swatch-agent

echo "=== Aggregator Permissions ==="
kubectl auth can-i --list --as=system:serviceaccount:k8swatch:k8swatch-aggregator

echo "=== AlertManager Permissions ==="
kubectl auth can-i --list --as=system:serviceaccount:k8swatch:k8swatch-alertmanager

# 3. Check for privilege escalation
echo "=== Checking for privilege escalation risks ==="
kubectl get clusterroles -o jsonpath='{range .items[?(@.rules[*].verbs[*]=="*")]}{.metadata.name}{"\n"}{end}'

# 4. Document findings
# Create audit report with:
# - Current permissions
# - Changes since last audit
# - Recommendations for reduction
```

### RBAC Change Procedure

```bash
# 1. Create PR with RBAC changes
# 2. Security team review required
# 3. Test in non-production cluster
# 4. Apply to production:
kubectl apply -f deploy/rbac/

# 5. Verify no service disruption
kubectl rollout restart -n k8swatch \
  daemonset/k8swatch-agent \
  deployment/k8swatch-aggregator \
  deployment/k8swatch-alertmanager

# 6. Monitor logs for permission errors
kubectl logs -n k8swatch -l app.kubernetes.io/name=k8swatch \
  | grep -i "forbidden\|unauthorized" | tail -20
```

---

## Security Incident Response

### Suspected Credential Compromise

**Severity:** Critical  
**Response Time:** < 1 hour

```bash
# 1. Identify compromised credential
# Check logs for unusual patterns
kubectl logs -n k8swatch -l app.kubernetes.io/component=agent \
  | grep -E "auth_failed|unauthorized" | tail -50

# 2. Rotate credential immediately (see Credential Rotation above)

# 3. Audit access logs
# For PostgreSQL:
kubectl run -it --rm postgres-audit --image=postgres:15 --restart=Never -- \
  psql -h <postgres-host> -U postgres -d postgres -c \
  "SELECT * FROM pg_stat_activity WHERE usename = 'k8swatch_reader';"

# 4. Check for unauthorized data access
# Review database audit logs if enabled

# 5. Document incident
# - Time of detection
# - Affected systems
# - Actions taken
# - Root cause (if known)

# 6. Post-incident review
# - Update runbook if needed
# - Consider additional monitoring
```

### mTLS Failure

**Severity:** High  
**Response Time:** < 30 minutes

```bash
# 1. Identify affected components
kubectl get pods -n k8swatch -o wide

# 2. Check certificate status
kubectl get certificate -n k8swatch

# 3. Review logs for TLS errors
kubectl logs -n k8swatch -l app.kubernetes.io/component=aggregator \
  | grep -i "tls\|certificate\|handshake" | tail -50

# 4. Test mTLS connectivity
kubectl run -it --rm tls-test --image=nicolaka/netshoot --restart=Never -- \
  openssl s_client -connect k8swatch-aggregator.k8swatch:50051 \
  -cert /etc/k8swatch/tls/tls.crt -key /etc/k8swatch/tls/tls.key \
  -CAfile /etc/k8swatch/tls/ca.crt

# 5. If certificate expired, force renewal (see Certificate Management)

# 6. If CA compromised, reissue all certificates
./deploy/tls/generate-certs.sh ./tls-certs-new
# Follow manual certificate generation procedure
```

### Unauthorized Access Detected

**Severity:** Critical  
**Response Time:** < 15 minutes

```bash
# 1. Isolate affected component
kubectl cordon <node-name>  # If node compromised
kubectl scale daemonset k8swatch-agent -n k8swatch --replicas=0  # Stop agents

# 2. Preserve evidence
kubectl logs -n k8swatch -l app.kubernetes.io/name=k8swatch > incident-logs-$(date +%Y%m%d-%H%M%S).txt

# 3. Rotate all credentials
# - Database passwords
# - API keys
# - TLS certificates
# - Notification webhooks

# 4. Review RBAC for unauthorized changes
kubectl get clusterroles k8swatch-agent k8swatch-aggregator k8swatch-alertmanager \
  -o yaml | diff - baseline-rbac.yaml

# 5. Check for unauthorized resources
kubectl get secrets -n k8swatch
kubectl get configmaps -n k8swatch
kubectl get pods -n k8swatch

# 6. Engage security team
# - Notify CISO
# - Start incident response procedure
# - Consider external forensics if needed
```

---

## Emergency Procedures

### Complete System Compromise

**Severity:** Critical  
**Response Time:** Immediate

```bash
# 1. Shut down all K8sWatch components
kubectl scale daemonset k8swatch-agent -n k8swatch --replicas=0
kubectl scale deployment k8swatch-aggregator -n k8swatch --replicas=0
kubectl scale deployment k8swatch-alertmanager -n k8swatch --replicas=0

# 2. Delete all secrets (credentials will need to be recreated)
kubectl delete secrets -n k8swatch -l app.kubernetes.io/name=k8swatch

# 3. Delete all certificates
kubectl delete certificates -n k8swatch -l app.kubernetes.io/name=k8swatch

# 4. Revoke all database users
# PostgreSQL
psql -h <host> -U postgres -c "DROP ROLE IF EXISTS k8swatch_reader;"
# MySQL
mysql -h <host> -u root -p -e "DROP USER IF EXISTS 'k8swatch_reader'@'%';"
# MongoDB
mongosh -h <host> --eval 'db.getSiblingDB("admin").dropUser("k8swatch_reader")'

# 5. Revoke all API keys
# Elasticsearch
curl -X DELETE -u elastic:<password> http://<host>:9200/_security/api_key/*

# 6. Rebuild from scratch
# Follow installation guide with new credentials
```

### Rapid Rollback Procedure

```bash
# 1. Rollback to previous deployment
kubectl rollout undo deployment/k8swatch-aggregator -n k8swatch
kubectl rollout undo deployment/k8swatch-alertmanager -n k8swatch
kubectl rollout undo daemonset/k8swatch-agent -n k8swatch

# 2. Verify rollback
kubectl rollout status deployment/k8swatch-aggregator -n k8swatch
kubectl rollout status daemonset/k8swatch-agent -n k8swatch

# 3. Monitor for stability
kubectl logs -n k8swatch -l app.kubernetes.io/name=k8swatch --tail=50
```

---

## Contact Information

| Role | Contact | Escalation |
|------|---------|------------|
| On-call SRE | oncall-sre@example.com | PagerDuty: k8swatch-critical |
| Security Team | security@example.com | PagerDuty: security-critical |
| Platform Team | platform@example.com | Slack: #platform-support |
| CISO | ciso@example.com | Phone: +1-XXX-XXX-XXXX |

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-02-21 | Security Team | Initial version |
