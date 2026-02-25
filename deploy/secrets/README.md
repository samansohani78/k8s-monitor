# Secrets Configuration

This directory contains Kubernetes Secret manifests for K8sWatch.

**IMPORTANT**: Do not commit actual secrets to version control!

## Required Secrets

1. **notification-config.yaml** - Slack, PagerDuty, Email credentials
2. **redis-auth.yaml** - Redis authentication
3. **postgres-health-check.yaml** - PostgreSQL health check user
4. **mysql-health-check.yaml** - MySQL health check user
5. **mongodb-health-check.yaml** - MongoDB health check user
6. **elasticsearch-api-key.yaml** - Elasticsearch API key
7. **kafka-health-check.yaml** - Kafka SASL credentials

## Create Secrets

```bash
# Example: Create notification config secret
kubectl create secret generic notification-config \
  --from-literal=slack-webhook-url="YOUR_WEBHOOK_URL" \
  --from-literal=pagerduty-routing-key="YOUR_ROUTING_KEY" \
  -n k8swatch

# Example: Create Redis auth secret
kubectl create secret generic redis-auth \
  --from-literal=password="YOUR_PASSWORD" \
  -n k8swatch
```

## Templates

See the example templates in `examples/` directory for secret structure.
