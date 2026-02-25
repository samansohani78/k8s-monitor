# K8sWatch Examples

This directory contains example Target and AlertRule configurations for K8sWatch.

## Target Examples

### Core Infrastructure

| Example | File | Description |
|---------|------|-------------|
| Network | `target-network.yaml` | TCP connectivity check |
| DNS | `target-dns.yaml` | DNS resolution check |
| HTTP | `target-http.yaml` | HTTP endpoint check |
| HTTPS | `target-https.yaml` | HTTPS with TLS validation |
| Kubernetes | `target-kubernetes.yaml` | K8s API server health |

### Databases

| Example | File | Description |
|---------|------|-------------|
| PostgreSQL | `target-postgresql.yaml` | PostgreSQL health check |
| MySQL | `target-mysql.yaml` | MySQL health check |
| Redis | `target-redis.yaml` | Redis health check |
| MongoDB | `target-mongodb.yaml` | MongoDB health check |

### Search & Storage

| Example | File | Description |
|---------|------|-------------|
| Elasticsearch | `target-elasticsearch.yaml` | ES cluster health |
| MinIO | `target-minio.yaml` | S3-compatible storage |

### Messaging

| Example | File | Description |
|---------|------|-------------|
| Kafka | `target-kafka.yaml` | Kafka broker health |
| RabbitMQ | `target-rabbitmq.yaml` | RabbitMQ health |

### Identity & Proxy

| Example | File | Description |
|---------|------|-------------|
| Keycloak | `target-keycloak.yaml` | OIDC provider health |
| Nginx | `target-nginx.yaml` | Nginx health check |

### Synthetic Targets

| Example | File | Description |
|---------|------|-------------|
| Internal Canary | `target-internal-canary.yaml` | Internal canary service |
| External HTTP | `target-external-http.yaml` | External API check |
| Node Egress | `target-node-egress.yaml` | Node outbound connectivity |

## AlertRule Examples

| Example | File | Description |
|---------|------|-------------|
| P0 Critical | `alertrule-p0-critical.yaml` | P0 critical alerting |
| P1 Warning | `alertrule-p1-warning.yaml` | P1 warning alerting |
| Database | `alertrule-database.yaml` | Database-specific rules |

## Quick Start

```bash
# Install CRDs first
kubectl apply -f config/crd/bases/

# Apply example targets
kubectl apply -f examples/target-http.yaml
kubectl apply -f examples/target-postgresql.yaml

# Apply alert rules
kubectl apply -f examples/alertrule-p0-critical.yaml

# View targets
kubectl get targets -n k8swatch

# View alert rules
kubectl get alertrules -n k8swatch
```

## Configuration Reference

### Target Spec

```yaml
apiVersion: k8swatch.io/v1
kind: Target
metadata:
  name: my-target
  namespace: k8swatch
spec:
  type: http  # Target type
  endpoint:
    dns: example.com  # Endpoint configuration
    port: 443
  networkModes:
    - pod
    - host
  layers:
    L0_nodeSanity:
      enabled: true
    L1_dns:
      enabled: true
    L2_tcp:
      enabled: true
      timeout: 5s
    L3_tls:
      enabled: true
      validationMode: strict
    L4_protocol:
      enabled: true
      method: GET
      statusCode: 200
    L5_auth:
      enabled: false
    L6_semantic:
      enabled: false
  schedule:
    interval: 30s
    timeout: 15s
  alerting:
    criticalityOverride: P1
```

### AlertRule Spec

```yaml
apiVersion: k8swatch.io/v1
kind: AlertRule
metadata:
  name: my-alertrule
  namespace: k8swatch
spec:
  targetSelector:
    namespace: k8swatch
    labels:
      team: platform
  trigger:
    consecutiveFailures: 3
    blastRadius:
      - node
      - zone
  severity:
    base: warning
  recovery:
    consecutiveSuccesses: 2
    autoResolve: true
  stormPrevention:
    groupBy:
      - namespace
      - failureLayer
    cooldownPeriod: 5m
```
