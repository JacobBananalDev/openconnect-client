# Security Model

## Current implementation

The foreground `occlient` CLI runs as the invoking user and does not elevate
privileges. The desktop GUI and explicit CLI daemon commands use the shared
IPC client to communicate with `cmd/occlientd` over an engine-backed Unix
socket. The GUI is launched unprivileged and does not execute OpenConnect.

The daemon requires root and authorizes root or members of the
administrator-managed `openconnect-client` group. It allows only
connection configurations in the administrator-managed policy. The project
does not install a systemd unit or privileged service automatically.
The [daemon and IPC architecture](../architecture/daemon-and-ipc.md) describes
the intended boundary and its current limitations. An experimental
[systemd unit](../../systemd/occlientd.service) is provided for manual
installation; it is not a hardened production configuration.

## Process execution

- OpenConnect is started through Go's process APIs with arguments passed as
  separate values; no shell command string is constructed.
- The foreground process runs with the invoking user's permissions. It does
  not automatically use `sudo`, setuid, or polkit.
- The executable can be selected with `OPENCONNECT_PATH`; otherwise the
  runner searches `PATH` and common system locations. The process owner is
  responsible for trusting that executable and environment. The daemon
  requires its executable and search-path directories to be root-owned,
  non-writable by group or others, free of ACLs, and free of symlinks.
- The process adapter redirects standard input from `/dev/null` and discards
  standard output and error by default. The CLI does not collect credentials
  or handle interactive authentication.
- A custom `--script` runs as part of the user's OpenConnect invocation. The
  foreground CLI checks that the path exists and is not a directory; that is
  not an allowlist or a guarantee against time-of-check/time-of-use changes.

The engine passes server, protocol, certificate, and script values through
argument arrays. Configuration validation currently requires a non-empty
server and protocol; it does not establish that a server is trusted or that a
protocol is supported by a particular OpenConnect build.

## IPC safeguards and limits

The transport package provides a versioned JSON protocol with strict field and
duplicate-key validation, UTF-8 checks, 4 KiB message limits, 10-second
connection deadlines, and a 32-connection cap. The Unix server sets socket
mode `0660`, obtains Linux `SO_PEERCRED` and `SO_PEERGROUPS`, requires an
authorizer, and refuses occupied paths. Cleanup removes only the socket inode
it created. Group membership is checked from kernel peer credentials,
including supplementary groups; unsupported credential retrieval fails closed.
The existing runtime directory must be root-owned, group-owned by the
authorized group, mode `0750`, and free of ACLs.

The root-controlled policy is a bounded, strictly decoded allowlist. Each entry
requires a trusted script. Explicitly requested options must match exactly;
when a request omits the script, the daemon selects it only if the matching
server, protocol, and certificate identify one distinct policy script.
Policy, scripts, certificates, executables, search paths, and their ancestors
must be root-owned, not group- or world-writable, without ACLs or symlinks.
Configured files are rechecked before use. Missing policy denies connections;
invalid or unsafe policy prevents startup. These checks do not protect against
a malicious or misconfigured administrator or changes by root. The daemon
does not log request contents, credentials, tokens, cookies, or raw subprocess
output.

The GUI binding exposes status, connect, and disconnect operations; connect
accepts server, protocol, and optional CA certificate fields, but not a script
or arbitrary executable. The current screen supplies only server and protocol.
The daemon remains responsible for authorization and policy enforcement.

## Connection state and authentication

The engine remains `connecting` after OpenConnect starts; it does not yet
detect authentication completion, tunnel configuration, or working routes.
OpenConnect may require network privileges that the foreground user does not
have.

The client does not implement credential storage, SAML, browser-based
authentication, MFA, or automatic reconnection. The production GUI uses
embedded frontend assets; no additional navigation restrictions are claimed
here.

## Verification boundaries

Unit tests use fake process runners and temporary Unix sockets; they do not
require a VPN endpoint, credentials, root privileges, or a systemd installation.
Real VPN connectivity, privileged service permissions, and systemd hardening
remain opt-in verification and have not been established by these tests.
Terminal closure and service-stop behavior on an installed system have not
been verified; see the [manual verification guide](../manual-verification.md).
