#!/bin/sh
set -eu

source scripts/lib/gui-env.sh
prepare_gui_environment

exec ./build/occlient-gui "$@"
