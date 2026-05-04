// Package server wires the mcpsmithy tool engine to the MCP SDK.
//
// The MCP framing (initialize, tools/list, tools/call, list-changed
// notifications) and transports (stdio, Streamable HTTP) live in
// github.com/modelcontextprotocol/go-sdk/mcp. This package owns the
// bridge: turn each Engine tool into an mcp.Tool, decode arguments,
// and pipe results back as CallToolResult.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/iorubs/mcpsmithy/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "mcpsmithy"
	serverVersion = "0.1.0"
)

// Engine is the interface that tool engines must implement to be served
// by the MCP server. Both tools.Engine (config-driven) and setup.Engine
// (config-authoring) satisfy this interface.
type Engine interface {
	Tools() map[string]config.Tool
	Execute(ctx context.Context, name string, params map[string]any) (string, error)
}

// Server adapts an Engine to mcp.Server. It supports two transports:
// stdio (default) and Streamable HTTP, selected via Option.
type Server struct {
	mu         sync.Mutex
	engine     Engine
	mcp        *mcp.Server
	registered map[string]struct{}
	transport  mcp.Transport
	httpAddr   string
}

// Option configures a Server.
type Option func(*Server)

// WithHTTP configures the server to use Streamable HTTP bound to addr (e.g. ":8080").
func WithHTTP(addr string) Option {
	return func(s *Server) {
		s.transport = nil
		s.httpAddr = addr
	}
}

// New creates a server using the stdio transport by default.
func New(eng Engine, opts ...Option) *Server {
	s := &Server{
		engine:     eng,
		registered: make(map[string]struct{}),
		transport:  &mcp.StdioTransport{},
	}
	for _, o := range opts {
		o(s)
	}
	s.mcp = mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)
	s.registerToolsLocked(eng)
	return s
}

// SwapEngine atomically replaces the running engine. Adding/removing
// tools triggers tools/list_changed notifications via the SDK.
func (s *Server) SwapEngine(eng Engine) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.registered) > 0 {
		names := make([]string, 0, len(s.registered))
		for n := range s.registered {
			names = append(names, n)
		}
		s.mcp.RemoveTools(names...)
	}
	s.registered = make(map[string]struct{})
	s.engine = eng
	s.registerToolsLocked(eng)
}

// Serve starts the configured transport and blocks until ctx is
// cancelled or the underlying connection closes.
func (s *Server) Serve(ctx context.Context) error {
	if s.httpAddr != "" {
		return s.serveHTTP(ctx)
	}
	return s.mcp.Run(ctx, s.transport)
}

func (s *Server) serveHTTP(ctx context.Context) error {
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s.mcp },
		nil,
	)
	httpSrv := &http.Server{Addr: s.httpAddr, Handler: withCtxValues(ctx, handler)}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shut)
	}()
	slog.InfoContext(ctx, "HTTP transport listening", "addr", s.httpAddr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// registerToolsLocked must be called with s.mu held.
func (s *Server) registerToolsLocked(eng Engine) {
	for name, t := range eng.Tools() {
		name, t := name, t
		tool := &mcp.Tool{
			Name:        name,
			Description: t.Description,
			InputSchema: buildJSONSchema(t.Params),
		}
		s.mcp.AddTool(tool, s.handlerFor(name, t))
		s.registered[name] = struct{}{}
	}
}

// handlerFor builds the SDK ToolHandler for one engine tool.
func (s *Server) handlerFor(name string, t config.Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
		}

		if t.LogParams == nil || *t.LogParams {
			slog.DebugContext(ctx, "tool/call", "tool", name, "params", args)
		} else {
			slog.DebugContext(ctx, "tool/call", "tool", name)
		}

		s.mu.Lock()
		eng := s.engine
		s.mu.Unlock()

		start := time.Now()
		out, err := eng.Execute(ctx, name, args)
		duration := time.Since(start).Milliseconds()
		if err != nil {
			slog.InfoContext(ctx, "tool/call done", "tool", name, "duration_ms", duration, "error", err)
			return errorResult(fmt.Sprintf("Error: %v", err)), nil
		}
		slog.InfoContext(ctx, "tool/call done", "tool", name, "duration_ms", duration)
		return textResult(out), nil
	}
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: true,
	}
}
