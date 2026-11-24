package app

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
)

// TestConnectMissingServer verifies error when --server is missing.
func TestConnectMissingServer(t *testing.T) {
	runner := &vpn.FakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	handler := NewHandler(runner, stdout, stderr)

	code := handler.Connect([]string{})
	if code != 2 {
		t.Errorf("missing --server: got exit code %d, want 2", code)
	}

	errOut := stderr.String()
	if !bytes.Contains([]byte(errOut), []byte("--server is required")) {
		t.Errorf("error output should mention --server is required; got: %s", errOut)
	}
}

// TestConnectInvalidCAcertPath verifies error for invalid CA cert file.
func TestConnectInvalidCAcertPath(t *testing.T) {
	runner := &vpn.FakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	handler := NewHandler(runner, stdout, stderr)

	code := handler.Connect([]string{
		"--server", "vpn.example.com",
		"--ca-cert", "/nonexistent/ca.crt",
	})
	if code != 1 {
		t.Errorf("invalid file path: got exit code %d, want 1", code)
	}

	errOut := stderr.String()
	if !bytes.Contains([]byte(errOut), []byte("not found")) {
		t.Errorf("error output should mention file not found; got: %s", errOut)
	}
}

// TestConnectInvalidScriptPath verifies error for invalid vpnc-script file.
func TestConnectInvalidScriptPath(t *testing.T) {
	runner := &vpn.FakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	handler := NewHandler(runner, stdout, stderr)

	code := handler.Connect([]string{
		"--server", "vpn.example.com",
		"--script", "/nonexistent/vpnc-script",
	})
	if code != 1 {
		t.Errorf("invalid file path: got exit code %d, want 1", code)
	}

	errOut := stderr.String()
	if !bytes.Contains([]byte(errOut), []byte("not found")) {
		t.Errorf("error output should mention file not found; got: %s", errOut)
	}
}

// TestConnectHelp verifies help output.
func TestConnectHelp(t *testing.T) {
	runner := &vpn.FakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	handler := NewHandler(runner, stdout, stderr)

	code := handler.Connect([]string{"-h"})
	if code != 0 {
		t.Errorf("help: got exit code %d, want 0", code)
	}

	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("Usage: occlient connect")) {
		t.Errorf("help output should include usage; got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("--server")) {
		t.Errorf("help output should mention --server; got: %s", out)
	}
}

// TestConnectUnknownFlag verifies error for unknown flags.
func TestConnectUnknownFlag(t *testing.T) {
	runner := &vpn.FakeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	handler := NewHandler(runner, stdout, stderr)

	code := handler.Connect([]string{
		"--server", "vpn.example.com",
		"--unknown-flag", "value",
	})
	if code != 2 {
		t.Errorf("unknown flag: got exit code %d, want 2", code)
	}

	errOut := stderr.String()
	if errOut == "" {
		t.Error("error output should not be empty")
	}
}

// TestConnectProtocolDefault verifies default protocol is "anyconnect".
func TestConnectProtocolDefault(t *testing.T) {
	// Create a temporary file to avoid file validation errors.
	tmpFile, err := os.CreateTemp("", "ca-*.crt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	runner := &vpn.FakeRunner{
		ExitDelay: 10 * time.Second, // Keep process "running" long enough for test
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	handler := NewHandler(runner, stdout, stderr)

	// Run Connect in a goroutine with a timeout, since it blocks until disconnected.
	codeChan := make(chan int, 1)
	go func() {
		codeChan <- handler.Connect([]string{
			"--server", "vpn.example.com",
		})
	}()

	// Wait for startup.
	time.Sleep(150 * time.Millisecond)

	// Check output during "connection".
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("anyconnect")) {
		t.Errorf("output should mention anyconnect protocol; got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("vpn.example.com")) {
		t.Errorf("output should mention server; got: %s", out)
	}

	// In a real scenario, Ctrl+C would trigger disconnect. For testing,
	// we just verify the connection was attempted successfully.
}

// TestConnectValidConfig verifies successful connect with valid config.
func TestConnectValidConfig(t *testing.T) {
	// Create a temporary CA cert file to pass validation.
	tmpFile, err := os.CreateTemp("", "ca-*.crt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	runner := &vpn.FakeRunner{
		ExitDelay: 500 * time.Millisecond, // Keep process "running" during test
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	handler := NewHandler(runner, stdout, stderr)

	// Run Connect in a goroutine with a timeout, since it blocks until disconnected.
	codeChan := make(chan int, 1)
	go func() {
		codeChan <- handler.Connect([]string{
			"--server", "vpn.example.com:8443",
			"--protocol", "pulse",
			"--ca-cert", tmpFile.Name(),
		})
	}()

	// Wait for startup and verification, then timeout.
	time.Sleep(100 * time.Millisecond)

	// In a real scenario, we'd send Ctrl+C here. For now, just check output.
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("vpn.example.com")) {
		t.Errorf("output should mention server; got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("pulse")) {
		t.Errorf("output should mention protocol; got: %s", out)
	}
}

// TestValidateFilePath_MissingFile verifies error for nonexistent file.
func TestValidateFilePath_MissingFile(t *testing.T) {
	err := validateFilePath("/nonexistent/file.crt", "test file")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("not found")) {
		t.Errorf("error should mention not found; got: %v", err)
	}
}

// TestValidateFilePath_Directory verifies error when path is a directory.
func TestValidateFilePath_Directory(t *testing.T) {
	err := validateFilePath("/tmp", "test file")
	if err == nil {
		t.Fatal("expected error for directory")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("directory")) {
		t.Errorf("error should mention directory; got: %v", err)
	}
}

// TestValidateFilePath_ValidFile verifies success for valid file.
func TestValidateFilePath_ValidFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = validateFilePath(tmpFile.Name(), "test file")
	if err != nil {
		t.Errorf("valid file should not error; got: %v", err)
	}
}
