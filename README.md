# OpenConnect Client

OpenConnect Client is an early-stage Linux command-line application for managing
connections through the OpenConnect executable. The current CLI supports
foreground connections. The CLI also supports an experimental background
daemon with administrator-controlled connection policy; service installation
is manual.

The client passes OpenConnect arguments without shell interpolation and does not
collect or persist credentials.

## Requirements

- Linux
- GNU Guix for the project development environment
- OpenConnect installed on the host and discoverable through `PATH`, a common
  system location, or `OPENCONNECT_PATH`

The project shell provides development tools; it does not install OpenConnect or
grant VPN network privileges.

## Quick Start

**Prerequisites:** Linux, Git, GNU Guix, and an OpenConnect executable available
on the host.

```bash
# Clone and enter project shell
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
./scripts/shell.sh
```

**Inside the project shell:**

```bash
# Build the CLI and daemon
./run.ysh build

# Test the binary
./build/occlient --version
./build/occlient -h
./build/occlient connect -h

# Run tests
./run.ysh unit test

# Format and lint
./run.ysh fmt
./run.ysh lint

# Launch your editor (VS Code or Codium)
./run.ysh penv code .
```

**Exit the project shell:**
```bash
exit
```

---

## Usage

### Connect to a VPN

```bash
# Basic connection (uses default "anyconnect" protocol)
./build/occlient connect --server vpn.example.com

# With custom protocol and CA cert
./build/occlient connect \
  --server vpn.example.com:8443 \
  --protocol pulse \
  --ca-cert /path/to/ca-bundle.crt

# With custom vpnc-script
./build/occlient connect \
  --server vpn.example.com \
  --script /path/to/vpnc-script
```

**Supported protocols:** anyconnect, nc, gp, pulse, f5, fortinet, array

The CLI does not collect credentials or provide interactive authentication. Its
OpenConnect process receives `/dev/null` as standard input, so it cannot answer
interactive prompts.

**Disconnect:** Press Ctrl+C to stop the foreground connection.

### Daemon commands

The CLI can send `connect --background --server vpn.example.com`, `status`,
and `disconnect` to the daemon. A background request does not fall back to
foreground mode or elevate privileges. The daemon accepts root and members of
an administrator-managed group, and denies connections unless an exact
configuration is approved. A `connecting` state does not confirm tunnel
readiness. See the [systemd guide](docs/systemd.md) for setup and troubleshooting
and [manual verification](docs/manual-verification.md) for test limitations.

### Environment Variables

```bash
# Override OpenConnect binary location (3-tier search: env var → PATH → common paths)
export OPENCONNECT_PATH=/custom/path/to/openconnect
./build/occlient connect --server vpn.example.com
```

---

## Development

### Setup

1. **Clone repository**
   ```bash
   git clone https://github.com/JacobBananalDev/openconnect-client.git
   cd openconnect-client
   ```

2. **Configure git hooks** (one-time)
   ```bash
   git config core.hooksPath .githooks
   ```

3. **Activate project environment**
   ```bash
   ./scripts/shell.sh
   ```

### Available Commands

**Build and Run:**
```bash
./run.ysh build          # Compile build/occlient and build/occlientd
./run.ysh run --version  # Run binary with arguments
```

**Testing:**
```bash
./run.ysh unit test                 # Run all tests with coverage
./run.ysh unit test --race          # With race detector
./run.ysh unit test --count 5       # Run multiple times
./run.ysh unit test -- ./internal/vpn -run TestName  # Select a test
```

Use `-r` for `--race` and `-c N` for `--count N`. Arguments after `--`
replace the default `./...` target and are forwarded to Go. The existing
`./run.ysh unit-test` command remains an alias.

**Code Quality:**
```bash
./run.ysh fmt            # Format code (gofmt)
./run.ysh lint           # Lint with go vet
./run.ysh vuln           # Security scan (govulncheck)
```

**Editor:**
```bash
./run.ysh penv code .    # Open VS Code
./run.ysh penv codium .  # Open Codium
./run.ysh editor         # Use VISUAL or EDITOR
```

## Documentation

- [Contributing](CONTRIBUTING.md)
- [Development guide](docs/development.md)
- [Experimental systemd service](docs/systemd.md)
- [Architecture](docs/architecture/daemon-and-ipc.md)
- [Security model](docs/security/security-model.md)
- [Manual verification](docs/manual-verification.md)

---

## Common Issues

### OpenConnect Not Found

**Problem:** `openconnect not found; install with: apt-get install openconnect`

**Solution:**
```bash
# Install OpenConnect on host
sudo apt-get install openconnect

# Or specify custom location
export OPENCONNECT_PATH=/path/to/openconnect
./build/occlient connect --server vpn.example.com
```

### Project Environment Activation

**Problem:** `The project environment is not active.`

**Solution:**
```bash
./scripts/shell.sh
# Then run commands inside the shell
```

### Git Hooks Not Running

**Problem:** Hooks don't run before commit/push

**Solution:**
```bash
# Configure hooks (one-time setup)
git config core.hooksPath .githooks

# Verify configuration
git config core.hooksPath
# Should output: .githooks
```

### Tests Fail Locally but Pass in CI

**Problem:** Race conditions or timing issues

**Solution:**
```bash
# Run tests multiple times
./run.ysh unit test --count 10

# Run with race detector
./run.ysh unit test --race

# Run one package with the race detector
./run.ysh unit test --race -- ./internal/vpn
```

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE).
