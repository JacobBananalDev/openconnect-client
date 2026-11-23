# Phase 3 Plan — OpenConnect VPN Engine Integration

**Project:** OpenConnect Client  
**Repository:** https://github.com/JacobBananalDev/openconnect-client  
**Proposed branch:** `feat/openconnect-vpn-engine`  
**Status:** Not started — inspect the repository to determine actual progress  
**Prerequisite:** Phase 2 merged into the repository's default branch  
**Scope:** Safe, testable OpenConnect process integration, VPN lifecycle, CLI controls, and documentation

## Goal

Connect the existing Go application and CLI to the system OpenConnect executable through a small, testable backend. Implement truthful connection states, start/stop behavior, cancellation, and sanitized diagnostics. Do not introduce the desktop GUI or SAML browser login yet.

**Important:** A working VPN tunnel requires an installed OpenConnect executable, network access, suitable server credentials, and OS-level tunnel privileges. A Guix development environment does not automatically grant those privileges. Do not claim successful real-world VPN connectivity unless it has actually been tested with an authorized endpoint.

## Out of scope

- Wails, React, TypeScript, desktop UI, tray icon, or GUI packaging
- SAML/SSO browser flows, MFA automation, or credential capture/storage
- `sudo` automation, setuid binaries, polkit policy installation, or a privileged daemon
- systemd services, background auto-connect, startup persistence, or unattended reconnection
- Passwords, tokens, cookies, or secrets in command arguments, logs, or config files
- `.deb`/`.rpm`, release pipelines, telemetry, or cloud APIs
- Replacing the OpenConnect engine with a custom VPN implementation
- Automatically modifying routing, DNS, firewall rules, or the host network configuration outside OpenConnect's authorized behavior

If any item becomes necessary, explain the reason and request approval for a separate scoped task or later phase.

## Mandatory interactive workflow

1. Work on **one task at a time**; within Task 1, complete **one step at a time**.
2. **Before any edits:** describe the task, proposed design, exact files affected, alternatives/tradeoffs, and validation plan. Ask for approval.
3. **After edits:** show the relevant diff, actual commands and results, limitations, and the next task (not started). Ask for approval.
4. Do not proceed until I explicitly approve. If I request revisions, stay on the current task.
5. Never create or switch branches, commit, push, merge, or change remote settings for me. I will create the branch manually.
6. Never run `sudo`, install packages globally, alter host network settings, or connect to a VPN without my explicit permission.
7. Preserve Phase 1 Guix/YSH workflows and Phase 2 CLI architecture. Do not introduce Docker or another build system.
8. Never invent package names, claim tests passed without running them, or fake connected states.
9. Keep any sensitive material out of logs, terminal output, Git, tests, and documentation.
10. Stop after Phase 3. Do not start Phase 4 without permission.

**Before implementation ask:** “Does this approach look good, or would you like to modify anything before I implement it?”

**After implementation ask:** “Does this implementation look correct, or would you like to make any changes before we move on?”

## Task 1 — Verify Phase 2 merge, create branch manually, inspect architecture

**No source edits during Task 1.**

### Step 1.1 — Inspect repository only (start here)

- Inspect current branch, `git status`, remotes, remote default branch, and existing feature branches.
- Verify Phase 2 was merged; do not assume it was.
- Inspect the real Go module, CLI, configuration, tests, Guix manifest, YSH tasks, and contributor documentation.
- Check whether `openconnect` is available on the host or within the development environment, without installing it.
- Report findings and **stop**. Do not switch branches.

### Step 1.2 — Have me create the branch

Only after confirming Phase 2 is merged and the working tree is clean, give me commands to run manually. If the default branch is `main`:

```bash
git switch main
git pull --ff-only origin main
git switch -c feat/openconnect-vpn-engine
```

If the default branch differs, the branch exists, or the tree is dirty, adjust guidance safely. **Do not execute these commands yourself.** Wait for my confirmation.

### Step 1.3 — Verify my branch

Confirm `feat/openconnect-vpn-engine` is active and inspect status. Report and stop.

### Step 1.4 — Propose architecture

Review actual Phase 2 code and propose minimal interfaces and package placement for:

- An OpenConnect executable adapter using `os/exec` and `exec.CommandContext` (never shell interpolation)
- A lifecycle controller with explicit connection states and concurrency rules
- CLI commands for initiating, checking, and stopping a connection
- Test seams using a fake process/runner so tests never require root or a real VPN
- A safe privilege strategy and user-visible error messages

Discuss the crucial distinction between **CLI foreground process ownership** and **persistent connections across separate CLI invocations**. A `disconnect` command in a new process cannot control an earlier process without an approved IPC/daemon mechanism. Recommend a minimal honest Phase 3 design, such as a foreground `connect` command that terminates on Ctrl+C, with status limited to what can actually be observed. **Do not promise persistent `connect/status/disconnect` commands without an approved architecture.**

Ask approval before Task 2.

## Task 2 — Design the VPN engine contract

- Define only the minimal types needed by actual use cases: connection request, states, events/errors, and lifecycle methods.
- Distinguish `disconnected`, `connecting`, `connected` (only after reliable confirmation), `disconnecting`, and `failed` if justified.
- Specify concurrency behavior: duplicate starts, cancellation, process exit, cleanup, and race safety.
- Define how endpoint addresses and nonsecret options are validated.
- Do not store credentials, implement authentication, or launch a VPN yet.
- Add focused tests for state transitions if code is introduced.
- Show diff and results; wait for approval.

## Task 3 — Integrate the OpenConnect executable safely

- Propose whether `openconnect` should be a Guix-managed executable or an explicit host prerequisite, considering TUN privileges, DNS scripts, and host integration. Obtain approval before changing `manifest.scm`.
- Resolve the executable path safely and report missing-executable errors clearly.
- Use Go `os/exec` argument arrays; never use `sh -c`, `eval`, or interpolated shell commands.
- Pass only validated, nonsecret arguments; avoid secrets in process arguments or environment where feasible.
- Treat OpenConnect stdout/stderr as potentially sensitive; sanitize diagnostics and avoid verbose raw logging by default.
- Handle context cancellation and child process cleanup; explain signal/process-group behavior and limitations.
- Use fake executables or injected runners for tests; **do not connect to a real VPN in automated tests**.
- Show diff and actual tests; wait for approval.

## Task 4 — Connection lifecycle and state reporting

- Implement the approved lifecycle controller using the process adapter.
- Report a `connected` state only when supported by a reliable observable event; otherwise use a more accurate `running`/`connecting` state.
- Handle startup failure, early exit, cancellation, repeated attempts, and unexpected termination.
- Avoid leaked processes and goroutines; document shutdown semantics.
- Keep state scoped honestly to the owning process unless an approved cross-process service exists.
- Add deterministic tests for success, failure, cancellation, and races where practical.
- Show diff and results; wait for approval.

## Task 5 — CLI integration

- Review existing CLI conventions and propose a minimal interface before coding.
- Prefer a foreground command such as `occlient connect --server <URL>` that shows meaningful progress and exits cleanly on Ctrl+C.
- Do not expose a standalone `disconnect` or persistent `status` command unless a real supported process-control mechanism exists; explain alternatives first.
- Clearly distinguish an OpenConnect process starting from a confirmed VPN connection.
- Reject unsupported flags and missing endpoints with useful errors and exit codes.
- Do not prompt for, store, or print passwords/tokens. If authentication is required, document that this phase is not a full SAML/SSO solution.
- Add CLI tests without requiring privileges or a VPN server.
- Show diff and results; wait for approval.

## Task 6 — Security and privilege boundary review

- Document how OpenConnect interacts with TUN devices, routes, DNS, and privileges on Linux.
- Ensure the Go CLI and future GUI remain unprivileged by default.
- Do not implement a root helper, setuid, polkit policy, or `sudo` wrapper in this phase.
- Review process arguments, output sanitization, secrets, signal handling, and endpoint validation.
- Explain what would be required for a secure privileged helper in a later phase.
- No host changes or real connection attempts without my explicit approval.
- Show findings and request review.

## Task 7 — Testing and YSH integration

- Preserve existing `run.ysh` tasks and the Guix shell workflow.
- Add only approved, useful tasks (if needed) for unit tests, build, vet, and optional race checks.
- Test with fake process implementations, including failures and cancellation.
- Run `gofmt`, `go test ./...`, `go vet ./...`, and `go test -race ./...` where supported.
- Report skipped checks and reasons. Do not claim a VPN tunnel was tested if it was not.
- Show diff and results; wait for approval.

## Task 8 — Documentation and safe manual verification plan

- Update `README.md` and `docs/development.md` to reflect **only** implemented behavior.
- Document how OpenConnect is obtained/located according to the approved design.
- Document foreground CLI usage, Ctrl+C behavior, error handling, and known privilege limitations.
- Explain that SAML/SSO, persistent background connections, and the GUI are not implemented yet.
- Provide a **manual verification checklist** for an authorized test VPN endpoint, including consent before any network/privilege-changing commands. Do not include real secrets or server addresses.
- Show diff and wait for approval.

## Task 9 — Final validation and Phase 3 handoff

Verify and report:

- [ ] Phase 2 was merged before branching.
- [ ] All work is on `feat/openconnect-vpn-engine`, not directly on `main`.
- [ ] Phase 1 Guix/YSH and Phase 2 CLI behavior still work.
- [ ] OpenConnect invocation avoids shell interpolation.
- [ ] No secrets are logged, committed, or passed via unsafe command arguments.
- [ ] No unapproved host changes or privileged operations occurred.
- [ ] Process cancellation and cleanup are covered by tests.
- [ ] Connection state reporting is truthful and does not fabricate success.
- [ ] CLI error paths and exit statuses are tested.
- [ ] `go test ./...`, `go vet ./...`, and formatting checks pass, or limitations are reported.
- [ ] Documentation matches the implementation.
- [ ] No GUI, SAML, background daemon, packaging, or credential storage was introduced.

Present changed files, final architecture, commands, actual validation results, known limitations, and suggested next-phase options. Do not commit, push, or merge without approval. Do not begin Phase 4.

## Git workflow

**Branch:** `feat/openconnect-vpn-engine` — created manually by me.

Suggested commits, only when I approve:

- `feat: define VPN engine lifecycle`
- `feat: add OpenConnect process adapter`
- `feat: expose foreground VPN connect command`
- `test: cover VPN process and cancellation behavior`
- `docs: document VPN engine limitations`

After validation, propose a pull request into `main`. Do not create or merge it without permission.

## Resume on another computer

1. Clone/update the repository and enter `./scripts/shell.sh`.
2. Launch the IDE using `./run.ysh penv code .`.
3. Ask Copilot to read `.github/copilot-instructions.md` and `docs/plans/phase-3-vpn-engine.md`.
4. Inspect actual branch, source, tests, and plan status to find the first incomplete task.
5. Do not redo completed work or advance without my approval.

## START HERE

**Begin with Task 1, Step 1.1 only.** Inspect Git and verify Phase 2 is merged into the default branch. Do not modify files, create branches, or switch branches. Report findings and wait for my response.
