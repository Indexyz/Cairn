# cairn-dispatcher (Go)

Go implementation of the Cairn dispatcher. It speaks the HTTP protocol in
`docs/specs/server-protocol.md` to the Python server; it never touches the
database.

## Why Go

The dispatcher is all concurrency and process plumbing: one container per
project, streamed worker output, heartbeat leases, cancellations, container
cleanup. Goroutines, `context` and `os/exec` fit that better than threads, and
the result is one static binary instead of a Python runtime.

## Layout

| Path | Mirrors |
| --- | --- |
| `internal/config` | `cairn/dispatcher/config.py` |
| `internal/contracts`, `internal/outputparser` | `contracts.py`, `output_parser.py` |
| `internal/protocol` | `protocol/client.py` |
| `internal/workers`, `internal/workers/adapters` | `workers/*`, `workers/adapters/*` |
| `internal/runtime` | `runtime/*` (backends, cancellation, heartbeat, healthchecks) |
| `internal/tasks` | `tasks/{bootstrap,explore,reason,common}.py` |
| `internal/scheduler` | `scheduler/loop.py`, `scheduler/worker_select.py` |
| `internal/prompting/prompts` | bundled `cairn/src/cairn/dispatcher/prompts/*` (kept byte-identical; a test enforces it) |
| `e2e` | `tests/test_mock_end_to_end.py` (runs against the real Python server) |

## Build, run, test

```bash
go build -o cairn-dispatcher ./cmd/cairn-dispatcher
./cairn-dispatcher --config dispatch.yaml
./cairn-dispatcher --config dispatch.yaml --startup-healthcheck-only
./cairn-dispatcher --config dispatch.yaml --once

go test -race ./...        # unit tests
go test ./e2e -timeout 300s # end-to-end parity against the Python server
```

The mock worker still shells out to `python3` because it must also run inside
the Python worker image; every other adapter invokes its own CLI.

## Parity notes

- Lease identities stay `model@instance` (`CAIRN_INSTANCE_ID`), container names
  stay instance scoped.
- `active` and `blocked` projects are runnable; blocked projects declare no new
  intents but finish outstanding ones and may complete.
- Server-side `403`/`409` are handled like the Python dispatcher (release,
  retry later, or defer completion) rather than treated as fatal.
- Log lines, reason checkpoints, unhealthy/rejected backoff and startup
  healthcheck output keep the Python shapes so operator workflows do not change.
