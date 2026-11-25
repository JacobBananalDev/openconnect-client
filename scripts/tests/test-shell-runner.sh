#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
temp_dir=$(mktemp -d)
host_dir="$temp_dir/host tools"
cleanup() {
	rm -f -- "$host_dir/code" "$host_dir/codium" "$host_dir/cursor" \
		"$host_dir/openconnect" "$host_dir/go" "$host_dir/gcc" \
		"$temp_dir/stdout" "$temp_dir/stderr"
	rmdir -- "$host_dir" "$temp_dir"
}
trap cleanup 0

if [ -n "${GUIX_ENVIRONMENT:-}" ]; then
	echo 'Run this test from outside any Guix environment.' >&2
	exit 1
fi

mkdir "$host_dir"
for tool in code codium cursor openconnect go gcc; do
	cat > "$host_dir/$tool" <<'EOF'
#!/bin/sh
printf 'host-tool:%s:%s\n' "${0##*/}" "$*"
EOF
	chmod +x "$host_dir/$tool"
done

cd "$repo_root"
if GUIX_ENVIRONMENT=/already-active ./scripts/shell.sh > "$temp_dir/stdout" 2> "$temp_dir/stderr"; then
	echo 'Nested Guix environment was accepted.' >&2
	exit 1
fi
grep -q 'a Guix environment is already active' "$temp_dir/stderr"

if ! env PATH="$host_dir:$PATH" GOROOT=/conflicting/go CC=/conflicting/cc \
	CXX=/conflicting/cxx CGO_CFLAGS=-conflicting CGO_LDFLAGS=-conflicting \
	GOENV=/conflicting/goenv GOTOOLCHAIN=local DISPLAY=:987 \
	LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 \
	VISUAL="$host_dir/code" OPENCONNECT_PATH=/custom/openconnect \
	./scripts/shell.sh > "$temp_dir/stdout" 2> "$temp_dir/stderr" <<'EOF'
set -eu
test "$OPENCONNECT_CLIENT_ENV" = 1
test "$GOENV" = off
test "$GOTOOLCHAIN" = auto
test "$GOPATH" = "$PWD/.go"
test "$GOMODCACHE" = "$PWD/.go/pkg/mod"
test "${GOROOT-unset}" = unset
test "${CC-unset}" = unset
test "${CXX-unset}" = unset
test "${CGO_CFLAGS-unset}" = unset
test "${CGO_LDFLAGS-unset}" = unset
test "$DISPLAY" = :987
test "$LANG" = en_US.UTF-8
test "$(locale charmap)" = UTF-8
test "$OPENCONNECT_PATH" = /custom/openconnect
test "$(command -v go)" = "$GUIX_ENVIRONMENT/bin/go"
test "$(command -v gcc)" = "$GUIX_ENVIRONMENT/bin/gcc"
./run.ysh penv code 'project argument'
./run.ysh editor 'editor argument'
codium codium-argument
cursor cursor-argument
openconnect openconnect-argument
exit
EOF
then
	cat "$temp_dir/stderr" >&2
	echo 'Project shell isolation test failed.' >&2
	exit 1
fi

if grep -q 'locale: Cannot set' "$temp_dir/stderr"; then
	cat "$temp_dir/stderr" >&2
	echo 'Project shell could not load its locale.' >&2
	exit 1
fi
grep -qx 'host-tool:code:project argument' "$temp_dir/stdout"
grep -qx 'host-tool:code:editor argument' "$temp_dir/stdout"
grep -qx 'host-tool:codium:codium-argument' "$temp_dir/stdout"
grep -qx 'host-tool:cursor:cursor-argument' "$temp_dir/stdout"
grep -qx 'host-tool:openconnect:openconnect-argument' "$temp_dir/stdout"
echo 'Project shell isolation tests passed.'
