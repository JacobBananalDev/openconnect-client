# Development Guide

## Environment setup

On Linux, install Git and GNU Guix, clone the repository, and enter the pinned
project environment from a shell that is not already inside Guix:

```bash
./scripts/shell.sh
```

The pure shell provides Go, Node/npm, YSH, and native GUI build dependencies.
Go uses the version selected by `go.mod`; downloaded toolchains and modules
are stored under the ignored `.go/` directory. Host OpenConnect and supported
editors remain accessible as described in the [README](../README.md#quick-start).
Exit with `exit`.

Optionally enable the project's pre-commit hook once per clone:

```bash
git config core.hooksPath .githooks
```

The hook requires the project environment and runs formatting, lint, Go and
frontend unit tests, vulnerability scanning, and staged filename and
whitespace checks. It can modify unstaged Go files; review changes before
committing. It does not
perform secret detection. There is no pre-push hook.

## Project layout

| Path | Purpose |
|---|---|
| `cmd/occlient/` | CLI entry point |
| `cmd/occlientd/` | Daemon entry point |
| `cmd/occlient-gui/` | Desktop entry point |
| `internal/app/` | CLI handlers |
| `internal/daemon/` | Daemon lifecycle and connection policy |
| `internal/ipc/` | Shared IPC protocol and transport |
| `internal/gui/` | Desktop adapter, frontend, and embedded assets |
| `internal/vpn/` | VPN engine and process management |
| `scripts/` | Environment, build, development, and runner fixtures |
| `run.ysh` | Project task runner |

## Build and run

```bash
./run.ysh build-prod     # Production-tagged binaries in build/
./run.ysh build-dev      # Development binaries in build/dev/
./run.ysh run            # Production-build GUI
./run.ysh run-dev        # Development-build GUI
./run.ysh run --version  # CLI version
./run.ysh run --help     # All run selectors
./run.ysh run -c --server vpn.example.com --background
./run.ysh run -s         # Daemon status
./run.ysh run -d         # Daemon disconnect
```

Each build installs locked frontend dependencies locally and replaces all three
binaries for its variant. Rebuild after source changes or branch switches; after
a failed build, build that variant again before running. Neither run task
builds, installs a service, or elevates privileges. See the
[README build and run notes](../README.md#build-run-and-develop)
for argument forwarding, lock handling, and GUI launch details.

For frontend hot reload and Go rebuilds:

```bash
./run.ysh watch-gui
```

The dev task requires a graphical display and an unprivileged user. It uses
project-local Wails and Vite tools, and cannot run concurrently with a build.
Stop it with Ctrl+C. `build-dev` and `watch-gui` generate tracked bindings
automatically. Restart the watcher after changing bound Go methods. The GUI
in `build-dev` can honor `FRONTEND_DEVSERVER_URL`; the production-tagged GUI
does not.

## Testing and quality

```bash
./run.ysh unit-test
./run.ysh unit-test --race
./run.ysh unit-test --count 5
./run.ysh unit-test -- ./internal/vpn -run TestName
./run.ysh fmt
./run.ysh lint
./run.ysh vuln
```

`fmt` writes Go files while excluding the project-local `.go/` cache. The
vulnerability task may need network access. The unit-test task runs frontend
tests once and installs locked frontend dependencies locally if absent.
Explicit Go selections after `--` skip frontend tests; `--race` and `--count`
apply only to Go. The [manual verification guide](manual-verification.md) lists
runner fixtures and separates unprivileged checks from optional service and
VPN verification.

CI checks Go formatting, vet, shell syntax, Go and frontend tests, application
builds, and vulnerabilities. The YSH fixture step in the current workflow
runs four non-Guix fixtures; the Guix integration fixture is local
only. See [CONTRIBUTING](../CONTRIBUTING.md#submitting-changes) for CI details.

## Architecture and state

The desktop adapter and background CLI share the IPC client; the daemon
authorizes peers, applies policy, and owns the VPN engine. The foreground CLI
uses the engine directly. See the [daemon architecture](architecture/daemon-and-ipc.md)
and [security model](security/security-model.md).

The engine starts in `disconnected`, moves to `connecting` when it starts
OpenConnect, and moves to `failed` if the process exits unexpectedly.
Disconnect transitions through `disconnecting` to `disconnected` when the
process is reaped. The protocol includes `connected`, but the engine does not
currently confirm tunnel readiness or transition to that state.

The process runner passes arguments separately without shell interpolation.
The foreground CLI validates supplied file paths; daemon requests additionally
require an administrator-approved configuration and trusted privileged files.
The daemon can select a unique policy-approved script for a scriptless request.
Interactive authentication and credential persistence are not implemented.

## Debugging

Use focused tests and the race detector before expanding the test scope:

```bash
./run.ysh unit-test --race -- ./internal/daemon
./run.ysh unit-test -- ./internal/vpn -run TestName
./run.ysh unit-test -- ./internal/vpn -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

For GUI development and task help, see the [README](../README.md). For daemon
setup and `policy_denied` troubleshooting, see the [systemd guide](systemd.md).
Check CI logs for hosted failures rather than assuming the local environment
matches CI. The current workflow uses the Go version from `go.mod`, not a
multiple-version matrix.
