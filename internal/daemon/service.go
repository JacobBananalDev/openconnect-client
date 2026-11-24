package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
)

// Service owns a VPN engine and exposes its lifecycle through IPC.
type Service struct {
	mu       sync.Mutex
	engine   *vpn.Engine
	server   *ipc.Server
	policy   func(vpn.ConnectionConfig) bool
	stopping atomic.Bool
}

// NewService creates a service. A nil policy denies every connect request.
func NewService(path string, runner vpn.ProcessRunner, authorizer ipc.Authorizer, policy func(vpn.ConnectionConfig) bool) (*Service, error) {
	if runner == nil {
		return nil, errors.New("VPN process runner is required")
	}
	service := &Service{engine: vpn.NewEngine(runner), policy: policy}
	server, err := ipc.NewServer(path, service, authorizer)
	if err != nil {
		return nil, err
	}
	service.server = server
	return service, nil
}

// Serve listens for authorized IPC requests.
func (s *Service) Serve(ctx context.Context) error {
	return s.server.Serve(ctx)
}

// Shutdown stops IPC and disconnects the owned VPN process.
func (s *Service) Shutdown(ctx context.Context) error {
	s.stopping.Store(true)
	stopCtx, cancel := context.WithTimeout(ctx, shutdownTimeout-time.Second)
	defer cancel()
	serverErr := s.server.Shutdown(stopCtx)
	s.mu.Lock()
	defer s.mu.Unlock()
	disconnectErr := s.engine.Disconnect(stopCtx)
	if errors.Is(disconnectErr, context.DeadlineExceeded) && ctx.Err() == nil {
		disconnectErr = errors.Join(disconnectErr, s.engine.Disconnect(ctx))
	}
	return errors.Join(serverErr, disconnectErr)
}

// Handle executes a validated request from an authorized peer.
func (s *Service) Handle(ctx context.Context, request ipc.Request, _ ipc.PeerCredentials) ipc.Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopping.Load() || ctx.Err() != nil {
		return ipc.NewErrorResponse(request.RequestID, "internal_error", "Daemon is stopping or request expired")
	}

	switch request.Command {
	case ipc.CommandConnect:
		var params ipc.ConnectParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return ipc.NewErrorResponse(request.RequestID, "invalid_request", "Invalid connection parameters")
		}
		config := vpn.ConnectionConfig{
			ServerURL: params.Server,
			Protocol:  params.Protocol,
			CACert:    params.CACert,
			Script:    params.Script,
		}
		if err := config.Validate(); err != nil {
			return ipc.NewErrorResponse(request.RequestID, "invalid_request", "Invalid connection parameters")
		}
		if s.policy == nil || !s.policy(config) {
			return ipc.NewErrorResponse(request.RequestID, "policy_denied", "Connection is not permitted")
		}
		if err := s.engine.Connect(context.Background(), config); err != nil {
			if errors.Is(err, vpn.ErrAlreadyConnecting) {
				return ipc.NewErrorResponse(request.RequestID, "already_active", "A connection is already active")
			}
			log.Printf("occlientd: failed to start OpenConnect: %v", err)
			return ipc.NewErrorResponse(request.RequestID, "process_error", "Failed to start OpenConnect")
		}
	case ipc.CommandDisconnect:
		disconnectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.engine.Disconnect(disconnectCtx); err != nil {
			log.Printf("occlientd: failed to disconnect OpenConnect: %v", err)
			if disconnectCtx.Err() != nil {
				return ipc.NewErrorResponse(request.RequestID, "timeout", "Disconnect timed out")
			}
			return ipc.NewErrorResponse(request.RequestID, "internal_error", "Failed to disconnect")
		}
	case ipc.CommandStatus:
	default:
		return ipc.NewErrorResponse(request.RequestID, "invalid_request", "Unknown command")
	}

	response, err := ipc.NewSuccessResponse(request.RequestID, struct {
		State string `json:"state"`
	}{State: s.engine.State().String()})
	if err != nil {
		return ipc.NewErrorResponse(request.RequestID, "internal_error", "Failed to encode response")
	}
	return response
}
