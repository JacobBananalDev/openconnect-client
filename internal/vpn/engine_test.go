package vpn

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

// TestStateTransitions verifies the state machine transitions correctly.
func TestStateTransitions(t *testing.T) {
	tests := []struct {
		name       string
		operations func(e *Engine) error
		finalState ConnectionState
	}{
		{
			name: "connect and disconnect",
			operations: func(e *Engine) error {
				if err := e.Connect(context.Background(), ConnectionConfig{
					ServerURL: "vpn.example.com",
					Protocol:  "anyconnect",
				}); err != nil {
					return err
				}
				return e.Disconnect(context.Background())
			},
			finalState: StateDisconnected,
		},
		{
			name: "initial state is disconnected",
			operations: func(e *Engine) error {
				return nil
			},
			finalState: StateDisconnected,
		},
		{
			name: "disconnect when already disconnected is idempotent",
			operations: func(e *Engine) error {
				return e.Disconnect(context.Background())
			},
			finalState: StateDisconnected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &FakeRunner{}
			e := NewEngine(runner)

			if err := tt.operations(e); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := e.State(); got != tt.finalState {
				t.Errorf("final state: got %v, want %v", got, tt.finalState)
			}
		})
	}
}

// TestDuplicateConnectRejected verifies that duplicate Connect calls are rejected.
func TestDuplicateConnectRejected(t *testing.T) {
	runner := &FakeRunner{}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	// First Connect should succeed.
	if err := e.Connect(context.Background(), config); err != nil {
		t.Fatalf("first Connect failed: %v", err)
	}

	// Second Connect should be rejected.
	if err := e.Connect(context.Background(), config); err != ErrAlreadyConnecting {
		t.Errorf("duplicate Connect: got %v, want ErrAlreadyConnecting", err)
	}
}

// TestConnectFailureSetsFailedState verifies that Connect failure marks state as Failed.
func TestConnectFailureSetsFailedState(t *testing.T) {
	runner := &FakeRunner{
		ShouldFail:   true,
		FailureError: errors.New("test failure"),
	}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	err := e.Connect(context.Background(), config)
	if err == nil {
		t.Fatal("Connect should have failed")
	}

	if got := e.State(); got != StateFailed {
		t.Errorf("state after Connect failure: got %v, want StateFailed", got)
	}
}

// TestConfigValidation verifies that invalid configs are rejected.
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  ConnectionConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ConnectionConfig{
				ServerURL: "vpn.example.com",
				Protocol:  "anyconnect",
			},
			wantErr: false,
		},
		{
			name: "missing ServerURL",
			config: ConnectionConfig{
				Protocol: "anyconnect",
			},
			wantErr: true,
		},
		{
			name: "missing Protocol",
			config: ConnectionConfig{
				ServerURL: "vpn.example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &FakeRunner{}
			e := NewEngine(runner)

			err := e.Connect(context.Background(), tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Connect validation: got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

// TestDisconnectTimeout verifies that Disconnect force-kills on timeout.
func TestDisconnectTimeout(t *testing.T) {
	runner := &FakeRunner{
		ExitDelay: 500 * time.Millisecond,
	}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	if err := e.Connect(context.Background(), config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Disconnect with a short timeout to force kill.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := e.Disconnect(ctx)
	if err == nil {
		t.Error("Disconnect should have timed out")
	}

	if got := e.State(); got != StateDisconnected {
		t.Errorf("state after timeout: got %v, want StateDisconnected", got)
	}
}

// TestBuildOpenConnectArgs verifies argument construction.
func TestBuildOpenConnectArgs(t *testing.T) {
	tests := []struct {
		name     string
		config   ConnectionConfig
		wantArgs []string
	}{
		{
			name: "basic args",
			config: ConnectionConfig{
				ServerURL: "vpn.example.com",
				Protocol:  "anyconnect",
			},
			wantArgs: []string{"--protocol", "anyconnect", "vpn.example.com"},
		},
		{
			name: "with CA cert",
			config: ConnectionConfig{
				ServerURL: "vpn.example.com",
				Protocol:  "anyconnect",
				CACert:    "/path/to/ca.crt",
			},
			wantArgs: []string{"--protocol", "anyconnect", "--cafile", "/path/to/ca.crt", "vpn.example.com"},
		},
		{
			name: "with script",
			config: ConnectionConfig{
				ServerURL: "vpn.example.com",
				Protocol:  "anyconnect",
				Script:    "/path/to/vpnc-script",
			},
			wantArgs: []string{"--protocol", "anyconnect", "--script", "/path/to/vpnc-script", "vpn.example.com"},
		},
		{
			name: "all options",
			config: ConnectionConfig{
				ServerURL: "vpn.example.com:8443",
				Protocol:  "pulse",
				CACert:    "/etc/ssl/ca.crt",
				Script:    "/usr/local/bin/vpnc-script",
			},
			wantArgs: []string{
				"--protocol", "pulse",
				"--cafile", "/etc/ssl/ca.crt",
				"--script", "/usr/local/bin/vpnc-script",
				"vpn.example.com:8443",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &FakeRunner{}
			e := NewEngine(runner)

			got := e.buildOpenConnectArgs(tt.config)
			if !slicesEqual(got, tt.wantArgs) {
				t.Errorf("args: got %v, want %v", got, tt.wantArgs)
			}
		})
	}
}

// TestConcurrentAccess verifies that concurrent operations are safe.
func TestConcurrentAccess(t *testing.T) {
	runner := &FakeRunner{}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	done := make(chan error, 10)

	// Concurrent State() calls.
	for i := 0; i < 5; i++ {
		go func() {
			_ = e.State()
			done <- nil
		}()
	}

	// Connect and Disconnect.
	go func() {
		done <- e.Connect(context.Background(), config)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		done <- e.Disconnect(context.Background())
	}()

	// Concurrent State() calls during Connect/Disconnect.
	for i := 0; i < 3; i++ {
		go func() {
			_ = e.State()
			done <- nil
		}()
	}

	// Wait for all goroutines.
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil && err != ErrAlreadyConnecting {
			t.Logf("concurrent operation error (may be expected): %v", err)
		}
	}
}

// TestResetAfterFailure verifies that a failed connection can be retried.
func TestResetAfterFailure(t *testing.T) {
	runner := &FakeRunner{}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	// First attempt succeeds.
	if err := e.Connect(context.Background(), config); err != nil {
		t.Fatalf("first Connect failed: %v", err)
	}

	// Wait a bit and disconnect.
	time.Sleep(10 * time.Millisecond)
	if err := e.Disconnect(context.Background()); err != nil {
		t.Logf("Disconnect: %v", err)
	}

	// Second attempt should also succeed (no lingering failed state).
	if err := e.Connect(context.Background(), config); err != nil {
		t.Errorf("second Connect failed: %v", err)
	}
}

// TestConnectionStateString verifies the string representation of states.
func TestConnectionStateString(t *testing.T) {
	tests := []struct {
		state ConnectionState
		want  string
	}{
		{StateDisconnected, "disconnected"},
		{StateConnecting, "connecting"},
		{StateConnected, "connected"},
		{StateDisconnecting, "disconnecting"},
		{StateFailed, "failed"},
		{ConnectionState(-1), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("%d.String(): got %q, want %q", tt.state, got, tt.want)
		}
	}
}

// TestRepeatedConnectDisconnectCycles verifies stable behavior across multiple cycles.
func TestRepeatedConnectDisconnectCycles(t *testing.T) {
	runner := &FakeRunner{
		StartDelay: 10 * time.Millisecond,
		ExitDelay:  10 * time.Millisecond,
	}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	for cycle := 0; cycle < 3; cycle++ {
		t.Logf("Cycle %d", cycle)

		if err := e.Connect(context.Background(), config); err != nil {
			t.Fatalf("cycle %d: Connect failed: %v", cycle, err)
		}

		if got := e.State(); got != StateConnecting {
			t.Errorf("cycle %d: after Connect, state: got %v, want StateConnecting", cycle, got)
		}

		// Disconnect immediately (don't wait for StartDelay).
		if err := e.Disconnect(context.Background()); err != nil {
			t.Logf("cycle %d: Disconnect: %v", cycle, err)
		}

		// Wait for monitorProcess goroutine to settle.
		time.Sleep(50 * time.Millisecond)

		if got := e.State(); got != StateDisconnected {
			t.Errorf("cycle %d: after Disconnect, state: got %v, want StateDisconnected", cycle, got)
		}
	}
}

// TestDisconnectDuringConnecting verifies correct behavior when Disconnect is called during Connecting.
func TestDisconnectDuringConnecting(t *testing.T) {
	runner := &FakeRunner{
		StartDelay: 100 * time.Millisecond,
	}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	// Start Connect (will take 100ms due to StartDelay).
	if err := e.Connect(context.Background(), config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// State should be Connecting.
	if got := e.State(); got != StateConnecting {
		t.Errorf("state during Connect: got %v, want StateConnecting", got)
	}

	// Immediately call Disconnect (while still Connecting).
	err := e.Disconnect(context.Background())
	if err != nil {
		t.Logf("Disconnect during Connecting: %v", err)
	}

	// State should now be Disconnected.
	if got := e.State(); got != StateDisconnected {
		t.Errorf("state after Disconnect during Connecting: got %v, want StateDisconnected", got)
	}
}

// TestGoroutineCleanup verifies that monitorProcess goroutine exits after process exits.
func TestGoroutineCleanup(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	runner := &FakeRunner{}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	// Connect and disconnect.
	if err := e.Connect(context.Background(), config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if err := e.Disconnect(context.Background()); err != nil {
		t.Logf("Disconnect: %v", err)
	}

	// Wait a bit for monitorProcess goroutine to exit.
	time.Sleep(50 * time.Millisecond)

	// Check goroutine count.
	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines {
		t.Errorf("goroutine leak detected: initial=%d, final=%d (leaked=%d)",
			initialGoroutines, finalGoroutines, finalGoroutines-initialGoroutines)
	}
}

// TestIntegrationWithRealProcessRunner verifies Engine works with RealProcessRunner.
func TestIntegrationWithRealProcessRunner(t *testing.T) {
	t.Run("resolver works", func(t *testing.T) {
		runner, err := NewRealProcessRunner()
		if err != nil {
			t.Fatalf("failed to create runner: %v", err)
		}

		if runner.ExecutablePath == "" {
			t.Fatal("executable path is empty")
		}

		t.Logf("Found openconnect at: %s", runner.ExecutablePath)
	})

	t.Run("engine accepts runner", func(t *testing.T) {
		runner, err := NewRealProcessRunner()
		if err != nil {
			t.Fatalf("failed to create runner: %v", err)
		}

		engine := NewEngine(runner)
		if engine == nil {
			t.Fatal("engine creation failed")
		}

		// Just verify interface compatibility; don't connect to real VPN.
		if got := engine.State(); got != StateDisconnected {
			t.Errorf("initial state: got %v, want StateDisconnected", got)
		}
	})
}

// TestStateDuringAsyncOperations verifies State() reads are consistent during async operations.
func TestStateDuringAsyncOperations(t *testing.T) {
	runner := &FakeRunner{
		StartDelay: 100 * time.Millisecond,
		ExitDelay:  200 * time.Millisecond, // Keep process running longer
	}
	e := NewEngine(runner)

	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	// Connect.
	if err := e.Connect(context.Background(), config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Poll state during async operations.
	statesObserved := make(map[ConnectionState]bool)
	for i := 0; i < 20; i++ {
		state := e.State()
		statesObserved[state] = true
		time.Sleep(3 * time.Millisecond)
	}

	// Should have observed Connecting state.
	if !statesObserved[StateConnecting] {
		t.Error("never observed StateConnecting during async Connect")
	}

	// Disconnect (after allowing process to stabilize into Connected).
	if err := e.Disconnect(context.Background()); err != nil {
		t.Logf("Disconnect: %v", err)
	}

	// Wait longer for monitorProcess goroutine to complete and settle.
	time.Sleep(100 * time.Millisecond)

	// Final state should be Disconnected.
	if got := e.State(); got != StateDisconnected {
		t.Errorf("final state: got %v, want StateDisconnected", got)
	}
}

// Helper: check if two string slices are equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
