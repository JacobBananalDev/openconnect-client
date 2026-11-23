package main

import (
	"fmt"
	"io"
	"os"

	"github.com/JacobBananalDev/openconnect-client/internal/app"
	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
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
	case "connect":
		runner, err := vpn.NewRealProcessRunner()
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		handler := app.NewHandler(runner, stdout, stderr)
		return handler.Connect(args[1:])
	default:
		fmt.Fprintf(stderr, "Error: unknown command %q\n", cmd)
		fmt.Fprintf(stderr, "Use 'occlient -h' for help\n")
		return 2
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: occlient [command]

Commands:
  connect [options]          Connect to a VPN server
  help, -h, --help           Show this help message
  version, -v, --version     Show version

Use 'occlient connect -h' for detailed connection options.
`)
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
