# Manual Verification

This guide separates unprivileged checks from optional service and VPN tests.
The CLI starts the real OpenConnect executable; it does not use a fake runner.
Automated tests use fakes and temporary sockets unless otherwise noted.

## Unprivileged checks

Enter the project environment with `./scripts/shell.sh`, then run:

```bash
./run.ysh build
./build/occlient --version
./build/occlient -h
./build/occlient connect -h
./build/occlient status -h
./build/occlient disconnect -h
./run.ysh unit test
./run.ysh unit test --race
gofmt -s -l .
./run.ysh lint
./run.ysh vuln
```

Both `build/occlient` and `build/occlientd` should exist. The formatting check
should print no files. The vulnerability check resolves `govulncheck@latest`
and may require network access.

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
manually installed service and an exact policy as described in the
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
