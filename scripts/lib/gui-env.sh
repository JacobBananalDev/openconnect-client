prepare_gui_environment() {
	if [ "$(id -u)" -eq 0 ]; then
		echo 'Error: run the desktop GUI as an unprivileged user.' >&2
		return 1
	fi

	if [ "${GUIX_LOCPATH+x}" = x ]; then
		remaining=$GUIX_LOCPATH
		canonical=
		separator=
		while :; do
			entry=${remaining%%:*}
			if [ -n "$entry" ]; then
				if ! entry=$(realpath -m -- "$entry"); then
					echo 'Error: could not resolve GUIX_LOCPATH for the desktop GUI.' >&2
					return 1
				fi
			fi
			canonical=$canonical$separator$entry
			separator=:
			case "$remaining" in
				*:* ) ;;
				*) break ;;
			esac
			if [ "$remaining" = "${remaining#*:}" ]; then
				break
			fi
			remaining=${remaining#*:}
		done
		export GUIX_LOCPATH=$canonical
	fi
}
