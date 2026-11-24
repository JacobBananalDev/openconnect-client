#!/bin/sh
set -eu

cd "$(dirname "$0")/../.."
temp_dir=$(mktemp -d)
trap 'rm -f -- "$temp_dir/go" "$temp_dir/args" "$temp_dir/expected" "$temp_dir/output"; rmdir -- "$temp_dir"' EXIT

cat > "$temp_dir/go" <<'EOF'
#!/bin/sh
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

export TEST_GO_STATUS=42
if ./run.ysh unit test > "$temp_dir/output" 2>&1; then
	echo 'failed test command returned success' >&2
	exit 1
else
	status=$?
	test "$status" -eq 42
fi

echo 'Unit runner checks passed'
