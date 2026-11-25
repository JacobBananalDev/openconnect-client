#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
temp_dir=$(mktemp -d)
fixture=$temp_dir/project
tools=$temp_dir/tools
real_mv=$(command -v mv)
cleanup() {
	rm -f -- "$fixture/run.ysh" "$fixture/scripts/build.sh" "$fixture/scripts/lib/taskfile.ysh" \
		"$fixture/internal/gui/web/dist/index.html" "$fixture/build/occlient" \
		"$fixture/build/occlientd" "$fixture/build/occlient-gui" "$fixture/build/tools/wails3" \
		"$tools/go" "$tools/npm" "$tools/pkg-config" "$tools/mv" "$temp_dir/log" "$temp_dir/output" \
		"$temp_dir/minimal/dirname" "$temp_dir/minimal/mkdir" "$temp_dir/minimal/rm" \
		"$temp_dir/minimal/rmdir" "$temp_dir/minimal/mktemp" "$temp_dir/minimal/go"
	rmdir -- "$fixture/scripts/lib" "$fixture/scripts" "$fixture/internal/gui/frontend" \
		"$fixture/internal/gui/web/dist" "$fixture/internal/gui/web" "$fixture/internal/gui" \
		"$fixture/internal" "$fixture/build/tools" "$fixture/build" "$fixture" "$tools" "$temp_dir/minimal" "$temp_dir"
}
trap cleanup 0

mkdir -p "$fixture/scripts/lib" "$fixture/internal/gui/frontend" "$fixture/internal/gui/web/dist" \
	"$fixture/build/tools" "$tools" "$temp_dir/minimal"
cp -- "$repo_root/run.ysh" "$fixture/run.ysh"
cp -- "$repo_root/scripts/build.sh" "$fixture/scripts/build.sh"
cp -- "$repo_root/scripts/lib/taskfile.ysh" "$fixture/scripts/lib/taskfile.ysh"
touch "$fixture/build/tools/wails3"

cat > "$tools/npm" <<'EOF'
#!/bin/sh
set -eu
printf 'npm %s\n' "$*" >> "$TEST_LOG"
if [ "${TEST_FAILURE:-}" = "npm-$1" ] || { [ "${TEST_FAILURE:-}" = "npm-build" ] && [ "$*" = "run build" ]; }; then
	exit 23
fi
if [ "$*" = "run build" ]; then
	printf '<html></html>\n' > ../web/dist/index.html
fi
EOF
cat > "$tools/go" <<'EOF'
#!/bin/sh
set -eu
printf 'go %s\n' "$*" >> "$TEST_LOG"
[ "${GOROOT+x}" != x ] && [ "$CC" = gcc ] && [ "$CXX" = g++ ] &&
	[ "$CGO_CFLAGS" = '' ] && [ "$CGO_CPPFLAGS" = '' ] &&
	[ "$CGO_CXXFLAGS" = '' ] && [ "$CGO_LDFLAGS" = '' ] &&
	[ "$CGO_ENABLED" = 1 ] && [ "$GOTOOLCHAIN" = auto ]
if [ "$2" = -tags ]; then
	[ "$3" = production ]
	shift 2
fi
name=${4##*/}
if [ "${TEST_FAILURE:-}" = "go-$name" ]; then
	exit 24
fi
if [ "${TEST_FAILURE:-}" = "interrupt-$name" ]; then
	kill -TERM "$PPID"
	exit 0
fi
if [ "${TEST_FAILURE:-}" != "missing-$name" ]; then
	printf '#!/bin/sh\nexit 0\n' > "$3"
	if [ "${TEST_FAILURE:-}" != "nonexec-$name" ]; then
		chmod +x "$3"
	fi
fi
EOF
cat > "$tools/pkg-config" <<'EOF'
#!/bin/sh
set -eu
printf 'pkg-config %s\n' "$*" >> "$TEST_LOG"
[ "${TEST_FAILURE:-}" != pkg-config ]
EOF
cat > "$tools/mv" <<'EOF'
#!/bin/sh
set -eu
if [ "${TEST_FAILURE:-}" = publish ] && [ "$3" = build/occlientd ]; then
	exit 25
fi
exec "$TEST_REAL_MV" "$@"
EOF
chmod +x "$tools/npm" "$tools/go" "$tools/pkg-config" "$tools/mv"
export PATH="$tools:$PATH" OPENCONNECT_CLIENT_ENV=1 TEST_LOG="$temp_dir/log" TEST_REAL_MV="$real_mv"
export GOROOT=/invalid CC=/invalid CXX=/invalid CGO_CFLAGS=invalid CGO_CPPFLAGS=invalid CGO_CXXFLAGS=invalid CGO_LDFLAGS=invalid
cd "$fixture"

assert_no_binaries() {
	for name in occlient occlientd occlient-gui; do
		if [ -e "build/$name" ]; then
			echo "unexpected build/$name after failed build" >&2
			exit 1
		fi
	done
	[ ! -e build/.build-lock ]
	set -- build/.build-stage.*
	if [ -e "$1" ]; then
		echo 'staging directory left behind' >&2
		exit 1
	fi
	[ -e build/tools/wails3 ]
}

if ! ./run.ysh build > "$temp_dir/output" 2>&1; then
	cat "$temp_dir/output" >&2
	exit 1
fi
for name in occlient occlientd occlient-gui; do
	[ -f "build/$name" ] && [ -x "build/$name" ]
done
grep -q 'npm ci' "$TEST_LOG"
grep -q 'npm run build' "$TEST_LOG"
grep -q 'go build -tags production -o .* ./cmd/occlient-gui' "$TEST_LOG"
grep -q 'Built: build/occlient, build/occlientd and build/occlient-gui' "$temp_dir/output"

for failure in pkg-config npm-ci npm-build go-occlientd missing-occlient-gui nonexec-occlient publish interrupt-occlientd; do
	for name in occlient occlientd occlient-gui; do
		printf 'old binary\n' > "build/$name"
	done
	if TEST_FAILURE=$failure ./run.ysh build > "$temp_dir/output" 2>&1; then
		echo "build unexpectedly succeeded: $failure" >&2
		exit 1
	else
		status=$?
	fi
	if [ "$failure" = interrupt-occlientd ] && [ "$status" -ne 143 ]; then
		echo "unexpected interrupt status: $status" >&2
		exit 1
	fi
	grep -q 'build failed' "$temp_dir/output"
	if grep -q '^Built:' "$temp_dir/output"; then
		echo "success reported after failure: $failure" >&2
		exit 1
	fi
	assert_no_binaries
done

for tool in dirname mkdir rm rmdir mktemp; do
	ln -s "$(command -v "$tool")" "$temp_dir/minimal/$tool"
done
ln -s "$tools/go" "$temp_dir/minimal/go"
if PATH="$temp_dir/minimal" /bin/sh scripts/build.sh > "$temp_dir/output" 2>&1; then
	echo 'build unexpectedly accepted missing npm' >&2
	exit 1
fi
grep -q 'npm was not found' "$temp_dir/output"
assert_no_binaries

mkdir build/.build-lock
if ./run.ysh build > "$temp_dir/output" 2>&1; then
	echo 'build unexpectedly accepted an existing lock' >&2
	exit 1
fi
grep -q 'another build may be running' "$temp_dir/output"
rmdir build/.build-lock

mkdir build/.dev-lock
if ./run.ysh build > "$temp_dir/output" 2>&1; then
	echo 'build unexpectedly accepted a development lock' >&2
	exit 1
fi
grep -q 'development mode may be running' "$temp_dir/output"
[ ! -e build/.build-lock ]
rmdir build/.dev-lock

if OPENCONNECT_CLIENT_ENV=0 ./run.ysh build > "$temp_dir/output" 2>&1; then
	echo 'build unexpectedly accepted an inactive environment' >&2
	exit 1
fi
grep -q 'project environment is not active' "$temp_dir/output"

echo 'Build runner checks passed'
