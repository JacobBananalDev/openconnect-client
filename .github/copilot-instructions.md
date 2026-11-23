# OpenConnect Client — Copilot Instructions

## Project

OpenConnect Client is a planned open-source Linux VPN desktop application. The long-term architecture may include Go, Wails, React/TypeScript, OpenConnect, browser-based SAML/SSO and MFA, an optional CLI, and a separately authorized privileged VPN helper. **Do not implement future features until the relevant phase is approved.**

Repository: `https://github.com/JacobBananalDev/openconnect-client`  
License: Apache-2.0 (preserve existing license).

## Mandatory interactive workflow

1. Work on **one task at a time**. For Task 1, work on **one step at a time**.
2. Before editing, describe the objective, exact files affected, implementation approach, and tradeoffs. **Ask for approval and wait.**
3. Implement only what was approved. Do not make unrelated changes.
4. After editing, show the relevant diff, explain decisions, and report validation commands with actual results. Identify anything untested.
5. Ask whether the result looks correct or needs modification. **Wait for explicit approval before proceeding.**
6. If changes are requested, revise the current task and present it again. Do not infer approval from silence.
7. Never create/switch branches, commit, push, open/merge pull requests, change GitHub settings, install host dependencies, or use `sudo` without explicit approval. The user will manually create the Phase 1 branch.
8. Do not begin Phase 2 without explicit approval.

### Before each task

Report **Task**, **Plan**, **Files affected**, **Design decisions/tradeoffs**, then ask:  
“Does this approach look good, or would you like to modify anything before I implement it?”

### After each task

Report **Completed**, **Changes/diff**, **Validation**, **Limitations**, **Next task (not started)**, then ask:  
“Does this implementation look correct, or would you like to make any changes before we move on?”

## Source of truth and progress

- Follow [`docs/plans/phase-1-dev-environment.md`](../docs/plans/phase-1-dev-environment.md) for the current Phase 1 scope and task order.
- Inspect the repository before assuming a task is complete. A checked box alone is not proof of implementation.
- At the end of an approved task, propose a progress-checklist update; obtain approval before editing it.
- When resuming on another machine, check Git status, branch, actual files, and the plan; continue at the first incomplete task. Never overwrite uncommitted work.

## Phase 1 technical principles

- Target reproducible, project-local development tools using **GNU Guix time-machine** with pinned channels and a manifest.
- Use **YSH/Oils** for `run.ysh`; verify real YSH syntax, package availability, and execution rather than guessing.
- Contributor workflow: `./scripts/shell.sh`, then `./run.ysh penv code .` (or `codium .` / `cursor .`).
- `penv` launches IDEs using the active project environment; it does not install tools or activate Guix itself.
- `editor` prefers `VISUAL`, then `EDITOR`; handle arguments safely and avoid `eval`.
- Avoid global Go, Node, npm, `gopls`, or `dlv` installations. Do not introduce Docker in Phase 1.
- Do not scaffold Go, Wails, React, VPN logic, packaging, or a privileged helper during Phase 1.
- Use least privilege, safe shell quoting, clear errors, minimal dependencies, and accurate documentation.
- Never invent a Guix commit, package name, validation result, or supported capability. Surface blockers and ask for a decision.

## Git workflow

- Phase 1 branch: `chore/guix-dev-environment`; do not commit directly to `main`.
- **Task 1 starts by inspecting Git only.** Report status and instruct the user how to create the branch manually; do not execute branch commands.
- Small, meaningful commits are encouraged only after explicit permission.
- Recommend a pull request into `main` after Phase 1 validation, but do not create or merge it without approval.
