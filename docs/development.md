# Development Guide

**OpenConnect Client — Developer Workflow and Architecture**

---

## Table of Contents

- [Environment Setup](#environment-setup)
- [Project Structure](#project-structure)
- [Build and Run](#build-and-run)
- [Testing](#testing)
- [Architecture Overview](#architecture-overview)
- [Workflow](#workflow)
- [Debugging](#debugging)
- [Troubleshooting](#troubleshooting)

---

## Environment Setup

### One-Time Setup

```bash
# Clone repository
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client

# Configure git hooks (prevents bad commits)
git config core.hooksPath .githooks

# Verify configuration
git config core.hooksPath
# Output: .githooks
```

### Daily Development

**Activate project environment:**
```bash
./scripts/shell.sh
```

This sets `OPENCONNECT_CLIENT_ENV=1` and loads:
- Go 1.26.3+ (from Guix)
- YSH/Oils (build script language)
- GNU make, bash, curl, etc.

**Exit project environment:**
```bash
exit
```

---

## Project Structure

### Directory Layout

```
openconnect-client/
├── .github/
│   └── workflows/
│       └── ci.yml              # GitHub Actions CI pipeline
├── .githooks/
│   ├── pre-commit              # Local quality gates
│   └── pre-push                # Test suite verification
├── cmd/
│   ├── occlient/
│   │   ├── main.go             # CLI entry point
│   │   └── main_test.go
│   └── occlientd/
│       └── main.go             # Daemon entry point
├── internal/
│   ├── app/
│   │   ├── cli.go              # CLI handler (flags, validation)
│   │   └── cli_test.go
│   ├── daemon/                 # Daemon lifecycle
│   ├── ipc/                    # Unix socket protocol and transport
│   └── vpn/
│       ├── types.go            # Core types (Engine, ConnectionState)
│       ├── engine.go           # VPN engine controller
│       ├── engine_test.go
│       ├── process.go          # OpenConnect adapter
│       ├── process_test.go
│       └── fake.go             # Test fake implementation
├── docs/
│   ├── plans/
│   │   ├── phase-1-dev-environment.md
│   │   ├── phase-2-go-foundation.md
│   │   ├── phase-3-vpn-engine.md
│   │   └── phase-4-background-vpn-service.md
│   ├── architecture/
│   │   └── daemon-and-ipc.md
│   ├── security/
│   │   └── security-model.md
│   ├── testing/
│   │   └── phase-3-testing.md
│   └── manual-verification.md
├── scripts/
│   ├── shell.sh                # Guix environment loader
│   └── lib/
│       └── taskfile.ysh        # YSH task utilities
├── manifest.scm                # Guix environment definition
├── run.ysh                      # Build/test/lint entry point
├── go.mod                       # Go module declaration
├── go.sum                       # Dependency hash file
├── CONTRIBUTING.md             # Contributor guidelines
├── LICENSE                      # Apache 2.0
└── README.md                    # Project overview
```

### Key Files

| File | Purpose |
|------|---------|
| `run.ysh` | Build script (format, lint, test, build) |
| `manifest.scm` | Guix environment (pinned dependencies) |
| `cmd/occlient/main.go` | CLI entry point, command dispatch |
| `internal/vpn/engine.go` | VPN engine (state machine, process manager) |
| `internal/app/cli.go` | CLI handler (flags, validation, signals) |
| `.github/workflows/ci.yml` | GitHub Actions CI pipeline |
| `CONTRIBUTING.md` | Contributor guide (git hooks, code style, testing) |

---

## Build and Run

### Building the Binaries

```bash
# Build the CLI and daemon
./run.ysh build

# Output:
# ✓ Built: build/occlient and build/occlientd
```

### Running the Binary

```bash
# Run with help
./build/occlient -h
./build/occlient connect -h

# Run version
./build/occlient --version

# The connect command runs the real OpenConnect executable.
# Use unit tests for fake-process verification without a VPN.
```

### Live Development Workflow

```bash
# Terminal 1: Watch and rebuild on changes
while inotifywait -e modify internal/ cmd/ ; do
    ./run.ysh build
done

# Terminal 2: Edit code and test
# (auto-rebuild happens on save)
./build/occlient --version
```

---

## Testing

### Quick Test

```bash
# Run all tests (41 tests, ~1.6s)
./run.ysh unit test
```

### Detailed Testing

```bash
# Run specific package
./run.ysh unit test -- ./internal/vpn
./run.ysh unit test -- ./internal/app
./run.ysh unit test -- ./cmd/occlient

# Run specific test
./run.ysh unit test -- ./internal/vpn -run TestGoroutineCleanup

# Run with race detector (slower, finds concurrency bugs)
./run.ysh unit test --race

# Run multiple times (finds flakiness)
./run.ysh unit test --count 5

# Generate coverage report
./run.ysh unit test -- ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
# Open coverage.html in browser
```

### Test Structure

**Files:**
- `*_test.go` files contain tests for each package
- Tests use fake implementations (no real processes, network, or credentials)

**Coverage target:** 75% minimum. CI currently runs tests without enforcing a
coverage threshold.

**Test categories:**
- **Unit tests:** Individual functions and components
- **Integration tests:** Component interactions (state machine, signals)
- **Safety tests:** Argument injection, file validation, state consistency

---

## Architecture Overview

### VPN Engine State Machine

```
Disconnected (initial)
      ↑           ↓
      │      Connect()
      │           ↓
      │      Connecting
      │           ↓
      │      Connected
      │      ↙      ↖
 Disconnect()      Unexpected exit
      ↓                 ↓
Disconnecting       Failed
      ↓                 ↓
      └────→ Disconnected
```

**State definitions:**
- `StateDisconnected` (0): Initial; no connection active
- `StateConnecting` (1): Connect() called; process starting
- `StateConnected` (2): Process running; tunnel active (assumed)
- `StateDisconnecting` (3): Disconnect() called; process terminating
- `StateFailed` (4): Process exited unexpectedly while Connected/Connecting

### Process Management

**Three-tier executable resolution:**
1. `OPENCONNECT_PATH` environment variable
2. `PATH` environment variable (exec.LookPath)
3. Common system paths (/usr/sbin/openconnect, /usr/bin/openconnect, etc.)

**Argument passing:**
- Always array-based: `exec.Command("openconnect", "--server", serverURL)`
- Never shell-interpolated (prevents injection attacks)

**Signal handling:**
1. SIGTERM sent on Disconnect (graceful shutdown)
2. 5-second timeout for clean exit
3. SIGKILL sent if timeout expires (force termination)

### CLI Handler

**Flag parsing:**
- Required: `--server` (VPN server endpoint)
- Optional: `--protocol` (default: "anyconnect")
- Optional: `--ca-cert` (custom CA certificate path)
- Optional: `--script` (custom vpnc-script path)

**Validation:**
- File paths: absolute, normalized, exist, not directories
- Server URL: required (no default)
- Protocol: validated against known types

**Error codes:**
- 0: Success or Ctrl+C disconnect
- 1: Runtime error (file not found, process failed, etc.)
- 2: Usage error (missing flag, invalid flag, etc.)

---

## Workflow

### Local Development Loop

```bash
# 1. Activate project environment
./scripts/shell.sh

# 2. Make code changes
# (edit cmd/occlient/main.go, internal/vpn/engine.go, etc.)

# 3. Format and lint
./run.ysh fmt
./run.ysh lint

# 4. Run tests
./run.ysh unit test

# 5. Build and test binary
./run.ysh build
./build/occlient --version
./build/occlient connect -h

# 6. Commit changes
git add .
git commit -m "feature: Add new CLI flag"
# (pre-commit hook runs: format, lint, secrets check)

# 7. Push to GitHub
git push origin feat/my-feature
# (pre-push hook runs: full test suite, govulncheck)

# 8. Create pull request on GitHub
# (GitHub Actions CI runs: format, lint, test, vulnerability scan, build)

# 9. Merge after review
```

### Commit Guidelines

**Message format:**
```
✅ Brief description (50 chars max)

Detailed explanation of changes. Include context, rationale, and
any relevant implementation details. Keep lines under 72 characters.

Example:
  Add CLI error handling for missing --server flag
  
  Rejects connection attempts without --server and exits with
  code 2 (usage error). Improves user experience with clear
  error message and usage help.
```

---

## Debugging

### Print Debugging

```go
// Add debug output to identify issues
fmt.Fprintf(os.Stderr, "DEBUG: state=%v, err=%v\n", e.state, err)

// Run with output
./run.ysh unit test -- ./internal/vpn -run TestXxx
```

### Race Detector

```bash
# Compile with race detection
./run.ysh unit test --race

# Output shows:
# WARNING: DATA RACE
# Write at 0x... by goroutine...
# Read at 0x... by goroutine...
```

### Debugger (Delve)

```bash
# Install (if not in Guix environment)
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug test
dlv test ./internal/vpn

# In dlv prompt:
# (dlv) break TestConnectDisconnect
# (dlv) continue
# (dlv) print engine.state
# (dlv) next
```

### Profiling

```bash
# CPU profile
./run.ysh unit test -- ./internal/vpn -cpuprofile=cpu.prof
go tool pprof cpu.prof
# (pprof) top
# (pprof) list Engine.Connect

# Memory profile
./run.ysh unit test -- ./internal/vpn -memprofile=mem.prof
go tool pprof mem.prof
# (pprof) top
# (pprof) list monitorProcess
```

---

## Troubleshooting

### "Project environment is not active"

**Problem:**
```
Error: The project environment is not active. Run: ./scripts/shell.sh
```

**Solution:**
```bash
./scripts/shell.sh
# Then run commands inside the shell
```

### "code is not properly formatted"

**Problem:** Pre-commit hook rejects unformatted code

**Solution:**
```bash
./run.ysh fmt
# Then re-commit
git commit -m "..."
```

### "go vet found issues"

**Problem:** Linter detects problems

**Solution:**
```bash
go vet ./...
# Fix issues reported by go vet
# (unused imports, type mismatches, etc.)
```

### "tests failed"

**Problem:** Test failures block pre-push

**Solution:**
```bash
./run.ysh unit test
# Review failures and fix code
./run.ysh unit test -- ./internal/vpn -run TestXxx  # Debug specific test
```

### "openconnect not found"

**Problem:**
```
Error: openconnect not found; install with: apt-get install openconnect
```

**Solution:**
```bash
# Install on host
sudo apt-get install openconnect

# Or use custom path
export OPENCONNECT_PATH=/path/to/openconnect
./build/occlient connect --server vpn.example.com
```

### "git hooks not running"

**Problem:** Hooks don't execute before commit/push

**Solution:**
```bash
git config core.hooksPath .githooks
# Verify
git config core.hooksPath
# Should output: .githooks
```

### "GitHub Actions CI failed"

**Problem:** Push succeeded locally but CI failed

**Solution:**
```bash
# GitHub Actions tests multiple Go versions (1.21, 1.22, 1.23)
# and may detect issues not found locally

# Test locally with specific Go version
go version

# If mismatch, pin version in local testing
# (or manually test against different Go versions)

# Review CI logs on GitHub
# Actions > <workflow> > <run> > Jobs
```

---

## Code Style Guide

### Go Conventions

- **Formatting:** `gofmt -s -w .` (enforced by `./run.ysh fmt`)
- **Naming:** PascalCase for exported, camelCase for unexported
- **Errors:** Return explicitly; wrap with context using `%w`
- **Comments:** Exported items start with package name
- **Testing:** `TestXxx` with `t.Run()` for subtests

### No Shell Injection

**REQUIRED:** Never interpolate command arguments into shell strings.

```go
// ✅ CORRECT
exec.Command("openconnect", "--server", url)

// ❌ WRONG
exec.Command("bash", "-c", fmt.Sprintf("openconnect %s", url))
```

### File Validation

Always validate file paths before use:

```go
// ✅ CORRECT
if info, err := os.Stat(path); err != nil || info.IsDir() {
    return fmt.Errorf("invalid file: %s", path)
}
```

---

## Resources

**Documentation:**
- [CONTRIBUTING.md](../CONTRIBUTING.md) — Contributor guide
- [docs/architecture/daemon-and-ipc.md](../docs/architecture/daemon-and-ipc.md) — Daemon and IPC design
- [docs/security/security-model.md](../docs/security/security-model.md) — Security model
- [docs/testing/phase-3-testing.md](../docs/testing/phase-3-testing.md) — Test strategy
- [docs/plans/phase-3-vpn-engine.md](../docs/plans/phase-3-vpn-engine.md) — Phase 3 tasks

**Tools:**
- [Go Documentation](https://golang.org/doc/)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Delve Debugger](https://github.com/go-delve/delve)
- [GoVulnCheck](https://github.com/golang/vuln)

---

## Next Steps

After completing development:

1. Run full test suite: `./run.ysh unit test`
2. Check linting: `./run.ysh lint`
3. Verify git hooks configured: `git config core.hooksPath`
4. Commit and push: `git commit` → `git push`
5. Create PR on GitHub
6. Wait for GitHub Actions CI to pass
7. Request review
8. Merge after approval

Happy coding! 🚀
