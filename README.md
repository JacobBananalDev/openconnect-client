# OpenConnect Client

OpenConnect Client is an early-stage Linux desktop VPN client project. The
intended product will use OpenConnect and support enterprise authentication
flows such as SAML/SSO and MFA. Those application capabilities are planned;
this repository does not yet contain a functional VPN client.

The current focus is Phase 1: a reproducible, project-local development
environment. The project toolchain is provided through a pinned GNU Guix
channel and manifest rather than requiring global Go, Node.js, or YSH
installations.

## Development

Phase 1 targets Linux with GNU Guix and has only been checked on a Linux host.
Windows contributors may be able to use WSL2; macOS contributors need a Linux
virtual machine or remote Linux host. These non-Linux-host options have not
been validated.

```bash
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
./scripts/shell.sh
```

When the project shell opens, launch VS Code (installed on the host):

```bash
./run.ysh penv code .
```

Exit the project shell with `exit`. See the [development guide](docs/development.md)
for prerequisites, alternative IDE/editor workflows, and troubleshooting.

See the [Phase 1 development-environment plan](docs/plans/phase-1-dev-environment.md)
for architecture, task scope, validation status, and remaining work.

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE).
