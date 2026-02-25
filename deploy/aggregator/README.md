# K8sWatch Aggregator Deployment

This directory contains Kubernetes manifests for deploying the K8sWatch Aggregator.

## Components

| Component | Description |
|-----------|-------------|
| `aggregator.yaml` | Deployment, Service, RBAC for aggregator |
| `hpa.yaml` | HorizontalPodAutoscaler for auto-scaling |
| `redis.yaml` | Redis deployment for state backup |
| `pdb.yaml` | PodDisruptionBudget for high availability |
| `kustomization.yaml` | Kustomize configuration |

## Quick Start

### Deploy with kubectl

```bash
# Apply all manifests
kubectl apply -k deploy/aggregator/

# Or apply individual files
kubectl apply -f deploy/aggregator/aggregator.yaml
kubectl apply -f deploy/aggregator/hpa.yaml
kubectl apply -f deploy/aggregator/redis.yaml
kubectl apply -f deploy/aggregator/pdb.yaml
```

### Deploy with kustomize

```bash
# Build manifests
kustomize build deploy/aggregator/

# Apply with kustomize
kustomize build deploy/aggregator/ | kubectl apply -f -
```

## Configuration

### Aggregator Deployment

| Parameter | Default | Description |
|-----------|---------|-------------|
| replicas | 3 | Number of aggregator replicas |
| cpu request | 100m | CPU request per replica |
| memory request | 256Mi | Memory request per replica |
| cpu limit | 500m | CPU limit per replica |
| memory limit | 512Mi | Memory limit per replica |

### HorizontalPodAutoscaler

| Parameter | Default | Description |
|-----------|---------|-------------|
| minReplicas | 3 | Minimum number of replicas |
| maxReplicas | 10 | Maximum number of replicas |
| CPU target | 70% | Target CPU utilization |
| Memory target | 80% | Target memory utilization |

### Redis State Store

| Parameter | Default | Description |
|-----------|---------|-------------|
| image | redis:7-alpine | Redis image |
| cpu request | 50m | CPU request |
| memory request | 64Mi | Memory request |
| cpu limit | 200m | CPU limit |
| memory limit | 256Mi | Memory limit |

## High Availability

The aggregator is designed for high availability:

- **3 replicas** by default
- **Pod anti-affinity** across zones
- **PodDisruptionBudget** ensures min 2 replicas available
- **Rolling updates** with maxUnavailable: 0
- **Redis backup** for state persistence

## Monitoring

### Health Endpoints

- `/healthz` - Liveness probe endpoint
- `/ready` - Readiness probe endpoint

### Metrics

The aggregator exposes Prometheus metrics on the HTTP port (8080):

- `aggregator_results_received_total` - Total results received
- `aggregator_results_rejected_total` - Total results rejected
- `aggregator_targets_tracked` - Number of targets being tracked
- `aggregator_alerts_firing` - Number of currently firing alerts

## Troubleshooting

### Check aggregator status

```bash
kubectl get pods -n k8swatch -l app.kubernetes.io/component=aggregator
kubectl get deployment k8swatch-aggregator -n k8swatch
kubectl get hpa k8swatch-aggregator -n k8swatch
```

### View aggregator logs

```bash
kubectl logs -n k8swatch -l app.kubernetes.io/component=aggregator
```

### Check Redis status

```bash
kubectl get pods -n k8swatch -l app.kubernetes.io/name=k8swatch-redis
kubectl logs -n k8swatch -l app.kubernetes.io/name=k8swatch-redis
```

### Test gRPC connectivity

```bash
# Port forward aggregator
kubectl port-forward -n k8swatch svc/k8swatch-aggregator 50051:50051

# Test with grpcurl
grpcurl -plaintext localhost:50051 k8swatch.v1.ResultService/HealthCheck
```

## Security

The aggregator deployment includes security best practices:

- Runs as non-root user (UID 1000)
- Read-only root filesystem
- All capabilities dropped
- No privilege escalation
- RBAC with least-privilege permissions

## Scaling

The aggregator auto-scales based on CPU and memory usage:

- Scales up when CPU > 70% or Memory > 80%
- Scales down after 5 minutes of low usage
- Maximum 100% increase or 4 pods per 15 seconds (scale up)
- Maximum 10% decrease per minute (scale down)

## Upgrades

To upgrade the aggregator:

```bash
# Update image tag in aggregator.yaml
# Then apply
kubectl apply -k deploy/aggregator/

# Watch rollout status
kubectl rollout status deployment/k8swatch-aggregator -n k8swatch
```

The rolling update strategy ensures zero downtime:
- Creates new pod first (maxSurge: 1)
- Waits for readiness
- Terminates old pod
- Repeats until all pods updated
