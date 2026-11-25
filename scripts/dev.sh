#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir/.."

if [ "${OPENCONNECT_CLIENT_ENV:-}" != 1 ]; then
	echo 'Error: the project environment is not active. Run: ./scripts/shell.sh' >&2
	exit 1
fi
. scripts/lib/gui-env.sh
prepare_gui_environment
if [ -z "${DISPLAY:-}" ] && [ -z "${WAYLAND_DISPLAY:-}" ]; then
	echo 'Error: a graphical display is required for development mode.' >&2
	exit 1
fi

build_lock=0
dev_lock=0
watcher=
interrupted=0
starting_watcher=0
signal_sent=0
cleanup() {
	status=$?
	set +e
	if [ "$dev_lock" -eq 1 ]; then
		rmdir -- build/.dev-lock || status=1
	fi
	if [ "$build_lock" -eq 1 ]; then
		rmdir -- build/.build-lock || status=1
	fi
	exit "$status"
}
signal_watcher() {
	if [ "$signal_sent" -eq 0 ] && [ -n "$watcher" ]; then
		signal_sent=1
		if ! kill -s INT "$watcher" 2>/dev/null && kill -0 "$watcher" 2>/dev/null; then
			echo 'Error: could not interrupt the development watcher; waiting for it to exit.' >&2
		fi
	fi
}
forward_signal() {
	if [ "$interrupted" -eq 0 ]; then
		interrupted=$1
		if [ -z "$watcher" ] && [ "$starting_watcher" -eq 0 ]; then
			exit "$interrupted"
		fi
		echo 'Stopping the development watcher; waiting for it to exit...' >&2
		signal_watcher
	fi
}
trap cleanup 0
trap 'forward_signal 130' INT
trap 'forward_signal 143' TERM

mkdir -p build
if ! mkdir build/.build-lock 2>/dev/null; then
	echo 'Error: another build may be running, or build/.build-lock is stale.' >&2
	exit 1
fi
build_lock=1
if ! mkdir build/.dev-lock 2>/dev/null; then
	echo 'Error: development mode may already be running, or build/.dev-lock is stale.' >&2
	exit 1
fi
dev_lock=1
rmdir -- build/.build-lock
build_lock=0

for tool in go npm gcc g++ pkg-config; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "Error: $tool was not found in the project environment." >&2
		exit 1
	fi
done
if ! pkg-config --exists gtk4 webkitgtk-6.0; then
	echo 'Error: GTK4 and WebKitGTK 6.0 development files are required in the project environment.' >&2
	exit 1
fi

libgcc_file=$(readlink -f "$(gcc -print-file-name=libgcc_s.so.1)")
libgcc_dir=$(dirname "$libgcc_file")
export LD_LIBRARY_PATH="$libgcc_dir${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

unset GOROOT
export CC=gcc CXX=g++ CGO_CFLAGS= CGO_CPPFLAGS= CGO_CXXFLAGS= CGO_LDFLAGS= CGO_ENABLED=1 GOTOOLCHAIN=auto
. scripts/lib/wails.sh
ensure_wails
generate_bindings
mkdir -p build/watch

(
	cd internal/gui/frontend
	npm ci
)

starting_watcher=1
# Background jobs can inherit SIGINT ignored; Guix coreutils resets it for Wails.
env --default-signal=INT ./build/tools/wails3 dev -config scripts/wails-dev.yml -port 9245 &
watcher=$!
starting_watcher=0
if [ "$interrupted" -ne 0 ]; then
	signal_watcher
fi
while :; do
	if wait "$watcher"; then
		status=0
	else
		status=$?
	fi
	# A trapped signal can interrupt wait before the watcher exits.
	if [ "$interrupted" -eq 0 ] || ! kill -0 "$watcher" 2>/dev/null; then
		break
	fi
done
watcher=
if [ "$interrupted" -ne 0 ]; then
	exit "$interrupted"
fi
exit "$status"
