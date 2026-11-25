package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestConnectionServiceCommands(t *testing.T) {
	var (
		mu       sync.Mutex
		requests []ipc.Request
	)
	client := startFakeDaemon(t, ipc.HandlerFunc(func(_ context.Context, request ipc.Request, _ ipc.PeerCredentials) ipc.Response {
		mu.Lock()
		requests = append(requests, request)
		mu.Unlock()

		state := "disconnected"
		if request.Command == ipc.CommandConnect {
			state = "connecting"
		}
		response, err := ipc.NewSuccessResponse(request.RequestID, ConnectionStatus{State: state})
		if err != nil {
			return ipc.NewErrorResponse(request.RequestID, "internal_error", "Failed to create response")
		}
		return response
	}))
	service := newStartedConnectionService(t, client, context.Background())

	status, err := service.Status()
	if err != nil || status.State != "disconnected" {
		t.Fatalf("Status() = (%+v, %v), want disconnected", status, err)
	}
	connected, err := service.Connect(ConnectOptions{
		Server:   "vpn.example.com",
		Protocol: "anyconnect",
		CACert:   "/etc/ssl/certs/vpn-ca.pem",
	})
	if err != nil || connected.State != "connecting" {
		t.Fatalf("Connect() = (%+v, %v), want connecting", connected, err)
	}
	disconnected, err := service.Disconnect()
	if err != nil || disconnected.State != "disconnected" {
		t.Fatalf("Disconnect() = (%+v, %v), want disconnected", disconnected, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 3 {
		t.Fatalf("request count = %d, want 3", len(requests))
	}
	wantCommands := []ipc.Command{ipc.CommandStatus, ipc.CommandConnect, ipc.CommandDisconnect}
	for i, request := range requests {
		if request.Command != wantCommands[i] || request.RequestID == "" {
			t.Errorf("request[%d] = %+v, want command %q and request ID", i, request, wantCommands[i])
		}
	}
	var params ipc.ConnectParams
	if err := ipc.DecodeStrictJSON(requests[1].Params, &params); err != nil {
		t.Fatalf("decode connect parameters: %v", err)
	}
	if params != (ipc.ConnectParams{
		Server:   "vpn.example.com",
		Protocol: "anyconnect",
		CACert:   "/etc/ssl/certs/vpn-ca.pem",
	}) {
		t.Fatalf("connect parameters = %+v", params)
	}
}

func TestConnectionServiceSurfacesDaemonErrors(t *testing.T) {
	client := startFakeDaemon(t, ipc.HandlerFunc(func(_ context.Context, request ipc.Request, _ ipc.PeerCredentials) ipc.Response {
		return ipc.NewErrorResponse(request.RequestID, "policy_denied", "Connection is not permitted")
	}))
	service := newStartedConnectionService(t, client, context.Background())

	_, err := service.Connect(ConnectOptions{Server: "vpn.example.com", Protocol: "anyconnect"})
	if err == nil || !strings.Contains(err.Error(), "policy_denied") {
		t.Fatalf("Connect() error = %v, want policy_denied", err)
	}
}

func TestConnectionServiceRejectsInvalidState(t *testing.T) {
	client := startFakeDaemon(t, ipc.HandlerFunc(func(_ context.Context, request ipc.Request, _ ipc.PeerCredentials) ipc.Response {
		response, err := ipc.NewSuccessResponse(request.RequestID, ConnectionStatus{State: "online"})
		if err != nil {
			return ipc.NewErrorResponse(request.RequestID, "internal_error", "Failed to create response")
		}
		return response
	}))
	service := newStartedConnectionService(t, client, context.Background())

	if _, err := service.Status(); err == nil || !strings.Contains(err.Error(), "invalid connection state") {
		t.Fatalf("Status() error = %v, want invalid-state error", err)
	}
}

func TestConnectionServiceRequiresStartup(t *testing.T) {
	service, err := NewConnectionService(blockingDoer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Status(); err == nil || !strings.Contains(err.Error(), "not started") {
		t.Fatalf("Status() error = %v, want not-started error", err)
	}
}

func TestConnectionServiceBoundsAndCancelsRequests(t *testing.T) {
	doer := blockingDoer{started: make(chan context.Context, 1)}
	service, err := NewConnectionService(doer)
	if err != nil {
		t.Fatal(err)
	}
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	if err := service.ServiceStartup(appCtx, application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		_, err := service.Status()
		result <- err
	}()
	requestCtx := <-doer.started
	if _, ok := requestCtx.Deadline(); !ok {
		t.Fatal("request context has no deadline")
	}
	cancelApp()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Status() error = %v, want context cancellation", err)
	}
}

type blockingDoer struct {
	started chan context.Context
}

func (d blockingDoer) Do(ctx context.Context, _ ipc.Request) (ipc.Response, error) {
	d.started <- ctx
	<-ctx.Done()
	return ipc.Response{}, ctx.Err()
}

func newStartedConnectionService(t *testing.T, client requestDoer, ctx context.Context) *ConnectionService {
	t.Helper()
	service, err := NewConnectionService(client)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ServiceStartup(ctx, application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	return service
}

func startFakeDaemon(t *testing.T, handler ipc.Handler) ipc.Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	server, err := ipc.NewServer(path, handler, ipc.AuthorizerFunc(func(context.Context, ipc.PeerCredentials) error {
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			t.Errorf("shutdown fake daemon: %v", err)
		}
		if err := <-serveDone; err != nil {
			t.Errorf("serve fake daemon: %v", err)
		}
	})
	waitForFakeSocket(t, path)
	return ipc.Client{SocketPath: path}
}

func waitForFakeSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("fake daemon socket %s was not created", path)
}
