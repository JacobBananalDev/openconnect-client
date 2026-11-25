# Contributing to OpenConnect Client

**Thank you for your interest in contributing!** This document outlines guidelines and workflows for contributing to the project.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Git Hooks](#git-hooks)
- [Making Changes](#making-changes)
- [Code Style](#code-style)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Project Structure](#project-structure)

---

## Getting Started

### Prerequisites

- **Linux system** (development and testing on Linux only)
- **Git** for version control
- **Guix** (for reproducible development environment)
- **YSH/Oils** (for build scripts, provided by Guix)

### Clone the Repository

```bash
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
```

---

## Development Setup

### Activate Project Environment

The project uses a Guix-based development environment for reproducibility:

```bash
./scripts/shell.sh
```

This command:
1. Checks for existing active Guix environment (errors if nested)
2. Loads the project manifest with pinned tool versions
3. Sets `OPENCONNECT_CLIENT_ENV=1` environment variable
4. Provides Go, YSH, and other development tools

**Important:** Enter `./scripts/shell.sh` before running project tasks or
committing with the configured hook. The hook runs the tasks; it does not skip
them when the project environment is missing.

### Quick Build and Test

```bash
# Build the CLI, daemon and desktop GUI
./run.ysh build-prod
./run.ysh build-dev

# Run the binary
./build/occlient --help

# Launch the desktop GUI
./run.ysh run

# Run the GUI with development reloads
./run.ysh watch-gui

# Run all tests
./run.ysh unit-test

# Format code
./run.ysh fmt

# Run linter
./run.ysh lint

# Run security scan
./run.ysh vuln
```

Both builds use Guix-provided npm to install locked dependencies locally and
replace all three binaries for their selected variant. Development binaries
are separate from production-tagged binaries. Rebuild after source changes or
branch switches; after a failed build, run it again before using that variant.
See the
[build notes](README.md#build-run-and-develop) for compiler settings and lock cleanup.
See the [run commands](README.md#available-commands) for CLI and daemon
selectors and their privilege requirements.
Development mode uses local Wails/Vite tools and cannot run concurrently with a
build; see the [development notes](README.md#quick-start).

---

## Git Hooks

### Setup Hooks (One-time)

Configure Git to use the project's hooks:

```bash
git config core.hooksPath .githooks
```

This command only needs to run once per clone.

### Hook Checks

#### Pre-commit Hook (`.githooks/pre-commit`)

Runs **before each commit**. Checks:

- ✅ Code formatting (`gofmt`)
- ✅ Linting (`go vet`)
- ✅ Go and frontend unit tests and vulnerability scan
- ✅ Staged filename and whitespace checks

The formatting task writes project Go files, including unstaged files; review
and stage any intended formatting changes. The hook does not detect secrets.

**If pre-commit fails:**
```bash
# Fix formatting
./run.ysh fmt

# Fix linting errors
# (address go vet warnings in your code)

# Try commit again
git commit
```

**Bypass (not recommended):**
```bash
git commit --no-verify
```

There is no pre-push hook; run the applicable checks before pushing.

---

## Making Changes

### Branch Naming

- **For features:** Use `feat/description`
- **For bug fixes:** Use `fix/description`
- **For documentation:** Use `docs/description`
- **For refactoring:** Use `refactor/description`

### Commit Messages

Write clear, descriptive commit messages:

```
✅ Brief summary (50 chars max)

Detailed explanation of changes, why they were made, and any relevant
context. Keep lines under 72 characters for readability.

Example:
  Add CLI error handling for missing --server flag
  
  Rejects connection attempts without --server and exits with
  code 2 (usage error). Improves user experience with clear
  error message and usage help.
```

### Code Organization

```
openconnect-client/
├── cmd/
│   ├── occlient/
│   │   ├── main.go              # CLI entry point
│   │   └── main_test.go
│   ├── occlientd/               # Daemon entry point
│   └── occlient-gui/            # Desktop entry point
├── internal/
│   ├── app/
│   │   ├── cli.go               # CLI handler
│   │   └── cli_test.go
│   ├── daemon/                  # Daemon lifecycle
│   ├── gui/                     # Desktop adapter and frontend
│   ├── ipc/                     # Unix socket protocol and transport
│   └── vpn/
│       ├── types.go             # Core types and interfaces
│       ├── engine.go            # VPN engine controller
│       ├── engine_test.go
│       ├── process.go           # OpenConnect process adapter
│       ├── process_test.go
│       └── fake.go              # Test fake implementation
├── docs/
│   ├── architecture/
│   │   └── daemon-and-ipc.md
│   ├── plans/                   # Project plans and task checklists
│   ├── security/
│   │   └── security-model.md
│   └── testing/                 # Testing strategy
└── run.ysh                       # Build/test entry point
```

---

## Code Style

### Go Conventions

- **Formatting:** Use `gofmt` (enforced by pre-commit hook)
- **Naming:** 
  - Exported types/functions: PascalCase (e.g., `Engine`, `Connect`)
  - Unexported: camelCase (e.g., `monitorProcess`)
  - Constants: UPPERCASE (e.g., `StateConnected`)
- **Comments:**
  - Exported items: Begin with package-qualified name (e.g., `// Engine manages VPN connections.`)
  - Unexported: Optional, focus on "why" not "what"
  - No trailing comments on single-line code blocks
- **Error handling:**
  - Return errors explicitly; don't panic in libraries
  - Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- **Testing:**
  - Test files: `*_test.go`
  - Test functions: `TestXxx` where Xxx is the function/behavior tested
  - Subtests for multiple scenarios: Use `t.Run()`

### No Shell Injection

**REQUIRED:** All external command arguments must be passed as `[]string` array, never shell-interpolated.

```go
// ✅ CORRECT: Array-based argument passing
exec.Command("openconnect", "--server", serverURL, "--protocol", protocol)

// ❌ WRONG: Shell interpolation (never use)
exec.Command("bash", "-c", fmt.Sprintf("openconnect %s", serverURL))
```

### No Hardcoded Credentials

- Never commit passwords, tokens, or API keys
- Do not pass credentials or tokens through flags, environment variables, or
  logs. Interactive authentication is not supported yet.

---

## Testing

### Running Tests

```bash
# All tests with output
./run.ysh unit-test

# Specific package
./run.ysh unit-test -- ./internal/vpn

# Specific test
./run.ysh unit-test -- ./internal/vpn -run TestGoroutineCleanup

# With race detection
./run.ysh unit-test --race

# Multiple runs (detect flakiness)
./run.ysh unit-test --count 5

# Coverage report
./run.ysh unit-test -- ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

The frontend suite runs once unless Go tests are selected after `--`. Race and
count options apply only to Go. The task installs locked frontend dependencies
locally if absent. See the [manual verification guide](docs/manual-verification.md)
for task-runner fixtures and checks that require an authorized test host.

### Writing Tests

- **Fakes over mocks:** Use fake implementations (see `internal/vpn/fake.go`) rather than mocking libraries
- **Test public APIs:** Focus on exported functions and their contracts
- **Table-driven tests:** Use `t.Run()` for multiple related test cases
- **Deterministic:** Tests should not depend on timing, randomness, or external state
- **Fast:** Tests should complete in milliseconds; avoid sleep where possible
- **Isolation:** Ordinary tests should not require a VPN endpoint, sudo, or
  credentials. Some lifecycle tests use temporary sockets or a test subprocess.

### Coverage Expectations

- **Guideline minimum:** 75%; CI currently does not enforce a coverage threshold.
- **Target:** 80%+
- **Focus on:** Public APIs, error paths, state machine transitions

---

## Submitting Changes

### Create a Pull Request

1. **Push your branch** to GitHub
   ```bash
   git push origin feat/my-feature
   ```

2. **Create a Pull Request** with:
   - Clear title describing the change
   - Detailed description of what changed and why
   - Reference any related issues (e.g., "Closes #123")
   - Link to relevant documentation

3. **Wait for CI** to pass (GitHub Actions will run automatically):
   - Format check
   - Linting
   - Test suite
   - Security scan
   - Build verification

   CI runs the YSH task-runner fixtures using a checksum-verified Oils release
   matching the pinned Guix version. The Guix shell integration test
   (`sh scripts/tests/test-shell-runner.sh`) runs locally from outside a Guix
   environment; hosted CI does not install Guix.

4. **Code review:** At least one approval required before merging

5. **Merge:** Use "Squash and merge" for clean history (unless commit history is valuable)

### PR Checklist

Before submitting a PR, ensure:

- [ ] Code formatted: `./run.ysh fmt`
- [ ] Tests pass: `./run.ysh unit-test`
- [ ] Linting passes: `./run.ysh lint`
- [ ] Security scan passes: `./run.ysh vuln`
- [ ] No hardcoded secrets or credentials
- [ ] Documentation updated (if applicable)
- [ ] Tests added for new functionality
- [ ] Commit messages clear and descriptive

---

## Project Structure

### Directory Overview

```
.github/workflows/     # GitHub Actions CI/CD
.githooks/            # Pre-commit hook
cmd/occlient/         # CLI binary (main entry point)
cmd/occlientd/        # Daemon entry point
cmd/occlient-gui/     # Desktop entry point
internal/
  ├── app/            # CLI handler and argument parsing
  ├── daemon/         # Daemon lifecycle and policy
  ├── gui/            # Desktop adapter and frontend
  ├── ipc/            # Shared IPC client and server
  └── vpn/            # VPN engine and OpenConnect integration
docs/
  ├── plans/          # Phase milestones and task checklists
  ├── security/       # Security boundaries and threat model
  └── testing/        # Testing strategy and coverage
scripts/
  ├── shell.sh        # Guix environment loader
  └── lib/            # Shared shell utilities
manifest.scm          # Guix manifest (pinned dependencies)
run.ysh              # Build/test/lint entry point
go.mod               # Go module file
go.sum               # Go dependency hashes
```

### Key Files

| File | Purpose |
|------|---------|
| `manifest.scm` | Defines Guix environment (Go, YSH, tools) |
| `run.ysh` | Build script (format, lint, test, build commands) |
| `go.mod` | Go module declaration |
| `.github/workflows/ci.yml` | GitHub Actions CI pipeline |
| `.githooks/pre-commit` | Local commit validation |
| `docs/architecture/daemon-and-ipc.md` | Daemon and IPC design |
| `docs/security/security-model.md` | Current security boundaries and limitations |
| `docs/testing/phase-3-testing.md` | Testing strategy and coverage |

---

## Common Tasks

### Add a New Package

1. Create directory under `internal/` with public types
2. Create `*_test.go` file with comprehensive tests
3. Update documentation if public API
4. Ensure tests pass: `./run.ysh unit-test`

### Add a New CLI Flag

1. Add flag definition in `internal/app/cli.go`
2. Add validation logic
3. Add tests in `cli_test.go`
4. Update help text in `printConnectHelp()`
5. Test manually: `./build/occlient connect -h`

### Fix a Bug

1. Write a **failing test** that reproduces the bug
2. Fix the code to make the test pass
3. Ensure no other tests break: `./run.ysh unit-test`
4. Document the fix in commit message

### Improve Performance

1. Add benchmarks: `./run.ysh unit-test -- ./package -bench=.`
2. Profile with pprof if needed
3. Measure improvement before/after
4. Keep tests deterministic (no timing-based assertions)

---

## Questions?

- **Documentation:** See `docs/` directory for detailed guides
- **Architecture:** See `docs/architecture/daemon-and-ipc.md`
- **Testing:** See `docs/testing/phase-3-testing.md`
- **Security:** See `docs/security/security-model.md`

---

## License

All contributions are licensed under Apache-2.0 (see LICENSE file).

By submitting a PR, you agree to license your contributions under this license.
