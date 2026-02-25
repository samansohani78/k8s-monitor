# K8sWatch Improvement Plan

Last updated: 2026-02-25

## Scope
This plan covers unfinished code paths, flaky/failing tests, and documentation drift found during a repository audit.

## Progress Snapshot
- [x] P0-1 AlertEvent persistence implemented in aggregator result handling.
- [x] P0-2 Connection error classification stabilized for restricted/sandboxed environments.
- [x] P0-3 Alertmanager HTTP listener tests made environment-resilient.
- [x] P2-1 `examples/README.md` reconciled with existing example files.
- [x] P1-1 Scale framework completed (Target CRUD + Prometheus metrics collection).
- [x] P1-2 Secret-backed refs implemented for HTTP/Redis auth and TLS.
- [x] P1-3 Host network mode propagation implemented in result client metadata.
- [x] P1-4 Clock skew validation now performs real NTP comparison when configured.
- [x] P2-2 SLO document TODO labels replaced with explicit planned status.
- [x] P2-3 Added top-level `README.md` quick start.
- [x] P1-5 Manual kind scale smoke validation executed (in-cluster CRD CRUD).

## Current Findings

### P0 (Correctness / Reliability)
1. Completed: Alert lifecycle persistence integrated with `AlertEvent` create/update behavior.
   - `cmd/aggregator/main.go`
2. Completed: Connection error mapping stabilized under sandbox/network-restricted runtime.
   - `internal/checker/executor.go`
3. Completed: Alertmanager channel/API tests now skip cleanly when local socket bind is not permitted.
   - `internal/alertmanager/api_test.go`
   - `internal/alertmanager/channels/channels_test.go`

### P1 (Incomplete Features)
1. Completed: Scale test framework now creates and cleans up `Target` CRs and collects Prometheus metrics.
   - `tests/scale/scale_test.go`
2. Completed: Security/auth references now resolve Kubernetes Secrets for HTTP/Redis and TLS materials.
   - `internal/checker/secrets.go`
   - `internal/checker/http.go`
   - `internal/checker/redis.go`
3. Completed: Host network mode is propagated in submitted result metadata.
   - `internal/agent/result_client.go`
4. Completed: Node sanity clock skew check uses NTP server comparison when `ntpServer` is configured.
   - `internal/checker/l0_node_sanity.go`

### P2 (Docs / Runbook / Examples)
1. Completed: `examples/README.md` references now match existing files.
2. Completed: SLO document TODO markers replaced with explicit `Planned` status.
   - `docs/slos.md`
3. Completed: Top-level contributor/operator quick start added.
   - `README.md`

## Execution Plan

## Phase 1: Stabilize Core Behavior (P0)
- [x] Implement AlertEvent create/update integration in aggregator handler.
- [x] Normalize connection error mapping (`refused`, `reset`, `operation not permitted`, `network unreachable`) and align tests.
- [x] Make HTTP server tests environment-resilient (prefer IPv4 loopback or skip on unsupported bind capability).
- Verification:
  - [x] `GOCACHE=/tmp/go-build go test ./...`

## Phase 2: Complete Missing Features (P1)
- [x] Implement scale test target CRUD via Kubernetes API.
- [x] Implement Prometheus queries for scale metrics and final report generation.
- [x] Implement secret-backed auth/TLS refs for HTTP/Redis layers.
- [x] Add `host` network mode propagation in result client metadata.
- [x] Implement NTP-based clock skew comparison when configured.
- Verification:
  - [x] focused unit tests per module
  - [x] manual kind-scale dry run with captured results (environment-adapted)

## Phase 3: Documentation & Developer UX (P2)
- [x] Reconcile `examples/README.md` with actual files.
- [x] Update `docs/slos.md` to mark implemented/planned SLIs and alert rule status.
- [x] Add a short top-level `README.md` for quick start (build, deploy, verify).
- Verification:
  - [x] link/file existence check for examples/docs
  - [x] markdown review

## Definition of Done
- No unresolved TODOs in production paths for alert handling and scale test core functions.
- Test suite passes in CI and is resilient in constrained local environments.
- Docs match the repo state; examples are runnable as written.

## Current Validation State
- `GOCACHE=/tmp/go-build go test ./...` passes.
- Repo-wide search confirms no remaining `TODO`/`FIXME` markers in `cmd/`, `internal/`, `tests/`, `README.md`, `examples/`, or `docs/slos.md`.
- Manual kind smoke validation completed:
  - Created temporary cluster: `kind create cluster --name k8swatch-smoke`
  - Applied CRDs into cluster control-plane
  - Created 20 `Target` CRs with `k8swatch.io/scale-run=manual-smoke`
  - Verified count = `20`
  - Deleted all smoke targets and verified count = `0`
  - Deleted temporary cluster after validation
- Note: host-to-kind API access in this environment was intermittent; validation was completed via in-cluster `kubectl` execution through Docker.
