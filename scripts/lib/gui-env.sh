prepare_gui_environment() {
	if [[ $EUID == 0 ]]; then
		echo 'Error: run the desktop GUI as an unprivileged user.' >&2
		return 1
	fi

	if [[ ${GUIX_LOCPATH+x} ]]; then
		local remaining=$GUIX_LOCPATH canonical= separator= entry
		while :; do
			entry=${remaining%%:*}
			if [[ -n $entry ]]; then
				if ! entry=$(realpath -m -- "$entry"); then
					echo 'Error: could not resolve GUIX_LOCPATH for the desktop GUI.' >&2
					return 1
				fi
			fi
			canonical+=$separator$entry
			separator=:
			if [[ $remaining != *:* ]]; then
				break
			fi
			remaining=${remaining#*:}
		done
		export GUIX_LOCPATH=$canonical
	fi
}
