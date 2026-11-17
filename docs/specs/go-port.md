# Port to Go

Cairn is Go end to end. `server/` is the API + SQLite graph store, `dispatcher/`
runs workers. They are separate binaries (module `server` and module
`dispatcher`) that only speak the HTTP protocol in `server-protocol.md`.

## Delivered

Server:

- schema, WAL/busy-timeout/`BEGIN IMMEDIATE` write serialization and legacy
  column migrations
- every route: settings, models and `/load`, notifications, projects, fork,
  hints, intents, complete/reopen, YAML/timeline export
- validation with FastAPI-compatible 422 payloads, status codes and detail
  strings
- step-limit blocking, lease expiry, reason leases, `model@instance` load
  accounting
- notification rendering and Discord/Telegram/webhook delivery after the
  response commits
- model health probes, `MOCK_*` validation, embedded web UI, `cairn serve` CLI
- test suites for the workflow, models, notifications, step limits, health
  probes and migrations

Dispatcher: see the earlier milestones in git history — config, drivers,
backends, tasks, scheduler, whole-loop parity with a live server, local and
container e2e.

## Wiring

- `go.work` spans both modules
- Nix builds `packages.<system>.server` and the combined `cairn` package; the
  NixOS module runs `cairn serve` and `cairn-dispatcher`
- Dockerfile builds both binaries into an alpine image; compose healthchecks
  through the server route
- CI runs `gofmt`, `go vet`, `go test -race ./server/... ./dispatcher/...` and the e2e parity suite

## Removed

- the whole Python tree (`cairn/`), `pyproject.toml`, `uv.lock`, every Python
  test and the Python Nix wrapper

## Remaining Python

Only the mock worker's helper script: it shells out to `python3` so it can also
run inside the worker container image. Every real adapter invokes its own CLI.

## Verification

- `gofmt`, `go vet`, `go test -race ./server/... ./dispatcher/...` across both modules
- `dispatcher/e2e`: bootstrap completion plus the local/container parity run
  against the real server, with the container leg on rootless podman
- `nix flake check --no-build`, `nix build .#cairn`
- web lint/build
