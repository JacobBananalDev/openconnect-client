# OpenConnect Client

OpenConnect Client is a Linux VPN desktop application project. It provides a safe,
testable interface to the OpenConnect VPN engine. Future phases will add enterprise
authentication flows (SAML/SSO, MFA), background daemon capabilities, and desktop UI.

## Current Phase

**Phase 3 (VPN Engine):** Fully functional OpenConnect process integration with safe
argument passing, state management, and CLI controls. Single-connection foreground mode.

**Previous phases:**
- Phase 2: Go CLI foundation with testing infrastructure
- Phase 1: Reproducible development environment (GNU Guix)

## Features

### Phase 3 (Current)

✅ **VPN Engine**
- Safe OpenConnect process management (no shell injection)
- State machine (Disconnected → Connecting → Connected → Disconnecting)
- Graceful shutdown on Ctrl+C
- Unexpected disconnection detection
- 41 unit tests (81.8% coverage)

✅ **CLI Interface**
- `occlient connect --server vpn.example.com --protocol anyconnect`
- Flag support: `--server`, `--protocol`, `--ca-cert`, `--script`
- Proper error codes (0=success, 1=runtime error, 2=usage error)
- Help output: `occlient -h`, `occlient connect -h`

✅ **Quality Assurance**
- Comprehensive test suite (no real network/credentials needed)
- Argument injection safety verified
- Security boundary documentation
- Git hooks for local quality gates
- GitHub Actions CI/CD (automated testing on every PR)

### Planned (Phase 4+)

🔄 **Daemon Architecture:** Background VPN manager with privilege separation
🔄 **Credentials/Auth:** Secure credential storage and browser-based SSO
🔄 **Desktop UI:** Graphical interface (Wails, React/TypeScript)
🔄 **Persistence:** Connection profiles, state recovery

### Out of Scope (Phase 3)

❌ GUI, desktop UI, or tray icon
❌ Background daemon or persistent connections
❌ Credentials or password handling
❌ SAML/SSO/MFA flows
❌ Privilege escalation (setuid, sudo, polkit)
❌ Automatic routing or DNS management

## Quick Start

**Prerequisites:** Linux system with Git. Guix and OpenConnect are managed by the project environment.

```bash
# Clone and enter project shell
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
./scripts/shell.sh
```

**Inside the project shell:**

```bash
# Build the binary
./run.ysh build

# Test the binary
./build/occlient --version
./build/occlient -h
./build/occlient connect -h

# Run tests
./run.ysh unit-test

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

**Interactive prompts:** OpenConnect will prompt for credentials/MFA directly (not handled by occlient in Phase 3)

**Disconnect:** Press Ctrl+C to gracefully disconnect

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
./run.ysh build          # Compile binary to build/occlient
./run.ysh run --version  # Run binary with arguments
```

**Testing:**
```bash
./run.ysh unit-test      # Run all tests with coverage
go test ./...            # Raw go test
go test ./... -race      # With race detector
go test ./... -count=5   # Run multiple times
```

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

### Documentation and Links

**Getting Started:**
- **[CONTRIBUTING.md](CONTRIBUTING.md)** — Contributor guide, git hooks, code style, testing
- **[docs/development.md](docs/development.md)** — Developer workflow and architecture

**Architecture & Design:**
- **[docs/plans/phase-3-vpn-engine.md](docs/plans/phase-3-vpn-engine.md)** — Phase 3 tasks and progress
- **[docs/security/phase-3-boundaries.md](docs/security/phase-3-boundaries.md)** — Security model and threat analysis

**Testing & Quality:**
- **[docs/testing/phase-3-testing.md](docs/testing/phase-3-testing.md)** — Test suite (41 tests, 81.8% coverage)
- **[docs/manual-verification.md](docs/manual-verification.md)** — Manual testing procedures

**Previous Phases:**
- **[docs/plans/phase-1-dev-environment.md](docs/plans/phase-1-dev-environment.md)** — Guix environment setup
- **[docs/plans/phase-2-go-foundation.md](docs/plans/phase-2-go-foundation.md)** — Go CLI foundation

---

## System Requirements

**Supported:**
- Linux (Ubuntu 20.04+, Debian 11+, Fedora 35+, or similar)
- x86_64 architecture

**Not Supported (Phase 3):**
- Windows (not tested; WSL2 possible but unvalidated)
- macOS (not tested; needs Linux VM/remote host)

**Prerequisites:**
- Git
- Guix (provided by `./scripts/shell.sh`)
- OpenConnect binary (installed on host or via `apt-get install openconnect`)

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
go test ./... -count=10

# Run with race detector
go test ./... -race

# Run with verbose output
go test ./... -v -race
```

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE).
