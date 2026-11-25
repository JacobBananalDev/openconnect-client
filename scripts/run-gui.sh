#!/bin/sh
set -eu

. scripts/lib/gui-env.sh
prepare_gui_environment

binary=$1
shift
exec "$binary" "$@"
