# Dispatcher port to Go

Status of moving `cairn/src/cairn/dispatcher` to `dispatcher/` (Go). The server
stays Python for now; both sides only share the HTTP protocol, so they can be
upgraded and rolled back independently.

## Done

- config loading/validation, including `MOCK_*` behavior rules and prompt
  placeholder validation
- HTTP client, typed protocol models, `model@instance` lease identity
- output extraction, payload contracts for reason/explore/bootstrap
- worker drivers (`claudecode`, `codex`, `pi`, `mock`), health probes, proxy
  handling, session extraction
- runtime: cancellation reasons, heartbeat leases with grace, host subprocesses
  (process group + SIGTERM/SIGKILL), per-project containers via the Docker API,
  archive writes, local backend
- task runners and scheduler loop, including caps, reason checkpoints,
  unhealthy/rejected backoff, container cleanup and blocked-project semantics
- `cairn-dispatcher` CLI (`--config`, `--once`, `--startup-healthcheck-only`,
  `--log-level`)
- shipping: Nix package (`packages.<system>.dispatcher`, also installed into
  `cairn`), NixOS module dispatcher unit, container image stage,
  `docker compose`, CI job incl. `go vet`, `-race` and the end-to-end parity
  test
- ported test suites: config, contracts, health, protocol, prompting, runtime,
  tasks, scheduler helpers, mock end-to-end; prompt drift guard against the
  Python tree

## Deliberately still on the Python side

- the server (FastAPI, SQLite, Pydantic models, notifications)
- `cairn serve`
- `uv run --project cairn cairn dispatch` remains as the previous dispatcher
  until the server port lands

## Remaining before deleting the Python dispatcher

- `tests/test_local_execution.py` scenarios (fake CLI scripts) in Go
- notification/reason edge cases reachable only through a live dispatcher on a
  long-running server session
- parity run of a real dispatch workload (container and local) against a
  staging server
- delete `cairn/src/cairn/dispatcher`, the Python `dispatch` command, the
  duplicated prompt tree and the Python-only flake wiring; CI then drops the
  pylint-style dispatcher tests from the Python job

## Porting rules applied

- behaviour over idiom where they conflict: log text, retry timing and status
  handling match Python so existing operator runbooks hold
- no DB access from Go; everything goes through the protocol
- Python 403/409 responses are normal control flow, not errors
- each ported package carries the Python test intent, not a looser rewrite
