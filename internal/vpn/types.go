package vpn

import (
	"context"
	"os"
	"time"
)

// ConnectionState represents the observable state of a VPN connection.
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateDisconnecting
	StateFailed
)

// String returns a human-readable name for the connection state.
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateDisconnecting:
		return "disconnecting"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ConnectionConfig holds endpoint and non-secret connection options.
// No credentials, tokens, or passwords are stored here—these are
// provided by OpenConnect's authentication mechanisms (stdin, environment,
// or external tools) in future phases.
type ConnectionConfig struct {
	// ServerURL is the VPN server endpoint, e.g., "vpn.example.com:443"
	ServerURL string

	// Protocol is the VPN protocol, e.g., "anyconnect", "nc", "gp", "pulse".
	// Must be validated against openconnect --help supported-protocols.
	Protocol string

	// CACert is an optional path to a custom CA certificate file.
	CACert string

	// Script is an optional path to a custom vpnc-script override.
	Script string
}

// Validate checks the config for obviously invalid values.
// It does not connect or probe the server; only syntax checks.
func (c ConnectionConfig) Validate() error {
	if c.ServerURL == "" {
		return &ValidationError{Field: "ServerURL", Reason: "required"}
	}
	if c.Protocol == "" {
		return &ValidationError{Field: "Protocol", Reason: "required"}
	}
	// Additional validation (e.g., protocol whitelist, URL format) added in later phases.
	return nil
}

// ValidationError describes a config validation failure.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return "validation error: " + e.Field + ": " + e.Reason
}

// ConnectionEvent represents a lifecycle event from the engine.
// Events are immutable and timestamped for diagnostics.
type ConnectionEvent struct {
	// State is the new state after this event.
	State ConnectionState

	// Timestamp records when the event occurred.
	Timestamp time.Time

	// Error is non-nil if State == StateFailed, explaining why.
	Error error

	// Message is a user-visible diagnostic or progress update.
	Message string
}

// ProcessRunner abstracts process execution for testing.
// A real implementation uses os/exec; tests use a fake.
type ProcessRunner interface {
	// StartCommand starts a command with the given arguments.
	// No shell interpolation is performed; args are passed as-is to exec.
	// The returned Process is non-nil on success.
	StartCommand(ctx context.Context, cmd string, args []string) (Process, error)
}

// Process represents a running subprocess.
// It provides lifecycle control and diagnostics without exposing raw os.Cmd.
type Process interface {
	// Wait blocks until the process exits.
	// Returns nil on success, or an error (e.g., exec.ExitError) on failure.
	Wait() error

	// Signal sends an OS signal to the process.
	// Returns an error if the process has already exited or signal delivery fails.
	Signal(sig os.Signal) error

	// PID returns the process ID.
	// May return 0 if the process has not started or has exited.
	PID() int
}
