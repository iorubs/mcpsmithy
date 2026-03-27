package server

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/tools"
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

func testServer(t *testing.T, input string) string {
	t.Helper()
	eng := newTestEngine(t)
	reader := strings.NewReader(input)
	var out bytes.Buffer
	srv := New(eng, reader, &out)
	_ = srv.Serve(context.Background())
	return out.String()
}

func TestProtocol(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSubstr string
		wantErr    bool
	}{
		{
			"initialize",
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
			"protocolVersion",
			false,
		},
		{
			"initialize version negotiation",
			`{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":"1999-01-01","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
			"protocolVersion",
			false,
		},
		{
			"ping",
			`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
			"result",
			false,
		},
		{
			"tools/list",
			`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
			"echo_tool",
			false,
		},
		{
			"unknown method",
			`{"jsonrpc":"2.0","id":4,"method":"unknown/method"}`,
			"-32601",
			true,
		},
		{
			"tools/call",
			`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"echo_tool","arguments":{}}}`,
			"hello world",
			false,
		},
		{
			"tools/call unknown tool",
			`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}`,
			"nonexistent",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := testServer(t, tt.input+"\n")
			if !strings.Contains(out, tt.wantSubstr) {
				t.Fatalf("expected %q in output, got: %s", tt.wantSubstr, out)
			}
			hasErr := strings.Contains(out, `"error"`) || strings.Contains(out, `"isError"`)
			if tt.wantErr && !hasErr {
				t.Fatalf("expected error in output, got: %s", out)
			}
		})
	}
}

func TestMalformedJSONDoesNotTerminate(t *testing.T) {
	// Send a bad JSON line followed by a valid ping. The server should
	// skip the bad line and still process the ping.
	input := "not valid json\n" +
		`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	out := testServer(t, input)

	// Should contain a parse error response for the bad line.
	if !strings.Contains(out, "-32700") {
		t.Fatalf("expected parse error (-32700) in output, got: %s", out)
	}
	// Should still process the valid ping.
	if !strings.Contains(out, `"result"`) {
		t.Fatalf("expected ping result in output, got: %s", out)
	}
}

func TestSwapEngineSendsToolsListChanged(t *testing.T) {
	// After SwapEngine, the server should emit a tools/list_changed
	// notification so connected clients re-fetch the tool list.
	eng := newTestEngine(t)
	reader := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var out bytes.Buffer
	srv := New(eng, reader, &out)

	// Serve processes the ping, then hits EOF.
	_ = srv.Serve(context.Background())

	// Now swap the engine — should write a notification to stdout.
	out.Reset()
	eng2 := newTestEngine(t)
	srv.SwapEngine(eng2)

	data := out.String()
	if !strings.Contains(data, "notifications/tools/list_changed") {
		t.Fatalf("expected tools/list_changed notification, got: %s", data)
	}
}
