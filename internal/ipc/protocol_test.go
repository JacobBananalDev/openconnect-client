package ipc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEncodeDecodeRequest(t *testing.T) {
	request := connectRequest(t)

	encoded, err := EncodeRequest(request)
	if err != nil {
		t.Fatalf("EncodeRequest() error = %v", err)
	}
	if encoded[len(encoded)-1] != '\n' {
		t.Fatal("encoded request is not newline-terminated")
	}

	decoded, err := DecodeRequest(encoded[:len(encoded)-1])
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if decoded.RequestID != request.RequestID || decoded.Command != CommandConnect {
		t.Fatalf("decoded request = %+v, want request ID %q and connect command", decoded, request.RequestID)
	}

	var params ConnectParams
	if err := json.Unmarshal(decoded.Params, &params); err != nil {
		t.Fatalf("decode connect params: %v", err)
	}
	if params.Server != "vpn.example.com" || params.Protocol != "anyconnect" {
		t.Fatalf("decoded params = %+v, want server and protocol", params)
	}
}

func TestDecodeRequestRejectsInvalidMessages(t *testing.T) {
	tests := []struct {
		name string
		data string
		code string
	}{
		{
			name: "unsupported version",
			data: `{"version":2,"request_id":"r1","command":"status","params":{}}`,
			code: "unsupported_version",
		},
		{
			name: "duplicate top-level key",
			data: `{"version":1,"version":1,"request_id":"r1","command":"status","params":{}}`,
			code: "invalid_request",
		},
		{
			name: "unknown top-level field",
			data: `{"version":1,"request_id":"r1","command":"status","params":{},"extra":true}`,
			code: "invalid_request",
		},
		{
			name: "duplicate parameter key",
			data: `{"version":1,"request_id":"r1","command":"connect","params":{"server":"one","server":"two","protocol":"anyconnect"}}`,
			code: "invalid_request",
		},
		{
			name: "case-insensitive duplicate parameter key",
			data: `{"version":1,"request_id":"r1","command":"connect","params":{"server":"one","SERVER":"two","protocol":"anyconnect"}}`,
			code: "invalid_request",
		},
		{
			name: "unknown parameter field",
			data: `{"version":1,"request_id":"r1","command":"connect","params":{"server":"vpn.example.com","protocol":"anyconnect","extra":true}}`,
			code: "invalid_request",
		},
		{
			name: "empty request ID",
			data: `{"version":1,"request_id":"","command":"status","params":{}}`,
			code: "invalid_request",
		},
		{
			name: "unknown command",
			data: `{"version":1,"request_id":"r1","command":"restart","params":{}}`,
			code: "invalid_request",
		},
		{
			name: "missing connect protocol",
			data: `{"version":1,"request_id":"r1","command":"connect","params":{"server":"vpn.example.com"}}`,
			code: "invalid_request",
		},
		{
			name: "non-object params",
			data: `{"version":1,"request_id":"r1","command":"status","params":null}`,
			code: "invalid_request",
		},
		{
			name: "trailing JSON",
			data: `{"version":1,"request_id":"r1","command":"status","params":{}} {}`,
			code: "invalid_request",
		},
		{
			name: "malformed JSON",
			data: `{"version":1`,
			code: "invalid_request",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeRequest([]byte(test.data))
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) {
				t.Fatalf("DecodeRequest() error = %v, want ProtocolError", err)
			}
			if protocolErr.Code != test.code {
				t.Fatalf("error code = %q, want %q", protocolErr.Code, test.code)
			}
		})
	}
}

func TestEncodeRequestRejectsOversizedMessage(t *testing.T) {
	request := Request{
		Version:   ProtocolVersion,
		RequestID: "r1",
		Command:   CommandConnect,
		Params:    json.RawMessage(`{"server":"` + strings.Repeat("x", MaxMessageBytes) + `","protocol":"anyconnect"}`),
	}
	if _, err := EncodeRequest(request); err == nil {
		t.Fatal("EncodeRequest() error = nil, want size error")
	}
}

func TestDecodeRequestRejectsInvalidUTF8(t *testing.T) {
	data := append(
		[]byte(`{"version":1,"request_id":"r1","command":"status","params":{"value":"`),
		0xff,
	)
	data = append(data, []byte(`"}}`)...)

	if _, err := DecodeRequest(data); err == nil {
		t.Fatal("DecodeRequest() error = nil, want invalid UTF-8 error")
	}
}

func TestDecodeRequestRejectsOversizedMessage(t *testing.T) {
	data := []byte(`{"version":1,"request_id":"r1","command":"status","params":{}}`)
	data = append(data[:len(data)-1], []byte(`,"padding":"`+strings.Repeat("x", MaxMessageBytes)+`"}`)...)
	if _, err := DecodeRequest(data); err == nil {
		t.Fatal("DecodeRequest() error = nil, want size error")
	}
}

func TestEncodeDecodeResponse(t *testing.T) {
	response, err := NewSuccessResponse("r1", map[string]string{"state": "disconnected"})
	if err != nil {
		t.Fatalf("NewSuccessResponse() error = %v", err)
	}
	encoded, err := EncodeResponse(response)
	if err != nil {
		t.Fatalf("EncodeResponse() error = %v", err)
	}
	decoded, err := DecodeResponse(encoded[:len(encoded)-1])
	if err != nil {
		t.Fatalf("DecodeResponse() error = %v", err)
	}
	if !decoded.OK || decoded.RequestID != "r1" {
		t.Fatalf("decoded response = %+v, want success for r1", decoded)
	}

	failure := NewErrorResponse("r1", "unauthorized", "Client is not authorized")
	if _, err := EncodeResponse(failure); err != nil {
		t.Fatalf("EncodeResponse(failure) error = %v", err)
	}
}

func TestDecodeResponseRejectsInvalidMessages(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "duplicate key",
			data: `{"version":1,"version":1,"request_id":"r1","ok":true,"data":{},"error":null}`,
		},
		{
			name: "unknown field",
			data: `{"version":1,"request_id":"r1","ok":true,"data":{},"error":null,"extra":true}`,
		},
		{
			name: "success without data",
			data: `{"version":1,"request_id":"r1","ok":true,"data":null,"error":null}`,
		},
		{
			name: "failure without error",
			data: `{"version":1,"request_id":"r1","ok":false,"data":null,"error":null}`,
		},
		{
			name: "malformed data",
			data: `{"version":1,"request_id":"r1","ok":true,"data":{,"error":null}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeResponse([]byte(test.data)); err == nil {
				t.Fatal("DecodeResponse() error = nil, want error")
			}
		})
	}
}

func connectRequest(t *testing.T) Request {
	t.Helper()
	return Request{
		Version:   ProtocolVersion,
		RequestID: "r1",
		Command:   CommandConnect,
		Params:    json.RawMessage(`{"server":"vpn.example.com","protocol":"anyconnect"}`),
	}
}
