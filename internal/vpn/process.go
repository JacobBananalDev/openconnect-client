package vpn

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// resolveOpenConnectPath finds the OpenConnect executable.
// Search order: OPENCONNECT_PATH env var, PATH search, common system paths.
func resolveOpenConnectPath() (string, error) {
	// Priority 1: Explicit environment variable.
	if envPath := os.Getenv("OPENCONNECT_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
		return "", fmt.Errorf("OPENCONNECT_PATH points to non-existent file: %s", envPath)
	}

	// Priority 2: Search PATH.
	if path, err := exec.LookPath("openconnect"); err == nil {
		return path, nil
	}

	// Priority 3: Try common system locations.
	commonPaths := []string{
		"/usr/sbin/openconnect",
		"/usr/bin/openconnect",
		"/usr/local/bin/openconnect",
	}
	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", &NotFoundError{
		Executable: "openconnect",
		Searched:   append([]string{"PATH"}, commonPaths...),
	}
}

// NotFoundError is returned when OpenConnect executable cannot be located.
type NotFoundError struct {
	Executable string
	Searched   []string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf(
		"OpenConnect executable not found in PATH or common system locations\n"+
			"Install with: sudo apt-get install openconnect (Ubuntu/Debian)\n"+
			"Or set OPENCONNECT_PATH=/path/to/openconnect\n"+
			"Searched: %v",
		e.Searched,
	)
}

// RealProcessRunner implements ProcessRunner using os/exec.
// It is responsible for starting real OpenConnect subprocesses.
type RealProcessRunner struct {
	ExecutablePath string
	// Stderr is an optional writer for process stderr.
	// If nil, stderr is redirected to io.Discard (silent).
	Stderr io.Writer
	// Stdout is an optional writer for process stdout.
	// If nil, stdout is redirected to io.Discard (silent).
	Stdout io.Writer
}

// NewRealProcessRunner creates a process runner after resolving the OpenConnect executable.
func NewRealProcessRunner() (*RealProcessRunner, error) {
	path, err := resolveOpenConnectPath()
	if err != nil {
		return nil, err
	}
	return &RealProcessRunner{
		ExecutablePath: path,
		Stderr:         io.Discard,
		Stdout:         io.Discard,
	}, nil
}

// StartCommand starts an OpenConnect process with the given arguments.
// No shell interpolation; arguments are passed directly to exec.
func (r *RealProcessRunner) StartCommand(ctx context.Context, cmd string, args []string) (Process, error) {
	if cmd != "openconnect" {
		return nil, fmt.Errorf("unsupported command: %q (only 'openconnect' supported)", cmd)
	}

	// Create command with context for cancellation support.
	command := exec.CommandContext(ctx, r.ExecutablePath, args...)

	// Redirect stdin from /dev/null (no password prompts in this phase).
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return nil, fmt.Errorf("failed to open stdin: %w", err)
	}
	command.Stdin = stdin

	// Redirect stdout/stderr to configured writers (or discard).
	stdout := r.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := r.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	command.Stdout = stdout
	command.Stderr = stderr

	// Start the process (non-blocking).
	if err := command.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to start OpenConnect: %w", err)
	}

	return &ExecProcess{
		cmd:   command,
		stdin: stdin,
	}, nil
}

// ExecProcess wraps an exec.Cmd and implements the Process interface.
type ExecProcess struct {
	cmd   *exec.Cmd
	stdin *os.File
}

// Wait blocks until the process exits, then cleans up resources.
func (ep *ExecProcess) Wait() error {
	defer ep.stdin.Close()
	return ep.cmd.Wait()
}

// Signal sends an OS signal to the process.
func (ep *ExecProcess) Signal(sig os.Signal) error {
	if ep.cmd.Process == nil {
		return fmt.Errorf("process not started or already exited")
	}
	return ep.cmd.Process.Signal(sig)
}

// PID returns the process ID.
func (ep *ExecProcess) PID() int {
	if ep.cmd.Process == nil {
		return 0
	}
	return ep.cmd.Process.Pid
}

// IsRunning returns whether the process is still running.
func (ep *ExecProcess) IsRunning() bool {
	if ep.cmd.Process == nil {
		return false
	}
	// Check if process has exited (non-blocking).
	return ep.cmd.ProcessState == nil
}

// CommandString returns a string representation of the command for debugging.
func (ep *ExecProcess) CommandString() string {
	return ep.cmd.String()
}
