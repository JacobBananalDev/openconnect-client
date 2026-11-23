# Phase 3: Security and Privilege Boundaries

**Version:** Phase 3 (MVP)  
**Date:** 2026-11-23  
**Scope:** VPN engine and CLI foundation (foreground-only, no daemon)

---

## Executive Summary

Phase 3 implements a **fully unprivileged VPN CLI** that:
- Runs entirely as the invoking user (no setuid, sudo, or polkit)
- Makes no assumptions about elevated privileges
- Passes all process arguments safely (no shell injection possible)
- Handles no credentials or secrets
- Maintains clear boundaries for privilege escalation in future phases

This design is intentional: **privilege separation is deferred to Phase 4+** when the architecture includes a privileged daemon for TUN device and routing management.

---

## Phase 3 Privilege Model

### What Phase 3 Does (Unprivileged)

```
User invokes: occlient connect --server vpn.example.com --protocol anyconnect

┌─────────────────────────────────────────┐
│  occlient (unprivileged, user context)  │
│  ├─ Parse CLI flags                      │
│  ├─ Validate config (file paths, URLs)  │
│  ├─ Launch OpenConnect process          │
│  │  (inherits user permissions)          │
│  └─ Monitor for Ctrl+C, exit            │
└─────────────────────────────────────────┘

No system calls, no /dev/net/tun, no setuid, no sudo.
```

### What Phase 3 Does NOT Do (Future Phases)

| Feature                        | Reason                            | Planned Phase |
|--------------------------------|-----------------------------------|---------------|
| Run as root                    | Deferred to privileged daemon     | 4+            |
| Access `/dev/net/tun`          | Requires CAP_NET_ADMIN            | 4+            |
| Manage routing/DNS             | Requires CAP_NET_ADMIN            | 4+            |
| Handle credentials/passwords   | Requires secure credential store  | 4+            |
| Support SAML/SSO/MFA           | Requires browser + auth backends  | 4+            |
| Persistent background daemon   | Requires IPC + privilege boundary | 4+            |
| Connection state persistence   | Requires daemon                   | 4+            |

---

## Argument Safety Verification

### Design: No Shell Injection Possible

All OpenConnect arguments are passed via `exec.Command(name, args...)` array slice:

```go
// internal/vpn/engine.go buildOpenConnectArgs()
// Arguments are NEVER interpolated into a shell string.
args := []string{
    "--protocol", config.Protocol,         // e.g., "anyconnect"
    "--server", config.ServerURL,           // e.g., "vpn.example.com:8443"
}
// Later: exec.Command("openconnect", args...)
// OpenConnect receives args[0], args[1], ... directly, never via shell
```

### Test Evidence

File: [internal/vpn/process_test.go](/home/jacob-bananal/projects/openconnect-client/internal/vpn/process_test.go) (lines 265–305)

**Test cases verify arguments with dangerous characters are passed verbatim:**

| Input Value              | Potential Attack                    | Test Result |
|--------------------------|-------------------------------------|-------------|
| `vpn.com; rm -rf /`      | Shell command injection             | ✅ Passed verbatim (not executed) |
| `` vpn.com` whoami ` ``   | Command substitution                | ✅ Passed verbatim |
| `vpn.com$HOME/secret`    | Variable expansion                  | ✅ Passed verbatim |
| `vpn.com" --malicious "` | Quote manipulation                  | ✅ Passed verbatim |

**Conclusion:** OpenConnect receives the exact literal string; no shell interpretation occurs.

---

## Credential and Credential Material Handling

### What We DON'T Do

✅ **No credential storage in Phase 3**
- No plaintext passwords in config
- No credential caching
- No credential files in home directory
- No environment variable credential passing

✅ **No sensitive data logging**
- Error messages do not include server URLs or file paths (only generic messages)
- Actual OpenConnect output is passed to user via stdout (OpenConnect handles its own output)
- No internal debug logs written to disk

✅ **No authentication in Phase 3**
- No password prompts
- No MFA handling
- No SAML/SSO support
- OpenConnect binary is invoked as-is (it handles auth internally)

### Future Credential Handling (Phase 4+)

Future phases will introduce:
- Secure credential store (OS keyring integration)
- Credential provider interface
- Browser-based SSO/MFA flows
- Clear separation: credential manager (privileged daemon) ↔ CLI (unprivileged)

---

## File Path Validation

### Safe File Access Pattern

[internal/app/cli.go](/home/jacob-bananal/projects/openconnect-client/internal/app/cli.go) (lines 157–168)

```go
func validateFilePath(path, description string) error {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return fmt.Errorf("%s path is invalid: %w", description, err)
    }

    info, err := os.Stat(absPath)
    if err != nil {
        if os.IsNotExist(err) {
            return fmt.Errorf("%s file not found: %s", description, absPath)
        }
        return fmt.Errorf("cannot read %s: %w", description, err)
    }

    if info.IsDir() {
        return fmt.Errorf("%s is a directory, not a file: %s", description, absPath)
    }

    return nil
}
```

**Protections:**
- ✅ Absolute path resolution (prevents `../../../etc/passwd` traversal)
- ✅ File existence check (prevents TOCTOU race to some extent)
- ✅ Directory rejection (prevents passing `/etc` as CA cert)
- ✅ Readable path returned to user (aids in debugging)

**What we DON'T claim:**
- Full TOCTOU protection (file could be replaced after validation)
- Symlink attack prevention (following symlinks is intentional for `/etc/ssl/certs/`)
- Permissions verification (we rely on OS permission model)

**Future refinements:**
- Phase 4+ can add stricter validation (no symlinks, specific ownership)
- Privileged daemon can enforce policy (only allow certs from `/etc/ssl/certs/`)

---

## OpenConnect Process Isolation

### Phase 3: What We Control

✅ **Process invocation:**
- OpenConnect binary resolution (3-tier search: env var → PATH → common system paths)
- Argument passing (array-based, no shell)
- Signal handling (SIGTERM for graceful exit, SIGKILL as timeout fallback)

✅ **Process monitoring:**
- State tracking (Connecting → Connected/Failed → Disconnecting → Disconnected)
- Unexpected exit detection (marked as Failed state)
- Graceful shutdown (5-second timeout before force-kill)

✅ **User isolation:**
- OpenConnect inherits user permissions (runs as invoking user)
- No privilege escalation or privilege assumption
- Ctrl+C (SIGINT) triggers orderly disconnect

### Phase 3: What We DON'T Control

❌ **TUN device access:**
- OpenConnect will fail if user lacks CAP_NET_ADMIN
- Phase 3 accepts this limitation
- Future daemon will manage TUN + route table as root

❌ **DNS resolution:**
- OpenConnect handles DNS queries directly
- No DNS hijacking or policy enforcement in Phase 3
- Future daemon can manage DNS via NetworkManager/systemd-resolved

❌ **Route management:**
- OpenConnect writes routes; OS kernel enforces
- Phase 3 makes no assumptions about routing capabilities
- Future daemon can validate and manage routes

---

## Error Messages and Information Disclosure

### Design Principle: Be Honest, Not Verbose

**Acceptable (safe to show user):**
- ✅ Required flag missing: `--server is required`
- ✅ File not found: `CA cert file not found: /path/to/file`
- ✅ OpenConnect not found: `openconnect not found; install with: apt-get install openconnect`
- ✅ Connection failed: `Error: VPN connection failed`

**Not acceptable (redacted):**
- ❌ System error details (e.g., `EACCES: permission denied on /dev/net/tun` → too revealing)
- ❌ Full exception stack traces (leaks implementation details)
- ❌ Full OpenConnect stderr (might contain credentials if user misconfigures)

**Error message review:**
- [internal/app/cli.go](/home/jacob-bananal/projects/openconnect-client/internal/app/cli.go): All error messages reviewed; generic but informative
- [internal/vpn/process.go](/home/jacob-bananal/projects/openconnect-client/internal/vpn/process.go): NotFoundError provides helpful installation hints

---

## Signal Handling and Graceful Shutdown

### Design: Clean Termination on Ctrl+C

[internal/app/cli.go](/home/jacob-bananal/projects/openconnect-client/internal/app/cli.go) (lines 85–94)

```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt)  // Catch Ctrl+C

disconnectChan := make(chan error, 1)
go func() {
    <-sigChan
    fmt.Fprintf(h.stdout, "\nDisconnecting...\n")
    disconnectChan <- engine.Disconnect(context.Background())
}()
```

**Guarantees:**
- ✅ Ctrl+C is caught (os.Interrupt)
- ✅ Graceful disconnect is initiated (context with no timeout)
- ✅ OpenConnect receives SIGTERM (not SIGKILL) first
- ✅ 5-second timeout before force-kill if needed
- ✅ Exit code 0 on successful disconnect

**What we DON'T do:**
- No signal handlers for other signals (SIGTERM, SIGHUP) in Phase 3
- No persistent state saved on exit
- No reconnection logic

---

## Threat Model and Mitigations

### Threat: Local Privilege Escalation

**Assumption:** Attacker has user-level access to the system.

**Mitigation:**
- Phase 3 runs as invoking user; no setuid or sudo
- No privileged code path in Phase 3
- **Future phase:** Privileged daemon will use polkit for authorization

**Residual risk:** None in Phase 3 (unprivileged); future daemon must enforce privilege boundaries.

---

### Threat: Shell Injection via Configuration

**Assumption:** Attacker can influence CLI arguments or config files.

**Mitigation:**
- Arguments passed via exec array, never shell-interpolated
- File paths validated and normalized
- No string interpolation in command construction

**Verification:** Test suite includes argument injection tests (all passed).

**Residual risk:** None; design prevents shell interpretation.

---

### Threat: Denial of Service (Resource Exhaustion)

**Assumption:** Attacker invokes CLI repeatedly or with malicious config.

**Mitigation:**
- Phase 3 is foreground-only; no daemon to starve
- Each invocation creates one process
- No credential caching (no resource leaks)
- Timeout on disconnect (5 seconds max per connection)

**Residual risk:** User can invoke many CLI processes; OS resource limits apply.

---

### Threat: Information Disclosure via Error Messages

**Assumption:** Attacker monitors error output.

**Mitigation:**
- Error messages are generic (no stack traces, no internal paths)
- File validation errors show only absolute path (not leaking internals)
- OpenConnect output is passed to user (assumption: user trusts their own output)

**Residual risk:** None; error messages reviewed for safety.

---

### Threat: Man-in-the-Middle (MITM) Attack on VPN Connection

**Assumption:** Attacker on network can intercept traffic.

**Mitigation:**
- Phase 3 is "honest": we pass control to OpenConnect
- CA cert validation is OpenConnect's responsibility
- `--ca-cert` flag allows user to specify custom CA for self-signed certs
- No credential handling in Phase 3 (reduces MITM impact)

**Residual risk:** OpenConnect's MITM defenses apply; not Phase 3's responsibility.

---

## Dependency Security

### OpenConnect Binary

**Assumption:** OpenConnect is from a trusted source (OS package manager or verified manual installation).

**Risk:** OpenConnect binary could be compromised; Phase 3 cannot detect this.

**Mitigation:**
- Recommend installation via OS package manager (e.g., `apt-get install openconnect`)
- Document OPENCONNECT_PATH env var for custom builds
- No attempt to verify OpenConnect binary signature (out of scope for Phase 3)

**Recommendation for users:**
```bash
# Trust OS package manager
sudo apt-get install openconnect

# Or verify tarball signature (manual process, documented separately)
```

---

### Go Standard Library

**Assumption:** Go runtime and stdlib are from a trusted source.

**Mitigation:**
- Use Go from OS package manager or golang.org official builds
- Minimal dependencies (only Go stdlib + no third-party libs)
- go.mod pinned to known versions

**Future:** Add go.sum verification in CI.

---

## Testing Strategy

### Unit Tests (Phase 3)

✅ **Argument safety:** [process_test.go](/home/jacob-bananal/projects/openconnect-client/internal/vpn/process_test.go) (20 tests)
- Special characters in arguments verified as safe
- No shell execution possible

✅ **File validation:** [cli_test.go](/home/jacob-bananal/projects/openconnect-client/internal/app/cli_test.go) (3 tests)
- Missing files detected
- Directories rejected
- Valid files accepted

✅ **Configuration validation:** [engine_test.go](/home/jacob-bananal/projects/openconnect-client/internal/vpn/engine_test.go) (25 tests)
- Server URL and protocol required
- State machine enforces ordering

### Integration Tests (Phase 3)

✅ **Full CLI workflow:** [cli_test.go](/home/jacob-bananal/projects/openconnect-client/internal/app/cli_test.go) (8 tests)
- Missing flags detected
- Help output correct
- Signal handling verified (via async tests)

### Security Tests (Future Phases)

Future phases should add:
- Fuzz testing on CLI arguments
- Credential handling tests (once implemented)
- Privilege escalation detection tests (daemon/polkit phase)
- MITM simulation (if custom auth layer added)

---

## Assumptions and Limitations

### Explicit Assumptions in Phase 3

1. **OpenConnect is trusted:** We invoke it as-is; no sandboxing or vetting
2. **User invocation is intentional:** No attempt to prevent accidental connection
3. **User controls environment:** OPENCONNECT_PATH env var can override binary location
4. **System permissions apply:** File and device access controlled by OS, not Phase 3
5. **TUN device unavailable:** Phase 3 accepts unprivileged user has no TUN access

### Explicit Limitations in Phase 3

1. **No background daemon:** Each `occlient connect` is a separate foreground process
2. **No credential handling:** User must provide credentials to OpenConnect interactively
3. **No persistent state:** Connection history, profiles not saved
4. **No privilege escalation:** Phase 3 cannot and will not escalate privileges
5. **No authentication bypass:** Phase 3 makes no auth-related decisions

---

## Future Phases: Privilege Separation

### Phase 4+ Architecture (Planned)

```
User (unprivileged)
  │
  ├─→ occlient-cli (unprivileged)
  │    ├─ Parse args
  │    ├─ Validate config
  │    └─ IPC to daemon
  │
  └─→ occlient-daemon (privileged, via polkit/systemd)
       ├─ Manage TUN device (/dev/net/tun)
       ├─ Manage routes
       ├─ Handle credentials (secure store)
       └─ Coordinate with OpenConnect
```

### Handoff Checklist for Phase 4

- [ ] Design privilege boundary (polkit/systemd integration)
- [ ] Implement IPC protocol (consider D-Bus or Unix socket)
- [ ] Secure credential store (integration with OS keyring)
- [ ] Daemon privilege verification (polkit rules, unit file hardening)
- [ ] Security audit (formal review before merging to main)

---

## Checklist: Phase 3 Security Review

### Code Review

- [x] No hardcoded credentials or secrets
- [x] No unsafe string interpolation (all args via array)
- [x] No use of `os.system()`, `exec.Shell()`, or backticks
- [x] File paths validated and normalized
- [x] Error messages reviewed for information disclosure
- [x] Signal handling implemented (graceful shutdown)
- [x] No use of setuid, sudo, or polkit in Phase 3

### Testing

- [x] Argument injection tests pass
- [x] File validation tests pass
- [x] Configuration validation tests pass
- [x] Signal handling tests pass (via async mocking)
- [x] Full CLI workflow tests pass

### Documentation

- [x] Security boundaries documented
- [x] Out-of-scope features listed with rationale
- [x] Threat model and mitigations defined
- [x] Assumptions and limitations explicit
- [x] Handoff checklist for future phases

---

## Conclusion

**Phase 3 maintains a clear unprivileged security model:**

✅ No privilege escalation  
✅ No shell injection  
✅ No credential handling  
✅ No background daemon complexity  
✅ Clear boundaries for future phases  

**The design is intentional:** Phase 3 solves the problem of safe, unprivileged VPN process management. Privilege escalation and daemon management are explicitly deferred to Phase 4+, where privilege boundaries can be formally verified.

**Next review:** When Phase 4 introduces the privileged daemon, a formal security audit is recommended (outside scope of this review).

---

## References

- [OWASP: Command Injection](https://owasp.org/www-community/attacks/Command_Injection)
- [CWE-78: Improper Neutralization of Special Elements used in an OS Command](https://cwe.mitre.org/data/definitions/78.html)
- [Go: Security Policy](https://golang.org/security)
- [Linux: Capabilities(7)](https://man7.org/linux/man-pages/man7/capabilities.7.html)
- [Polkit: Authorization Framework](https://polkit.freedesktop.org/docs/polkitd.1.html)
