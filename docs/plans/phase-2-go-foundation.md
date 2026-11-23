# Phase 2 Plan — Go Application Foundation and CLI

**Project:** OpenConnect Client  
**Repository:** https://github.com/JacobBananalDev/openconnect-client  
**Phase branch:** `feat/go-project-foundation`  
**Status:** Not started — verify actual repository state before updating  
**Prerequisite:** Phase 1 merged into the repository's default branch  
**Scope:** Go application architecture, minimal CLI, configuration foundation, tests, and contributor documentation

## Goal

Establish a small, testable Go application that can be run and built entirely within the Phase 1 GNU Guix development environment. Provide a CLI entry point and a clean foundation for future VPN connectivity and GUI integration, **without implementing either in this phase**.

A contributor should be able to enter the existing Guix shell, run the project's Go checks, and execute a minimal `occlient` CLI. The exact CLI commands and package layout must be proposed and approved before implementation.

## Out of scope

- Actual VPN connections, tunnels, or OpenConnect process invocation
- SAML/SSO, browser authentication, MFA, or credential storage
- Root privileges, `sudo`, systemd services, polkit, or privileged helpers
- Wails, React, TypeScript UI, desktop windows, or GUI scaffolding
- `.deb`/`.rpm` packages, installers, auto-update, or release automation
- Docker, databases, network APIs, telemetry, or cloud services
- Global installation of Go or development tools

Do not add placeholder VPN functionality that pretends to connect. Keep future interfaces minimal and introduce them only when a concrete use case warrants them.

## Working agreement — mandatory

1. Work on **one task at a time**. In Task 1, work on **one step at a time**.
2. Before editing, explain the proposed design, alternatives/tradeoffs, and exact files affected. **Wait for my approval**.
3. Implement only the approved scope; show the relevant diff and actual test/validation output.
4. Ask whether I approve or want changes. Do not proceed until I explicitly approve.
5. If I request revisions, remain on the same task until I approve the result.
6. Never create/switch branches, commit, push, merge, or change GitHub settings without my permission. I will create the branch myself.
7. Never run `sudo`, install tools globally, modify my host configuration, or fetch and execute unverified scripts.
8. Preserve the Phase 1 Guix/YSH environment and contributor workflow. Do not silently replace it with Docker, Make, or global tool installations.
9. Do not claim tests passed unless you ran them; distinguish unverified behavior from verified results.
10. Stop at the end of Phase 2. Do not begin Phase 3 without explicit approval.

### Before each implementation

Report: **Task**, **Plan**, **Files affected**, **Design decisions**, and **Validation plan**. Ask:

> Does this approach look good, or would you like to modify anything before I implement it?

### After each implementation

Report: **Completed**, **Changes/diff**, **Validation results**, **Limitations**, and **Next task (not started)**. Ask:

> Does this implementation look correct, or would you like to make any changes before we move on?

## Task 1 — Confirm Phase 1 merge, manually create branch, inspect architecture

**No source changes during Task 1.**

### Step 1.1 — Inspect Git and Phase 1 state (start here)

- Inspect current branch, `git status`, remotes, and remote default branch.
- Verify whether Phase 1 is actually merged into the default branch; do not assume it based on this document.
- Inspect the existing `channels.scm`, `manifest.scm`, `scripts/shell.sh`, `run.ysh`, `.github/copilot-instructions.md`, README, and development guide if present.
- Report findings and stop. **Do not create or switch branches.**

### Step 1.2 — Have me create the feature branch

Only after verifying Phase 1 is merged and the working tree is clean, give me the appropriate commands. For a `main` default branch:

```bash
git switch main
git pull --ff-only origin main
git switch -c feat/go-project-foundation
```

If the default branch is not `main`, adjust. If the branch already exists or changes are uncommitted, explain safe alternatives. **Do not execute branch creation commands for me.** Wait for my confirmation.

### Step 1.3 — Verify branch

Verify active branch is `feat/go-project-foundation` and inspect status. Stop and report.

### Step 1.4 — Propose Go architecture

Inspect actual repository conventions and available Guix Go version. Propose a minimal package layout, CLI command names, test strategy, and how existing YSH tasks should be extended. Do not create files. Ask for approval before Task 2.

## Proposed target layout (subject to Task 1 approval)

```text
openconnect-client/
├── .github/
│   └── copilot-instructions.md
├── cmd/
│   └── occlient/
│       └── main.go
├── internal/
│   └── app/                  # Minimal CLI orchestration, if justified
├── docs/
│   ├── development.md
│   └── plans/
│       ├── phase-1-dev-environment.md
│       └── phase-2-go-foundation.md
├── scripts/
│   └── shell.sh
├── channels.scm
├── manifest.scm
├── run.ysh
├── go.mod
├── README.md
├── LICENSE
└── .gitignore
```

This is a proposal, **not** a requirement to create every directory. Prefer fewer packages and files until needed. Do not add `go.sum` unless dependency resolution actually produces one.

## Task 2 — Initialize the Go module

- Propose the module path `github.com/JacobBananalDev/openconnect-client` and Go version compatible with the existing pinned Guix toolchain.
- Create `go.mod` using Go **inside the existing Guix shell**; do not install Go globally.
- Avoid third-party dependencies for now.
- Verify `go mod` commands and explain any version/toolchain constraints.
- Do not scaffold CLI code during this task.
- Show changes and wait for approval.

## Task 3 — Minimal CLI entry point

- Propose and implement `cmd/occlient/main.go` with a simple, predictable entry point.
- Recommend an initial `--help`/`help` and `version` experience; agree on exact command behavior before coding.
- Use the Go standard library initially. Avoid a CLI framework unless justified and approved.
- Send normal output to stdout, errors to stderr; use meaningful exit codes.
- Do not claim VPN support or simulate a connected state.
- Prefer an injectable `Run`-style function to make CLI behavior testable without spawning a subprocess, if it keeps code simple.
- Verify `go run ./cmd/occlient --help` (or the approved equivalent).
- Show changes and wait for approval.

## Task 4 — Establish minimal application boundaries

- Review whether CLI parsing and application behavior need separation.
- Introduce `internal/app` or another small internal package **only if it reduces complexity**.
- Keep the command entry point thin and ensure future GUI code could reuse core application logic.
- Avoid premature interfaces, dependency injection frameworks, empty `internal/vpn` packages, and speculative service abstractions.
- No network operations, privileged processes, or VPN implementation.
- Add tests for any new behavior.
- Show changes and wait for approval.

## Task 5 — Configuration approach (design first)

- Propose a small configuration model for future use, but implement only immediately useful behavior.
- Prefer explicit CLI flags and/or environment variables where appropriate; avoid requiring a configuration file before there is a real need.
- If a config file is justified, propose its location using XDG conventions (`XDG_CONFIG_HOME` / `~/.config` fallback), format, precedence, and validation **before** adding dependencies or writing files.
- Never store credentials, tokens, passwords, or VPN secrets in configuration.
- Do not add profile management or VPN-specific settings until a later phase.
- Add tests for any implemented configuration behavior.
- Show changes and wait for approval.

## Task 6 — Go testing and quality checks

- Add focused unit tests for the implemented CLI and application behavior.
- Cover successful usage, invalid arguments, help/version behavior, and error paths where applicable.
- Use `go test ./...`, `go vet ./...`, and `gofmt` through the pinned Guix environment.
- Avoid network-dependent tests and external test frameworks unless approved.
- Report real command output, including any limitations.
- Show changes and wait for approval.

## Task 7 — Integrate Go tasks with the existing YSH runner

- Inspect and preserve existing `run.ysh` commands (`penv`, `editor`, `help`).
- Propose a small set of Go-specific tasks, such as `build`, `test`, `fmt`, and `vet`; obtain approval on names and behavior first.
- Keep implementation in genuine YSH/Oils syntax and validate using the actual interpreter.
- Run within the existing Guix shell; do not silently activate a second environment.
- Ensure build output goes to an ignored local directory (for example `build/`), not the repository root, if a build task is added.
- Avoid adding Make, Docker, or an additional task-runner framework.
- Verify each added task and show results.
- Stop and request review.

## Task 8 — Update contributor documentation

- Update `docs/development.md` and `README.md` only as necessary.
- Document the actual CLI commands, Go build/test workflow, and any YSH tasks added.
- Clearly label the application as an early foundation, **not a functioning VPN client**.
- Keep the README concise; put detailed developer instructions in `docs/development.md`.
- Preserve the existing Phase 1 onboarding commands.
- Update the plan's task status only with my approval.
- Show changes and wait for approval.

## Task 9 — Final Phase 2 validation and handoff

Verify and report each applicable item:

- [ ] Phase 1 was merged before branching.
- [ ] Work was performed on `feat/go-project-foundation`, not directly on `main`.
- [ ] Existing Guix shell and IDE workflow still work.
- [ ] `go.mod` uses the intended module path and compatible Go version.
- [ ] CLI help and version commands behave as approved.
- [ ] Invalid CLI input produces a useful error and exit code.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] Go files are formatted with `gofmt`.
- [ ] Any added YSH commands work and existing ones remain intact.
- [ ] Build output and local files are ignored appropriately.
- [ ] README and development guide reflect implemented behavior.
- [ ] No global development tool installs or unnecessary dependencies were introduced.
- [ ] No secrets, VPN credentials, privileged helper, SAML implementation, or GUI were added.

Provide final file tree, changed-file summary, exact contributor commands, tests and results, known limitations, and a **proposed** Phase 3 scope. Ask me before committing, pushing, or opening a pull request. Do not begin Phase 3.

## Git workflow

- Branch: `feat/go-project-foundation` (created manually by me).
- Small, descriptive commits **only when I explicitly approve**.
- Suggested commit messages (adapt to actual changes):
  - `feat: initialize Go module`
  - `feat: add occlient CLI foundation`
  - `test: cover CLI behavior`
  - `chore: add Go tasks to YSH runner`
  - `docs: document Go development workflow`
- When Phase 2 is complete and validated, propose a pull request into `main`; do not create or merge it without permission.

## How to resume on another computer

1. Clone or update the repository and enter the existing Guix environment using `./scripts/shell.sh`.
2. Open the IDE with `./run.ysh penv code .`.
3. Ask Copilot to read `.github/copilot-instructions.md` and this plan.
4. Have Copilot inspect the branch, Git status, actual files, and tests to identify the first incomplete task.
5. Do not infer completion from checklist text alone. Do not redo completed work without approval.
6. Continue with one task at a time and approval checkpoints.

## START HERE

**Begin with Task 1, Step 1.1 only.** Inspect Git and verify that Phase 1 has been merged into the default branch. Do not modify files, switch branches, or create the Phase 2 branch. Report your findings and wait for my response.
