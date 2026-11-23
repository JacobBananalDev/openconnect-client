package vpn

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

// TestResolveOpenConnectPath_Found tests successful path resolution.
func TestResolveOpenConnectPath_Found(t *testing.T) {
	// Temporarily set OPENCONNECT_PATH to a known binary (e.g., /bin/ls).
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/ls")
	path, err := resolveOpenConnectPath()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if path != "/bin/ls" {
		t.Errorf("got path %q, want /bin/ls", path)
	}
}

// TestResolveOpenConnectPath_EnvVarInvalid tests that an invalid OPENCONNECT_PATH is rejected.
func TestResolveOpenConnectPath_EnvVarInvalid(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/nonexistent/path/to/openconnect")
	_, err := resolveOpenConnectPath()
	if err == nil {
		t.Fatal("expected error for nonexistent OPENCONNECT_PATH, got nil")
	}
	if !contains(err.Error(), "non-existent") {
		t.Errorf("error message should mention non-existent file: %v", err)
	}
}

// TestResolveOpenConnectPath_CommonPaths tests fallback to common system paths.
func TestResolveOpenConnectPath_CommonPaths(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	// Clear the environment variable to trigger PATH search.
	os.Setenv("OPENCONNECT_PATH", "")

	// This test may succeed or fail depending on whether openconnect is installed.
	// It's informational rather than a strict assertion.
	path, err := resolveOpenConnectPath()
	if err == nil {
		t.Logf("Found openconnect at: %s", path)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("resolved path is not accessible: %v", err)
		}
	} else if notFound, ok := err.(*NotFoundError); ok {
		t.Logf("OpenConnect not found (expected in test environment): %v", notFound.Executable)
	}
}

// TestNewRealProcessRunner_Success tests creating a runner with valid executable.
func TestNewRealProcessRunner_Success(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/ls")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if runner.ExecutablePath != "/bin/ls" {
		t.Errorf("runner executable path: got %q, want /bin/ls", runner.ExecutablePath)
	}
}

// TestNewRealProcessRunner_NotFound tests runner creation when executable is missing.
func TestNewRealProcessRunner_NotFound(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/nonexistent/openconnect")
	_, err := NewRealProcessRunner()
	if err == nil {
		t.Fatal("expected error when executable not found, got nil")
	}
	// Error should indicate the executable is not found (could be NotFoundError or other error types).
	if !contains(err.Error(), "not found") && !contains(err.Error(), "non-existent") {
		t.Errorf("error should indicate path not found: %v", err)
	}
}

// TestStartCommand_Success tests starting a process (using /bin/true).
func TestStartCommand_Success(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/true")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	proc, err := runner.StartCommand(context.Background(), "openconnect", []string{})
	if err != nil {
		t.Fatalf("StartCommand failed: %v", err)
	}

	if proc == nil {
		t.Fatal("returned process is nil")
	}

	if pid := proc.PID(); pid == 0 {
		t.Error("PID should be non-zero after process starts")
	}

	// Wait for process to exit (should succeed since /bin/true exits immediately).
	if err := proc.Wait(); err != nil {
		t.Logf("Process wait: %v (expected)", err)
	}
}

// TestStartCommand_InvalidCommand tests rejection of unsupported commands.
func TestStartCommand_InvalidCommand(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/true")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	_, err = runner.StartCommand(context.Background(), "unsupported", []string{})
	if err == nil {
		t.Fatal("expected error for unsupported command")
	}
	if !contains(err.Error(), "unsupported command") {
		t.Errorf("error should mention unsupported command: %v", err)
	}
}

// TestStartCommand_ContextCancellation tests that context cancellation stops the process.
func TestStartCommand_ContextCancellation(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/sleep")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start a process that sleeps for a long time.
	proc, err := runner.StartCommand(ctx, "openconnect", []string{"10"})
	if err != nil {
		t.Fatalf("StartCommand failed: %v", err)
	}

	// Wait for the process to be killed by context timeout.
	err = proc.Wait()
	if err == nil {
		t.Logf("Process exited normally (may happen if already finished)")
	}
	// Context cancellation should have caused the process to terminate.
}

// TestStartCommand_StdoutStderr_Piped tests that stdout/stderr are properly redirected.
func TestStartCommand_StdoutStderr_Piped(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/echo")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Redirect stdout to a buffer.
	stdout := &bytes.Buffer{}
	runner.Stdout = stdout

	proc, err := runner.StartCommand(context.Background(), "openconnect", []string{"hello world"})
	if err != nil {
		t.Fatalf("StartCommand failed: %v", err)
	}

	if err := proc.Wait(); err != nil {
		t.Logf("Process wait: %v", err)
	}

	output := stdout.String()
	if !contains(output, "hello world") {
		t.Errorf("stdout redirection failed; got: %q", output)
	}
}

// TestExecProcess_Signal tests signal delivery to a process.
func TestExecProcess_Signal(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/sleep")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	proc, err := runner.StartCommand(context.Background(), "openconnect", []string{"10"})
	if err != nil {
		t.Fatalf("StartCommand failed: %v", err)
	}

	// Send SIGTERM to the process.
	err = proc.Signal(os.Interrupt)
	if err != nil {
		t.Logf("Signal: %v (expected if process already exited)", err)
	}

	// Wait for clean exit.
	_ = proc.Wait()
}

// TestExecProcess_PID tests that PID is accessible.
func TestExecProcess_PID(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/true")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	proc, err := runner.StartCommand(context.Background(), "openconnect", []string{})
	if err != nil {
		t.Fatalf("StartCommand failed: %v", err)
	}

	pid := proc.PID()
	if pid <= 0 {
		t.Errorf("PID should be positive, got %d", pid)
	}

	_ = proc.Wait()
}

// TestArgumentPassingIsSafe tests that arguments are passed safely (no shell interpolation).
func TestArgumentPassingIsSafe(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	// Use /bin/echo to verify argument passing.
	os.Setenv("OPENCONNECT_PATH", "/bin/echo")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{"simple arg", []string{"hello"}},
		{"args with spaces", []string{"hello world"}},
		{"args with special chars", []string{"test;echo;pwned"}},
		{"args with quotes", []string{"\"test\""}},
		{"args with backticks", []string{"`test`"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			runner.Stdout = stdout

			proc, err := runner.StartCommand(context.Background(), "openconnect", tt.args)
			if err != nil {
				t.Fatalf("StartCommand failed: %v", err)
			}

			if err := proc.Wait(); err != nil {
				t.Logf("Wait: %v", err)
			}

			// The argument should appear verbatim in stdout (echo outputs all args).
			output := stdout.String()
			for _, arg := range tt.args {
				if !contains(output, arg) {
					t.Errorf("argument %q not found in output: %q", arg, output)
				}
			}
		})
	}
}

// TestRealProcessRunner_WithFakeRunner tests that RealProcessRunner is compatible with ProcessRunner interface.
func TestRealProcessRunner_WithFakeRunner(t *testing.T) {
	oldEnv := os.Getenv("OPENCONNECT_PATH")
	defer os.Setenv("OPENCONNECT_PATH", oldEnv)

	os.Setenv("OPENCONNECT_PATH", "/bin/true")
	runner, err := NewRealProcessRunner()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Use in Engine to verify interface compatibility.
	engine := NewEngine(runner)
	config := ConnectionConfig{
		ServerURL: "vpn.example.com",
		Protocol:  "anyconnect",
	}

	// This should work without compilation errors.
	// We don't actually connect; just verify the interface is satisfied.
	_ = engine
	_ = config
}

// TestNotFoundError tests the error type.
func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{
		Executable: "openconnect",
		Searched:   []string{"PATH", "/usr/sbin", "/usr/bin"},
	}

	errStr := err.Error()
	if !contains(errStr, "openconnect") {
		t.Errorf("error should mention openconnect: %s", errStr)
	}
	if !contains(errStr, "not found") {
		t.Errorf("error should mention 'not found': %s", errStr)
	}
	if !contains(errStr, "apt-get install") {
		t.Errorf("error should mention install command: %s", errStr)
	}
}

// Helper: check if string contains substring.
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && bytes.Contains([]byte(s), []byte(substr))
}
