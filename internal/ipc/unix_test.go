package ipc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestClientServerRoundTripAndSocketPermissions(t *testing.T) {
	peerReceived := make(chan PeerCredentials, 1)
	server, stop := startTestServer(t,
		HandlerFunc(func(ctx context.Context, request Request, peer PeerCredentials) Response {
			peerReceived <- peer
			response, err := NewSuccessResponse(request.RequestID, map[string]string{"state": "disconnected"})
			if err != nil {
				return NewErrorResponse(request.RequestID, "internal_error", "Failed to create response")
			}
			return response
		}),
		allowPeer(),
	)

	info, err := os.Stat(server.path)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o660 {
		t.Fatalf("socket permissions = %04o, want 0660", got)
	}

	response, err := (Client{SocketPath: server.path}).Do(context.Background(), statusRequest())
	if err != nil {
		t.Fatalf("Client.Do() error = %v", err)
	}
	if !response.OK || response.RequestID != "status-1" {
		t.Fatalf("response = %+v, want successful correlated response", response)
	}

	select {
	case peer := <-peerReceived:
		if peer.PID != os.Getpid() || peer.UID != uint32(os.Geteuid()) || peer.GID != uint32(os.Getegid()) {
			t.Fatalf("peer identity = %+v, want current process credentials", peer)
		}
		groups, err := os.Getgroups()
		if err != nil {
			t.Fatal(err)
		}
		if len(peer.Groups) != len(groups) {
			t.Fatalf("peer groups = %v, want %v", peer.Groups, groups)
		}
		for i, group := range groups {
			if peer.Groups[i] != uint32(group) {
				t.Fatalf("peer groups = %v, want %v", peer.Groups, groups)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not receive peer credentials")
	}

	if err := stop(); err != nil {
		t.Fatalf("stop server: %v", err)
	}
	if _, err := os.Lstat(server.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket path after shutdown: err = %v, want not-exist", err)
	}
}

func TestServerRejectsUnauthorizedPeerBeforeHandler(t *testing.T) {
	var handlerCalled atomic.Bool
	server, stop := startTestServer(t,
		HandlerFunc(func(context.Context, Request, PeerCredentials) Response {
			handlerCalled.Store(true)
			return Response{}
		}),
		AuthorizerFunc(func(context.Context, PeerCredentials) error {
			return errors.New("denied")
		}),
	)

	response, err := (Client{SocketPath: server.path}).Do(context.Background(), statusRequest())
	if err != nil {
		t.Fatalf("Client.Do() error = %v", err)
	}
	if response.OK || response.Error == nil || response.Error.Code != "unauthorized" {
		t.Fatalf("response = %+v, want unauthorized error", response)
	}
	if handlerCalled.Load() {
		t.Fatal("handler was called for an unauthorized peer")
	}
	if err := stop(); err != nil {
		t.Fatalf("stop server: %v", err)
	}
}

func TestServerRejectsMalformedAndOversizedFrames(t *testing.T) {
	var handlerCalled atomic.Bool
	server, stop := startTestServer(t,
		HandlerFunc(func(context.Context, Request, PeerCredentials) Response {
			handlerCalled.Store(true)
			return Response{}
		}),
		allowPeer(),
	)
	validFrame, err := EncodeRequest(statusRequest())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		frame []byte
	}{
		{name: "missing newline", frame: validFrame[:len(validFrame)-1]},
		{name: "extra newline", frame: append(append([]byte(nil), validFrame...), '\n')},
		{name: "invalid JSON", frame: []byte("{\n")},
		{name: "oversized", frame: append(bytes.Repeat([]byte{'x'}, MaxMessageBytes), '\n')},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: server.path, Net: "unix"})
			if err != nil {
				t.Fatalf("dial socket: %v", err)
			}
			if err := writeFull(connection, test.frame); err != nil {
				t.Fatalf("write frame: %v", err)
			}
			if err := connection.CloseWrite(); err != nil {
				t.Fatalf("close request stream: %v", err)
			}
			if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			response, err := io.ReadAll(connection)
			_ = connection.Close()
			if err != nil {
				t.Fatalf("read rejected response: %v", err)
			}
			if len(response) != 0 {
				t.Fatalf("rejected frame response = %q, want connection closed", response)
			}
		})
	}
	if handlerCalled.Load() {
		t.Fatal("handler was called for an invalid frame")
	}
	if err := stop(); err != nil {
		t.Fatalf("stop server: %v", err)
	}
}

func TestServerCapsConcurrentConnections(t *testing.T) {
	started := make(chan struct{}, MaxConcurrentConnections+1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() {
		releaseOnce.Do(func() { close(release) })
	}
	server, stop := startTestServer(t,
		HandlerFunc(func(ctx context.Context, request Request, _ PeerCredentials) Response {
			started <- struct{}{}
			select {
			case <-release:
				response, _ := NewSuccessResponse(request.RequestID, map[string]string{"state": "disconnected"})
				return response
			case <-ctx.Done():
				return NewErrorResponse(request.RequestID, "timeout", "Request timed out")
			}
		}),
		allowPeer(),
	)
	defer func() {
		releaseAll()
		if err := stop(); err != nil {
			t.Errorf("stop server: %v", err)
		}
	}()

	frame, err := EncodeRequest(statusRequest())
	if err != nil {
		t.Fatal(err)
	}
	connections := make([]*net.UnixConn, 0, MaxConcurrentConnections)
	for i := 0; i < MaxConcurrentConnections; i++ {
		connection := dialAndSend(t, server.path, frame)
		connections = append(connections, connection)
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("only %d concurrent handlers started", i)
		}
	}

	overflow, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: server.path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	sendErr := writeFull(overflow, frame)
	if sendErr == nil {
		sendErr = overflow.CloseWrite()
	}
	if sendErr != nil && !errors.Is(sendErr, syscall.EPIPE) && !errors.Is(sendErr, syscall.ECONNRESET) && !errors.Is(sendErr, syscall.ENOTCONN) {
		t.Fatalf("send overflow request: %v", sendErr)
	}
	if err := overflow.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	overflowResponse, err := io.ReadAll(overflow)
	_ = overflow.Close()
	if err != nil && !errors.Is(err, syscall.ECONNRESET) {
		t.Fatalf("read overflow connection: %v", err)
	}
	if len(overflowResponse) != 0 {
		t.Fatalf("overflow response = %q, want connection closed", overflowResponse)
	}
	select {
	case <-started:
		t.Fatal("server exceeded its concurrent connection limit")
	default:
	}

	releaseAll()
	for _, connection := range connections {
		if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		_, err := io.ReadAll(connection)
		_ = connection.Close()
		if err != nil {
			t.Fatalf("read concurrent response: %v", err)
		}
	}
}

func TestServerRefusesOccupiedSocketPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(path, HandlerFunc(func(context.Context, Request, PeerCredentials) Response {
		return Response{}
	}), allowPeer())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background()); err == nil {
		t.Fatal("Serve() error = nil, want occupied-path error")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing path: %v", err)
	}
	if string(content) != "keep" {
		t.Fatalf("existing path content = %q, want preserved", content)
	}
}

func TestServerRefusesStaleSocketPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(path, HandlerFunc(func(context.Context, Request, PeerCredentials) Response {
		return Response{}
	}), allowPeer())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background()); err == nil {
		t.Fatal("Serve() error = nil, want occupied-path error")
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("stale socket path was removed or changed: info=%v err=%v", info, err)
	}
}

func TestServerDoesNotRemoveReplacedSocketPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	server, err := NewServer(path, HandlerFunc(func(context.Context, Request, PeerCredentials) Response {
		return Response{}
	}), allowPeer())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(ctx)
	}()
	waitForSocket(t, path)

	if err := os.Remove(path); err != nil {
		t.Fatalf("replace socket path: %v", err)
	}
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := <-serveDone; err == nil || !strings.Contains(err.Error(), "refusing to remove") {
		t.Fatalf("Serve() error = %v, want safe-cleanup error", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replacement path: %v", err)
	}
	if string(content) != "keep" {
		t.Fatalf("replacement path content = %q, want preserved", content)
	}
}

func TestClientHonorsContextDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		connection, err := listener.AcceptUnix()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = readFrame(connection)
		time.Sleep(100 * time.Millisecond)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = (Client{SocketPath: path}).Do(ctx, statusRequest())
	if err == nil {
		t.Fatal("Client.Do() error = nil, want deadline error")
	}
	<-serverDone
}

func TestClientRejectsMismatchedResponseID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		connection, err := listener.AcceptUnix()
		if err != nil {
			serverDone <- err
			return
		}
		defer connection.Close()
		if _, err := readFrame(connection); err != nil {
			serverDone <- err
			return
		}
		response, err := NewSuccessResponse("different-id", map[string]string{"state": "disconnected"})
		if err != nil {
			serverDone <- err
			return
		}
		frame, err := EncodeResponse(response)
		if err == nil {
			err = writeFull(connection, frame)
		}
		serverDone <- err
	}()

	_, err = (Client{SocketPath: path}).Do(context.Background(), statusRequest())
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Client.Do() error = %v, want request ID mismatch", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("fake server error: %v", err)
	}
}

func startTestServer(t *testing.T, handler Handler, authorizer Authorizer) (*Server, func() error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	server, err := NewServer(path, handler, authorizer)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(ctx)
	}()
	waitForSocket(t, path)

	var stopOnce sync.Once
	var stopErr error
	stop := func() error {
		stopOnce.Do(func() {
			cancel()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
			defer shutdownCancel()
			shutdownErr := server.Shutdown(shutdownCtx)
			stopErr = errors.Join(shutdownErr, <-serveDone)
		})
		return stopErr
	}
	t.Cleanup(func() {
		if err := stop(); err != nil {
			t.Errorf("stop test server: %v", err)
		}
	})
	return server, stop
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("socket %s was not created", path)
}

func allowPeer() Authorizer {
	return AuthorizerFunc(func(context.Context, PeerCredentials) error { return nil })
}

func statusRequest() Request {
	return Request{
		Version:   ProtocolVersion,
		RequestID: "status-1",
		Command:   CommandStatus,
		Params:    []byte("{}"),
	}
}

func dialAndSend(t *testing.T, path string, frame []byte) *net.UnixConn {
	t.Helper()
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	if err := writeFull(connection, frame); err != nil {
		t.Fatalf("write request: %v", err)
	}
	if err := connection.CloseWrite(); err != nil {
		t.Fatalf("close request stream: %v", err)
	}
	return connection
}
