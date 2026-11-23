#!/bin/bash

set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

if [[ -n ${GUIX_ENVIRONMENT:-} ]]; then
	echo "Error: a Guix environment is already active; open a fresh shell before starting this project environment." >&2
	exit 1
fi

if [[ $# -ne 0 ]]; then
	echo "Usage: ./scripts/shell.sh" >&2
	exit 2
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

exec guix time-machine --channels="$REPO_ROOT/channels.scm" -- shell \
	--manifest="$REPO_ROOT/manifest.scm" \
	-- \
	env OPENCONNECT_CLIENT_ENV=1 bash --noprofile --norc -i
