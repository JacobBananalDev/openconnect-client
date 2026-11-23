# Development Guide

OpenConnect Client is an early-stage Linux desktop VPN client project. The
application is not functional yet; Phase 1 establishes the reproducible
development environment and contributor workflow.

## Supported environment

Phase 1 targets Linux with GNU Guix. Windows contributors may be able to use
WSL2, while macOS contributors need a Linux virtual machine or remote Linux
host. These non-Linux-host options have not been validated for this project.

GNU Guix is a host prerequisite. The repository does not install Guix or
modify the host's global profiles. A graphical session is needed to launch an
IDE. VS Code, Codium, Cursor, and text editors are host applications; they are
not installed by the project manifest.

## Start the development environment

Clone the repository and enter its pinned development shell:

```bash
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
./scripts/shell.sh
```

The script uses GNU Guix time-machine with the official channel pinned in
`channels.scm` and the toolchain defined in `manifest.scm`. The shell provides
Go, Node.js, Git, C/C++ build tools, pkg-config, and YSH/Oils without requiring
project-specific global installs. It starts a clean interactive Bash and sets
`OPENCONNECT_CLIENT_ENV=1` inside that shell.

You can check the marker and key tools from the development shell:

```bash
printf '%s\n' "$OPENCONNECT_CLIENT_ENV"
command -v go node ysh
```

The marker should print `1`, and the tool paths should resolve into the Guix
profile. To leave the environment, run:

```bash
exit
```

Run `./scripts/shell.sh` from an ordinary shell, not from an already active
Guix environment. The script rejects arguments and nested Guix environments.

## Launch an IDE

From inside the development shell, launch an IDE that is already installed on
the host:

```bash
./run.ysh penv code .
```

Other supported executable names can be passed the same way:

```bash
./run.ysh penv codium .
./run.ysh penv cursor .
```

`penv` requires the active project environment and forwards its arguments and
the launched command's exit status. It does not install an IDE or activate
Guix.

VS Code may reuse an already-running process. In that case, the existing
process may not inherit the Guix environment. An isolated instance can be
started with its own profile:

```bash
./run.ysh penv code \
  --user-data-dir="$HOME/.local/share/openconnect-client-vscode" \
  --extensions-dir="$HOME/.local/share/openconnect-client-vscode/extensions" \
  --new-window .
```

This creates separate VS Code settings and extensions. The isolated launch was
tested, but environment inheritance by the VS Code process could not be
verified; treat it as a workaround, not a confirmed integration.

## Use a default editor

`editor` uses `VISUAL` first, then `EDITOR`. Set the chosen variable to one
executable path; command strings containing extra words or flags are not
split or evaluated. Pass editor-specific arguments after the task name:

```bash
export VISUAL=nano
./run.ysh editor README.md
```

Use a file or arguments accepted by the selected editor. For IDEs that open a
project directory, use `penv`, for example `./run.ysh penv code .`.

## Task help

The task runner currently has no public project tasks. Internal environment
helpers are intentionally omitted from the task list. Run:

```bash
./run.ysh
./run.ysh list-tasks
./run.ysh help penv
```

As public project tasks are added, they will appear in the task list with
summaries. `NO_COLOR=1` disables terminal color.

## Troubleshooting

- **`guix` not found:** Install GNU Guix according to its official
  documentation, then retry. The project script does not install host tools.
- **Guix environment already active:** Run `exit` until back in an ordinary
  shell, then start `./scripts/shell.sh`.
- **Inactive project environment:** Start the shell before invoking `penv` or
  `editor`. Those tasks require `OPENCONNECT_CLIENT_ENV=1`.
- **IDE executable not found:** Install the IDE on the host and use its actual
  executable name. IDE packages are not part of `manifest.scm`.
- **VS Code opens in an existing session:** The existing process may be
  reused. Use the isolated-profile command above if a separate instance is
  needed; note that its environment inheritance has not been verified.
- **No graphical IDE window:** Confirm the host has an active graphical
  session. The project shell preserves the graphical-session variables
  available to it, but graphical integration has only been checked on Linux.

## Scope and development principles

Phase 1 covers development tooling and onboarding only. It does not implement
VPN connections, OpenConnect integration, SAML/SSO, MFA, a GUI, packaging, or
a privileged helper. Keep project dependencies in the Guix channel and
manifest, use least privilege, and do not add global project-specific
installations.
