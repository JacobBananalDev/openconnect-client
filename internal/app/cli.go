package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
)

type ipcClient interface {
	Do(context.Context, ipc.Request) (ipc.Response, error)
}

// Handler manages CLI command execution.
type Handler struct {
	runner vpn.ProcessRunner
	client ipcClient
	stdout io.Writer
	stderr io.Writer
}

// NewHandler creates a CLI handler.
func NewHandler(runner vpn.ProcessRunner, stdout, stderr io.Writer) *Handler {
	return &Handler{
		runner: runner,
		client: ipc.Client{SocketPath: ipc.DefaultSocketPath},
		stdout: stdout,
		stderr: stderr,
	}
}

// Connect handles the "connect" command.
// Establishes a VPN connection and blocks until disconnected or Ctrl+C.
func (h *Handler) Connect(args []string) int {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Suppress default flag error output

	server := fs.String("server", "", "VPN server endpoint (required)")
	protocol := fs.String("protocol", "anyconnect", "VPN protocol")
	caCert := fs.String("ca-cert", "", "Path to custom CA certificate")
	script := fs.String("script", "", "Path to custom vpnc-script")
	background := fs.Bool("background", false, "Use the background daemon")

	// Parse flags.
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			h.printConnectHelp()
			return 0
		}
		fmt.Fprintf(h.stderr, "Error: %v\n", err)
		h.printConnectHelp()
		return 2
	}

	// Check for required flags.
	if *server == "" {
		fmt.Fprintf(h.stderr, "Error: --server is required\n")
		h.printConnectHelp()
		return 2
	}

	// Validate file paths if provided.
	if *caCert != "" {
		if err := validateFilePath(*caCert, "CA cert"); err != nil {
			fmt.Fprintf(h.stderr, "Error: %v\n", err)
			return 1
		}
	}

	if *script != "" {
		if err := validateFilePath(*script, "vpnc-script"); err != nil {
			fmt.Fprintf(h.stderr, "Error: %v\n", err)
			return 1
		}
	}

	// Build connection config.
	config := vpn.ConnectionConfig{
		ServerURL: *server,
		Protocol:  *protocol,
		CACert:    *caCert,
		Script:    *script,
	}

	// Validate config.
	if err := config.Validate(); err != nil {
		fmt.Fprintf(h.stderr, "Error: %v\n", err)
		h.printConnectHelp()
		return 2
	}

	if *background {
		if fs.NArg() != 0 {
			fmt.Fprintln(h.stderr, "Error: unexpected arguments")
			return 2
		}
		var err error
		if config.CACert != "" {
			config.CACert, err = filepath.Abs(config.CACert)
			if err != nil {
				fmt.Fprintf(h.stderr, "Error: resolve CA cert path: %v\n", err)
				return 1
			}
		}
		if config.Script != "" {
			config.Script, err = filepath.Abs(config.Script)
			if err != nil {
				fmt.Fprintf(h.stderr, "Error: resolve vpnc-script path: %v\n", err)
				return 1
			}
		}
		return h.daemonCommand(ipc.CommandConnect, ipc.ConnectParams{
			Server: config.ServerURL, Protocol: config.Protocol, CACert: config.CACert, Script: config.Script,
		})
	}

	// Create VPN engine.
	engine := vpn.NewEngine(h.runner)

	// Set up Ctrl+C handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	disconnectChan := make(chan error, 1)
	go func() {
		select {
		case <-sigChan:
			fmt.Fprintf(h.stdout, "\nDisconnecting...\n")
			disconnectChan <- engine.Disconnect(context.Background())
		case <-ctx.Done():
		}
	}()

	// Initiate connection.
	fmt.Fprintf(h.stdout, "Connecting to %s (%s)...\n", *server, *protocol)
	fmt.Fprintf(h.stdout, "Press Ctrl+C to disconnect\n")

	if err := engine.Connect(ctx, config); err != nil {
		var notFound *vpn.NotFoundError
		if errors.As(err, &notFound) {
			fmt.Fprintf(h.stderr, "%v\n", notFound)
		} else {
			fmt.Fprintf(h.stderr, "Error: failed to connect: %v\n", err)
		}
		return 1
	}

	// Block until Ctrl+C, context cancellation, or connection failure.
	for {
		select {
		case <-time.After(100 * time.Millisecond):
			// Check if connection failed unexpectedly.
			state := engine.State()
			if state == vpn.StateFailed {
				fmt.Fprintf(h.stderr, "Error: VPN connection failed\n")
				return 1
			}
			// Keep polling for signal or failure.
			continue

		case disconnectErr := <-disconnectChan:
			if disconnectErr != nil {
				fmt.Fprintf(h.stdout, "Disconnect error: %v\n", disconnectErr)
			} else {
				fmt.Fprintf(h.stdout, "✓ Disconnected\n")
			}
			return 0

		case <-ctx.Done():
			// Context cancelled; engine should handle cleanup.
			return 0
		}
	}
}

// Status reports the daemon's connection state.
func (h *Handler) Status(args []string) int {
	return h.simpleDaemonCommand(ipc.CommandStatus, args)
}

// Disconnect requests that the daemon stop its connection.
func (h *Handler) Disconnect(args []string) int {
	return h.simpleDaemonCommand(ipc.CommandDisconnect, args)
}

func (h *Handler) simpleDaemonCommand(command ipc.Command, args []string) int {
	fs := flag.NewFlagSet(string(command), flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(h.stdout, "Usage: occlient %s\n", command)
			return 0
		}
		fmt.Fprintf(h.stderr, "Error: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(h.stderr, "Error: unexpected arguments")
		return 2
	}
	return h.daemonCommand(command, struct{}{})
}

func (h *Handler) daemonCommand(command ipc.Command, params any) int {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		fmt.Fprintf(h.stderr, "Error: generate request ID: %v\n", err)
		return 1
	}
	data, err := json.Marshal(params)
	if err != nil {
		fmt.Fprintf(h.stderr, "Error: encode request: %v\n", err)
		return 1
	}
	request := ipc.Request{Version: ipc.ProtocolVersion, RequestID: hex.EncodeToString(id[:]), Command: command, Params: data}
	response, err := h.client.Do(context.Background(), request)
	if err != nil {
		fmt.Fprintf(h.stderr, "Error: daemon request failed: %v\n", err)
		if command == ipc.CommandConnect {
			fmt.Fprintln(h.stderr, "The connection result may be unknown; check status before retrying.")
		}
		return 1
	}
	if !response.OK {
		if response.Error == nil {
			fmt.Fprintln(h.stderr, "Error: invalid daemon response")
			return 1
		}
		fmt.Fprintf(h.stderr, "Error: %s (%s)\n", response.Error.Message, response.Error.Code)
		return 1
	}
	var result struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(response.Data, &result); err != nil || !validDaemonState(result.State) {
		fmt.Fprintln(h.stderr, "Error: invalid daemon state response")
		return 1
	}
	if command == ipc.CommandConnect && result.State == vpn.StateFailed.String() {
		fmt.Fprintln(h.stderr, "Error: connection process exited; state: failed")
		return 1
	}
	fmt.Fprintf(h.stdout, "State: %s\n", result.State)
	if command == ipc.CommandConnect {
		fmt.Fprintln(h.stdout, "A started process does not confirm tunnel readiness.")
	}
	return 0
}

func validDaemonState(state string) bool {
	switch state {
	case "disconnected", "connecting", "connected", "disconnecting", "failed":
		return true
	default:
		return false
	}
}

// printConnectHelp prints the connect command usage.
func (h *Handler) printConnectHelp() {
	fmt.Fprintf(h.stdout, `Usage: occlient connect [options]

Options:
  --server <URL>             VPN server endpoint (required)
  --protocol <proto>         VPN protocol (default: "anyconnect")
                             Supported: anyconnect, nc, gp, pulse, f5, fortinet, array
  --ca-cert <path>          Path to custom CA certificate file
  --script <path>           Path to custom vpnc-script
  --background              Request a connection from the daemon

Examples:
  occlient connect --server vpn.example.com
  occlient connect --server vpn.example.com:8443 --protocol pulse
  occlient connect --server vpn.example.com --ca-cert /etc/ssl/ca.crt

This phase does not support:
  - Credentials or password prompts (future phase)
  - Background connections until daemon policy is configured
`)
}

// validateFilePath checks that a file exists and is readable.
func validateFilePath(path, description string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%s path is invalid: %w", description, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s file not found: %s", description, absPath)
		}
		return fmt.Errorf("cannot read %s: %w", description, err)
	}

	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a file: %s", description, absPath)
	}

	return nil
}
