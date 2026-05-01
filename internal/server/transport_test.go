package server

import (
	"bufio"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testHTTPServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	eng := newTestEngine(t)
	tp := newHTTP("")
	srv := &Server{engine: eng, tp: tp}
	hs := httptest.NewServer(tp.handler(srv.handle))
	t.Cleanup(hs.Close)
	return hs, srv
}

// connectSSE opens a GET /sse stream, waits for the endpoint event, and returns
// the message URL and a channel of subsequent data lines.
func connectSSE(t *testing.T, baseURL string) (messageURL string, lines <-chan string) {
	t.Helper()
	sseResp, err := http.Get(baseURL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sseResp.Body.Close() })

	ch := make(chan string, 32)
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(sseResp.Body)
		var notified bool
		for scanner.Scan() {
			line := scanner.Text()
			if !notified && strings.HasPrefix(line, "data: /message") {
				ready <- strings.TrimPrefix(line, "data: ")
				notified = true
				continue
			}
			if after, ok := strings.CutPrefix(line, "data: "); ok {
				ch <- after
			}
		}
	}()

	select {
	case messageURL = <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE endpoint event")
	}
	return messageURL, ch
}

func TestHTTPSSEEndpointEvent(t *testing.T) {
	hs, _ := testHTTPServer(t)

	resp, err := http.Get(hs.URL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	var endpointURL string
	done := make(chan struct{})
	go func() {
		defer close(done)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: /message?session_id=") {
				endpointURL = strings.TrimPrefix(line, "data: ")
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for endpoint event")
	}
	if endpointURL == "" {
		t.Fatal("did not receive endpoint event with session_id on /sse")
	}
	if !strings.Contains(endpointURL, "session_id=") {
		t.Fatalf("endpoint URL missing session_id: %q", endpointURL)
	}
}

func TestHTTPMethodNotAllowed(t *testing.T) {
	hs, _ := testHTTPServer(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"POST /sse", http.MethodPost, "/sse"},
		{"PUT /sse", http.MethodPut, "/sse"},
		{"GET /message", http.MethodGet, "/message"},
		{"PUT /message", http.MethodPut, "/message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, hs.URL+tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405, got %d", resp.StatusCode)
			}
		})
	}
}

func TestHTTPFullRoundTrip(t *testing.T) {
	hs, _ := testHTTPServer(t)

	messageURL, sseLines := connectSSE(t, hs.URL)

	postJSON := func(t *testing.T, body string) {
		t.Helper()
		resp, err := http.Post(hs.URL+messageURL, "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d", resp.StatusCode)
		}
	}

	readSSELine := func(t *testing.T) string {
		t.Helper()
		select {
		case line := <-sseLines:
			return line
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for SSE response")
			return ""
		}
	}

	tests := []struct {
		name       string
		req        string
		wantSubstr string
	}{
		{
			"initialize",
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
			"protocolVersion",
		},
		{
			"tools/list",
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
			"echo_tool",
		},
		{
			"tools/call",
			`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo_tool","arguments":{}}}`,
			"hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			postJSON(t, tt.req)
			data := readSSELine(t)
			if !strings.Contains(data, tt.wantSubstr) {
				t.Fatalf("expected %q in SSE response, got: %s", tt.wantSubstr, data)
			}
		})
	}
}

func TestHTTPPostMessageErrors(t *testing.T) {
	hs, _ := testHTTPServer(t)

	validMessageURL, _ := connectSSE(t, hs.URL)

	tests := []struct {
		name     string
		url      string
		body     string
		wantCode int
	}{
		{
			name:     "unknown session",
			url:      "/message?session_id=bogus",
			body:     `{"jsonrpc":"2.0","id":1,"method":"ping"}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "bad request body",
			url:      validMessageURL,
			body:     "not json",
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(hs.URL+tt.url, "application/json", bytes.NewBufferString(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.wantCode {
				t.Fatalf("expected %d, got %d", tt.wantCode, resp.StatusCode)
			}
		})
	}
}

func TestHTTPMultiClientIsolation(t *testing.T) {
	hs, _ := testHTTPServer(t)

	urlA, linesA := connectSSE(t, hs.URL)
	urlB, linesB := connectSSE(t, hs.URL)

	if urlA == urlB {
		t.Fatal("both clients got the same message URL")
	}

	resp, err := http.Post(hs.URL+urlA, "application/json",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":10,"method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	select {
	case data := <-linesA:
		if !strings.Contains(data, `"id":10`) && !strings.Contains(data, `"id": 10`) {
			t.Fatalf("client A got unexpected response: %s", data)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client A did not receive response")
	}

	select {
	case data := <-linesB:
		t.Fatalf("client B received response meant for A: %s", data)
	case <-time.After(200 * time.Millisecond):
		// Expected; no cross-talk.
	}
}

func TestStdioTransportServesCorrectly(t *testing.T) {
	// Regression test: ensure stdio.Serve still works after the
	// Transport interface refactor.
	input := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	out := testServer(t, input)
	if !strings.Contains(out, `"result"`) {
		t.Fatalf("expected ping result in output, got: %s", out)
	}
}

func TestHTTPNotifyBroadcastsToAllSessions(t *testing.T) {
	hs, srv := testHTTPServer(t)

	_, linesA := connectSSE(t, hs.URL)
	_, linesB := connectSSE(t, hs.URL)

	eng2 := newTestEngine(t)
	srv.SwapEngine(eng2)

	for name, ch := range map[string]<-chan string{"A": linesA, "B": linesB} {
		select {
		case data := <-ch:
			if !strings.Contains(data, "notifications/tools/list_changed") {
				t.Fatalf("client %s: expected tools/list_changed, got: %s", name, data)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("client %s: timeout waiting for notification", name)
		}
	}
}
