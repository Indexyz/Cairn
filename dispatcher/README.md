# cairn-dispatcher (Go)

Go implementation of the Cairn dispatcher. It speaks the HTTP protocol in
`docs/specs/server-protocol.md` to the server; it never touches the database.

## Why Go

The dispatcher is all concurrency and process plumbing: one container per
project, streamed worker output, heartbeat leases, cancellations, container
cleanup. Goroutines, `context` and `os/exec` fit that better than threads, and
the result is one static binary.

## Layout

| Path | Mirrors |
| --- | --- |
| `internal/config` | dispatch.yaml schema and `MOCK_*` rules |
| `internal/contracts`, `internal/outputparser` | stage payload contracts and JSON extraction |
| `internal/protocol` | server HTTP client |
| `internal/workers`, `internal/workers/adapters` | driver registry and the four CLIs |
| `internal/runtime` | backends, cancellation, heartbeat, healthchecks |
| `internal/tasks` | bootstrap, explore, reason |
| `internal/scheduler` | dispatch loop and worker selection |
| `internal/prompting/prompts` | embedded prompt groups (`default`, `mock`) |
| `e2e` | live-server parity runs (local and container) |

## Build, run, test

```bash
go build -o cairn-dispatcher ./cmd/cairn-dispatcher
./cairn-dispatcher --config dispatch.yaml
./cairn-dispatcher --config dispatch.yaml --startup-healthcheck-only
./cairn-dispatcher --config dispatch.yaml --once

go test -race ./...        # unit tests
go test ./e2e -timeout 300s # end-to-end parity against the real server
```

The mock worker still shells out to `python3` because it must also run inside
the worker container image; every other adapter invokes its own CLI.

## Parity notes

- Lease identities stay `model@instance` (`CAIRN_INSTANCE_ID`), container names
  stay instance scoped.
- `active` and `blocked` projects are runnable; blocked projects declare no new
  intents but finish outstanding ones and may complete.
- Server-side `403`/`409` are handled as the old implementation did (release,
  retry later, or defer completion) rather than treated as fatal.
- Log lines, reason checkpoints, unhealthy/rejected backoff and startup
  healthcheck output keep the original shapes so operator workflows do not change.
