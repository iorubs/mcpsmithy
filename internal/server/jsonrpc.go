package server

import (
	"encoding/json"
)

// jsonrpcVersion is the JSON-RPC protocol version string included in every message.
const jsonrpcVersion = "2.0"

// Standard JSON-RPC 2.0 error codes.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// Pre-declared errors for the standard JSON-RPC 2.0 error codes.
var (
	errParseError     = &rpcError{Code: codeParseError, Message: "parse error"}
	errMethodNotFound = &rpcError{Code: codeMethodNotFound, Message: "method not found"}
	errInvalidParams  = &rpcError{Code: codeInvalidParams, Message: "invalid params"}
)

// request represents a JSON-RPC 2.0 request or notification.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response represents a JSON-RPC 2.0 response.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface for JSON-RPC errors.
func (e *rpcError) Error() string {
	return e.Message
}
