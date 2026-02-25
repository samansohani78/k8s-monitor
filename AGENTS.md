# Repository Guidelines

## Project Structure & Ownership Boundaries
- `cmd/agent`, `cmd/aggregator`, `cmd/alertmanager`: process entrypoints and CLI wiring.
- `internal/agent`: target discovery, scheduling, and result delivery.
- `internal/checker`: layered health-check engine (`L0`-`L6`) and target-type implementations.
- `internal/aggregator`: result ingestion, correlation, alert evaluation, topology/storm controls.
- `internal/alertmanager`: routing, escalation, channel integrations (Slack, PagerDuty, webhook, email).
- `api/v1`: Kubernetes API/CRD types (`Target`, `AlertRule`, result models) and generated code.
- `deploy/`: Kubernetes manifests, RBAC, network policy, TLS assets, and kustomizations.
- `tests/`: integration and scale validation; `docs/runbooks` and `docs/security` for operations.

## Core Architecture (How Data Flows)
1. Agent executes layered checks for `Target` CRs on schedule.
2. Agent sends results to aggregator over gRPC (mTLS-capable path).
3. Aggregator correlates failures, tracks blast radius, and emits alert events.
4. Alertmanager applies rules/escalation and sends notifications.

Keep cross-module changes coherent: API shape changes in `api/v1` usually require updates in agent, aggregator, alertmanager, examples, and tests.

## Build, Test, and Local Development Commands
- `make help`: discover supported workflows.
- `make build`: compile all binaries into `bin/`.
- `make run-agent` / `make run-aggregator` / `make run-alertmanager`: local runs (expects `KUBECONFIG`).
- `make test`: race-enabled full test run with coverage output (`coverage.out`, `coverage.html`).
- `make test-unit`: focused unit tests for `api` + `internal`.
- `make lint`: run `golangci-lint`.
- `make security-scan`: enable `gosec` checks in lint.
- `make verify`: full local pre-PR gate.
- `make check-generated`: ensures `make generate` and `make manifests` are committed.
- `make kind-create && make deploy-kind`: spin up/test against local kind cluster.

## Coding Style & Naming Conventions
- Go version target in CI: **Go 1.23**.
- Always run `make fmt` and `make vet` before commit.
- Follow idiomatic Go naming: exported `PascalCase`, internal `camelCase`, package names lower-case.
- Keep packages cohesive; avoid circular dependencies across `internal/*` domains.
- Prefer table-driven tests for checker/alerting behaviors and explicit subtests (`t.Run("...")`).
- Never hand-edit generated files such as `api/v1/zz_generated.deepcopy.go`.

## Testing & Quality Gates
- Unit tests live beside implementation (`*_test.go` in `internal/`, `cmd/`, `api/v1/`).
- Integration coverage lives in `tests/integration`; scale scenarios in `tests/scale`.
- CI (`.github/workflows/ci.yaml`) enforces: tidy, fmt, vet, lint, test, security scan, generated-file check, and build.
- Before PR, run at minimum:
  - `make verify`
  - `make check-generated`
- If changing CRDs/proto/contracts, also run relevant regeneration (`make generate`, `make manifests`, `make generate-proto`) and include diffs.

## Security, Config, and Operational Expectations
- Preserve least-privilege RBAC and avoid broadening secret access without justification.
- Treat TLS/mTLS paths as default for service-to-service transport; update cert/config docs when behavior changes.
- Validate deployment-impacting changes with manifests in `deploy/` and examples in `examples/`.
- For alerting/aggregation changes, check runbooks in `docs/runbooks/` and keep troubleshooting steps aligned.

## Commit & Pull Request Guidelines
- Current git history is minimal, so use imperative, scoped commit messages.
- Recommended pattern: `<area>: <change>` (e.g., `checker: enforce timeout on dns layer`).
- Keep commits small and logically isolated (API, runtime behavior, manifests, docs).
- PRs should include:
  - what changed and why,
  - risk/rollback notes for deploy-affecting changes,
  - test evidence (commands run, key results),
  - linked issue/ticket.
- Include screenshots or metric snippets when dashboard/observability assets are changed.

## Contributor Workflow Checklist
1. Implement change in the owning module(s).
2. Update or add tests near the modified logic.
3. Regenerate artifacts if APIs/CRDs/proto changed.
4. Run `make verify` and `make check-generated`.
5. Update docs/examples/runbooks for behavior changes.
6. Open PR with clear scope, evidence, and rollout/rollback guidance.
