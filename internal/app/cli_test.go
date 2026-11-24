package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/JacobBananalDev/openconnect-client/internal/daemon"
	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
)

type clientFunc func(context.Context, ipc.Request) (ipc.Response, error)

func (f clientFunc) Do(ctx context.Context, request ipc.Request) (ipc.Response, error) {
	return f(ctx, request)
}

type foregroundSignalRunner struct{}

func (foregroundSignalRunner) StartCommand(context.Context, string, []string) (vpn.Process, error) {
	fmt.Fprintln(os.Stdout, "OCCLIENT_TEST_READY")
	return &foregroundSignalProcess{exit: make(chan struct{})}, nil
}

type foregroundSignalProcess struct {
	exit chan struct{}
	once sync.Once
}

func (p *foregroundSignalProcess) Wait() error {
	<-p.exit
	return nil
}
func (p *foregroundSignalProcess) Signal(signal os.Signal) error {
	if signal == syscall.SIGTERM {
		p.once.Do(func() { close(p.exit) })
	}
	return nil
}
func (p *foregroundSignalProcess) PID() int { return 1 }

func TestForegroundInterrupt(t *testing.T) {
	if os.Getenv("OCCLIENT_TEST_FOREGROUND_CHILD") == "1" {
		handler := NewHandler(foregroundSignalRunner{}, os.Stdout, os.Stderr)
		os.Exit(handler.Connect([]string{"--server", "vpn.example.com"}))
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestForegroundInterrupt$")
	cmd.Env = append(os.Environ(), "OCCLIENT_TEST_FOREGROUND_CHILD=1")
	cmd.Stdout = writer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		writer.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	exited := false
	t.Cleanup(func() {
		if !exited {
			_ = cmd.Process.Kill()
			<-done
		}
	})
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	type outputResult struct {
		text string
		err  error
	}
	ready := make(chan struct{}, 1)
	output := make(chan outputResult, 1)
	go func() {
		scanner := bufio.NewScanner(reader)
		var text strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			text.WriteString(line + "\n")
			if line == "OCCLIENT_TEST_READY" {
				ready <- struct{}{}
			}
		}
		output <- outputResult{text.String(), scanner.Err()}
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("foreground test process did not start")
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		exited = true
		if err != nil {
			t.Fatalf("foreground process: %v; stderr: %s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("foreground process did not exit after Ctrl+C")
	}
	select {
	case result := <-output:
		if result.err != nil || !strings.Contains(result.text, "Disconnected") {
			t.Fatalf("foreground output = %q, error = %v", result.text, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("foreground output did not close")
	}
}

func TestDaemonCommands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		command ipc.Command
		state   string
	}{
		{"background connect", []string{"connect", "--background", "--server", "vpn.example.com", "--protocol", "pulse"}, ipc.CommandConnect, "connecting"},
		{"status", []string{"status"}, ipc.CommandStatus, "connecting"},
		{"disconnect", []string{"disconnect"}, ipc.CommandDisconnect, "disconnected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			runner := &vpn.FakeRunner{ShouldFail: true}
			handler := NewHandler(runner, &stdout, &stderr)
			handler.client = clientFunc(func(_ context.Context, request ipc.Request) (ipc.Response, error) {
				if request.Command != tt.command {
					t.Errorf("command = %q, want %q", request.Command, tt.command)
				}
				if _, err := ipc.EncodeRequest(request); err != nil {
					t.Errorf("invalid request: %v", err)
				}
				if request.Command == ipc.CommandConnect {
					var params ipc.ConnectParams
					if err := json.Unmarshal(request.Params, &params); err != nil {
						t.Fatal(err)
					}
					if params.Server != "vpn.example.com" || params.Protocol != "pulse" {
						t.Errorf("connect params = %+v", params)
					}
				}
				return ipc.NewSuccessResponse(request.RequestID, map[string]string{"state": tt.state})
			})
			var code int
			switch tt.command {
			case ipc.CommandConnect:
				code = handler.Connect(tt.args[1:])
			case ipc.CommandStatus:
				code = handler.Status(tt.args[1:])
			case ipc.CommandDisconnect:
				code = handler.Disconnect(tt.args[1:])
			}
			if code != 0 || !strings.Contains(stdout.String(), "State: "+tt.state) {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
			}
			if tt.command == ipc.CommandConnect && !strings.Contains(stdout.String(), "does not confirm tunnel readiness") {
				t.Fatalf("missing readiness warning: %q", stdout.String())
			}
			if runner.Started {
				t.Fatal("daemon command started a local process")
			}
		})
	}
}

func TestDaemonCommandErrors(t *testing.T) {
	tests := []struct {
		name       string
		result     func(ipc.Request) (ipc.Response, error)
		wantOutput string
	}{
		{"unavailable", func(ipc.Request) (ipc.Response, error) {
			return ipc.Response{}, errors.New("socket unavailable")
		}, "check status before retrying"},
		{"denied", func(r ipc.Request) (ipc.Response, error) {
			return ipc.NewErrorResponse(r.RequestID, "policy_denied", "Connection is not permitted"), nil
		}, "policy_denied"},
		{"invalid state", func(r ipc.Request) (ipc.Response, error) {
			return ipc.NewSuccessResponse(r.RequestID, map[string]string{"state": "unknown"})
		}, "invalid daemon state"},
		{"process already failed", func(r ipc.Request) (ipc.Response, error) {
			return ipc.NewSuccessResponse(r.RequestID, map[string]string{"state": "failed"})
		}, "connection process exited"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			runner := &vpn.FakeRunner{}
			handler := NewHandler(runner, &stdout, &stderr)
			handler.client = clientFunc(func(_ context.Context, r ipc.Request) (ipc.Response, error) {
				return tt.result(r)
			})
			code := handler.Connect([]string{"--background", "--server", "vpn.example.com"})
			if code != 1 || !strings.Contains(stderr.String(), tt.wantOutput) {
				t.Fatalf("code = %d, stderr = %q", code, stderr.String())
			}
			if runner.Started {
				t.Fatal("failure fell back to foreground")
			}
		})
	}
}

func TestBackgroundConnectUsesAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(path, []byte("certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativePath, err := filepath.Rel(cwd, path)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	handler := NewHandler(&vpn.FakeRunner{}, &stdout, &stderr)
	handler.client = clientFunc(func(_ context.Context, request ipc.Request) (ipc.Response, error) {
		var params ipc.ConnectParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			t.Fatal(err)
		}
		if params.CACert != path {
			t.Errorf("CA path = %q, want %q", params.CACert, path)
		}
		return ipc.NewSuccessResponse(request.RequestID, map[string]string{"state": "connecting"})
	})
	if got := handler.Connect([]string{"--background", "--server", "vpn.example.com", "--ca-cert", relativePath}); got != 0 {
		t.Fatalf("connect = %d, stderr = %q", got, stderr.String())
	}
}

func TestDaemonCommandArgumentValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handler := NewHandler(&vpn.FakeRunner{}, &stdout, &stderr)
	handler.client = clientFunc(func(context.Context, ipc.Request) (ipc.Response, error) {
		t.Fatal("client called for invalid arguments")
		return ipc.Response{}, nil
	})
	if got := handler.Status([]string{"extra"}); got != 2 {
		t.Errorf("status with extra argument = %d", got)
	}
	if got := handler.Disconnect([]string{"--unknown"}); got != 2 {
		t.Errorf("disconnect with unknown flag = %d", got)
	}
	if got := handler.Connect([]string{"--background"}); got != 2 {
		t.Errorf("connect without server = %d", got)
	}
}

func TestDaemonCommandsOverUnixSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	runner := &vpn.FakeRunner{}
	authorizer := ipc.AuthorizerFunc(func(context.Context, ipc.PeerCredentials) error { return nil })
	service, err := daemon.NewService(path, runner, authorizer, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- daemon.Run(ctx, service) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("stop service: %v", err)
		}
	})
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon socket was not created")
		}
		time.Sleep(time.Millisecond)
	}
	var stdout, stderr bytes.Buffer
	handler := NewHandler(runner, &stdout, &stderr)
	handler.client = ipc.Client{SocketPath: path}
	if got := handler.Status(nil); got != 0 || !strings.Contains(stdout.String(), "State: disconnected") {
		t.Fatalf("status = %d, stdout = %q, stderr = %q", got, stdout.String(), stderr.String())
	}
	if got := handler.Connect([]string{"--background", "--server", "vpn.example.com"}); got != 1 || !strings.Contains(stderr.String(), "policy_denied") {
		t.Fatalf("connect = %d, stderr = %q", got, stderr.String())
	}
	if runner.Started {
		t.Fatal("denied background request started a process")
	}
}

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

	runner := &vpn.FakeRunner{ExitDelay: 10 * time.Millisecond}
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

	select {
	case <-codeChan:
	case <-time.After(time.Second):
		t.Fatal("foreground connect did not finish")
	}

	// Check output during "connection".
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("anyconnect")) {
		t.Errorf("output should mention anyconnect protocol; got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("vpn.example.com")) {
		t.Errorf("output should mention server; got: %s", out)
	}

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

	runner := &vpn.FakeRunner{ExitDelay: 10 * time.Millisecond}
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

	select {
	case <-codeChan:
	case <-time.After(time.Second):
		t.Fatal("foreground connect did not finish")
	}

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
