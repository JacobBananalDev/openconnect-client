# Development systemd service

The supplied service is experimental. It runs `occlientd` as root, accepts
root and members of the `openconnect-client` group, and permits only
administrator-approved connection configurations. No VPN connection starts
when the service starts. A systemd installation, a host OpenConnect executable,
and Linux support for `SO_PEERGROUPS` (introduced in Linux 4.13) are required.

The unit uses `/usr/local/sbin/occlientd` and creates
`/run/openconnect-client` with owner `root:openconnect-client` and mode `0750`.
Its default executable search path is `/usr/sbin:/usr/bin`. For another
OpenConnect location or helper search path, an administrator may use a
root-owned systemd drop-in. The executable, every search directory, and all
their ancestors must be root-owned, not writable by group or others, and
contain no symlinks or ACLs. This excludes paths beneath a group-writable or
non-root-owned Guix store. The unit does not install OpenConnect.

## Manual installation

Review the unit and target paths first. Do not overwrite an existing binary or
service belonging to another installation. From the project Guix shell:

```bash
./run.ysh build
```

The `openconnect-client` system group must exist before starting the unit.
An administrator should inspect any existing group before using it, or create
the group and add only users trusted to control the shared VPN:

```bash
getent group openconnect-client
# If the group does not exist:
sudo groupadd --system openconnect-client
# For each authorized user:
sudo usermod -aG openconnect-client USERNAME
```

Group changes generally require a new login. Every authorized member can
inspect or disconnect the shared connection.

Only if the target paths are appropriate, an administrator can install and
start the service manually:

```bash
sudo install -D -m 0755 build/occlientd /usr/local/sbin/occlientd
sudo install -D -m 0644 systemd/occlientd.service /etc/systemd/system/occlientd.service
sudo systemctl daemon-reload
sudo systemctl start occlientd.service
```

The installation does not enable the service at boot. Enabling the unit is an
explicit administrator decision; even if enabled, it does not initiate a VPN
connection automatically. Without a policy file, the daemon logs that
connections are denied but continues serving status and disconnect requests.

## Connection policy

The administrator may install `/etc/openconnect-client/policy.json`, for
example:

```json
{
  "connections": [
    {
      "server": "vpn.example.com",
      "protocol": "anyconnect",
      "script": "/usr/share/vpnc-scripts/vpnc-script"
    }
  ]
}
```

Replace the example with reviewed settings and a trusted installed script.
An optional `ca_cert` must match the request exactly; if omitted, clients
must omit it too. Every policy entry requires an explicit script. A request
that omits the script uses the policy's script only if the matching server,
protocol and certificate identify one distinct script; otherwise it is denied.
The GUI sends no script and its current screen cannot specify a CA certificate.
Script paths are restricted to ASCII letters, digits, `/`, `.`, `_`, and `-`, because
OpenConnect treats this option as a shell command. The server must not contain
credentials, a query, a fragment, whitespace, or begin with `-`. Supported
protocols are `anyconnect`, `nc`, `gp`, `pulse`, `f5`, `fortinet`, and `array`.
Do not put credentials or tokens in the policy.

The policy, configured script and optional CA file must be regular files;
they and every ancestor must be root-owned, not writable by group or others,
and free of symlinks and ACLs. The script must be executable by root. The
daemon checks configured files again before use. Review those files and their
dependencies, and ensure the default PATH contains trusted helper programs.
Invalid or unsafe policy prevents daemon startup. Policy changes take effect
after a service restart.

For example, after preparing a reviewed policy file:

```bash
sudo install -d -o root -g root -m 0755 /etc/openconnect-client
sudo install -o root -g root -m 0644 /path/to/reviewed-policy.json /etc/openconnect-client/policy.json
sudo systemctl restart occlientd.service
```

An authorized user can then request the approved configuration explicitly:

```bash
./build/occlient connect --background --server vpn.example.com --script /usr/share/vpnc-scripts/vpnc-script
```

For an unambiguous policy match, the script can also be omitted from a
background request; the daemon supplies it from the policy.

Startup does not confirm authentication or tunnel readiness.

## Inspect and stop

```bash
sudo systemctl status occlientd.service
sudo journalctl -u occlientd.service
./build/occlient status
sudo systemctl stop occlientd.service
```

With no active connection, status should report `disconnected`. Unapproved
background requests return `policy_denied`. The service stops accepting
IPC requests on shutdown and attempts to disconnect its child within a
bounded deadline. systemd sends the initial termination signal to the daemon
and force-kills remaining processes in its control group if needed.

## Troubleshooting

- **Cannot reach the daemon:** Check `systemctl status occlientd.service` and
  the journal. Confirm the runtime directory and socket exist with the
  ownership and modes described above.
- **Access denied:** Confirm the user belongs to `openconnect-client` in the
  current login session. Unsupported `SO_PEERGROUPS` retrieval fails closed.
- **`policy_denied`:** Compare the server, protocol, optional CA certificate,
  and any explicitly requested script with the policy. Without a requested
  script, matching entries must identify one distinct script. Restart the
  service after changing the policy.
- **Service fails to start or process fails to start:** Review the journal for
  policy, executable, PATH, ownership, permissions, symlink, or ACL errors.
  The service's fixed PATH differs from an interactive shell's PATH.
- **Connect request times out:** Check `./build/occlient status` before
  retrying; the daemon may already have accepted the request.

See the [manual verification guide](manual-verification.md) for lifecycle
checks and what has actually been tested.

## Uninstall

After confirming these paths belong to this installation:

```bash
sudo systemctl stop occlientd.service
# If explicitly enabled:
sudo systemctl disable occlientd.service
sudo rm /etc/systemd/system/occlientd.service /usr/local/sbin/occlientd
sudo systemctl daemon-reload
```

Remove any administrator-created policy, group, or drop-in separately after
reviewing its contents and any other uses. The runtime directory is managed
by systemd.

## Security and verification limits

Root is currently required. Full network capabilities, routing and DNS
requirements, filesystem access, and compatibility with OpenConnect scripts
have not been verified. This unit is not a hardened production service.
Policy, script contents, helper programs, and service environment are trusted
administrator responsibilities. Do not treat successful service startup as
proof of VPN connectivity or tunnel readiness.

On the development host, OpenConnect reports a vpnc-compatible configuration
script, and the installed script uses network configuration tools and DNS
helpers. A typical Linux tunnel needs access to `/dev/net/tun` and network
administration privileges; routes and DNS can require additional host-specific
access. The exact capability set and safe filesystem restrictions must be
verified with a real connection before hardening the service.
