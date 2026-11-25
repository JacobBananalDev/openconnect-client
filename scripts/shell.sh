#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

if [ -n "${GUIX_ENVIRONMENT:-}" ]; then
	echo "Error: a Guix environment is already active; open a fresh shell before starting this project environment." >&2
	exit 1
fi

if [ "$#" -ne 0 ]; then
	echo "Usage: ./scripts/shell.sh" >&2
	exit 2
fi

if ! command -v guix >/dev/null 2>&1; then
	echo "Error: guix is required but was not found in PATH." >&2
	exit 127
fi

if [ ! -f "$REPO_ROOT/channels.scm" ]; then
	echo "Error: missing Guix channel file: $REPO_ROOT/channels.scm" >&2
	exit 1
fi

if [ ! -f "$REPO_ROOT/manifest.scm" ]; then
	echo "Error: missing Guix manifest: $REPO_ROOT/manifest.scm" >&2
	exit 1
fi

cd "$REPO_ROOT"

host_path=
for tool in code codium cursor openconnect; do
	if tool_path=$(command -v "$tool") && [ -f "$tool_path" ] && [ -x "$tool_path" ]; then
		tool_dir=$(CDPATH= cd -- "$(dirname -- "$tool_path")" && pwd -P)
		case ":$host_path:" in
			*":$tool_dir:"*) ;;
			*) host_path="${host_path:+$host_path:}$tool_dir" ;;
		esac
	fi
done
OPENCONNECT_CLIENT_HOST_PATH=$host_path
export OPENCONNECT_CLIENT_HOST_PATH

exec guix time-machine --channels="$REPO_ROOT/channels.scm" -- shell \
	--pure \
	--preserve='^(OPENCONNECT_CLIENT_HOST_PATH|OPENCONNECT_PATH|DISPLAY|WAYLAND_DISPLAY|XAUTHORITY|XDG_RUNTIME_DIR|DBUS_SESSION_BUS_ADDRESS|XDG_CURRENT_DESKTOP|DESKTOP_SESSION|SSH_AUTH_SOCK|TERM|COLORTERM|NO_COLOR|LANG|LC_.*|VISUAL|EDITOR)$' \
	--manifest="$REPO_ROOT/manifest.scm" \
	-- \
	env OPENCONNECT_CLIENT_ENV=1 GOENV=off GOTOOLCHAIN=auto \
	GOPATH="$REPO_ROOT/.go" GOMODCACHE="$REPO_ROOT/.go/pkg/mod" sh -c \
	'libgcc_file=$(readlink -f "$(gcc -print-file-name=libgcc_s.so.1)") &&
	libgcc_dir=$(dirname "$libgcc_file") &&
	export LD_LIBRARY_PATH="$libgcc_dir${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" &&
	if [ -n "${OPENCONNECT_CLIENT_HOST_PATH:-}" ]; then
		PATH="$PATH:$OPENCONNECT_CLIENT_HOST_PATH"
		export PATH
	fi &&
	unset OPENCONNECT_CLIENT_HOST_PATH &&
	exec env OPENCONNECT_CLIENT_ENV=1 \
		PS1="\\[\\033[01;32m\\]\\u@\\h\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\] \\[\\033[01;32m\\][ENV]\\$\\[\\033[00m\\] " \
		bash --noprofile --norc -i'
