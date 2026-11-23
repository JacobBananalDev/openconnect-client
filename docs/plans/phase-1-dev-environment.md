# Phase 1 Plan — Reproducible Development Environment

**Project:** OpenConnect Client  
**Repository:** https://github.com/JacobBananalDev/openconnect-client  
**Phase branch:** `chore/guix-dev-environment`  
**Status:** Complete — Tasks 1–9 are complete and validated, subject to the explicitly documented platform and VS Code limitations.
**Scope:** Development tooling and contributor onboarding only

**Supported contributor hosts:** Linux only for Phase 1. macOS and Windows
workflows are not supported by this plan.

## Goal

Make it easy for a Linux contributor to clone OpenConnect Client, enter a pinned GNU Guix development environment, and launch an IDE with the project toolchain, without installing Go, Node.js, or other project-specific dependencies globally.

The intended contributor workflow is:

```bash
git clone https://github.com/JacobBananalDev/openconnect-client.git
cd openconnect-client
./scripts/shell.sh
./run.ysh penv code .
```

Alternative IDEs:

```bash
./run.ysh penv codium .
./run.ysh penv cursor .
```

Optional default editor:

```bash
./run.ysh editor README.md
```

Pass a file or arguments accepted by the configured editor. Exit the
development shell with `exit`.

## Current progress and validation

- The working branch is `chore/guix-dev-environment`. It was already checked out when Task 1 began; no branch operation was performed.
- Task 1 Git inspection and architecture review are complete. The user manually created the phase branch before Task 2; it was verified active. No branch commands were run during this work.
- The official Guix channel is pinned to commit `c85a7f8d7e4ae16557cb58ac87b7613d0f4b111d`. `guix time-machine -C channels.scm -- describe --format=channels` resolved the configured pin.
- `manifest.scm` resolves against that pin and provides Go, Node.js, Git, GCC, pkg-config, Make, Bash, Coreutils, and YSH.
- `scripts/shell.sh` and `run.ysh` are executable. Targeted checks confirmed the environment marker, graphical-session variables, and pinned Go, Node.js, and YSH executables are available in the Guix shell; task help, argument forwarding, error handling, and exit-status forwarding were also checked.
- `./run.ysh penv code .` returned successfully, but the existing VS Code process ID was unchanged before and after the launch. The CLI therefore handed off to the existing process, whose Guix environment inheritance was not verified. A separate VS Code process and X11 window were launched with unique `--user-data-dir` and `--extensions-dir` paths, without stopping the existing process or changing global settings. The isolated process did not expose its environment variables for inspection, so marker inheritance remains unverified.
- The configured `nano` editor opened `README.md` through `./run.ysh editor README.md` and exited without edits. The contributor guide is documented in `docs/development.md`; WSL2/macOS options and the VS Code environment-inheritance limitation are explicitly identified as unvalidated.
- `README.md` provides a Linux/Guix quick start, links to `docs/development.md`, and separates opening the interactive shell from launching VS Code.

This progress summary records observed implementation and tests; the checklist below remains the task-level source of completion.

## Out of scope

No Go application scaffolding, VPN connection logic, OpenConnect integration, SAML/SSO, MFA, Wails, React, GUI dependencies, privileged VPN helper, Docker, Linux packages, or CI pipeline in Phase 1. Those belong to later approved phases.

## Phase 1 repository structure

```text
openconnect-client/
├── .github/
│   └── copilot-instructions.md
├── docs/
│   ├── development.md                 # Created in Task 7
│   └── plans/
│       └── phase-1-dev-environment.md
├── scripts/
│   ├── lib/
│   │   └── taskfile.ysh               # YSH task/help dispatch used by run.ysh
│   └── shell.sh                       # Created in Task 4
├── channels.scm                       # Created in Task 2
├── manifest.scm                       # Created in Task 3
├── run.ysh                            # Created in Task 5
├── README.md
├── LICENSE
└── .gitignore
```

The tree describes the planned Phase 1 result, not a guarantee that every listed
file exists yet. Verify the worktree and checklist before treating any task as
complete.

## Approval and task protocol

**Work on one task at a time; within Task 1, one step at a time.** Before any file change, explain the plan and exact files affected, then wait for approval. After implementation, show the diff and actual validation results, then ask for review. If revisions are requested, remain on the current task. Never automatically proceed.

The user must explicitly approve branch changes, commits, pushes, host installs, and any changes to the task plan. Never commit directly to `main` or begin Phase 2 without approval.

## Task 1 — Git branch setup, inspection, and architecture

**No source changes during Task 1.**

### Step 1.1 — Inspect Git (start here)

- Identify the current branch, Git status, remotes, default branch, uncommitted changes, and whether `chore/guix-dev-environment` already exists.
- Do not create/switch branches or modify files.
- Report findings and stop.

### Step 1.2 — Ask the user to create the branch

If the default branch is `main`, the working tree is clean, and the branch does not exist, instruct the user to run:

```bash
git switch main
git pull --ff-only origin main
git switch -c chore/guix-dev-environment
```

Adjust commands if the default branch differs. If there are uncommitted changes or the branch exists, explain safe options first. **Do not run the branch commands yourself.** Wait for the user's confirmation.

### Step 1.3 — Verify branch

Confirm the active branch is `chore/guix-dev-environment` and inspect working-tree status. Do not modify files.

### Step 1.4 — Inspect repository and host

Review README, LICENSE, `.gitignore`, existing structure, Linux distribution, Guix, YSH/Oils, existing tools, and GUI session relevant to IDE launch. No installs.

### Step 1.5 — Propose architecture

Explain Guix time-machine pinning, manifest dependencies, how YSH is provided, shell environment marker, IDE environment inheritance, host prerequisites, and exit behavior. Show proposed file layout.

### Step 1.6 — Ask for approval

Present findings and ask whether the architecture is approved before Task 2. Do not create files yet.

**Acceptance:** Branch manually created and verified, architecture approved, no files changed as part of Task 1.

## Task 2 — Pin Guix channel

**File:** `channels.scm`

- Use GNU Guix time-machine and a **verified** pinned Guix channel commit.
- Prefer the official channel; avoid unnecessary third-party channels.
- Never invent commit hashes; explain intentional updates.
- Validate Scheme/channel configuration where possible.
- Do not create the manifest yet.

**Acceptance:** Valid, reviewed channel pin and documented update approach.

## Task 3 — Guix development manifest

**File:** `manifest.scm`

- Select verified Guix packages for Go, Node.js, Git, C/C++ build utilities, pkg-config, and strictly necessary tools.
- Include YSH/Oils if actually available from the pinned channel.
- If YSH is unavailable, explain reproducible alternatives and ask before selecting one.
- Avoid global installations, Docker, Wails, GUI build dependencies, and unneeded packages.
- Verify manifest resolution and explain version upgrades.

**Acceptance:** Manifest resolves, or a documented blocker/decision is approved.

## Task 4 — Development shell

**File:** `scripts/shell.sh`

- Executable Bash script; resolve repo root independently of current directory.
- Check Guix and required files; provide actionable errors and exit codes.
- Enter an interactive shell via pinned Guix time-machine and manifest, conceptually:

```bash
guix time-machine -C channels.scm -- \
  shell -m manifest.scm -- bash
```

- Ensure `OPENCONNECT_CLIENT_ENV=1` is **inside** the launched development shell, not just the parent process.
- Preserve required graphical-session variables for launching IDEs.
- Reject unsupported arguments and fail clearly if Guix is already active.
- No `sudo`, container mode, global profile edits, or unsafe `eval`; safe quoting and documented `exit` behavior.
- Validate syntax and failure paths.

**Acceptance:** Environment starts, marker is present, and failures are clear.

## Task 5 — YSH/Oils task runner

**Files:** `run.ysh`, `scripts/lib/taskfile.ysh`

Implement genuine, validated YSH/Oils syntax for:

- **`penv EDITOR [ARG...]`** — check marker, require executable, forward arguments and exit status; intended for `code .`, `codium .`, `cursor .`. No installs or implicit Guix activation.
- **`editor [ARG...]`** — check marker, prefer `VISUAL` then `EDITOR`, forward arguments safely, explain limitations for complex editor command strings rather than using `eval` or unsafe whitespace splitting.
- **`help`** and **`list-tasks`** — provide task usage and summaries from task documentation comments using the shared YSH helper in `scripts/lib/taskfile.ysh`.
- Task listings show public tasks only; mark helper commands with `## visibility: internal` to keep them callable but out of the list.

Do not automatically install `gopls`, `dlv`, or other tools when launching an IDE. Keep task-runner logic simple.

**Acceptance:** Executable YSH script with validated help, IDE forwarding, and error behavior.

## Task 6 — IDE integration validation

**Files:** Only propose changes if validation reveals an issue; get approval before edits.

- Test `./scripts/shell.sh` then `./run.ysh penv code .` in a graphical session when available.
- Verify marker, toolchain, argument forwarding, missing executable behavior, and rejection outside the environment.
- Test `editor` when configured.
- Investigate VS Code reusing an existing process outside Guix; propose a safe solution without terminating sessions or changing global settings.
- Report anything that could not be tested.

**Acceptance:** Verified IDE environment inheritance or an explicit, documented limitation.

## Task 7 — Contributor guide

**File:** `docs/development.md`

Document prerequisites, Guix setup, cloning, entering the shell, launching VS Code/other IDEs, default editor, exiting, troubleshooting, and development principles. Use copy-paste-ready commands matching actual implementation. Do not document nonexistent features.

**Acceptance:** Accurate, concise contributor guide.

## Task 8 — README development section

**File:** `README.md`

Preserve existing project overview and Apache-2.0 license references. Add a short Development section with clone, `./scripts/shell.sh`, `./run.ysh penv code .`, and a link to `docs/development.md`. Avoid claiming the VPN app is functional.

**Acceptance:** Clean, valid Markdown with accurate onboarding instructions.

## Task 9 — Final Phase 1 validation

Review actual repository contents and run appropriate tests. Report results, failures, and limitations. Ask permission before any final commit or push. Recommend a pull request into `main` only after approval.

### Completion checklist

Update these boxes only after verifying each result and obtaining approval for plan edits.

- [x] Task 1: Git inspection, manual chore branch, and architecture approval
  Git inspection and architecture approval are complete. The branch was already active; whether it was manually created before this session is not verified.
- [x] Task 2: Verified pinned `channels.scm`
- [x] Task 3: Resolving `manifest.scm` and YSH availability decision
- [x] Task 4: Executable development shell with working environment marker
- [x] Task 5: Executable YSH `run.ysh` with `penv`, `editor`, and task help
- [x] Task 6: IDE integration and error-path validation, with VS Code environment inheritance limitation documented
- [x] Task 7: Accurate `docs/development.md`
- [x] Task 8: README development quick start
- [x] Task 9: Final review and validation

### Final checks

- [x] Active development branch is `chore/guix-dev-environment`
- [x] No Phase 1 work committed directly to `main`
- [x] Guix pin is verified; manifest resolves
- [x] Shell and task runner are executable
- [x] `OPENCONNECT_CLIENT_ENV=1` exists inside Guix shell
- [x] `run.ysh help` works
- [x] IDE launch tested; VS Code process-reuse environment limitation documented
- [x] `run.ysh editor README.md` opens a file in the configured editor
- [x] `penv` rejects inactive environments and missing executables clearly
- [x] Argument forwarding and exit statuses are correct
- [x] Documentation matches tested behavior
- [x] No global project-specific dependency installations
- [x] No secrets or machine-specific data added

## Resuming on another computer

1. Clone or pull the repository; **check for local changes before pulling**.
2. Verify which branch contains Phase 1 work and switch safely to `chore/guix-dev-environment` if appropriate.
3. Open the repository in VS Code and ask Copilot:

> Read `.github/copilot-instructions.md` and `docs/plans/phase-1-dev-environment.md`. Inspect the current Git state and files, compare them to the checklist, and identify the first incomplete task. Propose only that task, and wait for my approval before editing. Do not commit or push without permission.

4. Verify actual implementation before trusting checked boxes. Git stores committed files, **not** Copilot chat history or uncommitted work.

## Future phases (not approved)

Go application foundation, VPN integration, SAML/SSO, privileged helper, GUI, packaging, and release automation will be planned separately after Phase 1 is complete.
