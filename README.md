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

Phase 1 is designed for Linux and has only been checked on a Linux host. On
Windows, WSL2 may provide a Linux environment for development. WSL is not
available on macOS; use a Linux virtual machine or remote Linux host instead.
These non-Linux-host options have not been validated for this project. The
development environment also requires GNU Guix, and launching an IDE requires
graphical-session integration.

```bash
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
./scripts/shell.sh
./run.ysh penv code .
```

The shell starts the pinned project environment. When finished, leave it with
`exit`. To launch another installed IDE, pass its executable instead, for
example `./run.ysh penv codium .` or `./run.ysh penv cursor .`. The optional
`./run.ysh editor .` task uses `VISUAL`, then `EDITOR`.

See the [Phase 1 development-environment plan](docs/plans/phase-1-dev-environment.md)
for architecture, task scope, validation status, and remaining work.

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE).
