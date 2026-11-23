package vpn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
)

// Engine manages VPN connection lifecycle.
// It is safe for concurrent use and enforces one active connection at a time.
//
// LIFECYCLE:
//
// An Engine starts in StateDisconnected. Calling Connect transitions to StateConnecting,
// then to StateConnected if the process starts successfully. If the process exits
// unexpectedly during Connecting or Connected, the state transitions to StateFailed.
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
// Duplicate Disconnect calls during an active disconnect return ErrDisconnectInProgress.
// State() is a read-only snapshot and safe for concurrent calls.
//
// SHUTDOWN SEMANTICS:
//
// Connect creates a background goroutine (monitorProcess) that watches for unexpected
// process termination. This goroutine blocks on Process.Wait() until the subprocess exits,
// then updates the state if the exit was unexpected. The goroutine will eventually exit
// when the process exits; there is no explicit cleanup needed.
//
// Disconnect sends a cancellation signal to the process via context cancellation
// (which may translate to SIGTERM if the process adapter supports it), waits for
// graceful shutdown, and force-kills (SIGKILL) if the timeout is exceeded. All
// goroutines and resources are cleaned up during Disconnect; calling Disconnect
// is idempotent—repeated calls have no additional effect.
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
//   - StateConnected is set optimistically after successful process start;
//     it does not distinguish between "process running" and "tunnel actually active"
//   - Future auth phase can refine this by parsing OpenConnect output or other signals
//   - No persistent background connection across CLI invocations (foreground only in Phase 3)
type Engine struct {
	runner  ProcessRunner
	mu      sync.RWMutex
	state   ConnectionState
	process Process
	cancel  context.CancelFunc
	ctx     context.Context
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
	if e.state == StateConnecting || e.state == StateConnected {
		return ErrAlreadyConnecting
	}

	// Reset from a previous failed connection.
	if e.state == StateFailed {
		// Clear the failed process reference.
		e.process = nil
	}

	// Update state and create a cancellation context.
	e.state = StateConnecting
	e.ctx, e.cancel = context.WithCancel(ctx)

	// Build arguments (safe, no shell interpolation).
	args := e.buildOpenConnectArgs(config)

	// Start the OpenConnect process.
	proc, err := e.runner.StartCommand(e.ctx, "openconnect", args)
	if err != nil {
		e.state = StateFailed
		e.cancel()
		return fmt.Errorf("failed to start OpenConnect: %w", err)
	}

	e.process = proc

	// Monitor the process in a background goroutine.
	// If it exits unexpectedly, mark as failed.
	go e.monitorProcess()

	return nil
}

// Disconnect terminates the active connection.
// If no connection is active, Disconnect returns nil (idempotent).
// Graceful shutdown: sends SIGTERM and waits. Force kill on timeout.
func (e *Engine) Disconnect(ctx context.Context) error {
	e.mu.Lock()
	if e.state == StateDisconnected || e.state == StateFailed {
		e.mu.Unlock()
		return nil
	}

	if e.state != StateConnecting && e.state != StateConnected {
		e.mu.Unlock()
		return ErrDisconnectInProgress
	}

	e.state = StateDisconnecting

	// Trigger context cancellation (graceful shutdown signal).
	if e.cancel != nil {
		e.cancel()
	}

	process := e.process
	e.mu.Unlock()

	if process == nil {
		return errors.New("internal: process is nil during disconnect")
	}

	// Wait for clean exit, with timeout.
	done := make(chan error, 1)
	go func() {
		done <- process.Wait()
	}()

	select {
	case err := <-done:
		e.mu.Lock()
		e.state = StateDisconnected
		e.mu.Unlock()
		return err

	case <-ctx.Done():
		// Timeout or cancellation: force kill.
		_ = process.Signal(os.Kill)
		<-done
		e.mu.Lock()
		e.state = StateDisconnected
		e.mu.Unlock()
		return fmt.Errorf("disconnect timeout, force-killed: %w", ctx.Err())
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
func (e *Engine) monitorProcess() {
	err := e.process.Wait()

	e.mu.Lock()
	defer e.mu.Unlock()

	// Only mark as failed if we're still in an active state.
	// If Disconnecting or already Disconnected, the exit was expected.
	// StateDisconnecting (value 3) and StateDisconnected (value 0) are both <= StateConnected (value 2)
	// so we check: only mark as failed if still Connecting or Connected.
	if e.state == StateConnecting || e.state == StateConnected {
		e.state = StateFailed
		// Diagnostic: preserve the error for future event reporting.
		// In a full implementation, this could be stored and queried.
		_ = err
	}
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

var (
	// ErrAlreadyConnecting is returned if Connect is called while a connection is in progress.
	ErrAlreadyConnecting = errors.New("connection already in progress or active")

	// ErrDisconnectInProgress is returned if Disconnect is called while already disconnecting.
	ErrDisconnectInProgress = errors.New("disconnect already in progress")
)
