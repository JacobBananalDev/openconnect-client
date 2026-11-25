#!/bin/bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd "$script_dir/.."

if [[ ${OPENCONNECT_CLIENT_ENV:-} != 1 ]]; then
	echo 'Error: the project environment is not active. Run: ./scripts/shell.sh' >&2
	exit 1
fi

mkdir -p build
if ! mkdir build/.build-lock 2>/dev/null; then
	echo 'Error: another build may be running, or build/.build-lock is stale; existing binaries may be stale.' >&2
	exit 1
fi
if [[ -e build/.dev-lock ]]; then
	rmdir -- build/.build-lock
	echo 'Error: development mode may be running, or build/.dev-lock is stale; stop it before building.' >&2
	exit 1
fi

stage=
success=0
cleanup() {
	local status=$?
	set +e
	if [[ -n $stage ]]; then
		rm -f -- "$stage/occlient" "$stage/occlientd" "$stage/occlient-gui" || status=1
		rmdir -- "$stage" || status=1
	fi
	rmdir -- build/.build-lock || status=1
	if [[ $success == 1 && $status == 0 ]]; then
		echo 'Built: build/occlient, build/occlientd and build/occlient-gui'
	else
		if ! rm -f -- build/occlient build/occlientd build/occlient-gui; then
			echo 'Error: could not remove all incomplete project binaries.' >&2
		fi
		echo 'Error: build failed; run ./run.ysh build again before using project binaries.' >&2
		if [[ $status == 0 ]]; then
			status=1
		fi
	fi
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

rm -f -- build/occlient build/occlientd build/occlient-gui
stage=$(mktemp -d build/.build-stage.XXXXXXXX)

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

(
	cd internal/gui/frontend
	npm ci
	npm run build
)
if [[ ! -s internal/gui/web/dist/index.html ]]; then
	echo 'Error: frontend build did not produce an index.html.' >&2
	exit 1
fi

for name in occlient occlientd occlient-gui; do
	tags=()
	if [[ $name == occlient-gui ]]; then
		tags=(-tags production)
	fi
	env -u GOROOT CC=gcc CXX=g++ CGO_CFLAGS= CGO_CPPFLAGS= CGO_CXXFLAGS= CGO_LDFLAGS= CGO_ENABLED=1 GOTOOLCHAIN=local \
		go build "${tags[@]}" -o "$stage/$name" "./cmd/$name"
	if [[ ! -f $stage/$name || ! -x $stage/$name ]]; then
		echo "Error: build did not produce an executable $name." >&2
		exit 1
	fi
done

for name in occlient occlientd occlient-gui; do
	mv -- "$stage/$name" "build/$name"
done
for name in occlient occlientd occlient-gui; do
	if [[ ! -f build/$name || ! -x build/$name ]]; then
		echo "Error: published artifact build/$name is not executable." >&2
		exit 1
	fi
done
success=1
