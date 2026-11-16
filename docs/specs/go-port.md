# Dispatcher port to Go

The dispatcher now lives in `dispatcher/` (Go). The server stays Python and the
two only share the documented HTTP protocol, so either side can be upgraded or
rolled back alone. `cairn serve` is the only Python command left.

## Delivered

- full dispatcher behaviour: config and `MOCK_*` validation, HTTP client, the
  four worker drivers, container and local backends, cancellation, heartbeat
  leases, startup healthchecks, the three task runners, worker selection and the
  scheduling loop (caps, reason checkpoints, unhealthy/rejected backoff,
  container cleanup, blocked-project semantics)
- `cairn-dispatcher` CLI: `--config`, `--once`, `--startup-healthcheck-only`,
  `--log-level`
- shipping: Nix package (`packages.<system>.dispatcher` and installed into
  `cairn`), NixOS module dispatcher unit, container image build stage,
  `docker compose`, CI job with `gofmt`, `go vet`, `go test -race`
- parity suites ported from Python: config (container and local modes), payload
  contracts, health probes, protocol client, prompting, runtime (subprocess
  group kill, timeout, cancel, containers, archive writes, heartbeats), task
  runners (including fake-CLI local runs), scheduler decisions, startup
  healthchecks, local CLI startup check
- end-to-end: mock workload against the real server in **local and container
  modes**, asserting identical facts and intent targets, plus the bootstrap
  completion scenario

## Removed

- `cairn/src/cairn/dispatcher` (all modules and the bundled prompt tree)
- the Python `dispatch` command
- dispatcher-only Python dependencies (`docker`) and lockfile entries
- dispatcher-only Python test suites and their `conftest.py`
- the prompt drift guard (the Go tree is now the only copy)

## Kept

- the Python server: FastAPI, SQLite, notifications, model CRUD
- model health probes and mock-behaviour validation, moved into
  `cairn/server/health.py` and `cairn/server/mock_behavior.py` so the Models
  page keeps its test button without the dispatcher package
- `dispatch.example.yaml` / `dispatch.local.example.yaml`

## Verification

- `go vet ./...`, `go test -race ./...`, `go test ./e2e` (live server, both
  execution modes)
- `pytest` for the remaining server suites
- `nix flake check --no-build` and `nix build .#cairn`

## Porting rules that keep holding

- behaviour over idiom where they conflict: log text, retry timing and status
  handling match the old dispatcher so runbooks stay valid
- no database access from Go; everything goes through the protocol
- Python `403`/`409` responses are normal control flow, not errors
- each ported package carries the original test intent, not a looser rewrite
