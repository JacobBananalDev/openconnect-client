# Daemon and IPC Architecture

## Status

This document describes the intended background-service boundary and the
current implementation status.

- `internal/ipc` provides the versioned JSON protocol and Unix socket client
  and server primitives.
- `cmd/occlientd` serves IPC and owns an engine. It authorizes root and the
  configured group and requires an administrator-managed connection allowlist.
- `occlient` exposes explicit background connect, status, and disconnect
  commands. An experimental systemd unit is supplied but not installed
  automatically.

## Scope

The target architecture is:

```text
occlient -> Unix domain socket -> occlientd -> internal/vpn.Engine -> OpenConnect
```

The daemon owns one engine and at most one active VPN connection. The
existing foreground CLI remains independent. Automatic connection at startup,
automatic reconnection, credential persistence, GUI, and browser-based SAML or
MFA flows are not part of this design.

The engine reports `connecting` after the OpenConnect process starts. It does
not yet detect authentication or tunnel readiness, and does not transition to
`connected`.

## Daemon lifecycle

- Startup creates the control socket and begins serving requests; it does not
  start OpenConnect.
- The daemon owns one engine for its lifetime. Connect and disconnect
  transitions are serialized.
- An accepted connect request uses a daemon-owned connection context, not the
  short-lived IPC request context. A client disconnect or response timeout
  therefore does not cancel an accepted connection.
- Status returns a snapshot of the engine state.
- On signal or orderly shutdown, the daemon stops IPC and disconnects the
  engine within a shared shutdown deadline. Only its own socket is removed.
- Disconnect sends SIGTERM, then SIGKILL after at most five seconds or if its
  context expires sooner; the engine remains `disconnecting` until the
  process has been reaped.
- An unexpected OpenConnect exit is reported as a failed connection. The
  daemon does not restart it automatically.

The engine's process-start state semantics and shutdown behavior must remain
covered by lifecycle tests. Starting the daemon or subprocess does not itself
grant TUN access, route privileges, or authentication readiness.

## Socket and privilege boundary

The supplied system service uses a system-managed runtime directory:

| Property | Decision |
|---|---|
| Socket | `/run/openconnect-client/control.sock` |
| Runtime directory | `/run/openconnect-client`, managed by systemd |
| Directory owner and mode | `root:<authorized-group>`, `0750` |
| Socket owner and mode | `root:<authorized-group>`, `0660` |
| Service identity | Root initially; least-privilege requirements remain to be verified |
| Client access | Root and members of the explicitly authorized group |

The directory mode controls traversal; the socket mode grants connect access.
The service must run with the authorized group so the socket has the expected
group ownership. The socket must not be world-accessible.

Linux `SO_PEERCRED` supplies the connecting process identity and primary GID;
`SO_PEERGROUPS` supplies its supplementary groups. The daemon authorizes root,
the designated primary group, or a peer with that supplementary group.
Socket permissions provide an additional boundary. The runtime directory and
its ancestors must be trusted, and access/default ACLs are rejected. The
daemon does not create the directory; the systemd unit manages it with mode
`0750`. Group setup and installation are manual as described in the
[systemd guide](../systemd.md). Systems without `SO_PEERGROUPS` fail closed.

Running the service as root is a material trust boundary: authorized clients
can request VPN operations through a privileged process. The service must
validate every request and apply administrator-controlled policy. The policy
allows exact server, protocol, certificate, and script combinations and
requires an explicit trusted script. Missing policy denies connections; an
invalid or unsafe policy prevents startup. Privileged file paths are checked
again before use. The policy and systemd hardening still need verification
against actual OpenConnect requirements. A separate privileged helper is not
part of this design.

The server refuses any pre-existing socket path rather than deleting a
potentially live or unrelated path. On shutdown it removes the socket only if
the path still identifies the socket it created. The runtime directory is
managed by systemd; the daemon does not create or repair it.

## IPC protocol

The protocol uses one UTF-8 JSON request and one JSON response per Unix stream
connection. Each message is one JSON object followed by a newline and EOF.
Reject malformed JSON, duplicate or unknown fields, extra data, missing
terminators, and unsupported versions. The request ID is a printable,
client-generated string of at most 64 bytes; it correlates the response and is
not an authentication token.

Protocol version 1 request:

```json
{
  "version": 1,
  "request_id": "client-generated-id",
  "command": "connect",
  "params": {
    "server": "vpn.example.com",
    "protocol": "anyconnect",
    "ca_cert": "/etc/ssl/certs/vpn-ca.pem",
    "script": "/usr/lib/openconnect/vpnc-script"
  }
}
```

Connect accepts only the engine's supported configuration fields. The daemon
validates inputs independently and applies administrator policy to fields with
privileged effects, including script paths.

Status and disconnect requests use an empty parameter object:

```json
{"version":1,"request_id":"client-generated-id","command":"status","params":{}}
```

Successful response:

```json
{
  "version": 1,
  "request_id": "client-generated-id",
  "ok": true,
  "data": {"state": "connecting"},
  "error": null
}
```

Failed response:

```json
{
  "version": 1,
  "request_id": "client-generated-id",
  "ok": false,
  "data": null,
  "error": {"code": "already_active", "message": "A connection is already active"}
}
```

Version 1 commands:

| Command | Success behavior | Defined errors |
|---|---|---|
| `connect` | Starts one engine connection and returns its current state, normally `connecting` | `invalid_request`, `unsupported_version`, `unauthorized`, `already_active`, `policy_denied`, `process_error`, `internal_error` |
| `status` | Returns `disconnected`, `connecting`, `connected`, `disconnecting`, or `failed` | `invalid_request`, `unsupported_version`, `unauthorized`, `internal_error` |
| `disconnect` | Requests disconnect; repeated requests while disconnected succeed | `invalid_request`, `unsupported_version`, `unauthorized`, `timeout`, `internal_error` |

Errors use stable codes and concise, non-sensitive messages. Transport failures
are reported by the client when no valid application response can be returned.
Messages must not include credentials, tokens, cookies, or raw OpenConnect
output.

## Bounds and operation semantics

- Maximum encoded request and response: 4 KiB each, including the newline.
- Maximum concurrent client connections: 32. Excess connections are closed
  without processing a request.
- Exactly one request is handled per connection; the server closes the
  connection after responding.
- Client exchange and server connection deadlines are 10 seconds. Disconnect
  has a bounded engine shutdown deadline of 5 seconds.
- A successful connect response means the daemon accepted the request and
  started the engine; it does not mean the tunnel is ready. If a client times
  out, it should query status rather than retry blindly.

## Foreground compatibility

`occlient connect --server ...` continues to run the engine in the CLI process,
wait for the user to disconnect, and work without a daemon. The background
command is explicit (`connect --background`) with separate `status` and
`disconnect` commands. An explicitly requested
background operation must fail clearly if the daemon is unavailable; it must
not fall back to foreground mode or elevate privileges silently.

Foreground custom certificate and script options are not automatically
authorized for privileged daemon use.

## Implementation notes

The transport enforces the protocol shape, message and connection bounds,
deadlines, socket mode, peer-credential retrieval, and safe refusal/cleanup of
occupied socket paths. Group authorization and policy checks are unit-tested;
privileged deployment remains unverified.
The [verification guide](../manual-verification.md) distinguishes automated
request-lifetime and shutdown checks from optional terminal and systemd tests.
