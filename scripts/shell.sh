#!/bin/bash

set -euo pipefail

if [[ $# -gt 1 ]]; then
	echo "Error: too many arguments" >&2
	exit 1
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

if [[ -n ${GUIX_ENVIRONMENT:-} ]]; then
	echo "The Guix shell is already active."
	exit 0
fi

if ! command -v guix >/dev/null 2>&1; then
	echo "Error: guix is required but was not found in PATH." >&2
	exit 127
fi

if [[ ! -f $REPO_ROOT/channels.scm ]]; then
	echo "Error: missing Guix channel file: $REPO_ROOT/channels.scm" >&2
	exit 1
fi

if [[ ! -f $REPO_ROOT/manifest.scm ]]; then
	echo "Error: missing Guix manifest: $REPO_ROOT/manifest.scm" >&2
	exit 1
fi

cd "$REPO_ROOT"

if [[ $# -eq 0 ]]; then
	login_shell=$(basename "$(getent passwd "$USER" | cut -d : -f 7)")
	guix time-machine --channels="$REPO_ROOT/channels.scm" -- shell \
		--manifest="$REPO_ROOT/manifest.scm" \
		"$login_shell" \
		-- \
		env OPENCONNECT_CLIENT_ENV=1 "$login_shell"
elif [[ $1 = -c ]] || [[ $1 = --container ]]; then
	guix time-machine --channels="$REPO_ROOT/channels.scm" -- shell \
		--manifest="$REPO_ROOT/manifest.scm" \
		--container \
		--network \
		--emulate-fhs \
		--writable-root \
		--expose="$HOME/.aws" \
		-- \
		env OPENCONNECT_CLIENT_ENV=1 bash --noprofile --norc -i
else
	echo "Unrecognized argument: $1" >&2
	exit 1
fi
