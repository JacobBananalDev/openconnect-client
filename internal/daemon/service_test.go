package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
)

type testRunner struct {
	started chan *testProcess
}

type failingRunner struct{}

func (failingRunner) StartCommand(context.Context, string, []string) (vpn.Process, error) {
	return nil, errors.New("injected startup failure")
}

type blockingRunner struct{ started chan *blockingProcess }

func (r blockingRunner) StartCommand(context.Context, string, []string) (vpn.Process, error) {
	process := &blockingProcess{exit: make(chan struct{}), signals: make(chan os.Signal, 4)}
	r.started <- process
	return process, nil
}

type blockingProcess struct {
	exit    chan struct{}
	signals chan os.Signal
	once    sync.Once
	waits   atomic.Int32
}

func (p *blockingProcess) Wait() error {
	p.waits.Add(1)
	<-p.exit
	return nil
}
func (p *blockingProcess) Signal(signal os.Signal) error {
	p.signals <- signal
	if signal == os.Kill {
		p.once.Do(func() { close(p.exit) })
	}
	return nil
}
func (p *blockingProcess) PID() int { return 1 }

func (r *testRunner) StartCommand(ctx context.Context, _ string, _ []string) (vpn.Process, error) {
	p := &testProcess{ctx: ctx, exit: make(chan struct{})}
	r.started <- p
	return p, nil
}

type testProcess struct {
	ctx   context.Context
	exit  chan struct{}
	once  sync.Once
	waits atomic.Int32
}

func (p *testProcess) finish() { p.once.Do(func() { close(p.exit) }) }
func (p *testProcess) Wait() error {
	p.waits.Add(1)
	<-p.exit
	return nil
}
func (p *testProcess) Signal(signal os.Signal) error {
	if signal == syscall.SIGTERM || signal == os.Kill {
		p.finish()
	}
	return nil
}
func (p *testProcess) PID() int { return 1 }

func startService(t *testing.T, authorizer ipc.Authorizer, policy func(vpn.ConnectionConfig) bool) (ipc.Client, *testRunner, context.CancelFunc, <-chan error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	runner := &testRunner{started: make(chan *testProcess, 16)}
	service, err := NewService(path, runner, authorizer, policy)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, service) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("service shutdown: %v", err)
			}
		case <-time.After(7 * time.Second):
			t.Error("service did not stop")
		}
	})
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket was not created")
		}
		time.Sleep(time.Millisecond)
	}
	return ipc.Client{SocketPath: path}, runner, cancel, done
}

func request(command ipc.Command) ipc.Request {
	params := json.RawMessage(`{}`)
	if command == ipc.CommandConnect {
		params = json.RawMessage(`{"server":"vpn.example.com","protocol":"anyconnect"}`)
	}
	return ipc.Request{Version: ipc.ProtocolVersion, RequestID: "test", Command: command, Params: params}
}

func call(t *testing.T, client ipc.Client, command ipc.Command) ipc.Response {
	t.Helper()
	response, err := client.Do(context.Background(), request(command))
	if err != nil {
		t.Fatalf("%s: %v", command, err)
	}
	return response
}

func state(t *testing.T, response ipc.Response) string {
	t.Helper()
	if !response.OK {
		t.Fatalf("unexpected response: %+v", response)
	}
	var data struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(response.Data, &data); err != nil {
		t.Fatal(err)
	}
	return data.State
}

func allowPeer() ipc.Authorizer {
	return ipc.AuthorizerFunc(func(context.Context, ipc.PeerCredentials) error { return nil })
}

func TestServiceLifecycleOverSocket(t *testing.T) {
	client, runner, _, _ := startService(t, allowPeer(), func(vpn.ConnectionConfig) bool { return true })
	if got := state(t, call(t, client, ipc.CommandStatus)); got != "disconnected" {
		t.Fatalf("initial state = %s", got)
	}
	if got := state(t, call(t, client, ipc.CommandConnect)); got != "connecting" {
		t.Fatalf("state after connect = %s", got)
	}
	process := <-runner.started
	select {
	case <-process.ctx.Done():
		t.Fatal("request completion canceled the VPN process context")
	default:
	}
	response := call(t, client, ipc.CommandConnect)
	if response.OK || response.Error.Code != "already_active" {
		t.Fatalf("duplicate connect: %+v", response)
	}
	if got := state(t, call(t, client, ipc.CommandDisconnect)); got != "disconnected" {
		t.Fatalf("state after disconnect = %s", got)
	}
	if got := process.waits.Load(); got != 1 {
		t.Fatalf("Wait called %d times", got)
	}
	if got := state(t, call(t, client, ipc.CommandDisconnect)); got != "disconnected" {
		t.Fatalf("repeated disconnect state = %s", got)
	}
}

func TestServicePolicyAndAuthorizationFailClosed(t *testing.T) {
	t.Run("nil policy", func(t *testing.T) {
		client, runner, _, _ := startService(t, allowPeer(), nil)
		response := call(t, client, ipc.CommandConnect)
		if response.OK || response.Error.Code != "policy_denied" {
			t.Fatalf("response = %+v", response)
		}
		select {
		case <-runner.started:
			t.Fatal("denied connect started a process")
		default:
		}
	})
	t.Run("unauthorized peer", func(t *testing.T) {
		deny := ipc.AuthorizerFunc(func(context.Context, ipc.PeerCredentials) error { return errors.New("denied") })
		client, runner, _, _ := startService(t, deny, func(vpn.ConnectionConfig) bool { return true })
		response := call(t, client, ipc.CommandConnect)
		if response.OK || response.Error.Code != "unauthorized" {
			t.Fatalf("response = %+v", response)
		}
		select {
		case <-runner.started:
			t.Fatal("unauthorized connect started a process")
		default:
		}
	})
}

func TestServiceEnforcesLoadedPolicy(t *testing.T) {
	dir := trustedTestRoot(t)
	script := filepath.Join(dir, "script")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := writePolicy(t, dir, `{"connections":[{"server":"vpn.example.com","protocol":"anyconnect","script":"`+script+`"}]}`)
	policy, err := loadPolicy(path, dir, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	client, runner, _, _ := startService(t, allowPeer(), policy.Allows)
	response := call(t, client, ipc.CommandConnect)
	if response.OK || response.Error == nil || response.Error.Code != "policy_denied" {
		t.Fatalf("unapproved request = %+v", response)
	}
	params, err := json.Marshal(ipc.ConnectParams{Server: "vpn.example.com", Protocol: "anyconnect", Script: script})
	if err != nil {
		t.Fatal(err)
	}
	req := request(ipc.CommandConnect)
	req.Params = params
	response, err = client.Do(context.Background(), req)
	if err != nil || !response.OK {
		t.Fatalf("approved request = %+v, error = %v", response, err)
	}
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("approved request did not start a process")
	}
}

func TestServiceMapsStartupFailure(t *testing.T) {
	service, err := NewService(filepath.Join(t.TempDir(), "control.sock"), failingRunner{}, allowPeer(), func(vpn.ConnectionConfig) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	response := service.Handle(context.Background(), request(ipc.CommandConnect), ipc.PeerCredentials{})
	if response.OK || response.Error == nil || response.Error.Code != "process_error" {
		t.Fatalf("response = %+v, want process_error", response)
	}
	if got := state(t, service.Handle(context.Background(), request(ipc.CommandStatus), ipc.PeerCredentials{})); got != "failed" {
		t.Fatalf("state = %s, want failed", got)
	}
}

func TestServiceUnexpectedExitAndShutdown(t *testing.T) {
	client, runner, cancel, _ := startService(t, allowPeer(), func(vpn.ConnectionConfig) bool { return true })
	call(t, client, ipc.CommandConnect)
	process := <-runner.started
	process.finish()
	deadline := time.Now().Add(time.Second)
	for {
		if state(t, call(t, client, ipc.CommandStatus)) == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("unexpected exit did not produce failed state")
		}
		time.Sleep(time.Millisecond)
	}
	call(t, client, ipc.CommandConnect)
	active := <-runner.started
	cancel()
	select {
	case <-active.exit:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not stop active process")
	}
}

func TestServiceConcurrentConnects(t *testing.T) {
	client, runner, _, _ := startService(t, allowPeer(), func(vpn.ConnectionConfig) bool { return true })
	const clients = 8
	results := make(chan ipc.Response, clients)
	errs := make(chan error, clients)
	for i := 0; i < clients; i++ {
		go func() {
			response, err := client.Do(context.Background(), request(ipc.CommandConnect))
			results <- response
			errs <- err
		}()
	}
	successes := 0
	for i := 0; i < clients; i++ {
		response, err := <-results, <-errs
		if err != nil {
			t.Fatal(err)
		}
		if response.OK {
			successes++
		} else if response.Error == nil || response.Error.Code != "already_active" {
			t.Fatalf("response = %+v", response)
		}
	}
	if successes != 1 || len(runner.started) != 1 {
		t.Fatalf("successes = %d, starts = %d; want one each", successes, len(runner.started))
	}
}

func TestDisconnectDeadlineForceKillsProcess(t *testing.T) {
	runner := blockingRunner{started: make(chan *blockingProcess, 1)}
	service, err := NewService(filepath.Join(t.TempDir(), "control.sock"), runner, allowPeer(), func(vpn.ConnectionConfig) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if response := service.Handle(context.Background(), request(ipc.CommandConnect), ipc.PeerCredentials{}); !response.OK {
		t.Fatalf("connect = %+v", response)
	}
	process := <-runner.started
	t.Cleanup(func() { process.once.Do(func() { close(process.exit) }) })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan ipc.Response, 1)
	go func() { done <- service.Handle(ctx, request(ipc.CommandDisconnect), ipc.PeerCredentials{}) }()
	select {
	case signal := <-process.signals:
		if signal != syscall.SIGTERM {
			t.Fatalf("first signal = %v, want SIGTERM", signal)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect did not signal process")
	}
	select {
	case response := <-done:
		if response.OK || response.Error == nil || response.Error.Code != "timeout" {
			t.Fatalf("disconnect = %+v, want timeout", response)
		}
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("context error = %v, want deadline exceeded", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("canceled disconnect did not return")
	}
	select {
	case signal := <-process.signals:
		if signal != os.Kill {
			t.Fatalf("second signal = %v, want SIGKILL", signal)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect did not force-kill process")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := service.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if got := process.waits.Load(); got != 1 {
		t.Fatalf("Wait called %d times, want once", got)
	}
}

func TestShutdownInterruptsInFlightDisconnect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	runner := blockingRunner{started: make(chan *blockingProcess, 1)}
	service, err := NewService(path, runner, allowPeer(), func(vpn.ConnectionConfig) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, service) }()
	finished := false
	t.Cleanup(func() {
		cancel()
		if !finished {
			select {
			case <-done:
			case <-time.After(7 * time.Second):
				t.Error("daemon did not stop during cleanup")
			}
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
	client := ipc.Client{SocketPath: path}
	if response := call(t, client, ipc.CommandConnect); !response.OK {
		t.Fatalf("connect = %+v", response)
	}
	process := <-runner.started
	t.Cleanup(func() { process.once.Do(func() { close(process.exit) }) })
	type clientResult struct {
		response ipc.Response
		err      error
	}
	requestDone := make(chan clientResult, 1)
	go func() {
		response, err := client.Do(context.Background(), request(ipc.CommandDisconnect))
		requestDone <- clientResult{response, err}
	}()
	select {
	case signal := <-process.signals:
		if signal != syscall.SIGTERM {
			t.Fatalf("first signal = %v, want SIGTERM", signal)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect did not start")
	}
	cancel()
	select {
	case signal := <-process.signals:
		if signal != os.Kill {
			t.Fatalf("shutdown signal = %v, want SIGKILL", signal)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not force-kill process")
	}
	select {
	case err := <-done:
		finished = true
		if err != nil {
			t.Fatalf("daemon shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not shut down")
	}
	select {
	case result := <-requestDone:
		if result.err == nil && (result.response.OK || result.response.Error == nil || result.response.Error.Code != "timeout") {
			t.Fatalf("interrupted request = %+v, error = %v", result.response, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect request did not stop")
	}
	if got := process.waits.Load(); got != 1 {
		t.Fatalf("Wait called %d times, want once", got)
	}
}
