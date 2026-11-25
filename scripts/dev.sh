#!/bin/bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd "$script_dir/.."

if [[ ${OPENCONNECT_CLIENT_ENV:-} != 1 ]]; then
	echo 'Error: the project environment is not active. Run: ./scripts/shell.sh' >&2
	exit 1
fi
source scripts/lib/gui-env.sh
prepare_gui_environment
if [[ -z ${DISPLAY:-} && -z ${WAYLAND_DISPLAY:-} ]]; then
	echo 'Error: a graphical display is required for development mode.' >&2
	exit 1
fi

build_lock=0
dev_lock=0
watcher=
interrupted=0
cleanup() {
	local status=$?
	set +e
	if [[ $dev_lock == 1 ]]; then
		rmdir -- build/.dev-lock || status=1
	fi
	if [[ $build_lock == 1 ]]; then
		rmdir -- build/.build-lock || status=1
	fi
	exit "$status"
}
forward_signal() {
	interrupted=$1
	if [[ -z $watcher ]]; then
		exit "$interrupted"
	fi
	if kill -0 "$watcher" 2>/dev/null; then
		kill -s INT "$watcher"
	fi
}
trap cleanup EXIT
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

unset GOROOT
export CC=gcc CXX=g++ CGO_CFLAGS= CGO_CPPFLAGS= CGO_CXXFLAGS= CGO_LDFLAGS= CGO_ENABLED=1 GOTOOLCHAIN=local
version=$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
if [[ -z $version ]]; then
	echo 'Error: unable to determine the Wails module version from go.mod.' >&2
	exit 1
fi
mkdir -p build/tools build/dev
if [[ -x build/tools/wails3 ]]; then
	if ! installed_version=$(./build/tools/wails3 version 2>&1); then
		echo "Error: could not determine the project-local Wails CLI version: $installed_version" >&2
		exit 1
	fi
else
	installed_version=
fi
if [[ $installed_version != "$version" ]]; then
	echo "Installing project-local Wails CLI $version..."
	GOBIN="$PWD/build/tools" go install "github.com/wailsapp/wails/v3/cmd/wails3@$version"
	if ! installed_version=$(./build/tools/wails3 version 2>&1); then
		echo "Error: could not determine the installed Wails CLI version: $installed_version" >&2
		exit 1
	fi
	if [[ $installed_version != "$version" ]]; then
		echo 'Error: the installed Wails CLI does not match go.mod.' >&2
		exit 1
	fi
fi

(
	cd internal/gui/frontend
	npm ci
)

set -m
./build/tools/wails3 dev -config scripts/wails-dev.yml -port 9245 &
watcher=$!
set +m
while :; do
	if wait "$watcher"; then
		status=0
	else
		status=$?
	fi
	if ! kill -0 "$watcher" 2>/dev/null; then
		break
	fi
done
watcher=
if [[ $interrupted != 0 ]]; then
	exit "$interrupted"
fi
exit "$status"
