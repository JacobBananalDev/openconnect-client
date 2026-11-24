# Phase 4 — Background VPN Service

## Objective
Introduce a Linux systemd-managed Go daemon (`occlientd`) that owns the existing VPN engine independently of CLI sessions. Preserve Phase 3 foreground behavior. Establish an IPC boundary suitable for a future desktop GUI.

## Scope and architecture
`occlient` → Unix domain socket → `occlientd` → existing `internal/vpn` engine → OpenConnect subprocess. One active connection at a time. SAML/browser authentication, GUI, distribution packaging and automatic reconnection are out of scope. Do not claim authenticated VPN support beyond existing capabilities.

## Workflow
Use `feat/background-vpn-service`. Enter the pinned environment with `./scripts/shell.sh`; optionally launch VS Code with `./run.ysh penv code .`. Inspect the existing repository, Phase 3 plan, and `.github/copilot-instructions.md` before implementation. Follow existing conventions and reuse dependencies. Work on exactly one task at a time: propose files, design, security implications and tests; wait for approval; implement; report changes and actual test results; wait for approval before proceeding. Never automatically create branches, commit, push, or append AI co-author trailers.

## Tasks

### 4.1 — Architecture and IPC contract
- [x] Complete — documented in [daemon-and-ipc.md](../architecture/daemon-and-ipc.md).

Document daemon lifecycle, single-session ownership, versioned request/response schema, connect/status/disconnect operations, bounded messages, deadlines, errors and authorization model. Decide socket location and privilege model before coding. Prefer a system service with a restricted runtime directory and explicit authorized group; evaluate peer credentials and least privilege. Define how foreground mode remains compatible. No implementation yet.

### 4.2 — Daemon foundation
- [x] Complete — lifecycle scaffold added in `cmd/occlientd` and `internal/daemon`.

Add `cmd/occlientd` and daemon package following existing layout. Implement startup, context cancellation, signal handling and graceful shutdown. Inject dependencies for testing. Avoid starting VPN automatically.

### 4.3 — Unix socket transport
- [x] Complete — v1 IPC protocol and bounded Unix socket client/server implemented in `internal/ipc`.

Implement server and client with bounded requests, timeouts, protocol validation and deterministic errors. Reject unauthorized peers; avoid world-writable sockets and unsafe stale socket deletion. Test malformed requests, concurrency, permissions and cleanup.

### 4.4 — VPN lifecycle integration
- [x] Complete — engine-backed daemon lifecycle integrated with interim root-only authorization and connect requests denied pending policy.

Reuse existing engine and fake runner. Serialize lifecycle transitions; reject duplicate connects; keep connection context independent of request context; handle unexpected process exit and daemon shutdown. Never report connected solely because a subprocess started; retain documented engine semantics and distinguish readiness limitations. Test races and cancellation.

### 4.5 — CLI integration
- [x] Complete — explicit background connect, status, and disconnect CLI commands added.

Preserve existing foreground connect command. Propose an explicit background option (e.g. `connect --background`) and `status`/`disconnect` commands. Provide clear errors when daemon unavailable; no silent privilege escalation or fallback. Validate inputs on server as well as client.

### 4.6 — systemd integration
- [x] Complete — experimental systemd unit and manual installation guide added.

Add service unit and documented manual installation for development. Use managed runtime directory, appropriate permissions, journal logging, graceful stop and sensible restart policy. Evaluate required network privileges and hardening against actual OpenConnect requirements. Do not install or enable services automatically. No automatic VPN connection at boot.

### 4.7 — Security review
- [x] Complete — group peer authorization and administrator-controlled exact connection policy implemented.

Enforce authorization before privileged operations. Do not allow arbitrary executable/script paths or unrestricted privileged OpenConnect options from IPC clients; use administrator-controlled allowlists/configuration. Avoid logging credentials, tokens or cookies. Do not introduce secret persistence. Document trust boundaries, group membership implications and remaining authentication limitations.

### 4.8 — Testing
- [x] Complete — lifecycle, cancellation, foreground, authorization, protocol, and concurrency tests verified.

Use fake process runner and temporary sockets for deterministic tests. Cover protocol, authorization, state transitions, concurrent requests, malformed input, timeouts, shutdown and foreground regression. Run race tests where supported. Real VPN and privileged systemd tests are opt-in manual verification, not required in ordinary CI.

### 4.9 — Development and CI
- [x] Complete — both binaries build through YSH and CI; existing quality checks retained.

Extend existing Guix/YSH build tasks to build both binaries and reuse existing unit-test, formatting, lint and vulnerability checks. Preserve CI coverage policy and hooks. Do not introduce global developer dependencies.

### 4.10 — Documentation and verification
- [ ] Documentation and automated verification complete; terminal-closure and privileged service verification remain pending.

Update README, contributor guide, architecture/security documentation and manual verification instructions. Document service setup, permissions, background connection lifecycle, logs, troubleshooting and uninstall. Verify terminal closure does not stop daemon-owned connection; verify disconnect and service stop clean up subprocesses. Record what was actually tested and what remains unverified.

## Definition of done
Both binaries build in pinned environment; foreground behavior preserved; daemon IPC is restricted and validated; lifecycle tests and existing CI pass; service configuration and manual verification documented; no secrets logged; no unsupported SAML or readiness claims. Review all changes before any user-approved commit or PR.
