#!/bin/bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
temp_dir=$(mktemp -d)
fixture=$temp_dir/project
child_pid=
cleanup() {
	if [[ -n $child_pid ]] && kill -0 "$child_pid" 2>/dev/null; then
		kill "$child_pid" 2>/dev/null || true
		wait "$child_pid" 2>/dev/null || true
	fi
	rm -f -- "$fixture/run.ysh" "$fixture/scripts/run-gui.sh" "$fixture/scripts/lib/taskfile.ysh" "$fixture/scripts/lib/gui-env.sh" \
		"$fixture/build/occlient" "$fixture/build/occlientd" "$fixture/build/occlient-gui" \
		"$temp_dir/fake-binary" "$temp_dir/args" "$temp_dir/expected" \
		"$temp_dir/stdout" "$temp_dir/stderr" "$temp_dir/locale" "$temp_dir/ready" \
		"$temp_dir/signal" "$temp_dir/locale-link"
	if [[ -d $fixture/build/.build-lock ]]; then
		rmdir -- "$fixture/build/.build-lock"
	fi
	rmdir -- "$fixture/scripts/lib" "$fixture/scripts" "$fixture/build" "$fixture" "$temp_dir/locale-dir" "$temp_dir"
}
trap cleanup EXIT

mkdir -p "$fixture/scripts/lib" "$fixture/build" "$temp_dir/locale-dir"
cp -- "$repo_root/run.ysh" "$fixture/run.ysh"
cp -- "$repo_root/scripts/run-gui.sh" "$fixture/scripts/run-gui.sh"
cp -- "$repo_root/scripts/lib/taskfile.ysh" "$fixture/scripts/lib/taskfile.ysh"
cp -- "$repo_root/scripts/lib/gui-env.sh" "$fixture/scripts/lib/gui-env.sh"
ln -s "$temp_dir/locale-dir" "$temp_dir/locale-link"
cat > "$temp_dir/fake-binary" <<'EOF'
#!/bin/bash
set -eu
printf '%s\n' "${0##*/}" "$@" > "$TEST_ARGS"
if [[ ${GUIX_LOCPATH+x} ]]; then
	printf 'SET:%s\n' "$GUIX_LOCPATH" > "$TEST_LOCALE"
else
	echo UNSET > "$TEST_LOCALE"
fi
if [[ ${TEST_WAIT:-0} == 1 ]]; then
	trap 'echo TERM > "$TEST_SIGNAL"; exit 43' TERM
	trap 'echo INT > "$TEST_SIGNAL"; exit 44' INT
	echo "$$" > "$TEST_READY"
	while :; do
		sleep 1 &
		wait "$!" || true
	done
fi
echo 'target stdout'
echo 'target stderr' >&2
exit "${TEST_STATUS:-0}"
EOF
chmod +x "$temp_dir/fake-binary"
for name in occlient occlientd occlient-gui; do
	cp -- "$temp_dir/fake-binary" "$fixture/build/$name"
done
export OPENCONNECT_CLIENT_ENV=1 TEST_ARGS="$temp_dir/args"
export TEST_LOCALE="$temp_dir/locale" TEST_READY="$temp_dir/ready" TEST_SIGNAL="$temp_dir/signal"
cd "$fixture"

assert_call() {
	local binary=$1
	local subcommand=$2
	shift 2
	./run.ysh run "$@" > "$temp_dir/stdout" 2> "$temp_dir/stderr"
	if [[ -n $subcommand ]]; then
		printf '%s\n' "$binary" "$subcommand" > "$temp_dir/expected"
	else
		printf '%s\n' "$binary" > "$temp_dir/expected"
	fi
	diff -u "$temp_dir/expected" "$TEST_ARGS"
	grep -qx 'target stdout' "$temp_dir/stdout"
	grep -qx 'target stderr' "$temp_dir/stderr"
}

assert_call occlient-gui ''
assert_call occlient-gui '' --gui
for selector in -c --connect connect; do assert_call occlient connect "$selector"; done
for selector in -d --disconnect disconnect; do assert_call occlient disconnect "$selector"; done
for selector in -s --status status; do assert_call occlient status "$selector"; done
for selector in -v --version version; do assert_call occlient --version "$selector"; done
assert_call occlient help help
assert_call occlientd '' --daemon

./run.ysh run -c --server 'vpn example' --protocol pulse --background '' -- --status > "$temp_dir/stdout" 2> "$temp_dir/stderr"
printf '%s\n' occlient connect --server 'vpn example' --protocol pulse --background '' -- --status > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_ARGS"
./run.ysh run -c -- --status > "$temp_dir/stdout" 2> "$temp_dir/stderr"
printf '%s\n' occlient connect --status > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_ARGS"
./run.ysh run -c -- -- --status > "$temp_dir/stdout" 2> "$temp_dir/stderr"
printf '%s\n' occlient connect -- --status > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_ARGS"
./run.ysh run -c --server --status > "$temp_dir/stdout" 2> "$temp_dir/stderr"
printf '%s\n' occlient connect --server --status > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_ARGS"
./run.ysh run --status --help > "$temp_dir/stdout" 2> "$temp_dir/stderr"
printf '%s\n' occlient status --help > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_ARGS"

for selector in --unknown -x --; do
	if ./run.ysh run "$selector" > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
		echo "unknown selector accepted: $selector" >&2
		exit 1
	fi
	grep -q 'Unknown run selector' "$temp_dir/stderr"
done
for selector in -c --connect connect -d --disconnect disconnect -s --status status --daemon --gui -v --version version; do
	if ./run.ysh run -c "$selector" > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
		echo "conflicting selector accepted: $selector" >&2
		exit 1
	fi
	grep -q 'Conflicting run selectors' "$temp_dir/stderr"
done

for name in occlient occlientd occlient-gui; do
	case $name in
		occlient) selector=--status ;;
		occlientd) selector=--daemon ;;
		occlient-gui) selector=--gui ;;
	esac
	rm -f -- "build/$name"
	if ./run.ysh run "$selector" > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
		echo "missing binary accepted: $name" >&2
		exit 1
	fi
	grep -q 'Run ./run.ysh build first' "$temp_dir/stderr"
	cp -- "$temp_dir/fake-binary" "build/$name"
	chmod -x "build/$name"
	if ./run.ysh run "$selector" > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
		echo "non-executable binary accepted: $name" >&2
		exit 1
	fi
	grep -q 'Run ./run.ysh build first' "$temp_dir/stderr"
	chmod +x "build/$name"
done

mkdir build/.build-lock
if ./run.ysh run --status > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'run accepted a build lock' >&2
	exit 1
fi
grep -q 'build may be running' "$temp_dir/stderr"
OPENCONNECT_CLIENT_ENV=0 ./run.ysh run --help > "$temp_dir/stdout"
grep -q -- '--connect' "$temp_dir/stdout"
rmdir build/.build-lock

if OPENCONNECT_CLIENT_ENV=0 ./run.ysh run --status > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'run accepted an inactive environment' >&2
	exit 1
fi
grep -q 'project environment is not active' "$temp_dir/stderr"
rm -f -- build/occlient
OPENCONNECT_CLIENT_ENV=0 ./run.ysh run --help > "$temp_dir/stdout"
grep -q -- '--connect' "$temp_dir/stdout"
OPENCONNECT_CLIENT_ENV=0 ./run.ysh run -h > "$temp_dir/stdout"
grep -q -- '--daemon' "$temp_dir/stdout"
cp -- "$temp_dir/fake-binary" build/occlient

if [[ $EUID == 0 ]]; then
	root_command=()
elif command -v unshare >/dev/null 2>&1 && unshare --user --map-root-user true 2>/dev/null; then
	root_command=(unshare --user --map-root-user)
else
	root_command=()
	echo 'Skipping GUI root guard test: unprivileged user namespaces unavailable' >&2
fi
if [[ $EUID == 0 || ${#root_command[@]} -gt 0 ]]; then
	if "${root_command[@]}" ./run.ysh run > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
		echo 'root GUI launch accepted' >&2
		exit 1
	fi
	grep -q 'unprivileged user' "$temp_dir/stderr"
fi

GUIX_LOCPATH="$temp_dir/locale-link::$temp_dir/locale-dir:" ./run.ysh run --gui > "$temp_dir/stdout" 2> "$temp_dir/stderr"
printf 'SET:%s::%s:\n' "$temp_dir/locale-dir" "$temp_dir/locale-dir" > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_LOCALE"
GUIX_LOCPATH="$temp_dir/locale-link" ./run.ysh run --version > "$temp_dir/stdout" 2> "$temp_dir/stderr"
printf 'SET:%s\n' "$temp_dir/locale-link" > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_LOCALE"
env -u GUIX_LOCPATH ./run.ysh run --gui > "$temp_dir/stdout" 2> "$temp_dir/stderr"
grep -qx UNSET "$TEST_LOCALE"
GUIX_LOCPATH= ./run.ysh run --gui > "$temp_dir/stdout" 2> "$temp_dir/stderr"
grep -qx 'SET:' "$TEST_LOCALE"

if TEST_STATUS=42 ./run.ysh run --status > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'nonzero target exit was lost' >&2
	exit 1
else
	[[ $? == 42 ]]
fi
grep -qx 'target stdout' "$temp_dir/stdout"
grep -qx 'target stderr' "$temp_dir/stderr"

for spec in '--daemon TERM 43' '--gui INT 44'; do
	read -r selector signal expected_status <<< "$spec"
	rm -f -- "$TEST_READY" "$TEST_SIGNAL"
	set -m
	TEST_WAIT=1 ./run.ysh run "$selector" > "$temp_dir/stdout" 2> "$temp_dir/stderr" &
	child_pid=$!
	set +m
	for _ in {1..100}; do
		[[ -s $TEST_READY ]] && break
		sleep 0.05
	done
	[[ -s $TEST_READY ]] || { echo "target did not start: $selector" >&2; exit 1; }
	[[ $(< "$TEST_READY") == "$child_pid" ]] || { echo 'target did not replace wrapper process' >&2; exit 1; }
	kill -s "$signal" "$child_pid"
	if wait "$child_pid"; then
		echo "signal handler status lost: $signal" >&2
		exit 1
	else
		[[ $? == "$expected_status" ]]
	fi
	child_pid=
	grep -qx "$signal" "$TEST_SIGNAL"
done

echo 'Run dispatcher checks passed'
