package gui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const daemonRequestTimeout = 10 * time.Second

type requestDoer interface {
	Do(context.Context, ipc.Request) (ipc.Response, error)
}

// ConnectOptions contains the non-secret connection fields accepted by IPC.
type ConnectOptions struct {
	Server   string `json:"server"`
	Protocol string `json:"protocol"`
	CACert   string `json:"ca_cert,omitempty"`
}

// ConnectionStatus is the daemon's reported VPN process state.
type ConnectionStatus struct {
	State string `json:"state"`
}

// ConnectionService exposes the supported daemon operations to Wails.
type ConnectionService struct {
	client requestDoer

	mu  sync.RWMutex
	ctx context.Context
}

// NewConnectionService creates a Wails service backed by the shared IPC client.
func NewConnectionService(client requestDoer) (*ConnectionService, error) {
	if client == nil {
		return nil, errors.New("IPC client is required")
	}
	return &ConnectionService{client: client}, nil
}

// ServiceStartup stores the application context used to bound and cancel IPC requests.
func (s *ConnectionService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if ctx == nil {
		return errors.New("application context is required")
	}

	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	return nil
}

// Status returns the current daemon connection state.
func (s *ConnectionService) Status() (ConnectionStatus, error) {
	return s.request(ipc.CommandStatus, struct{}{})
}

// Connect requests a policy-authorized daemon connection.
func (s *ConnectionService) Connect(options ConnectOptions) (ConnectionStatus, error) {
	params := ipc.ConnectParams{
		Server:   options.Server,
		Protocol: options.Protocol,
		CACert:   options.CACert,
	}
	return s.request(ipc.CommandConnect, params)
}

// Disconnect requests that the daemon stop its connection.
func (s *ConnectionService) Disconnect() (ConnectionStatus, error) {
	return s.request(ipc.CommandDisconnect, struct{}{})
}

func (s *ConnectionService) request(command ipc.Command, params any) (ConnectionStatus, error) {
	s.mu.RLock()
	parent := s.ctx
	s.mu.RUnlock()
	if parent == nil {
		return ConnectionStatus{}, errors.New("connection service is not started")
	}

	ctx, cancel := context.WithTimeout(parent, daemonRequestTimeout)
	defer cancel()

	requestID, err := newRequestID()
	if err != nil {
		return ConnectionStatus{}, err
	}
	encodedParams, err := json.Marshal(params)
	if err != nil {
		return ConnectionStatus{}, fmt.Errorf("encode daemon parameters: %w", err)
	}
	request := ipc.Request{
		Version:   ipc.ProtocolVersion,
		RequestID: requestID,
		Command:   command,
		Params:    encodedParams,
	}

	response, err := s.client.Do(ctx, request)
	if err != nil {
		return ConnectionStatus{}, fmt.Errorf("daemon %s request failed: %w", command, err)
	}
	if response.Version != ipc.ProtocolVersion || response.RequestID != requestID {
		return ConnectionStatus{}, errors.New("daemon returned a mismatched response")
	}
	if !response.OK {
		if response.Error == nil || strings.TrimSpace(response.Error.Code) == "" || strings.TrimSpace(response.Error.Message) == "" {
			return ConnectionStatus{}, errors.New("daemon returned an invalid error response")
		}
		return ConnectionStatus{}, fmt.Errorf(
			"daemon rejected %s request: %s (%s)",
			command,
			response.Error.Message,
			response.Error.Code,
		)
	}
	if response.Error != nil || len(response.Data) == 0 {
		return ConnectionStatus{}, errors.New("daemon returned an invalid success response")
	}

	var status ConnectionStatus
	if err := ipc.DecodeStrictJSON(response.Data, &status); err != nil {
		return ConnectionStatus{}, fmt.Errorf("decode daemon status: %w", err)
	}
	if !validDaemonState(status.State) {
		return ConnectionStatus{}, errors.New("daemon returned an invalid connection state")
	}
	return status, nil
}

func newRequestID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate request ID: %w", err)
	}
	return hex.EncodeToString(id[:]), nil
}

func validDaemonState(state string) bool {
	switch state {
	case "disconnected", "connecting", "connected", "disconnecting", "failed":
		return true
	default:
		return false
	}
}
