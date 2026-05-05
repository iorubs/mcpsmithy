package server

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iorubs/mcpsmithy/internal/config"
	"github.com/iorubs/mcpsmithy/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestEngine(t *testing.T) *tools.Engine {
	t.Helper()
	cfg := &config.Config{
		Version: "1",
		Project: config.Project{Name: "test", Description: "test project"},
		Tools: map[string]config.Tool{
			"echo_tool": {
				Description: "echoes input",
				Template:    "hello world",
			},
		},
	}
	eng, err := tools.New(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

// connect dials the SDK server in-process and returns an initialized client session.
func connect(t *testing.T, srv *Server, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	clientT, serverT := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	if _, err := srv.mcp.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, opts)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestInitializeAndPing(t *testing.T) {
	srv := New(newTestEngine(t))
	cs := connect(t, srv, nil)

	got := cs.InitializeResult()
	if got == nil || got.ServerInfo == nil {
		t.Fatalf("missing initialize result/serverInfo")
	}
	if got.ServerInfo.Name != serverName {
		t.Errorf("serverInfo.name = %q, want %q", got.ServerInfo.Name, serverName)
	}
	if got.ServerInfo.Version != serverVersion {
		t.Errorf("serverInfo.version = %q, want %q", got.ServerInfo.Version, serverVersion)
	}

	if err := cs.Ping(context.Background(), nil); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestToolsListAndCall(t *testing.T) {
	srv := New(newTestEngine(t))
	cs := connect(t, srv, nil)

	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(list.Tools) != 1 || list.Tools[0].Name != "echo_tool" {
		t.Fatalf("unexpected tools: %+v", list.Tools)
	}

	out, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "echo_tool",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if out.IsError {
		t.Fatalf("tool returned isError; content=%+v", out.Content)
	}
	if len(out.Content) == 0 {
		t.Fatalf("missing tool content")
	}
	tc, ok := out.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("first content not text: %T", out.Content[0])
	}
	if !strings.Contains(tc.Text, "hello world") {
		t.Errorf("tool output = %q, want contains %q", tc.Text, "hello world")
	}
}

func TestToolsCallUnknown(t *testing.T) {
	srv := New(newTestEngine(t))
	cs := connect(t, srv, nil)

	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "nonexistent",
		Arguments: map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error calling unknown tool, got nil")
	}
}

func TestSwapEngineEmitsListChanged(t *testing.T) {
	var changed atomic.Int32
	notified := make(chan struct{}, 1)

	srv := New(newTestEngine(t))
	cs := connect(t, srv, &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
			changed.Add(1)
			select {
			case notified <- struct{}{}:
			default:
			}
		},
	})

	// Initial list to ensure connection is up.
	if _, err := cs.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("list tools: %v", err)
	}

	srv.SwapEngine(newTestEngine(t))

	select {
	case <-notified:
	case <-time.After(2 * time.Second):
		t.Fatalf("did not receive tools/list_changed notification (count=%d)", changed.Load())
	}
}
