# K8sWatch Security Architecture

**Version:** 1.0  
**Last Updated:** 2026-02-21  
**Owner:** Security Team

---

## Overview

This document describes the security architecture of K8sWatch, a Kubernetes-native monitoring and alerting system. K8sWatch is designed with security-first principles to protect credentials, secure communications, and enforce least-privilege access.

---

## Security Principles

1. **Least Privilege** - All components run with minimal required permissions
2. **Defense in Depth** - Multiple security layers (RBAC, NetworkPolicy, mTLS, Secrets)
3. **Zero Trust** - No implicit trust; all communications authenticated and encrypted
4. **Secrets Management** - Credentials stored in Kubernetes Secrets, never in code
5. **Audit Logging** - All security-relevant events logged for compliance

---

## Threat Model

### Assets to Protect

| Asset | Sensitivity | Protection Method |
|-------|-------------|-------------------|
| Database credentials | High | Kubernetes Secrets, RBAC |
| API keys | High | Kubernetes Secrets, encryption |
| Check results | Medium | mTLS, internal network |
| Configuration | Medium | ConfigMaps, RBAC |
| TLS certificates | High | cert-manager, rotation |

### Threat Actors

| Actor | Capability | Mitigation |
|-------|------------|------------|
| External attacker | Network access | NetworkPolicy, mTLS, firewall |
| Compromised pod | Lateral movement | RBAC, NetworkPolicy, pod security |
| Malicious insider | Cluster access | Audit logging, RBAC, separation of duties |
| Credential theft | Secret access | Secret rotation, scoped RBAC |

---

## Security Controls

### 1. Access Control (RBAC)

#### Agent RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8swatch-agent
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
  resourceNames: ["k8swatch-config"]
- apiGroups: ["k8swatch.io"]
  resources: ["targets"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

**Principle:** Read-only access to configuration, write access only to events.

#### Aggregator RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8swatch-aggregator
rules:
- apiGroups: ["k8swatch.io"]
  resources: ["targets", "alertrules"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["k8swatch.io"]
  resources: ["alertevents"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch"]
  resourceNames: ["k8swatch-config", "k8swatch-tls-ca", "k8swatch-aggregator-tls"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

**Principle:** Scoped secret access for TLS certificates only.

#### AlertManager RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8swatch-alertmanager
rules:
- apiGroups: ["k8swatch.io"]
  resources: ["alertrules", "alertevents"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["k8swatch.io"]
  resources: ["targets"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
  resourceNames: ["k8swatch-notification-config"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

**Principle:** Scoped secret access for notification credentials only.

---

### 2. Encryption in Transit (TLS/mTLS)

#### mTLS Architecture

```
┌─────────────┐                    ┌─────────────┐
│   Agent     │ ◄──── mTLS ────►   │ Aggregator  │
│ (client)    │    (gRPC 50051)    │  (server)   │
└─────────────┘                    └─────────────┘
                                          │
                                          │ mTLS
                                          ▼
                                   ┌─────────────┐
                                   │AlertManager │
                                   │  (server)   │
                                   └─────────────┘
```

#### Certificate Hierarchy

```
k8swatch-ca-cert (Self-signed CA)
├── k8swatch-aggregator-tls (Server certificate)
│   └── CN: k8swatch-aggregator.k8swatch.svc.cluster.local
├── k8swatch-agent-tls (Client certificate)
│   └── CN: k8swatch-agent
└── k8swatch-alertmanager-tls (Server certificate)
    └── CN: k8swatch-alertmanager.k8swatch.svc.cluster.local
```

#### Certificate Properties

| Certificate | Type | Algorithm | Validity | Usage |
|-------------|------|-----------|----------|-------|
| CA | Self-signed | ECDSA P-384 | 5 years | Certificate signing |
| Aggregator | Server | ECDSA P-256 | 1 year | Server auth, Client auth |
| Agent | Client | ECDSA P-256 | 1 year | Client auth |
| AlertManager | Server | ECDSA P-256 | 1 year | Server auth, Client auth |

#### TLS Configuration (gRPC)

**Agent (Client):**
```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{agentCert},
    RootCAs:      caCertPool,
    ServerName:   "k8swatch-aggregator.k8swatch.svc.cluster.local",
    MinVersion:   tls.VersionTLS13,
}
```

**Aggregator (Server):**
```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{aggregatorCert},
    ClientAuth:   tls.RequireAndVerifyClientCert,
    ClientCAs:    caCertPool,
    MinVersion:   tls.VersionTLS13,
}
```

---

### 3. Encryption at Rest (Secrets)

#### Kubernetes Secrets Encryption

**Recommendation:** Enable encryption at rest for Kubernetes Secrets:

```yaml
# /etc/kubernetes/encryption-config.yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - aescbc:
          keys:
            - name: key1
              secret: <base64-encoded-32-byte-key>
      - identity: {}
```

**Cloud Providers:**
- **EKS:** Enable secrets encryption with KMS
- **GKE:** Enable Cloud KMS envelope encryption
- **AKS:** Enable Azure Key Vault encryption

#### Secret Types Used

| Secret Name | Type | Purpose |
|-------------|------|---------|
| `k8swatch-ca-cert` | Opaque | CA certificate and key |
| `k8swatch-aggregator-tls` | kubernetes.io/tls | Aggregator TLS certificate |
| `k8swatch-agent-tls` | kubernetes.io/tls | Agent TLS certificate |
| `k8swatch-alertmanager-tls` | kubernetes.io/tls | AlertManager TLS certificate |
| `postgres-health-check` | Opaque | PostgreSQL credentials |
| `mysql-health-check` | Opaque | MySQL credentials |
| `mongodb-health-check` | Opaque | MongoDB credentials |
| `redis-auth` | Opaque | Redis password |
| `elasticsearch-api-key` | Opaque | Elasticsearch API key |
| `k8swatch-notification-config` | Opaque | Notification credentials |

---

### 4. Network Security

#### Network Policies

**Agent NetworkPolicy:**
- Egress to aggregator (port 50051)
- Egress to DNS (port 53)
- Egress to external targets (HTTPS)
- Egress to internal targets (database ports)

**Aggregator NetworkPolicy:**
- Ingress from agents (port 50051)
- Ingress from AlertManager (health checks)
- Egress to AlertManager (port 8080)
- Egress to DNS and Kubernetes API

**AlertManager NetworkPolicy:**
- Ingress from aggregator (port 8080)
- Egress to notification channels (Slack, PagerDuty, SMTP)

#### hostNetwork Limitation

**Important:** Agent pods run with `hostNetwork: true`, which means:
- NetworkPolicies do **not** apply to agent pods
- Host network bypasses CNI network policies
- Security relies on:
  - Node-level firewalls (security groups, iptables)
  - mTLS for authentication and encryption
  - RBAC for authorization

**Recommendation:** Use CNI-specific network policies (Calico, Cilium) for hostNetwork pods.

---

### 5. Credential Management

#### Secret Rotation

**Design:** Agents re-fetch secrets on each check interval (stateless design).

**Rotation Procedure:**
1. Update secret in Kubernetes
2. Update credential in external system (database, API)
3. Agent picks up new credentials automatically (no restart needed)

#### Database User Permissions

**PostgreSQL:**
```sql
CREATE ROLE k8swatch_reader WITH 
  LOGIN PASSWORD '<password>'
  CONNECTION LIMIT 5;
GRANT CONNECT ON DATABASE postgres TO k8swatch_reader;
-- No table grants needed for SELECT 1
```

**MySQL:**
```sql
CREATE USER 'k8swatch_reader'@'%' 
  IDENTIFIED BY '<password>'
  WITH MAX_USER_CONNECTIONS 5;
-- No grants needed for SELECT 1
```

**MongoDB:**
```javascript
db.createUser({
  user: "k8swatch_reader",
  pwd: "<password>",
  roles: [{ role: "read", db: "admin" }]
});
```

**Principle:** Health-check users have minimal permissions - no table access, no write operations.

---

### 6. Pod Security

#### Pod Security Standards

K8sWatch agents require **baseline** or **privileged** Pod Security Standard:

| Feature | PSS Level | Reason |
|---------|-----------|--------|
| `hostNetwork: true` | Baseline | Required for node network perspective |
| `hostPath` mount (`/proc`) | Restricted | Required for L0 node sanity checks |
| `hostPID: true` (optional) | Privileged | Optional for advanced process monitoring |

**Namespace Configuration:**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: k8swatch
  labels:
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/audit: baseline
    pod-security.kubernetes.io/warn: restricted
```

#### Security Context

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
  readOnlyRootFilesystem: true
  seccompProfile:
    type: RuntimeDefault
```

---

### 7. Audit Logging

#### Kubernetes Audit Logging

Enable audit logging for secret access:

```yaml
# /etc/kubernetes/audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
  resources:
  - group: ""
    resources: ["secrets"]
  verbs: ["get", "list", "watch"]
- level: Request
  resources:
  - group: "k8swatch.io"
    resources: ["targets", "alertrules", "alertevents"]
```

#### Application Audit Logging

K8sWatch components log security-relevant events:

```json
{
  "level": "info",
  "timestamp": "2026-02-21T10:00:00Z",
  "component": "agent",
  "event": "secret_access",
  "target": "postgres-primary",
  "secret": "postgres-health-check",
  "namespace": "k8swatch"
}
```

---

## Compliance Mapping

| Control | SOC 2 | ISO 27001 | NIST | Implementation |
|---------|-------|-----------|------|----------------|
| Access Control | CC6.1 | A.9 | AC-3 | RBAC, mTLS |
| Encryption | CC6.1 | A.10 | SC-8 | TLS/mTLS, Secrets encryption |
| Audit Logging | CC7.2 | A.12 | AU-2 | Kubernetes audit, application logs |
| Credential Rotation | CC6.1 | A.9 | IA-5 | Secret rotation procedure |
| Network Security | CC6.6 | A.13 | SC-7 | NetworkPolicy, firewalls |

---

## Security Testing

### Automated Scans

```bash
# Run security linter
make security-scan

# Run vulnerability check
make govulncheck

# Run all verification
make verify
```

### Manual Testing

1. **RBAC Verification:**
   ```bash
   kubectl auth can-i get secrets --as=system:serviceaccount:k8swatch:k8swatch-agent
   ```

2. **mTLS Verification:**
   ```bash
   # Should succeed with valid certificate
   openssl s_client -connect k8swatch-aggregator.k8swatch:50051 \
     -cert agent.crt -key agent.key -CAfile ca.crt
   
   # Should fail without client certificate
   openssl s_client -connect k8swatch-aggregator.k8swatch:50051
   ```

3. **NetworkPolicy Verification:**
   ```bash
   # From agent pod, should succeed
   kubectl exec -n k8swatch <agent-pod> -- \
     curl -v k8swatch-aggregator.k8swatch.svc:50051
   
   # From unrelated pod, should fail
   kubectl exec -n default <test-pod> -- \
     curl -v k8swatch-aggregator.k8swatch.svc:50051
   ```

---

## Incident Response

### Credential Compromise

**Symptoms:**
- Unauthorized access to monitored services
- Suspicious database queries from health-check user
- Alert notifications to unknown channels

**Response:**
1. Rotate compromised credentials immediately
2. Update Kubernetes Secrets
3. Review audit logs for unauthorized access
4. Revoke and reissue TLS certificates if agent compromised
5. Investigate root cause

### Certificate Expiry

**Symptoms:**
- Agents failing to connect to aggregator
- mTLS handshake errors in logs

**Response:**
1. Check certificate expiry:
   ```bash
   kubectl get secret k8swatch-aggregator-tls -n k8swatch \
     -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -noout -dates
   ```
2. Force certificate renewal:
   ```bash
   kubectl delete secret k8swatch-aggregator-tls -n k8swatch
   kubectl rollout restart deployment/k8swatch-aggregator -n k8swatch
   ```
3. Verify renewal with cert-manager:
   ```bash
   kubectl get certificate k8swatch-aggregator-cert -n k8swatch
   ```

---

## Security Checklist

### Pre-Deployment

- [ ] Enable Kubernetes secrets encryption at rest
- [ ] Install cert-manager for certificate management
- [ ] Configure namespace with appropriate Pod Security Standard
- [ ] Review and approve RBAC permissions
- [ ] Generate strong passwords for all database users
- [ ] Configure network policies (if CNI supports)

### Post-Deployment

- [ ] Verify RBAC with `kubectl auth can-i`
- [ ] Test mTLS connectivity
- [ ] Verify network policies are enforced
- [ ] Enable audit logging for secrets
- [ ] Configure monitoring for certificate expiry
- [ ] Document runbook for credential rotation

### Ongoing

- [ ] Review security scans in CI/CD
- [ ] Rotate credentials every 90 days
- [ ] Audit RBAC permissions quarterly
- [ ] Review audit logs monthly
- [ ] Update dependencies for security patches

---

## References

- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [cert-manager Documentation](https://cert-manager.io/docs/)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [Kubernetes Audit Logging](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/)
- [gRPC TLS Configuration](https://grpc.io/docs/guides/security/)
