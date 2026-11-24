package ipc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	clientDeadline = 10 * time.Second
	serverDeadline = 10 * time.Second
	soPeerGroups   = 59
	maxPeerGroups  = 65536
)

// DefaultSocketPath is the system daemon's control socket.
const DefaultSocketPath = "/run/openconnect-client/control.sock"

// MaxConcurrentConnections bounds the number of requests handled at once.
const MaxConcurrentConnections = 32

// Handler processes a validated request from an authorized peer.
type Handler interface {
	Handle(ctx context.Context, request Request, peer PeerCredentials) Response
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(context.Context, Request, PeerCredentials) Response

// Handle invokes f.
func (f HandlerFunc) Handle(ctx context.Context, request Request, peer PeerCredentials) Response {
	return f(ctx, request, peer)
}

// Authorizer decides whether a peer may issue control requests.
type Authorizer interface {
	Authorize(ctx context.Context, peer PeerCredentials) error
}

// AuthorizerFunc adapts a function to Authorizer.
type AuthorizerFunc func(context.Context, PeerCredentials) error

// Authorize invokes f.
func (f AuthorizerFunc) Authorize(ctx context.Context, peer PeerCredentials) error {
	return f(ctx, peer)
}

// Client sends one request to a daemon Unix socket.
type Client struct {
	SocketPath string
}

// Do sends request and returns its correlated response.
func (c Client) Do(ctx context.Context, request Request) (Response, error) {
	if ctx == nil {
		return Response{}, errors.New("IPC client context is required")
	}
	if !filepath.IsAbs(c.SocketPath) {
		return Response{}, errors.New("IPC socket path must be absolute")
	}

	frame, err := EncodeRequest(request)
	if err != nil {
		return Response{}, err
	}

	exchangeCtx, cancel := context.WithTimeout(ctx, clientDeadline)
	defer cancel()

	dialer := net.Dialer{}
	connection, err := dialer.DialContext(exchangeCtx, "unix", c.SocketPath)
	if err != nil {
		return Response{}, fmt.Errorf("connect to daemon socket: %w", err)
	}
	defer connection.Close()
	stopClose := context.AfterFunc(exchangeCtx, func() {
		_ = connection.Close()
	})
	defer stopClose()

	deadline, ok := exchangeCtx.Deadline()
	if !ok {
		return Response{}, errors.New("IPC client deadline is unavailable")
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return Response{}, fmt.Errorf("set IPC client deadline: %w", err)
	}

	unixConnection, ok := connection.(*net.UnixConn)
	if !ok {
		return Response{}, errors.New("daemon connection is not a Unix socket")
	}
	if err := writeFull(unixConnection, frame); err != nil {
		return Response{}, fmt.Errorf("write IPC request: %w", err)
	}
	if err := unixConnection.CloseWrite(); err != nil {
		return Response{}, fmt.Errorf("finish IPC request: %w", err)
	}

	responseFrame, err := readFrame(unixConnection)
	if err != nil {
		return Response{}, fmt.Errorf("read IPC response: %w", err)
	}
	response, err := DecodeResponse(responseFrame)
	if err != nil {
		return Response{}, err
	}
	if response.RequestID != request.RequestID {
		return Response{}, errors.New("IPC response request_id does not match request")
	}
	return response, nil
}

// Server accepts bounded requests from authorized local peers.
type Server struct {
	path       string
	handler    Handler
	authorizer Authorizer
	semaphore  chan struct{}
	serveDone  chan struct{}

	mu           sync.Mutex
	started      bool
	listener     *net.UnixListener
	cancel       context.CancelFunc
	connections  map[*net.UnixConn]struct{}
	connectionWG sync.WaitGroup
}

// NewServer creates a single-use Unix socket server.
func NewServer(path string, handler Handler, authorizer Authorizer) (*Server, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("IPC socket path must be absolute")
	}
	if handler == nil {
		return nil, errors.New("IPC request handler is required")
	}
	if authorizer == nil {
		return nil, errors.New("IPC peer authorizer is required")
	}
	return &Server{
		path:        path,
		handler:     handler,
		authorizer:  authorizer,
		semaphore:   make(chan struct{}, MaxConcurrentConnections),
		serveDone:   make(chan struct{}),
		connections: make(map[*net.UnixConn]struct{}),
	}, nil
}

// Serve listens until ctx is canceled, the listener is shut down, or an
// unrecoverable accept error occurs.
func (s *Server) Serve(ctx context.Context) (returnErr error) {
	if ctx == nil {
		return errors.New("IPC server context is required")
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		cancel()
		return errors.New("IPC server can only be served once")
	}
	s.started = true
	s.cancel = cancel
	s.mu.Unlock()

	var listener *net.UnixListener
	var socketInfo os.FileInfo
	defer func() {
		cancel()
		if listener != nil {
			_ = listener.Close()
		}
		s.closeConnections()
		s.waitConnections()
		if socketInfo != nil {
			returnErr = errors.Join(returnErr, removeOwnedSocket(s.path, socketInfo))
		}
		close(s.serveDone)
	}()

	if runCtx.Err() != nil {
		return nil
	}
	if _, err := os.Lstat(s.path); err == nil {
		return fmt.Errorf("IPC socket path already exists: %s", s.path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect IPC socket path: %w", err)
	}

	address := &net.UnixAddr{Name: s.path, Net: "unix"}
	listener, returnErr = net.ListenUnix("unix", address)
	if returnErr != nil {
		return fmt.Errorf("listen on IPC socket: %w", returnErr)
	}
	listener.SetUnlinkOnClose(false)

	socketInfo, returnErr = os.Lstat(s.path)
	if returnErr != nil {
		return fmt.Errorf("inspect created IPC socket: %w", returnErr)
	}
	if socketInfo.Mode()&os.ModeSocket == 0 {
		return errors.New("created IPC path is not a socket")
	}
	if returnErr = os.Chmod(s.path, 0o660); returnErr != nil {
		return fmt.Errorf("set IPC socket permissions: %w", returnErr)
	}

	s.mu.Lock()
	s.listener = listener
	shuttingDown := runCtx.Err() != nil
	s.mu.Unlock()
	if shuttingDown {
		return nil
	}

	stopClose := context.AfterFunc(runCtx, func() {
		_ = listener.Close()
	})
	defer stopClose()

	for {
		connection, err := listener.AcceptUnix()
		if err != nil {
			if runCtx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept IPC connection: %w", err)
		}
		s.accept(connection, runCtx)
	}
}

// Shutdown closes the listener and active connections, then waits for Serve to
// finish cleanup.
func (s *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("IPC shutdown context is required")
	}

	s.mu.Lock()
	started := s.started
	cancel := s.cancel
	listener := s.listener
	s.mu.Unlock()
	if !started {
		return nil
	}

	if cancel != nil {
		cancel()
	}
	var closeErr error
	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			closeErr = fmt.Errorf("close IPC listener: %w", err)
		}
	}
	s.closeConnections()

	select {
	case <-s.serveDone:
		return closeErr
	case <-ctx.Done():
		return errors.Join(closeErr, fmt.Errorf("wait for IPC server shutdown: %w", ctx.Err()))
	}
}

func (s *Server) accept(connection *net.UnixConn, ctx context.Context) {
	select {
	case s.semaphore <- struct{}{}:
	default:
		_ = connection.Close()
		return
	}

	s.mu.Lock()
	s.connections[connection] = struct{}{}
	s.connectionWG.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.connectionWG.Done()
		defer func() {
			<-s.semaphore
			s.mu.Lock()
			delete(s.connections, connection)
			s.mu.Unlock()
			_ = connection.Close()
		}()
		s.handleConnection(connection, ctx)
	}()
}

func (s *Server) handleConnection(connection *net.UnixConn, parent context.Context) {
	requestCtx, cancel := context.WithTimeout(parent, serverDeadline)
	defer cancel()
	stopClose := context.AfterFunc(requestCtx, func() {
		_ = connection.Close()
	})
	defer stopClose()

	deadline, ok := requestCtx.Deadline()
	if !ok || connection.SetDeadline(deadline) != nil {
		return
	}

	data, err := readFrame(connection)
	if err != nil {
		return
	}
	request, err := DecodeRequest(data)
	if err != nil {
		var protocolErr *ProtocolError
		if !errors.As(err, &protocolErr) || !validRequestID(request.RequestID) {
			return
		}
		_ = writeResponse(connection, NewErrorResponse(request.RequestID, protocolErr.Code, protocolErr.Message))
		return
	}

	peer, err := getPeerCredentials(connection)
	if err != nil {
		log.Printf("occlientd: could not read peer credentials: %v", err)
		_ = writeResponse(connection, NewErrorResponse(request.RequestID, "unauthorized", "Client is not authorized"))
		return
	}
	if err := s.authorizer.Authorize(requestCtx, peer); err != nil {
		_ = writeResponse(connection, NewErrorResponse(request.RequestID, "unauthorized", "Client is not authorized"))
		return
	}

	response := s.handler.Handle(requestCtx, request, peer)
	if response.RequestID != request.RequestID || response.Version != ProtocolVersion {
		response = NewErrorResponse(request.RequestID, "internal_error", "Daemon could not process the request")
	}
	if err := writeResponse(connection, response); err != nil {
		return
	}
}

func (s *Server) closeConnections() {
	s.mu.Lock()
	connections := make([]*net.UnixConn, 0, len(s.connections))
	for connection := range s.connections {
		connections = append(connections, connection)
	}
	s.mu.Unlock()

	for _, connection := range connections {
		_ = connection.Close()
	}
}

func (s *Server) waitConnections() {
	s.connectionWG.Wait()
}

func writeResponse(connection *net.UnixConn, response Response) error {
	frame, err := EncodeResponse(response)
	if err != nil {
		fallback := NewErrorResponse(response.RequestID, "internal_error", "Daemon could not process the request")
		frame, err = EncodeResponse(fallback)
		if err != nil {
			return err
		}
	}
	return writeFull(connection, frame)
}

func readFrame(reader io.Reader) ([]byte, error) {
	frame, err := io.ReadAll(io.LimitReader(reader, MaxMessageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(frame) > MaxMessageBytes {
		return nil, errors.New("IPC message exceeds maximum size")
	}
	if len(frame) == 0 || frame[len(frame)-1] != '\n' || bytes.Count(frame, []byte{'\n'}) != 1 {
		return nil, errors.New("IPC message must be one newline-terminated JSON object")
	}
	data := frame[:len(frame)-1]
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("IPC message is empty")
	}
	return data, nil
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func getPeerCredentials(connection *net.UnixConn) (PeerCredentials, error) {
	rawConnection, err := connection.SyscallConn()
	if err != nil {
		return PeerCredentials{}, err
	}

	var credentials *syscall.Ucred
	var groups []uint32
	var controlErr error
	if err := rawConnection.Control(func(fd uintptr) {
		credentials, controlErr = syscall.GetsockoptUcred(
			int(fd),
			syscall.SOL_SOCKET,
			syscall.SO_PEERCRED,
		)
		if controlErr == nil {
			groups, controlErr = getPeerGroups(fd)
		}
	}); err != nil {
		return PeerCredentials{}, err
	}
	if controlErr != nil {
		return PeerCredentials{}, controlErr
	}
	return PeerCredentials{
		PID:    int(credentials.Pid),
		UID:    credentials.Uid,
		GID:    credentials.Gid,
		Groups: groups,
	}, nil
}

func getPeerGroups(fd uintptr) ([]uint32, error) {
	groups := make([]uint32, 16)
	for {
		length := uint32(len(groups) * 4)
		errno := getPeerGroupsSockopt(fd, groups, &length)
		if errno == syscall.ERANGE && length > uint32(len(groups)*4) && length <= maxPeerGroups*4 && length%4 == 0 {
			groups = make([]uint32, length/4)
			continue
		}
		if errno != 0 {
			return nil, fmt.Errorf("get peer supplementary groups: %w", errno)
		}
		if length > uint32(len(groups)*4) || length%4 != 0 {
			return nil, errors.New("invalid peer group list")
		}
		return groups[:length/4], nil
	}
}

func removeOwnedSocket(path string, createdInfo os.FileInfo) error {
	currentInfo, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect IPC socket during cleanup: %w", err)
	}
	if currentInfo.Mode()&os.ModeSocket == 0 || !os.SameFile(createdInfo, currentInfo) {
		return errors.New("IPC socket path changed; refusing to remove it")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove IPC socket: %w", err)
	}
	return nil
}
