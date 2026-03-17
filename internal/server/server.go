// Package server implements the MCP protocol handler.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/operator-assistant/mcpsmithy/internal/config"
)

// Engine is the interface that tool engines must implement to be served
// by the MCP server. Both tools.Engine (config-driven) and setup.Engine
// (config-authoring) satisfy this interface.
type Engine interface {
	Tools() map[string]config.Tool
	Execute(ctx context.Context, name string, params map[string]any) (string, error)
}

// Server processes MCP requests using the latest API version.
type Server struct {
	mu     sync.RWMutex
	engine Engine
	tp     Transport
}

// New creates a server using the stdio transport.
func New(eng Engine, r io.Reader, w io.Writer) *Server {
	return &Server{engine: eng, tp: newStdio(r, w)}
}

// NewHTTP creates a server using the HTTP/SSE transport bound to addr (e.g. ":8080").
func NewHTTP(eng Engine, addr string) *Server {
	return &Server{engine: eng, tp: newHTTP(addr)}
}

// SwapEngine atomically replaces the running engine.
func (s *Server) SwapEngine(eng Engine) {
	s.mu.Lock()
	s.engine = eng
	s.mu.Unlock()
}

// Serve starts the transport loop until the context is cancelled or the
// underlying connection closes.
func (s *Server) Serve(ctx context.Context) error {
	slog.InfoContext(ctx, "MCP server starting", "protocol", protocolVersion)
	return s.tp.Serve(ctx, s.handle)
}

func (s *Server) handle(ctx context.Context, req *request) *response {
	switch req.Method {
	case methodInitialize:
		return s.initialize(ctx, req)
	case methodNotificationsInit, methodNotificationsCancelled:
		return nil
	case methodToolsList:
		return s.toolsList(req)
	case methodToolsCall:
		return s.toolsCall(ctx, req)
	case methodPing:
		return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: map[string]any{}}
	default:
		if len(req.ID) == 0 || string(req.ID) == "null" {
			return nil
		}
		return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Error: errMethodNotFound}
	}
}

func (s *Server) initialize(ctx context.Context, req *request) *response {
	if len(req.Params) > 0 {
		var p initializeParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Error: errInvalidParams}
		}
		if p.ClientInfo.Name != "" {
			slog.InfoContext(ctx, "client connected", "client", p.ClientInfo.Name, "version", p.ClientInfo.Version)
		}
		if p.ProtocolVersion != "" && p.ProtocolVersion != protocolVersion {
			slog.WarnContext(ctx, "client requested different protocol version, responding with server version",
				"requested", p.ProtocolVersion, "server", protocolVersion)
		}
	}
	// Per MCP spec: always respond with the server's supported version.
	// The client decides whether to continue or disconnect.
	return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: initializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    capabilities{Tools: map[string]any{}},
		ServerInfo:      serverInfo{Name: serverName, Version: serverVersion},
	}}
}

func (s *Server) toolsList(req *request) *response {
	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	defs := make([]toolDefinition, 0, len(eng.Tools()))
	for name, t := range eng.Tools() {
		defs = append(defs, toolDefinition{
			Name:        name,
			Description: t.Description,
			InputSchema: buildJSONSchema(t.Params),
		})
	}
	return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: toolsListResult{Tools: defs}}
}

func (s *Server) toolsCall(ctx context.Context, req *request) *response {
	var p toolCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Error: errInvalidParams}
	}
	if p.Name == "" {
		return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Error: errInvalidParams}
	}

	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	if tool, ok := eng.Tools()[p.Name]; !ok || tool.LogParams == nil || *tool.LogParams {
		slog.DebugContext(ctx, "tool/call", "tool", p.Name, "params", p.Arguments)
	} else {
		slog.DebugContext(ctx, "tool/call", "tool", p.Name)
	}

	start := time.Now()
	out, err := eng.Execute(ctx, p.Name, p.Arguments)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		slog.InfoContext(ctx, "tool/call done", "tool", p.Name, "duration_ms", duration, "error", err)
		return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: textResult(fmt.Sprintf("Error: %v", err), true)}
	}
	slog.InfoContext(ctx, "tool/call done", "tool", p.Name, "duration_ms", duration)
	return &response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: textResult(out, false)}
}

// textResult wraps text in the standard single-content toolResult.
func textResult(text string, isError bool) toolResult {
	return toolResult{
		Content: []toolContent{{Type: "text", Text: text}},
		IsError: isError,
	}
}
