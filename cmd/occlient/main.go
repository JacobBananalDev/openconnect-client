package main

import (
	"fmt"
	"io"
	"os"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	cmd := args[0]
	switch cmd {
	case "-h", "--help", "help":
		printHelp(stdout)
		return 0
	case "-v", "--version", "version":
		fmt.Fprintf(stdout, "occlient version 0.1.0 (dev)\n")
		return 0
	default:
		fmt.Fprintf(stderr, "Error: unknown command %q\n", cmd)
		fmt.Fprintf(stderr, "Use 'occlient -h' for help\n")
		return 2
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: occlient [command]

Commands:
  help, -h, --help           Show this help message
  version, -v, --version     Show version

No VPN connectivity implemented in this phase.
`)
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
