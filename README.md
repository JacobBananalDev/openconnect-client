# OpenConnect Client

**A Linux desktop and command-line foundation for OpenConnect.**

One VPN engine, two interfaces, and an administrator-controlled background
service—built around a reproducible, project-local developer workflow.

| Status | What it means |
|---|---|
| 🟢 **Available** | Foreground CLI, desktop foundation, and experimental daemon |
| 🟡 **Experimental** | Privileged service setup and real VPN compatibility require administrator verification |
| 🔵 **Planned** | SAML/SSO, MFA, profiles, credential management, and production packaging |

> **Early-stage software:** Starting OpenConnect reports `connecting`; it does
> not establish that authentication succeeded or that the tunnel is ready.
> The client cannot answer interactive authentication prompts and does not
> collect or persist credentials.

## Architecture

```text
Desktop GUI ──┐
              ├── Shared IPC client ── Unix socket ── Daemon ── VPN engine ── OpenConnect
CLI ──────────┘
 │
 └── Foreground mode ─────────────────────────────── VPN engine ── OpenConnect
```

The GUI and background CLI send requests to the daemon; they do not run the
privileged VPN process themselves. The daemon authorizes peers and applies an
administrator-managed connection policy. The foreground CLI runs independently
as the invoking user. Service installation is manual.

## Quick start

**Requirements:** Linux, Git, GNU Guix, and an OpenConnect executable available
on the host. A graphical display is needed for the GUI. The project shell
provides development tools, not OpenConnect or VPN network privileges.

```bash
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
./scripts/shell.sh
```

Inside the project shell:

```bash
./run.ysh build          # Build CLI, daemon, and desktop GUI
./run.ysh run            # Launch the GUI as an unprivileged user
./run.ysh unit test      # Run Go tests
./run.ysh penv code .    # Open an editor in the project environment
```

Use `./run.ysh penv codium .` or `./run.ysh penv cursor .` for other supported
editors. Exit the shell with `exit`.

The pure Guix shell uses pinned channels and a manifest, disables persistent Go
environment configuration, and rejects nested Guix environments. It preserves
selected desktop, terminal, locale, and SSH-agent settings. Detected host
editor and OpenConnect directories are appended after Guix's `PATH`; other
executables in those directories are also accessible. `OPENCONNECT_PATH` is
preserved when set.

Go selects the toolchain required by `go.mod`; downloaded toolchains and modules
live in the Git-ignored `.go/` directory. The Go build cache remains shared.
The runner prepares Go on startup; explicit help commands do not.

## Build, run, and develop

### 🟢 Build

```bash
./run.ysh build
```

The build installs lockfile-pinned frontend dependencies locally using
Guix-provided npm and produces:

```text
build/occlient       CLI
build/occlientd      daemon
build/occlient-gui   desktop GUI
```

Rebuild after source changes or branch switches. Once the environment check
passes and the build lock is acquired, a build removes the previous binaries;
after a failure, build again before running. Binaries may be stale if the
environment check or lock acquisition fails. Build and development sessions
cannot run concurrently. Remove a stale empty `build/.build-lock` or
`build/.dev-lock` only after confirming the corresponding session is not running.

Go builds clear inherited `GOROOT` and CGO flags and use the project environment's
GCC/G++ and the toolchain selected by `go.mod`.

### 🔵 Run

```bash
./run.ysh run                             # Desktop GUI
./run.ysh run --gui                       # Explicit GUI
./run.ysh run -c --server vpn.example.com # Foreground CLI connection
./run.ysh run -c --server vpn.example.com --background
./run.ysh run -s                          # Daemon status
./run.ysh run -d                          # Daemon disconnect
./run.ysh run --daemon                    # Start daemon directly; no setup or elevation
./run.ysh run --version                   # CLI version
./run.ysh run --help                      # Selectors and help
```

Build first: `run` does not build, install services, or elevate privileges.
Choose one operation as the first argument. An optional `--` immediately after
the selector ends wrapper parsing and is removed; use a second `--` to forward
one to the target. Other arguments are forwarded unchanged. Existing CLI
subcommands such as `./run.ysh run connect --help` remain available.

The GUI launcher canonicalizes `GUIX_LOCPATH` entries for WebKitGTK without
disabling its sandbox. The direct daemon selector does not configure its
required identity, trusted paths, socket directory, or policy; see the
[systemd guide](docs/systemd.md).

### 🟣 Develop

```bash
./run.ysh dev
```

Run as an unprivileged user with a graphical display. This uses a project-local
Wails CLI and locked frontend dependencies, starts Vite on `127.0.0.1:9245`,
hot reloads frontend edits, and rebuilds/restarts the GUI for Go changes. Stop
with Ctrl+C. When bound Go methods change, regenerate TypeScript bindings using
the project-local Wails CLI:

```bash
./build/tools/wails3 generate bindings -ts -i -d internal/gui/frontend/bindings ./cmd/occlient-gui
```

## Connections and security

### Foreground CLI

```bash
./build/occlient connect --server vpn.example.com
./build/occlient connect --server vpn.example.com:8443 --protocol pulse --ca-cert /path/to/ca-bundle.crt
./build/occlient connect --server vpn.example.com --script /path/to/vpnc-script
```

The foreground connection ends with Ctrl+C. The CLI supports `anyconnect`,
`nc`, `gp`, `pulse`, `f5`, `fortinet`, and `array`. OpenConnect receives
`/dev/null` as standard input, so interactive authentication is unavailable.
Arguments are passed without shell interpolation.

### 🟡 Background service and desktop

The daemon accepts root and members of its administrator-managed group. Its
policy controls allowed server, protocol, certificate, and script combinations.
When a request omits a script, the daemon selects it only if the other fields
identify one distinct approved script. The GUI does not expose a script field
or a custom CA field in its current screen. Profiles, Logs, and Settings are
placeholders.

Background requests never silently fall back to foreground mode or elevate
privileges. Closing the GUI does not request disconnection. For service setup,
policy requirements, and limitations, see the [systemd guide](docs/systemd.md),
[architecture](docs/architecture/daemon-and-ipc.md), and
[security model](docs/security/security-model.md).

## Quality checks

```bash
./run.ysh unit test                 # Go tests
./run.ysh unit test --race          # Race detector
./run.ysh unit test --count 5       # Repeat tests
./run.ysh unit test -- ./internal/vpn -run TestName
./run.ysh fmt                       # Format project Go files
./run.ysh lint                      # Go vet
./run.ysh vuln                      # Go vulnerability scan
./run.ysh tidy                      # Update go.mod and go.sum
```

Frontend tests: `cd internal/gui/frontend && npm test`. For grouped tasks and
examples, run `./run.ysh list-tasks` or `./run.ysh TASK -h` / `--help`.
Explicit help works outside the project shell when YSH is available. Set
`NO_COLOR=1` to disable terminal colors; redirected output is plain.

## Troubleshooting

| Problem | Next step |
|---|---|
| Project environment not active | Exit any existing Guix environment, then run `./scripts/shell.sh`. |
| OpenConnect not found | Install it on the host using your distribution's instructions, or set `OPENCONNECT_PATH` to its executable before entering the project shell. |
| Daemon unavailable or request denied | Check the [service setup and policy](docs/systemd.md); the GUI does not start or configure the daemon. |
| Git hooks not running | Run `git config core.hooksPath .githooks` once per clone. |
| Tests fail locally | Repeat with `./run.ysh unit test --count 10` or use `--race`; inspect the reported failure. |

## Documentation

- [Contributing](CONTRIBUTING.md) · [Development guide](docs/development.md)
- [Daemon and IPC architecture](docs/architecture/daemon-and-ipc.md) · [Security model](docs/security/security-model.md)
- [Experimental systemd service](docs/systemd.md) · [Manual verification](docs/manual-verification.md)
- [Phase 5 plan](docs/plans/phase-5-desktop-ui-foundation.md)

## License

Licensed under [Apache-2.0](LICENSE).
