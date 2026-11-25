#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
temp_dir=$(mktemp -d)
fixture=$temp_dir/project
tools=$temp_dir/tools
child_pid=
cleanup() {
	status=$?
	if [ "$status" -ne 0 ]; then
		[ ! -f "$temp_dir/stdout" ] || cat "$temp_dir/stdout" >&2
		[ ! -f "$temp_dir/stderr" ] || cat "$temp_dir/stderr" >&2
	fi
	if [ -n "$child_pid" ] && kill -0 "$child_pid" 2>/dev/null; then
		kill "$child_pid" 2>/dev/null || true
		wait "$child_pid" 2>/dev/null || true
	fi
	rm -f -- "$fixture/run.ysh" "$fixture/scripts/dev.sh" "$fixture/scripts/build.sh" \
		"$fixture/scripts/wails-dev.yml" "$fixture/scripts/lib/gui-env.sh" "$fixture/scripts/lib/taskfile.ysh" \
		"$fixture/build/tools/wails3" "$fixture/build/occlient" "$tools/go" "$tools/npm" "$tools/pkg-config" \
		"$temp_dir/wails3" "$temp_dir/log" "$temp_dir/stdout" "$temp_dir/stderr" \
		"$temp_dir/ready" "$temp_dir/signal" "$temp_dir/locale-link" \
		"$temp_dir/minimal/dirname" "$temp_dir/minimal/mkdir" "$temp_dir/minimal/rmdir" "$temp_dir/minimal/realpath" "$temp_dir/minimal/go"
	if [ -d "$fixture/build/.dev-lock" ]; then rmdir "$fixture/build/.dev-lock"; fi
	if [ -d "$fixture/build/.build-lock" ]; then rmdir "$fixture/build/.build-lock"; fi
	if [ -d "$fixture/.go" ]; then rmdir "$fixture/.go"; fi
	rmdir -- "$fixture/build/tools" "$fixture/build/dev" "$fixture/build" "$fixture/scripts/lib" "$fixture/scripts" \
		"$fixture/internal/gui/frontend" "$fixture/internal/gui" "$fixture/internal" "$fixture" \
		"$tools" "$temp_dir/minimal" "$temp_dir/locale-dir" "$temp_dir"
}
trap cleanup 0

mkdir -p "$fixture/scripts/lib" "$fixture/internal/gui/frontend" "$fixture/build/tools" "$fixture/build/dev" \
	"$tools" "$temp_dir/minimal" "$temp_dir/locale-dir"
cp -- "$repo_root/run.ysh" "$fixture/run.ysh"
cp -- "$repo_root/scripts/dev.sh" "$repo_root/scripts/build.sh" "$repo_root/scripts/wails-dev.yml" "$fixture/scripts/"
cp -- "$repo_root/scripts/lib/gui-env.sh" "$repo_root/scripts/lib/taskfile.ysh" "$fixture/scripts/lib/"
ln -s "$temp_dir/locale-dir" "$temp_dir/locale-link"

cat > "$tools/go" <<'EOF'
#!/bin/sh
set -eu
if [ "$1" = -C ] && [ "$3" = version ]; then exit 0; fi
printf 'go %s\n' "$*" >> "$TEST_LOG"
[ "${GOROOT+x}" != x ] && [ "$CC" = gcc ] && [ "$CXX" = g++ ] &&
	[ "$CGO_CFLAGS" = '' ] && [ "$CGO_CPPFLAGS" = '' ] &&
	[ "$CGO_CXXFLAGS" = '' ] && [ "$CGO_LDFLAGS" = '' ] &&
	[ "$CGO_ENABLED" = 1 ] && [ "$GOTOOLCHAIN" = auto ]
case $1 in
	list)
		[ "${TEST_FAILURE:-}" != go-list ] || exit 31
		echo "$TEST_VERSION"
		;;
	install)
		[ "${TEST_FAILURE:-}" != go-install ] || exit 32
		[ "$GOBIN" = "$TEST_PROJECT/build/tools" ]
		[ "$2" = "github.com/wailsapp/wails/v3/cmd/wails3@$TEST_VERSION" ]
		cp -- "$TEST_WAILS_TEMPLATE" "$GOBIN/wails3"
		;;
	*) exit 33 ;;
esac
EOF
cat > "$tools/npm" <<'EOF'
#!/bin/sh
set -eu
printf 'npm %s in %s\n' "$*" "$PWD" >> "$TEST_LOG"
[ "$*" = ci ] && [ "$PWD" = "$TEST_PROJECT/internal/gui/frontend" ]
[ "${TEST_FAILURE:-}" != npm ]
EOF
cat > "$tools/pkg-config" <<'EOF'
#!/bin/sh
[ "${TEST_FAILURE:-}" != pkg-config ]
EOF
cat > "$temp_dir/wails3" <<'EOF'
#!/bin/sh
set -eu
if [ "$1" = version ]; then
	echo "$TEST_VERSION" >&2
	exit
fi
printf 'wails %s\n' "$*" >> "$TEST_LOG"
printf 'locale %s\n' "${GUIX_LOCPATH-unset}" >> "$TEST_LOG"
if [ "${TEST_FAILURE:-}" = watcher ]; then exit 37; fi
if [ "${TEST_WAIT:-0}" = 1 ]; then
	trap 'echo INT > "$TEST_SIGNAL"; exit 0' INT
	echo "$$" > "$TEST_READY"
	while :; do
		sleep 1 &
		wait "$!" || true
	done
fi
EOF
chmod +x "$tools/go" "$tools/npm" "$tools/pkg-config" "$temp_dir/wails3"
export PATH="$tools:$PATH" OPENCONNECT_CLIENT_ENV=1 DISPLAY=:test TEST_PROJECT="$fixture"
export TEST_LOG="$temp_dir/log" TEST_WAILS_TEMPLATE="$temp_dir/wails3" TEST_VERSION=v0.0.2
export TEST_READY="$temp_dir/ready" TEST_SIGNAL="$temp_dir/signal"
export GOROOT=/invalid CC=/invalid CXX=/invalid CGO_CFLAGS=invalid CGO_CPPFLAGS=invalid CGO_CXXFLAGS=invalid CGO_LDFLAGS=invalid
cd "$fixture"

assert_unlocked() {
	[ ! -e build/.build-lock ] && [ ! -e build/.dev-lock ]
}

GUIX_LOCPATH="$temp_dir/locale-link" ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"
grep -q 'go install github.com/wailsapp/wails/v3/cmd/wails3@v0.0.2' "$TEST_LOG"
grep -q 'npm ci in ' "$TEST_LOG"
grep -q 'wails dev -config scripts/wails-dev.yml -port 9245' "$TEST_LOG"
grep -q "locale $temp_dir/locale-dir" "$TEST_LOG"
assert_unlocked

: > "$TEST_LOG"
./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"
if grep -q '^go install ' "$TEST_LOG"; then
	echo 'matching local Wails CLI was reinstalled' >&2
	exit 1
fi
assert_unlocked

printf '#!/bin/sh\necho v0.0.1 >&2\n' > build/tools/wails3
chmod +x build/tools/wails3
: > "$TEST_LOG"
./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"
grep -q 'go install github.com/wailsapp/wails/v3/cmd/wails3@v0.0.2' "$TEST_LOG"
assert_unlocked

for failure in go-list npm pkg-config watcher; do
	if TEST_FAILURE=$failure ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
		echo "development failure was ignored: $failure" >&2
		exit 1
	else
		status=$?
	fi
	if [ "$failure" = watcher ] && [ "$status" -ne 37 ]; then
		echo "watcher exit status was lost: $status" >&2
		exit 1
	fi
	assert_unlocked
done

rm -f -- build/tools/wails3
if TEST_FAILURE=go-install ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'failed Wails installation was ignored' >&2
	exit 1
fi
assert_unlocked
cp -- "$temp_dir/wails3" build/tools/wails3

mkdir build/.build-lock
if ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'development accepted an existing build lock' >&2
	exit 1
fi
grep -q 'build may be running' "$temp_dir/stderr"
[ ! -e build/.dev-lock ]
rmdir build/.build-lock

mkdir build/.dev-lock
printf 'existing binary\n' > build/occlient
if ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'development accepted an existing development lock' >&2
	exit 1
fi
grep -q 'development mode may already be running' "$temp_dir/stderr"
[ ! -e build/.build-lock ]
if ./run.ysh build > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'build accepted an existing development lock' >&2
	exit 1
fi
grep -q 'development mode may be running' "$temp_dir/stderr"
grep -qx 'existing binary' build/occlient
rmdir build/.dev-lock

if OPENCONNECT_CLIENT_ENV=0 ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'development accepted an inactive environment' >&2
	exit 1
fi
grep -q 'project environment is not active' "$temp_dir/stderr"
if DISPLAY= WAYLAND_DISPLAY= ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'development accepted a missing display' >&2
	exit 1
fi
grep -q 'graphical display is required' "$temp_dir/stderr"
if ./run.ysh dev unexpected > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'development accepted an unexpected argument' >&2
	exit 1
fi
grep -q 'Usage: ./run.ysh dev' "$temp_dir/stderr"
OPENCONNECT_CLIENT_ENV=0 ./run.ysh dev --help > "$temp_dir/stdout"
grep -q 'development reloads' "$temp_dir/stdout"

if command -v unshare >/dev/null 2>&1 && unshare --user --map-root-user true 2>/dev/null; then
	if unshare --user --map-root-user ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
		echo 'development accepted effective root' >&2
		exit 1
	fi
	grep -q 'unprivileged user' "$temp_dir/stderr"
else
	echo 'Skipping development root guard test: unprivileged user namespaces unavailable' >&2
fi
assert_unlocked

for tool in dirname mkdir rmdir realpath; do ln -s "$(command -v "$tool")" "$temp_dir/minimal/$tool"; done
ln -s "$tools/go" "$temp_dir/minimal/go"
if PATH="$temp_dir/minimal" /bin/sh scripts/dev.sh > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'development accepted missing npm' >&2
	exit 1
fi
grep -q 'npm was not found' "$temp_dir/stderr"
assert_unlocked

for spec in 'TERM 143' 'INT 130'; do
	set -- $spec
	signal=$1
	expected_status=$2
	rm -f -- "$TEST_READY" "$TEST_SIGNAL"
	TEST_WAIT=1 ./run.ysh dev > "$temp_dir/stdout" 2> "$temp_dir/stderr" &
	child_pid=$!
	i=0
	while [ "$i" -lt 100 ]; do
		[ -s "$TEST_READY" ] && break
		sleep 0.05
		i=$((i + 1))
	done
	[ -s "$TEST_READY" ] || { echo 'development watcher did not start' >&2; exit 1; }
	[ -e build/.dev-lock ] && [ ! -e build/.build-lock ]
	kill -s "$signal" "$child_pid"
	if wait "$child_pid"; then
		echo "development signal status lost: $signal" >&2
		exit 1
	else
		[ "$?" -eq "$expected_status" ]
	fi
	child_pid=
	assert_unlocked
done

echo 'Development runner checks passed'
