# OpenConnect Client

OpenConnect Client is an early-stage Linux desktop VPN client project. The
intended product will use OpenConnect and support enterprise authentication
flows such as SAML/SSO and MFA. Those application capabilities are planned;
this repository does not yet contain a functional VPN client.

## Current Phase

**Phase 2 (Go Foundation):** A minimal Go CLI foundation with testing and
build infrastructure. No VPN connectivity is implemented.

Previous: Phase 1 established a reproducible, project-local development
environment using GNU Guix.

## Development

Phase 1 targets Linux with GNU Guix and has only been checked on a Linux host.
Windows contributors may be able to use WSL2; macOS contributors need a Linux
virtual machine or remote Linux host. These non-Linux-host options have not
been validated.

### Quick Start

```bash
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
./scripts/shell.sh
```

Inside the shell, see available tasks:

```bash
./run.ysh list-tasks
```

Build and test the Go CLI:

```bash
./run.ysh build
./run.ysh run version
./run.ysh unit-test
```

Launch VS Code (installed on the host):

```bash
./run.ysh penv code .
```

Exit the project shell with `exit`.

### Documentation

- **[Development Guide](docs/development.md):** Detailed contributor workflow,
  IDE setup, Go development tasks, and troubleshooting.
- **[Phase 1 Plan](docs/plans/phase-1-dev-environment.md):** Development
  environment architecture and validation status.
- **[Phase 2 Plan](docs/plans/phase-2-go-foundation.md):** Go CLI foundation,
  architecture, and remaining tasks.

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE).
