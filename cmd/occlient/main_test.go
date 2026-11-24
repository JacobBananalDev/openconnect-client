package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{"help -h", []string{"-h"}, 0, "Usage: occlient"},
		{"help --help", []string{"--help"}, 0, "Usage: occlient"},
		{"help cmd", []string{"help"}, 0, "Usage: occlient"},
		{"version", []string{"version"}, 0, "0.1.0"},
		{"version -v", []string{"-v"}, 0, "0.1.0"},
		{"version --version", []string{"--version"}, 0, "0.1.0"},
		{"no args", []string{}, 0, "Usage: occlient"},
		{"invalid cmd", []string{"foo"}, 2, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			code := Run(tt.args, stdout, stderr)

			if code != tt.wantCode {
				t.Errorf("got exit code %d, want %d", code, tt.wantCode)
			}

			out := stdout.String() + stderr.String()
			if tt.wantOut != "" && !strings.Contains(out, tt.wantOut) {
				t.Errorf("output missing %q; got:\n%s", tt.wantOut, out)
			}
		})
	}
}

func TestInvalidCommandError(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := Run([]string{"invalid"}, stdout, stderr)

	if code != 2 {
		t.Errorf("invalid command should exit 2, got %d", code)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "unknown command") {
		t.Errorf("error output should contain 'unknown command'; got: %s", errOut)
	}
}

func TestConnectCommandExists(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	// connect without --server should fail with usage error (exit code 2)
	code := Run([]string{"connect"}, stdout, stderr)

	if code != 2 {
		t.Errorf("connect without --server should exit 2, got %d", code)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "--server is required") {
		t.Errorf("error output should mention --server is required; got: %s", errOut)
	}
}

func TestDaemonCommandsDoNotResolveOpenConnectForHelpOrValidation(t *testing.T) {
	t.Setenv("OPENCONNECT_PATH", "/nonexistent/openconnect")
	for _, tt := range []struct {
		args []string
		code int
	}{
		{[]string{"connect", "--background"}, 2},
		{[]string{"connect", "--help"}, 0},
		{[]string{"status", "--help"}, 0},
		{[]string{"disconnect", "--help"}, 0},
	} {
		var stdout, stderr bytes.Buffer
		if got := Run(tt.args, &stdout, &stderr); got != tt.code {
			t.Errorf("Run(%v) = %d, want %d; stderr = %q", tt.args, got, tt.code, stderr.String())
		}
	}
}
