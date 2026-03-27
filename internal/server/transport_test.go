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

	// Read the initial endpoint event — should contain a session_id query param.
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

func TestHTTPMessageMethodNotAllowed(t *testing.T) {
	hs, _ := testHTTPServer(t)

	resp, err := http.Get(hs.URL + "/message")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestHTTPFullRoundTrip(t *testing.T) {
	hs, _ := testHTTPServer(t)

	// Open SSE stream first so responses have somewhere to go.
	sseResp, err := http.Get(hs.URL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer sseResp.Body.Close()

	// Drain the initial endpoint event and extract the POST URL.
	scanner := bufio.NewScanner(sseResp.Body)
	endpointReady := make(chan string, 1)
	sseLines := make(chan string, 32)
	go func() {
		var notified bool
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: /message") && !notified {
				endpointReady <- strings.TrimPrefix(line, "data: ")
				notified = true
				continue
			}
			if after, ok := strings.CutPrefix(line, "data: "); ok {
				sseLines <- after
			}
		}
	}()

	var messageURL string
	select {
	case messageURL = <-endpointReady:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for endpoint event")
	}

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

func TestHTTPBadRequestBody(t *testing.T) {
	hs, _ := testHTTPServer(t)

	// Must connect SSE first to get a valid session_id.
	sseResp, err := http.Get(hs.URL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer sseResp.Body.Close()

	scanner := bufio.NewScanner(sseResp.Body)
	sidCh := make(chan string, 1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: /message?session_id=") {
				sidCh <- strings.TrimPrefix(line, "data: ")
				return
			}
		}
	}()

	var messageURL string
	select {
	case messageURL = <-sidCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for endpoint")
	}

	resp, err := http.Post(hs.URL+messageURL, "application/json", bytes.NewBufferString("not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad request, got %d", resp.StatusCode)
	}
}

func TestHTTPUnknownSession(t *testing.T) {
	hs, _ := testHTTPServer(t)

	// POST with a bogus session_id should return 400.
	resp, err := http.Post(hs.URL+"/message?session_id=bogus", "application/json",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown session, got %d", resp.StatusCode)
	}
}

func TestHTTPMultiClientIsolation(t *testing.T) {
	hs, _ := testHTTPServer(t)

	// Helper: connect an SSE stream and return (messageURL, sseLinesChan).
	connectClient := func(t *testing.T) (string, <-chan string) {
		t.Helper()
		sseResp, err := http.Get(hs.URL + "/sse")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { sseResp.Body.Close() })

		scanner := bufio.NewScanner(sseResp.Body)
		endpointCh := make(chan string, 1)
		lines := make(chan string, 32)
		go func() {
			var notified bool
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: /message") && !notified {
					endpointCh <- strings.TrimPrefix(line, "data: ")
					notified = true
					continue
				}
				if after, ok := strings.CutPrefix(line, "data: "); ok {
					lines <- after
				}
			}
		}()

		var msgURL string
		select {
		case msgURL = <-endpointCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for endpoint")
		}
		return msgURL, lines
	}

	urlA, linesA := connectClient(t)
	urlB, linesB := connectClient(t)

	if urlA == urlB {
		t.Fatal("both clients got the same message URL")
	}

	// Send a request through client A.
	resp, err := http.Post(hs.URL+urlA, "application/json",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":10,"method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Client A should receive the response.
	select {
	case data := <-linesA:
		if !strings.Contains(data, `"id":10`) && !strings.Contains(data, `"id": 10`) {
			t.Fatalf("client A got unexpected response: %s", data)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client A did not receive response")
	}

	// Client B should NOT receive anything.
	select {
	case data := <-linesB:
		t.Fatalf("client B received response meant for A: %s", data)
	case <-time.After(200 * time.Millisecond):
		// Expected — no cross-talk.
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

func TestNewHTTPConstructor(t *testing.T) {
	srv := NewHTTP(newTestEngine(t), ":0")
	if srv == nil {
		t.Fatal("NewHTTP returned nil")
	}
	if _, ok := srv.tp.(*httpTransport); !ok {
		t.Fatalf("expected *httpTransport, got %T", srv.tp)
	}
}

func TestHTTPNotifyBroadcastsToAllSessions(t *testing.T) {
	hs, srv := testHTTPServer(t)

	// Connect two SSE clients.
	connectAndReadLines := func(t *testing.T) <-chan string {
		t.Helper()
		sseResp, err := http.Get(hs.URL + "/sse")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { sseResp.Body.Close() })

		lines := make(chan string, 32)
		go func() {
			scanner := bufio.NewScanner(sseResp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if after, ok := strings.CutPrefix(line, "data: "); ok {
					lines <- after
				}
			}
		}()
		// Drain the endpoint event.
		select {
		case <-lines:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for endpoint event")
		}
		return lines
	}

	linesA := connectAndReadLines(t)
	linesB := connectAndReadLines(t)

	// Trigger SwapEngine — should broadcast notification to both clients.
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
