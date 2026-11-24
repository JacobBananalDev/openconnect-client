package ipc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// ProtocolVersion is the IPC schema version supported by this package.
const ProtocolVersion = 1

// MaxMessageBytes is the largest encoded request or response, including its
// terminating newline.
const MaxMessageBytes = 4 * 1024

// Command identifies an operation in a protocol v1 request.
type Command string

// CommandConnect, CommandStatus, and CommandDisconnect are the supported
// protocol v1 command names.
const (
	CommandConnect    Command = "connect"
	CommandStatus     Command = "status"
	CommandDisconnect Command = "disconnect"
)

// Request is one versioned command sent over the control socket.
type Request struct {
	Version   int             `json:"version"`
	RequestID string          `json:"request_id"`
	Command   Command         `json:"command"`
	Params    json.RawMessage `json:"params"`
}

// ConnectParams contains the connection options accepted by protocol v1.
type ConnectParams struct {
	Server   string `json:"server"`
	Protocol string `json:"protocol"`
	CACert   string `json:"ca_cert,omitempty"`
	Script   string `json:"script,omitempty"`
}

// Response is the result of one request.
type Response struct {
	Version   int             `json:"version"`
	RequestID string          `json:"request_id"`
	OK        bool            `json:"ok"`
	Data      json.RawMessage `json:"data"`
	Error     *ResponseError  `json:"error"`
}

// ResponseError is a stable protocol error code and a safe user-facing message.
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PeerCredentials identifies a process connected to the Unix socket.
type PeerCredentials struct {
	PID    int
	UID    uint32
	GID    uint32
	Groups []uint32
}

// ProtocolError is a deterministic request or response validation failure.
type ProtocolError struct {
	Code    string
	Message string
}

func (e *ProtocolError) Error() string {
	return e.Message
}

// NewSuccessResponse creates a successful response with JSON-encoded data.
func NewSuccessResponse(requestID string, data any) (Response, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Response{}, fmt.Errorf("encode response data: %w", err)
	}
	response := Response{
		Version:   ProtocolVersion,
		RequestID: requestID,
		OK:        true,
		Data:      raw,
	}
	if err := validateResponse(response); err != nil {
		return Response{}, err
	}
	return response, nil
}

// NewErrorResponse creates a failed response with a stable code and message.
func NewErrorResponse(requestID, code, message string) Response {
	return Response{
		Version:   ProtocolVersion,
		RequestID: requestID,
		Data:      json.RawMessage("null"),
		Error: &ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

// EncodeRequest validates and encodes a request as a newline-terminated frame.
func EncodeRequest(request Request) ([]byte, error) {
	if err := validateRequest(request); err != nil {
		return nil, err
	}
	return encodeFrame(request)
}

// DecodeRequest strictly decodes and validates one JSON request.
func DecodeRequest(data []byte) (Request, error) {
	if len(data)+1 > MaxMessageBytes {
		return Request{}, invalidRequest("request exceeds maximum size")
	}
	var request Request
	if err := DecodeStrictJSON(data, &request); err != nil {
		return Request{}, invalidRequest("malformed request")
	}
	if err := validateRequest(request); err != nil {
		return request, err
	}
	return request, nil
}

// EncodeResponse validates and encodes a response as a newline-terminated frame.
func EncodeResponse(response Response) ([]byte, error) {
	if err := validateResponse(response); err != nil {
		return nil, err
	}
	return encodeFrame(response)
}

// DecodeResponse strictly decodes and validates one JSON response.
func DecodeResponse(data []byte) (Response, error) {
	if len(data)+1 > MaxMessageBytes {
		return Response{}, errors.New("response exceeds maximum size")
	}
	var response Response
	if err := DecodeStrictJSON(data, &response); err != nil {
		return Response{}, errors.New("malformed response")
	}
	if err := validateResponse(response); err != nil {
		return Response{}, err
	}
	return response, nil
}

func validateRequest(request Request) error {
	if request.Version != ProtocolVersion {
		return &ProtocolError{Code: "unsupported_version", Message: "Unsupported protocol version"}
	}
	if !validRequestID(request.RequestID) {
		return invalidRequest("request_id must be 1 to 64 printable ASCII bytes")
	}

	switch request.Command {
	case CommandConnect:
		var params ConnectParams
		if err := decodeParams(request.Params, &params); err != nil {
			return invalidRequest("invalid connect parameters")
		}
		if strings.TrimSpace(params.Server) == "" || strings.TrimSpace(params.Protocol) == "" {
			return invalidRequest("connect requires server and protocol")
		}
	case CommandStatus, CommandDisconnect:
		var params struct{}
		if err := decodeParams(request.Params, &params); err != nil {
			return invalidRequest("invalid command parameters")
		}
	default:
		return invalidRequest("unknown command")
	}
	return nil
}

func validateResponse(response Response) error {
	if response.Version != ProtocolVersion {
		return &ProtocolError{Code: "unsupported_version", Message: "Unsupported protocol version"}
	}
	if !validRequestID(response.RequestID) {
		return errors.New("response has invalid request_id")
	}

	if response.OK {
		if response.Error != nil || len(response.Data) == 0 || bytes.Equal(bytes.TrimSpace(response.Data), []byte("null")) {
			return errors.New("successful response must contain data and no error")
		}
		if err := rejectDuplicateKeys(response.Data); err != nil {
			return errors.New("response data is malformed")
		}
		return nil
	}

	if response.Error == nil || strings.TrimSpace(response.Error.Code) == "" || strings.TrimSpace(response.Error.Message) == "" {
		return errors.New("failed response must contain an error")
	}
	if len(response.Data) > 0 && !bytes.Equal(bytes.TrimSpace(response.Data), []byte("null")) {
		return errors.New("failed response data must be null")
	}
	return nil
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return errors.New("parameters must be an object")
	}
	return DecodeStrictJSON(raw, target)
}

// DecodeStrictJSON rejects invalid UTF-8, duplicate keys, unknown fields, and
// trailing JSON values.
func DecodeStrictJSON(data []byte, target any) error {
	if !utf8.Valid(data) {
		return errors.New("JSON must be valid UTF-8")
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return nil
	}

	switch delim {
	case '{':
		var seen []string
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			for _, previous := range seen {
				if strings.EqualFold(previous, key) {
					return fmt.Errorf("duplicate object key %q", key)
				}
			}
			seen = append(seen, key)
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err := decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err := decoder.Token()
		return err
	default:
		return errors.New("unexpected JSON delimiter")
	}
}

func validRequestID(requestID string) bool {
	if len(requestID) == 0 || len(requestID) > 64 {
		return false
	}
	for i := 0; i < len(requestID); i++ {
		if requestID[i] < 0x20 || requestID[i] > 0x7e {
			return false
		}
	}
	return true
}

func invalidRequest(message string) *ProtocolError {
	return &ProtocolError{Code: "invalid_request", Message: message}
}

func encodeFrame(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode IPC message: %w", err)
	}
	data = append(data, '\n')
	if len(data) > MaxMessageBytes {
		return nil, errors.New("IPC message exceeds maximum size")
	}
	return data, nil
}
