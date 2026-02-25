# K8sWatch

K8sWatch is a Kubernetes-native monitoring and alerting system composed of three services:
- `agent`: executes layered health checks against `Target` CRs
- `aggregator`: ingests results, correlates failures, and manages alert state
- `alertmanager`: routes notifications (Slack, PagerDuty, webhook, email)

## Repository Layout
- `cmd/`: service entrypoints (`agent`, `aggregator`, `alertmanager`)
- `internal/`: core logic (checkers, aggregation, alerting, TLS, logging)
- `api/v1/`: CRD/API schemas (`Target`, `AlertRule`, `AlertEvent`)
- `deploy/`: Kubernetes manifests and kustomizations
- `examples/`: ready-to-apply `Target` and `AlertRule` samples
- `tests/`: integration and scale test suites

## Quick Start
```bash
# Build binaries
make build

# Run verification locally
make verify

# Create a local kind cluster and deploy
make kind-create
make deploy-kind
```

## Development Commands
```bash
make test            # full race-enabled tests + coverage
make lint            # golangci-lint
make check-generated # validate codegen/manifests are committed
make docker-build    # build all container images
```

## Applying Examples
```bash
kubectl apply -f config/crd/bases/
kubectl apply -f examples/target-http.yaml
kubectl apply -f examples/alertrule-p0-critical.yaml
```

## Contribution Expectations
- Run `make verify` before opening a PR.
- Include generated updates when CRD/API contracts change.
- Keep changes scoped, with tests and docs updated in the same PR.
