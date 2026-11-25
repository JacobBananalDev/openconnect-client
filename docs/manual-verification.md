# Manual Verification

This guide separates unprivileged checks from optional service and VPN tests.
The CLI starts the real OpenConnect executable; it does not use a fake runner.
Automated tests use fakes and temporary sockets unless otherwise noted.

## Unprivileged checks

Enter the project environment with `./scripts/shell.sh`, then run:

```bash
./run.ysh build-prod
./run.ysh build-dev
./build/occlient --version
./build/occlient -h
./build/occlient connect -h
./build/occlient status -h
./build/occlient disconnect -h
./run.ysh unit-test
./run.ysh unit-test --race
find . -path './.go' -prune -o -type f -name '*.go' -exec gofmt -s -l {} +
./run.ysh lint
./run.ysh vuln
sh scripts/tests/test-runner.sh
sh scripts/tests/test-unit-runner.sh
sh scripts/tests/test-build-runner.sh
sh scripts/tests/test-dev-runner.sh
```

All three executables should exist in both `build/` and `build/dev/`. The
formatting check should print no files.
The vulnerability check resolves `govulncheck@latest` and may require network
access. From a fresh shell outside Guix, run the separate Guix integration
fixture with `sh scripts/tests/test-shell-runner.sh`.

### Desktop smoke check

On an unprivileged graphical session, launch `./run.ysh run`. Confirm that the
window opens, navigation placeholders and theme toggle work, and the VPN form
has labeled server and protocol fields. If the daemon is unavailable, confirm
that it reports `Unable to verify` and an `Unknown` state rather than
`disconnected`; do not stop an existing daemon merely to perform this check.
Close the window. To check the
development workflow, run `./run.ysh watch-gui`, change frontend and Go source as
appropriate in a disposable checkout, observe reload and rebuild, and stop it
with Ctrl+C.

Check argument validation without starting OpenConnect:

```bash
./build/occlient connect
echo "exit status: $?"  # Expected: 2; --server is required
temp_dir=$(mktemp -d)
./build/occlient connect --server vpn.example.com --ca-cert "$temp_dir/missing"
status=$?
rmdir "$temp_dir"
echo "exit status: $status"  # Expected: 1; CA cert file not found
```

The lifecycle tests exercise request completion, disconnect, daemon shutdown,
and foreground Ctrl+C with fake processes. An engine test also checks that a
real test subprocess is reaped on disconnect; it does not start OpenConnect.
None of these checks establish real tunnel connectivity.

## Optional systemd and VPN checks

These checks require an administrator-approved test host and VPN endpoint, a
manually installed service and an approved policy as described in the
[systemd guide](systemd.md). They can alter routes and DNS. Do not use
production connectivity or credentials without authorization. The current
client cannot answer interactive OpenConnect authentication prompts.

Use an authorized group member in a fresh login. Start with no active daemon
connection, and confirm the service and policy are configured:

```bash
systemctl status occlientd.service
./build/occlient status
```

Status should report `disconnected`. Request the exact approved configuration,
including any policy-required CA certificate:

```bash
./build/occlient connect --background \
  --server vpn.example.com \
  --protocol anyconnect \
  --script /usr/share/vpnc-scripts/vpnc-script
./build/occlient status
```

Replace the example values with the approved policy. A successful request
normally reports `connecting`; it does not prove authentication or tunnel
readiness. If OpenConnect exits early, investigate the host configuration
instead of treating the lifecycle check as passed.

### Desktop and CLI shared lifecycle

Use an approved policy without a custom CA certificate, with one distinct
script for the selected server and protocol. The current GUI screen cannot
specify a custom CA certificate. From an authorized unprivileged session:

1. Launch `./run.ysh run`, confirm the daemon is available and the state is
   `disconnected`, then request the approved server and protocol in the GUI.
2. Check `./run.ysh run -s` and verify it reflects the daemon's state. A
   `connecting` result does not confirm tunnel readiness.
3. If the OpenConnect child remains running, close and reopen the GUI and
   verify the daemon still reports the same connection. Identify the child as
   described below if needed.
4. Disconnect using the CLI and confirm the GUI refreshes to `disconnected`.

Do not mark this check passed if the request is denied, authentication is
unsupported, or the process exits before persistence can be verified.

### Terminal closure and disconnect

1. While the process is active, use `systemctl status occlientd.service` or
   `ps -o pid,ppid,args --ppid "$(systemctl show -p MainPID --value occlientd.service)"`
   to identify the daemon's OpenConnect child PID. Avoid sharing output that
   contains sensitive configuration.
2. Close the terminal that issued the background request. In a new authorized
   login, run `./build/occlient status` and confirm that the same child is
   still running. This verifies independence from that terminal, not tunnel
   readiness.
3. Run `./build/occlient disconnect` and then `./build/occlient status`;
   confirm `disconnected` and confirm the identified child has exited, for
   example with `ps -p PID -o pid,ppid,args` using the recorded PID. Check
   the process identity if the PID has been reused.

### Service shutdown

With permission, start another approved background request and identify its
child PID as above. Stop the service and verify the child is gone:

```bash
sudo systemctl stop occlientd.service
systemctl status occlientd.service
ps -p PID -o pid,ppid,args  # Substitute the recorded PID
```

`ps` normally returns a nonzero status when no matching process exists. Check
process identity if the PID has been reused. Inspect
`sudo journalctl -u occlientd.service` for shutdown errors. Follow the
[uninstall instructions](systemd.md#uninstall) when finished, if appropriate.

## Phase 4 verification record

- Builds, unit tests, race tests, lint, formatting and socket lifecycle checks
  passed in the existing development environment. The disconnect test reaped
  a real test subprocess.
- The normal vulnerability task failed with module lookup disabled. A cached
  `govulncheck` scan found no called vulnerabilities and reported one
  vulnerability in a required module that was not apparently called.
- A fresh pinned Guix environment and GitHub Actions were not independently
  run. No privileged service installation, terminal-closure test, real VPN,
  routing or DNS verification was performed.

## Phase 5 verification record

- Earlier local checks passed for the unified build, Go tests with the race
  detector, frontend tests, all five runner fixtures, formatting, lint, and
  the vulnerability scan. The development launcher was also exercised under
  Xvfb with INT, TERM, and terminal Ctrl+C; the watcher and lock stopped.
- After the policy script-selection fix, targeted daemon, GUI adapter, CLI, and
  daemon-command Go tests passed with the race detector. A subsequent unified
  build, CLI help/version checks, frontend tests, and formatting check passed.
  The built GUI opened a window under Xvfb; this smoke check did not inspect
  its rendered controls or state.
- Hosted CI passed for commit `9882d80`. Subsequent local workflow changes
  passed both build variants, the combined Go race and frontend tests, and
  runner fixtures. Both GUI build variants opened windows under Xvfb;
  `watch-gui` opened a native window and connected to Vite. These smoke
  checks did not verify rendered controls or reloads.
  Hosted CI has not yet run for those subsequent changes.
- No privileged service installation, GUI/CLI shared lifecycle with a real
  daemon, real VPN, authentication, routes, or DNS were verified for Phase 5.
