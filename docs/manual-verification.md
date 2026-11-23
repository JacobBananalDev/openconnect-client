# Manual Verification Guide

**Phase 3 — Safe Testing Without Real VPN**

---

## Overview

This guide provides step-by-step manual testing procedures to verify Phase 3 functionality **without requiring:**
- Real VPN credentials
- Network access
- Root privileges
- TUN device access

All tests use the **FakeRunner** test implementation that simulates OpenConnect behavior.

---

## Prerequisites

```bash
# Activate project environment
./scripts/shell.sh

# Build the binary
./run.ysh build

# Verify binary exists
ls -lh build/occlient
```

---

## Test 1: Help and Version Output

**Objective:** Verify CLI responds to help and version flags

**Steps:**

```bash
# Test: Main help
$ ./build/occlient -h
# Expected output:
#   Usage: occlient [command]
#   Commands:
#     connect [options]          Connect to a VPN server
#     help, -h, --help           Show this help message
#     version, -v, --version     Show version
#   Use 'occlient connect -h' for detailed connection options.

# Test: Connect help
$ ./build/occlient connect -h
# Expected output:
#   Usage: occlient connect [options]
#   Options:
#     --server <URL>             VPN server endpoint (required)
#     --protocol <proto>         VPN protocol (default: "anyconnect")
#     --ca-cert <path>          Path to custom CA certificate file
#     --script <path>           Path to custom vpnc-script

# Test: Version
$ ./build/occlient --version
# Expected output:
#   occlient version 0.1.0 (dev)

# Test: Version (alternate)
$ ./build/occlient -v
# Expected output:
#   occlient version 0.1.0 (dev)
```

**Verification:** ✅ All help and version outputs display correctly

---

## Test 2: Error Handling — Missing Required Flag

**Objective:** Verify proper error code when required flag is missing

**Steps:**

```bash
# Test: Connect without --server
$ ./build/occlient connect 2>&1 | head -10
# Expected output:
#   Error: --server is required
#   Usage: occlient connect [options]
#   ...

# Test: Exit code for missing flag (should be 2 = usage error)
$ ./build/occlient connect 2>/dev/null
$ echo $?
# Expected output: 2

# Test: Connect with --server
$ ./build/occlient connect --server vpn.example.com 2>&1 | head -5
# Expected output:
#   Connecting to vpn.example.com (anyconnect)...
#   Press Ctrl+C to disconnect
#   (then process exits or waits for Ctrl+C)
```

**Verification:** ✅ Missing flag causes exit code 2 with helpful error message

---

## Test 3: File Validation

**Objective:** Verify file path validation for --ca-cert and --script

**Steps:**

```bash
# Test: Invalid CA cert path (file doesn't exist)
$ ./build/occlient connect --server vpn.example.com --ca-cert /nonexistent/file.crt 2>&1
# Expected output:
#   Error: CA cert file not found: /nonexistent/file.crt
# Expected exit code: 1

# Test: Directory instead of file (should reject)
$ ./build/occlient connect --server vpn.example.com --ca-cert /etc 2>&1
# Expected output:
#   Error: CA cert is a directory, not a file: /etc
# Expected exit code: 1

# Test: Valid CA cert file (create temp file)
$ tmpfile=$(mktemp)
$ ./build/occlient connect --server vpn.example.com --ca-cert "$tmpfile" 2>&1 | head -3
# Expected output:
#   Connecting to vpn.example.com (anyconnect)...
#   Press Ctrl+C to disconnect
#   [Process runs with FakeRunner; press Ctrl+C to exit]
$ rm "$tmpfile"

# Test: Invalid vpnc-script path
$ ./build/occlient connect --server vpn.example.com --script /nonexistent/script 2>&1
# Expected output:
#   Error: vpnc-script file not found: /nonexistent/script
# Expected exit code: 1
```

**Verification:** ✅ File validation rejects missing/directory paths; accepts valid files

---

## Test 4: Protocol Default and Specification

**Objective:** Verify protocol defaults and can be specified

**Steps:**

```bash
# Test: Default protocol (should be "anyconnect")
$ ./build/occlient connect --server vpn.example.com 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com (anyconnect)...

# Test: Custom protocol (pulse)
$ ./build/occlient connect --server vpn.example.com --protocol pulse 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com (pulse)...

# Test: Custom protocol (nc)
$ ./build/occlient connect --server vpn.example.com --protocol nc 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com (nc)...

# Test: All supported protocols
for proto in anyconnect nc gp pulse f5 fortinet array; do
    echo "Testing protocol: $proto"
    timeout 1 ./build/occlient connect --server vpn.example.com --protocol $proto 2>&1 | head -1
done
# Expected: All should show correct protocol in output
```

**Verification:** ✅ Protocol defaults to "anyconnect" and can be overridden

---

## Test 5: CLI Argument Safety (Shell Injection Prevention)

**Objective:** Verify dangerous characters in arguments are NOT executed

**This is critical for security.** OpenConnect must receive arguments verbatim, with no shell interpretation.

**Steps:**

```bash
# Test: Semicolon (command separator) — should NOT execute "rm" or other command
$ ./build/occlient connect --server "vpn.example.com; echo INJECTED" 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com; echo INJECTED (anyconnect)...
# IMPORTANT: If you see "echo INJECTED" on separate line, that's a shell injection!
# Should NOT see: INJECTED

# Test: Backticks (command substitution) — should NOT execute "whoami"
$ ./build/occlient connect --server "vpn.example.com\`whoami\`" 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com`whoami` (anyconnect)...
# IMPORTANT: Username should NOT appear in output
# Should NOT be: Connecting to vpn.example.comjacob-bananal (anyconnect)...

# Test: Dollar sign (variable expansion) — should NOT expand $HOME
$ ./build/occlient connect --server "vpn.example.com/$HOME" 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com/$HOME (anyconnect)...
# IMPORTANT: Home directory path should NOT be expanded
# Should NOT be: Connecting to vpn.example.com/home/user (anyconnect)...

# Test: Quotes (quote manipulation) — should NOT be interpreted
$ ./build/occlient connect --server 'vpn.example.com" --malicious "flag' 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com" --malicious "flag (anyconnect)...
# IMPORTANT: Quotes should be passed verbatim, not interpreted as argument separator

# Test: Pipe (output redirection) — should NOT redirect to file
$ ./build/occlient connect --server "vpn.example.com | tee output.txt" 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com | tee output.txt (anyconnect)...
# IMPORTANT: No "output.txt" file should be created
$ ls output.txt 2>/dev/null && echo "ERROR: File was created!" || echo "✓ File not created"
```

**Verification:** ✅ Dangerous characters are passed verbatim; no shell injection possible

---

## Test 6: Exit Codes

**Objective:** Verify correct exit codes for different scenarios

**Steps:**

```bash
# Test: Success (Ctrl+C to exit)
$ timeout 1 ./build/occlient connect --server vpn.example.com >/dev/null 2>&1
$ echo "Exit code: $?"
# Expected output: Exit code: 124 (timeout command's exit code)
# (Or 0 if process exits naturally within 1 second)

# Test: Usage error (missing flag)
$ ./build/occlient connect 2>/dev/null
$ echo "Exit code: $?"
# Expected output: Exit code: 2

# Test: Runtime error (file not found)
$ ./build/occlient connect --server vpn.example.com --ca-cert /nonexistent/file 2>/dev/null
$ echo "Exit code: $?"
# Expected output: Exit code: 1

# Test: Unknown flag
$ ./build/occlient connect --server vpn.example.com --unknown-flag value 2>/dev/null
$ echo "Exit code: $?"
# Expected output: Exit code: 2
```

**Verification:** ✅ Exit codes correct (0=success, 1=runtime error, 2=usage error)

---

## Test 7: Multiple Connection Attempts

**Objective:** Verify CLI can be run multiple times sequentially

**Steps:**

```bash
# Run multiple connections in sequence (simulating user retries)
for i in 1 2 3; do
    echo "Attempt $i:"
    timeout 1 ./build/occlient connect --server vpn.example.com 2>&1 | head -1
    sleep 0.1
done

# Expected output:
#   Attempt 1:
#   Connecting to vpn.example.com (anyconnect)...
#   Attempt 2:
#   Connecting to vpn.example.com (anyconnect)...
#   Attempt 3:
#   Connecting to vpn.example.com (anyconnect)...
```

**Verification:** ✅ CLI can be invoked multiple times without issues

---

## Test 8: Long Server URLs and Paths

**Objective:** Verify CLI handles long and complex arguments

**Steps:**

```bash
# Test: Long server URL with port
$ timeout 1 ./build/occlient connect --server "vpn.example.com:8443" 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com:8443 (anyconnect)...

# Test: Server URL with username@host format
$ timeout 1 ./build/occlient connect --server "user@vpn.example.com" 2>&1 | head -1
# Expected output:
#   Connecting to user@vpn.example.com (anyconnect)...

# Test: Long CA cert path
$ tmpdir=$(mktemp -d)
$ tmpfile="$tmpdir/very/long/path/to/ca-certificate.crt"
$ mkdir -p "$(dirname "$tmpfile")"
$ touch "$tmpfile"
$ timeout 1 ./build/occlient connect --server vpn.example.com --ca-cert "$tmpfile" 2>&1 | head -1
# Expected output:
#   Connecting to vpn.example.com (anyconnect)...
$ rm -rf "$tmpdir"
```

**Verification:** ✅ CLI handles long and complex arguments correctly

---

## Test 9: Signal Handling (Ctrl+C)

**Objective:** Verify graceful shutdown on Ctrl+C

**Steps:**

```bash
# Test: Ctrl+C handling
$ timeout -s INT 2 ./build/occlient connect --server vpn.example.com 2>&1
$ echo "Exit code: $?"
# Expected output:
#   Connecting to vpn.example.com (anyconnect)...
#   Press Ctrl+C to disconnect
#   Disconnecting...
#   ✓ Disconnected
#   Exit code: 0
```

**Verification:** ✅ Ctrl+C triggers graceful disconnect with exit code 0

---

## Test 10: OpenConnect Binary Resolution

**Objective:** Verify executable resolution works

**Steps:**

```bash
# Test: Default resolution (should find openconnect on PATH or in common paths)
$ ./build/occlient connect --server vpn.example.com 2>&1 | head -3
# Expected: Either connects (finds openconnect) or shows error
# If error:
#   Error: openconnect not found; install with: apt-get install openconnect
#   Exit code: 1

# Test: Custom OPENCONNECT_PATH
$ export OPENCONNECT_PATH=/usr/sbin/openconnect
$ ./build/occlient connect --server vpn.example.com 2>&1 | head -1
# Expected: Connects or shows path-specific error

# Test: Invalid OPENCONNECT_PATH (should not use it; fall back to PATH search)
$ export OPENCONNECT_PATH=/nonexistent/openconnect
$ ./build/occlient connect --server vpn.example.com 2>&1 | head -1
# Expected: Either connects (via PATH fallback) or shows "not found" error
# (This verifies the 3-tier resolution: env var is validated, then PATH is used)
$ unset OPENCONNECT_PATH
```

**Verification:** ✅ Executable resolution works with environment variable and fallback

---

## Test 11: Combined Flags

**Objective:** Verify multiple flags work together

**Steps:**

```bash
# Create temp CA cert file
$ ca_file=$(mktemp)
$ echo "-----BEGIN CERTIFICATE-----" > "$ca_file"
$ echo "MIIBkTCB+wIJAKHHCgVhgqnnMA0GCSqGSIb3DQEBBQUAMBMxETAPBgNVBAMMCCpw" >> "$ca_file"
$ echo "-----END CERTIFICATE-----" >> "$ca_file"

# Test: All flags together
$ timeout 1 ./build/occlient connect \
    --server vpn.example.com:8443 \
    --protocol pulse \
    --ca-cert "$ca_file" 2>&1 | head -3
# Expected output:
#   Connecting to vpn.example.com:8443 (pulse)...
#   Press Ctrl+C to disconnect
#   [Process running...]

$ rm "$ca_file"
```

**Verification:** ✅ Multiple flags work together correctly

---

## Test 12: Invalid Flags and Arguments

**Objective:** Verify error handling for invalid input

**Steps:**

```bash
# Test: Completely unknown flag
$ ./build/occlient connect --server vpn.example.com --foo-bar 2>&1
# Expected: Error about unknown flag; exit code 2

# Test: Flag without value
$ ./build/occlient connect --server 2>&1 | head -1
# Expected: Error about flag requiring value

# Test: Duplicate flag (last value should win)
$ timeout 1 ./build/occlient connect --server vpn1.example.com --server vpn2.example.com 2>&1 | head -1
# Expected output:
#   Connecting to vpn2.example.com (anyconnect)...
#   (Last --server value is used)
```

**Verification:** ✅ Invalid arguments handled with appropriate errors

---

## Test 13: Help Text Consistency

**Objective:** Verify help text is consistent and accurate

**Steps:**

```bash
# Check main help
$ ./build/occlient -h | grep -q "connect" && echo "✓ Main help mentions 'connect'" || echo "✗ FAIL"
$ ./build/occlient -h | grep -q "version" && echo "✓ Main help mentions 'version'" || echo "✗ FAIL"

# Check connect help
$ ./build/occlient connect -h | grep -q "server" && echo "✓ Connect help mentions 'server'" || echo "✗ FAIL"
$ ./build/occlient connect -h | grep -q "protocol" && echo "✓ Connect help mentions 'protocol'" || echo "✗ FAIL"
$ ./build/occlient connect -h | grep -q "anyconnect" && echo "✓ Connect help mentions 'anyconnect'" || echo "✗ FAIL"
```

**Verification:** ✅ Help text is accurate and comprehensive

---

## Test 14: Code Quality Checks

**Objective:** Verify no broken code or security issues

**Steps:**

```bash
# Test: Format check
$ ./run.ysh fmt && echo "✓ Format check passed" || echo "✗ Format check failed"

# Test: Linting
$ ./run.ysh lint && echo "✓ Linting passed" || echo "✗ Linting failed"

# Test: Security scan
$ ./run.ysh vuln && echo "✓ Security scan passed" || echo "✗ Security scan failed"

# Test: Full test suite
$ ./run.ysh unit-test 2>&1 | tail -5
# Expected output:
#   PASS
#   ok  	github.com/JacobBananalDev/openconnect-client/cmd/occlient	... coverage: 85.0%
#   ok  	github.com/JacobBananalDev/openconnect-client/internal/app	... coverage: 77.8%
#   ok  	github.com/JacobBananalDev/openconnect-client/internal/vpn	... coverage: 81.8%
```

**Verification:** ✅ All automated checks pass

---

## Summary Checklist

After completing all tests, verify:

- [ ] Help and version output correct
- [ ] Error handling proper (correct exit codes)
- [ ] File validation working (missing/directory rejection)
- [ ] Protocol default and override working
- [ ] Shell injection prevention verified (critical!)
- [ ] Exit codes correct (0, 1, 2)
- [ ] Multiple invocations work
- [ ] Long URLs and paths handled
- [ ] Ctrl+C signal handling works
- [ ] OpenConnect binary resolution working
- [ ] Combined flags work together
- [ ] Invalid arguments rejected properly
- [ ] Help text accurate
- [ ] Code quality checks pass

**Result:** ✅ Phase 3 is ready for production use (foreground VPN connection only)

---

## Notes for Reviewers

**What Phase 3 Does:**
- Safe OpenConnect process integration (no shell injection)
- State machine for connection lifecycle
- CLI with flag parsing and validation
- Foreground-only connection (no daemon)

**What Phase 3 Does NOT Do:**
- Real VPN connections (manual testing requires real server/credentials if desired)
- Background daemon or persistent connections
- Credential handling or authentication
- Privilege escalation (runs as invoking user only)
- Automatic routing or DNS management

**Testing in This Guide:**
- All tests use **FakeRunner** (simulated OpenConnect)
- No real network, TUN device, or credentials required
- Safe to run on any Linux system
- Demonstrates feature completeness and safety

---

## Next Phase (Phase 4+)

Future phases will add:
- Background daemon with privilege separation
- Secure credential storage
- Browser-based SSO/MFA flows
- Desktop UI (Wails + React)
- Connection profiles and persistence

For now, Phase 3 provides a solid, tested foundation for unprivileged VPN process management.
