# Scaling Test Results

**Test Date:** [DATE]  
**Test Environment:** [kind/production/staging]  
**K8sWatch Version:** [VERSION]

---

## Test Configuration

| Parameter | Value |
|-----------|-------|
| Node Count | [100/1000] |
| Target Count | [500/2000] |
| Check Interval | 30s |
| Test Duration | [10/30] minutes |
| Kubernetes Version | [VERSION] |
| CNI | [calico/flannel/cilium] |

---

## Test 1: 100 Nodes, 500 Targets

### Results Summary

| Metric | Result | SLO Target | Status |
|--------|--------|------------|--------|
| Total Checks | [COUNT] | N/A | ✅ |
| Success Rate | [XX.XX]% | > 95% | [✅/❌] |
| Avg Latency | [XXX]ms | < 1000ms | [✅/❌] |
| P95 Latency | [XXX]ms | < 3000ms | [✅/❌] |
| P99 Latency | [XXX]ms | < 5000ms | [✅/❌] |

### Resource Usage

| Component | CPU (avg) | Memory (avg) | CPU (peak) | Memory (peak) |
|-----------|-----------|--------------|------------|---------------|
| Aggregator (per pod) | [XXX]m | [XXX]Mi | [XXX]m | [XXX]Mi |
| Redis | [XXX]m | [XXX]Mi | [XXX]m | [XXX]Mi |
| Agent (per pod) | [XX]m | [XX]Mi | [XX]m | [XX]Mi |

### Observations

1. [Observation 1]
2. [Observation 2]
3. [Observation 3]

### Bottlenecks Identified

- [ ] Aggregator CPU
- [ ] Aggregator Memory
- [ ] Redis throughput
- [ ] Network bandwidth
- [ ] K8s API rate limiting
- [ ] Other: [SPECIFY]

### Recommendations

1. [Recommendation 1]
2. [Recommendation 2]
3. [Recommendation 3]

---

## Test 2: 1000 Nodes, 2000 Targets

### Results Summary

| Metric | Result | SLO Target | Status |
|--------|--------|------------|--------|
| Total Checks | [COUNT] | N/A | ✅ |
| Success Rate | [XX.XX]% | > 95% | [✅/❌] |
| Avg Latency | [XXX]ms | < 1000ms | [✅/❌] |
| P95 Latency | [XXX]ms | < 5000ms | [✅/❌] |
| P99 Latency | [XXX]ms | < 10000ms | [✅/❌] |

### Resource Usage

| Component | CPU (avg) | Memory (avg) | CPU (peak) | Memory (peak) |
|-----------|-----------|--------------|------------|---------------|
| Aggregator (per pod) | [XXX]m | [XXX]Mi | [XXX]m | [XXX]Mi |
| Redis | [XXX]m | [XXX]Mi | [XXX]m | [XXX]Mi |
| Agent (per pod) | [XX]m | [XX]Mi | [XX]m | [XX]Mi |

### Scaling Observations

| Metric | 100 Nodes | 1000 Nodes | Scaling Factor |
|--------|-----------|------------|----------------|
| Aggregator CPU | [XXX]m | [XXX]m | [X.X]x |
| Aggregator Memory | [XXX]Mi | [XXX]Mi | [X.X]x |
| Redis CPU | [XXX]m | [XXX]m | [X.X]x |
| Redis Memory | [XXX]Mi | [XXX]Mi | [X.X]x |

### Bottlenecks Identified

- [ ] Aggregator CPU saturation
- [ ] Aggregator Memory pressure
- [ ] Redis connection pool exhaustion
- [ ] Redis throughput limits
- [ ] Network bandwidth
- [ ] K8s API rate limiting
- [ ] Other: [SPECIFY]

### Recommendations for 1000+ Nodes

1. **Aggregator Scaling:**
   - Current: [X] replicas
   - Recommended: [X] replicas for 1000 nodes
   - CPU per replica: [XXX]m
   - Memory per replica: [XXX]Mi

2. **Redis Scaling:**
   - Current: Single instance
   - Recommended: [Single instance / Cluster mode]
   - Memory: [XXX]Mi

3. **Agent Configuration:**
   - Max concurrency: [10] (default)
   - Consider increasing to: [XX] for high-density nodes

---

## Scaling Recommendations

### Per 100 Nodes

| Component | Replicas | CPU Request | Memory Request | CPU Limit | Memory Limit |
|-----------|----------|-------------|----------------|-----------|--------------|
| Aggregator | 2 | 500m | 512Mi | 1000m | 1Gi |
| AlertManager | 1 | 200m | 256Mi | 500m | 512Mi |
| Redis | 1 | 250m | 256Mi | 500m | 512Mi |

### Per 1000 Nodes

| Component | Replicas | CPU Request | Memory Request | CPU Limit | Memory Limit |
|-----------|----------|-------------|----------------|-----------|--------------|
| Aggregator | 5 | 1000m | 1Gi | 2000m | 2Gi |
| AlertManager | 2 | 500m | 512Mi | 1000m | 1Gi |
| Redis | 3 (cluster) | 500m | 512Mi | 1000m | 1Gi |

### HPA Configuration

```yaml
# Aggregator HPA
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: k8swatch-aggregator
spec:
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

---

## Test Execution Log

```
[TIME] Starting scaling test with 100 nodes, 500 targets
[TIME] Targets created successfully
[TIME] Waiting for agent discovery...
[TIME] Test execution started
[TIME] Check rate: XXX checks/second
[TIME] P99 latency: XXXms
[TIME] Test completed
[TIME] Results validated
```

---

## Sign-off

**Test Executed By:** [NAME]  
**Date:** [DATE]  
**Review By:** [NAME]  

- [ ] Results reviewed
- [ ] Recommendations approved
- [ ] Resource quotas updated
- [ ] HPA configuration applied

---

**Next Test Scheduled:** [DATE]
