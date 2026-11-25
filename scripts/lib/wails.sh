ensure_wails() {
	version=$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
	if [ -z "$version" ]; then
		echo 'Error: unable to determine the Wails module version from go.mod.' >&2
		return 1
	fi
	mkdir -p build/tools
	if [ -x build/tools/wails3 ]; then
		if ! installed_version=$(./build/tools/wails3 version 2>&1); then
			echo "Error: could not determine the project-local Wails CLI version: $installed_version" >&2
			return 1
		fi
	else
		installed_version=
	fi
	if [ "$installed_version" != "$version" ]; then
		echo "Installing project-local Wails CLI $version..."
		GOBIN="$PWD/build/tools" go install "github.com/wailsapp/wails/v3/cmd/wails3@$version" || return
		if ! installed_version=$(./build/tools/wails3 version 2>&1); then
			echo "Error: could not determine the installed Wails CLI version: $installed_version" >&2
			return 1
		fi
		if [ "$installed_version" != "$version" ]; then
			echo 'Error: the installed Wails CLI does not match go.mod.' >&2
			return 1
		fi
	fi
}

generate_bindings() {
	./build/tools/wails3 generate bindings -ts -i -d internal/gui/frontend/bindings ./cmd/occlient-gui
}
