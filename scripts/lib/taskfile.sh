var _task_file = $(realpath "$0")

proc _print-help(task) {
	var script = r'''
		function trim_end(str) {
			sub(/[[:space:]]+$/, "", str)
			return str
		}

		BEGIN {
			BOLD = "\033[1m"
			RESET = "\033[0m"
		}

		match($0, /^## usage:[ \t]+/) {
			in_options = 0
			in_description = 0
			help["usage"] = substr($0, RLENGTH+1)
			next
		}

		match($0, /^## summary:[ \t]+/) {
			in_options = 0
			in_description = 0
			help["summary"] = substr($0, RLENGTH+1)
			next
		}

		/^## description:/ {
			in_options = 0
			in_description = 1
			next
		}

		/^## options:/ {
			in_options = 1
			in_description = 0
			next
		}

		/^##/ {
			if (in_options) {
				help["options"] = help["options"] "\n" substr($0, 4)
			} else if (in_description) {
				help["description"] = help["description"] "\n" substr($0, 4)
			}
			next
		}

		match($0, /^[ \t]*proc[ \t]+[^_]/) {
			task_name = substr($0, RLENGTH)
			match(task_name, /^[^ \t(]+/)
			task_name = substr(task_name, 0, RLENGTH)
			if (task_name == task) {
				if (help["usage"]) {
					print BOLD "USAGE: " RESET help["usage"] "\n"
				} else {
					print BOLD "USAGE: " RESET task " [-h, --help]\n"
				}
				if (help["summary"]) {
					print help["summary"] "\n"
				}
				if (help["description"]) {
					print BOLD "DESCRIPTION:" RESET trim_end(help["description"]) "\n"
				}
				print BOLD "OPTIONS:" RESET
				print "-h, --help"
				print "    print this help text"
				if (help["options"]) {
					print trim_end(help["options"])
				}
				exit
			}
			in_options = 0
			in_description = 0
			delete help["usage"]
			delete help["summary"]
			delete help["description"]
			delete help["options"]
		}
		'''
	awk -v task=$task $script $_task_file >&2
}

proc _list-tasks() {
	var script = r'''
		BEGIN {
			BOLD = "\033[1m"
			RESET = "\033[0m"
		}

		match($0, /^## summary:[ \t]+/) {
			summary = substr($0, RLENGTH+1)
			next
		}

		match($0, /^[ \t]*proc[ \t]+[^_]/) {
			task_name = substr($0, RLENGTH)
			match(task_name, /^[^ \t(]+/)
			task_name = substr(task_name, 0, RLENGTH)
			print BOLD task_name RESET (summary ? " - " summary : "")
			summary = ""
		}
		'''
	awk $script $_task_file | sort >&2
}

proc _print-list-tasks-help() {
	var bold = r'\e[1m'
	var reset = r'\e[0m'

	printf """
		${bold}USAGE:${reset} list-tasks [-h, --help]

		Lists all available tasks for the current file.

		${bold}OPTIONS:${reset}
		-h, --help
		    display this help text
		""" >&2
}

var show_help = len(ARGV) === 0
for arg in (ARGV) {
	if (arg === '--') {
		break
	}
	if (arg === '-h' or arg === '--help') {
		setvar show_help = true
	}
}
if (show_help) {
	if (len(ARGV) === 0) {
		_print-list-tasks-help
	} elif (ARGV[0] === "list-tasks") {
		_print-list-tasks-help
	} else {
		_print-help $[ARGV[0]]
	}
	exit
}

if (ARGV[0] === 'list-tasks') {
	_list-tasks
	exit
}

@ARGV
