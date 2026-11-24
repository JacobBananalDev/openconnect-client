package vpn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
)

const disconnectTimeout = 5 * time.Second

// Engine manages VPN connection lifecycle.
// It is safe for concurrent use and enforces one active connection at a time.
//
// LIFECYCLE:
//
// An Engine starts in StateDisconnected. Connect transitions to StateConnecting.
// Process startup does not establish tunnel readiness, so the engine does not
// transition to StateConnected. Unexpected process exit transitions to StateFailed.
//
// Calling Disconnect from StateConnecting or StateConnected transitions to
// StateDisconnecting, then to StateDisconnected when the process has exited.
// If Disconnect takes longer than the provided context timeout, the process is
// force-killed (SIGKILL) and cleanup continues.
//
// CONCURRENCY:
//
// Connect and Disconnect are mutex-protected; only one can be active at a time.
// Duplicate Connect calls are rejected with ErrAlreadyConnecting.
// Concurrent Disconnect calls wait for the same process exit.
// State() is a read-only snapshot and safe for concurrent calls.
//
// SHUTDOWN SEMANTICS:
//
// Connect creates a background goroutine (monitorProcess) that watches for unexpected
// process termination. This goroutine blocks on Process.Wait() until the subprocess exits,
// then updates the state if the exit was unexpected. The goroutine will eventually exit
// when the process exits; there is no explicit cleanup needed.
//
// Disconnect signals the process, waits for it to exit, and force-kills it
// after five seconds or when the provided context expires, whichever is first.
// The monitor owns the single Process.Wait call.
//
// RESOURCE MANAGEMENT:
//
// - stdin is redirected to /dev/null (no password prompts in Phase 3)
// - stdout and stderr are piped/redirected (not inherited from parent)
// - Context cancellation propagates to the subprocess
// - Mutex-protected state prevents race conditions
// - Process file descriptors are closed by Process.Wait()
//
// KNOWN LIMITATIONS:
//
//   - Tunnel readiness is not detected; a running process remains Connecting.
//   - Future auth phase can refine this by parsing OpenConnect output or other signals
type Engine struct {
	runner  ProcessRunner
	mu      sync.RWMutex
	state   ConnectionState
	process Process
	done    chan struct{}
}

// NewEngine creates a lifecycle controller with a process runner.
// The runner is responsible for starting processes (os/exec in production, fake in tests).
func NewEngine(runner ProcessRunner) *Engine {
	return &Engine{
		runner: runner,
		state:  StateDisconnected,
	}
}

// Connect initiates a VPN connection.
// Only one active connection is allowed per Engine instance.
// Duplicate Connect calls are rejected with ErrAlreadyConnecting.
func (e *Engine) Connect(ctx context.Context, config ConnectionConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Reject duplicate starts.
	if e.state == StateConnecting || e.state == StateConnected || e.state == StateDisconnecting {
		return ErrAlreadyConnecting
	}

	// Reset from a previous failed connection.
	if e.state == StateFailed {
		// Clear the failed process reference.
		e.process = nil
	}

	// Update state and create a cancellation context.
	e.state = StateConnecting
	processCtx, cancel := context.WithCancel(ctx)

	// Build arguments (safe, no shell interpolation).
	args := e.buildOpenConnectArgs(config)

	// Start the OpenConnect process.
	proc, err := e.runner.StartCommand(processCtx, "openconnect", args)
	if err != nil {
		e.state = StateFailed
		cancel()
		return fmt.Errorf("failed to start OpenConnect: %w", err)
	}

	e.process = proc
	e.done = make(chan struct{})

	// Monitor the process in a background goroutine.
	// If it exits unexpectedly, mark as failed.
	go e.monitorProcess(proc, e.done, cancel)

	return nil
}

// Disconnect terminates the active connection.
// If no connection is active, Disconnect returns nil (idempotent).
// Graceful shutdown: sends SIGTERM and waits. Force kill after at most five seconds.
func (e *Engine) Disconnect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, disconnectTimeout)
	defer cancel()
	e.mu.Lock()
	if e.state == StateDisconnected || e.state == StateFailed {
		e.mu.Unlock()
		return nil
	}

	first := e.state != StateDisconnecting
	if first {
		e.state = StateDisconnecting
	}
	process := e.process
	done := e.done
	e.mu.Unlock()

	if process == nil || done == nil {
		return errors.New("internal: process is nil during disconnect")
	}

	var signalErr error
	if first {
		signalErr = process.Signal(syscall.SIGTERM)
	}
	if errors.Is(signalErr, os.ErrProcessDone) {
		signalErr = nil
	}

	select {
	case <-done:
		if signalErr != nil {
			return fmt.Errorf("signal OpenConnect: %w", signalErr)
		}
		return nil

	case <-ctx.Done():
		killErr := process.Signal(os.Kill)
		if errors.Is(killErr, os.ErrProcessDone) {
			killErr = nil
		}
		return errors.Join(fmt.Errorf("disconnect deadline exceeded: %w", ctx.Err()), signalErr, killErr)
	}
}

// State returns the current connection state (read-only snapshot).
func (e *Engine) State() ConnectionState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// monitorProcess watches for unexpected process termination.
// Called in a background goroutine after Connect succeeds.
// If the process exits while Connecting or Connected, marks state as Failed.
// Once Disconnect has been called (state >= StateDisconnecting), process exits are expected.
func (e *Engine) monitorProcess(process Process, done chan struct{}, cancel context.CancelFunc) {
	_ = process.Wait()
	cancel()

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state == StateConnecting || e.state == StateConnected {
		e.state = StateFailed
	} else if e.state == StateDisconnecting {
		e.state = StateDisconnected
	}
	close(done)
}

// buildOpenConnectArgs constructs command-line arguments for OpenConnect.
// No shell interpolation; arguments are passed directly to exec.
func (e *Engine) buildOpenConnectArgs(config ConnectionConfig) []string {
	args := []string{
		"--protocol", config.Protocol,
	}

	if config.CACert != "" {
		args = append(args, "--cafile", config.CACert)
	}

	if config.Script != "" {
		args = append(args, "--script", config.Script)
	}

	// Server URL is the last positional argument.
	args = append(args, config.ServerURL)

	return args
}

// Errors returned by Engine.

// ErrAlreadyConnecting is returned if Connect is called while a connection is in progress.
var ErrAlreadyConnecting = errors.New("connection already in progress or active")
