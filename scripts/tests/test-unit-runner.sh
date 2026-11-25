#!/bin/sh
set -eu

cd "$(dirname "$0")/../.."
repo_root=$(pwd)
temp_dir=$(mktemp -d)
fixture="$temp_dir/project"
cleanup() {
	rm -f -- "$temp_dir/go" "$temp_dir/args" "$temp_dir/expected" "$temp_dir/output" \
		"$fixture/run.ysh" "$fixture/scripts/lib/taskfile.ysh" "$fixture/main.go" \
		"$fixture/dir with spaces/other.go" "$fixture/.go/invalid.go" "$fixture/invalid.go"
	if [ -d "$fixture/.go" ]; then rmdir "$fixture/.go"; fi
	if [ -d "$fixture/dir with spaces" ]; then rmdir "$fixture/dir with spaces"; fi
	if [ -d "$fixture/scripts/lib" ]; then rmdir "$fixture/scripts/lib"; fi
	if [ -d "$fixture/scripts" ]; then rmdir "$fixture/scripts"; fi
	if [ -d "$fixture" ]; then rmdir "$fixture"; fi
	rmdir -- "$temp_dir"
}
trap cleanup 0

cat > "$temp_dir/go" <<'EOF'
#!/bin/sh
if [ "$1" = -C ] && [ "$3" = version ]; then exit 0; fi
printf '%s\n' "$@" > "$TEST_GO_ARGS"
exit "${TEST_GO_STATUS:-0}"
EOF
chmod +x "$temp_dir/go"
export PATH="$temp_dir:$PATH"
export TEST_GO_ARGS="$temp_dir/args"
export OPENCONNECT_CLIENT_ENV=1

./run.ysh unit test
printf '%s\n' test -v -cover ./... > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_GO_ARGS"

./run.ysh unit-test --race --count 3 -- ./internal/vpn -run 'Test name'
printf '%s\n' test -v -cover -race -count=3 ./internal/vpn -run 'Test name' > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_GO_ARGS"

./run.ysh unit test -r -c 2
printf '%s\n' test -v -cover -race -count=2 ./... > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_GO_ARGS"

./run.ysh tidy
printf '%s\n' mod tidy > "$temp_dir/expected"
diff -u "$temp_dir/expected" "$TEST_GO_ARGS"

rm -f -- "$TEST_GO_ARGS"
if ./run.ysh tidy unexpected > "$temp_dir/output" 2>&1; then
	echo 'tidy accepted an unexpected argument' >&2
	exit 1
else
	test "$?" -eq 2
fi
test ! -e "$TEST_GO_ARGS"
grep -q 'Usage: ./run.ysh tidy' "$temp_dir/output"
OPENCONNECT_CLIENT_ENV=0 ./run.ysh tidy --help > "$temp_dir/output"
grep -q 'go mod tidy' "$temp_dir/output"
test ! -e "$TEST_GO_ARGS"

for count in '' 0 -1 01 invalid; do
	rm -f -- "$TEST_GO_ARGS"
	if ./run.ysh unit test --count "$count" > "$temp_dir/output" 2>&1; then
		printf 'invalid count accepted: %s\n' "$count" >&2
		exit 1
	fi
	test ! -e "$TEST_GO_ARGS"
	grep -q 'requires a positive' "$temp_dir/output"
done

for option in --count --unknown; do
	rm -f -- "$TEST_GO_ARGS"
	if ./run.ysh unit test "$option" > "$temp_dir/output" 2>&1; then
		printf 'invalid option accepted: %s\n' "$option" >&2
		exit 1
	fi
	test ! -e "$TEST_GO_ARGS"
done

rm -f -- "$TEST_GO_ARGS"
if ./run.ysh unit unknown > "$temp_dir/output" 2>&1; then
	echo 'unknown unit subcommand accepted' >&2
	exit 1
fi
test ! -e "$TEST_GO_ARGS"
grep -q 'Usage: ./run.ysh unit test' "$temp_dir/output"

./run.ysh unit test --help > "$temp_dir/output"
grep -q -- '--race' "$temp_dir/output"
grep -q -- '--count' "$temp_dir/output"
test ! -e "$TEST_GO_ARGS"

if OPENCONNECT_CLIENT_ENV=0 ./run.ysh unit test > "$temp_dir/output" 2>&1; then
	echo 'inactive project environment accepted' >&2
	exit 1
fi
test ! -e "$TEST_GO_ARGS"
grep -q 'project environment is not active' "$temp_dir/output"

mkdir -p "$fixture/scripts/lib" "$fixture/.go" "$fixture/dir with spaces"
cp "$repo_root/run.ysh" "$fixture/run.ysh"
cp "$repo_root/scripts/lib/taskfile.ysh" "$fixture/scripts/lib/taskfile.ysh"
printf 'package main\nfunc main(){println("hello")}\n' > "$fixture/main.go"
printf 'package main\nfunc other(){println("hello")}\n' > "$fixture/dir with spaces/other.go"
printf 'invalid go source\n' > "$fixture/.go/invalid.go"
(
	cd "$fixture"
	./run.ysh fmt > "$temp_dir/output" 2>&1
)
test -z "$(gofmt -l "$fixture/main.go" "$fixture/dir with spaces/other.go")"
printf 'invalid go source\n' > "$temp_dir/expected"
cmp "$temp_dir/expected" "$fixture/.go/invalid.go"

printf 'invalid go source\n' > "$fixture/invalid.go"
if (cd "$fixture" && ./run.ysh fmt > "$temp_dir/output" 2>&1); then
	echo 'fmt accepted invalid project Go source' >&2
	exit 1
fi
grep -q 'invalid.go' "$temp_dir/output"
if grep -q 'Code formatted' "$temp_dir/output"; then
	echo 'fmt reported success after a formatting error' >&2
	exit 1
fi

export TEST_GO_STATUS=42
if ./run.ysh unit test > "$temp_dir/output" 2>&1; then
	echo 'failed test command returned success' >&2
	exit 1
else
	status=$?
	test "$status" -eq 42
fi

if ./run.ysh tidy > "$temp_dir/output" 2>&1; then
	echo 'failed tidy command returned success' >&2
	exit 1
else
	test "$?" -eq 42
fi

echo 'Unit runner checks passed'
