#!/bin/bash
set -euo pipefail

source scripts/lib/gui-env.sh
prepare_gui_environment

exec ./build/occlient-gui "$@"
