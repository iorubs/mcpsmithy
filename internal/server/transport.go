package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const maxBufferSize = 4 * 1024 * 1024 // 4 MB

// Transport is the interface implemented by both the stdio and HTTP transports.
// Serve runs until the context is cancelled or the underlying connection closes,
// calling handle for every incoming request and routing responses back to the client.
type Transport interface {
	Serve(ctx context.Context, handle func(context.Context, *request) *response) error
}

// stdio handles newline-delimited JSON-RPC over stdin/stdout.
type stdio struct {
	scanner *bufio.Scanner
	encoder *json.Encoder
	mu      sync.Mutex
}

// newStdio creates a stdio transport.
func newStdio(r io.Reader, w io.Writer) *stdio {
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxBufferSize)

	return &stdio{
		scanner: sc,
		encoder: json.NewEncoder(w),
	}
}

// Serve runs the read-dispatch-write loop until the context is cancelled or
// the input stream reaches EOF.
func (t *stdio) Serve(ctx context.Context, handle func(context.Context, *request) *response) error {
	type result struct {
		req *request
		err error
	}
	ch := make(chan result, 1)

	go func() {
		for {
			req, err := t.readRequest(ctx)
			ch <- result{req, err}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r := <-ch:
			if r.err != nil {
				if errors.Is(r.err, io.EOF) {
					return nil
				}
				return r.err
			}
			if resp := handle(ctx, r.req); resp != nil {
				if err := t.writeResponse(ctx, resp); err != nil {
					return err
				}
			}
		}
	}
}

// readRequest reads and returns the next request.
// Malformed JSON lines are logged and skipped rather than terminating the server.
func (t *stdio) readRequest(ctx context.Context) (*request, error) {
	for t.scanner.Scan() {
		line := t.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			slog.WarnContext(ctx, "bad JSON-RPC request, skipping", "error", err)
			if werr := t.writeResponse(ctx, &response{JSONRPC: jsonrpcVersion, ID: json.RawMessage("null"), Error: errParseError}); werr != nil {
				slog.WarnContext(ctx, "failed to write parse error response", "error", werr)
			}
			continue
		}

		slog.DebugContext(ctx, "recv", "method", req.Method, "id", string(req.ID))
		return &req, nil
	}

	if err := t.scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner: %w", err)
	}
	return nil, io.EOF
}

// writeResponse writes a JSON-RPC response to stdout.
func (t *stdio) writeResponse(ctx context.Context, resp *response) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	slog.DebugContext(ctx, "send", "id", string(resp.ID))
	return t.encoder.Encode(resp)
}

// httpTransport implements MCP's SSE-based streaming transport:
//   - GET  /sse     — long-lived SSE stream; server pushes JSON-RPC responses here
//   - POST /message — client sends JSON-RPC requests here; server replies 202 Accepted
//
// Each SSE connection gets a unique session ID. The endpoint event includes the
// session ID as a query parameter so POST /message responses route to the correct
// SSE stream. This allows multiple clients to connect concurrently.
type httpTransport struct {
	addr     string
	sessions map[string]chan *response
	mu       sync.Mutex
}

// newHTTP creates an HTTP/SSE transport bound to addr (e.g. ":8080").
func newHTTP(addr string) *httpTransport {
	return &httpTransport{
		addr:     addr,
		sessions: make(map[string]chan *response),
	}
}

// newSessionID returns a random 16-hex-char session identifier.
func newSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// handler returns the HTTP mux used by this transport.
// Exposed so tests can wrap it with httptest.NewServer directly.
func (t *httpTransport) handler(handle func(context.Context, *request) *response) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", t.sseHandler)
	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		t.messageHandler(w, r, handle)
	})
	return mux
}

// Serve starts the HTTP server and blocks until ctx is cancelled.
func (t *httpTransport) Serve(ctx context.Context, handle func(context.Context, *request) *response) error {
	srv := &http.Server{Addr: t.addr, Handler: t.handler(handle)}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	slog.InfoContext(ctx, "HTTP transport listening", "addr", t.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// sseHandler opens a persistent SSE stream and forwards responses from a
// per-session channel. Each connection gets a unique session ID embedded in the
// endpoint URL so POST /message can route responses back to the correct stream.
func (t *httpTransport) sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sid := newSessionID()
	ch := make(chan *response, 16)

	t.mu.Lock()
	t.sessions[sid] = ch
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.sessions, sid)
		t.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Tell the client where to POST messages. The session ID rides along
	// transparently — the client uses this URL verbatim.
	fmt.Fprintf(w, "event: endpoint\ndata: /message?session_id=%s\n\n", sid)
	flusher.Flush()

	slog.DebugContext(r.Context(), "SSE client connected", "session", sid)
	for {
		select {
		case resp := <-ch:
			data, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			slog.DebugContext(r.Context(), "send", "id", string(resp.ID), "session", sid)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			slog.DebugContext(r.Context(), "SSE client disconnected", "session", sid)
			return
		}
	}
}

// messageHandler receives a JSON-RPC request, dispatches it, and enqueues the
// response on the session's channel identified by the session_id query parameter.
func (t *httpTransport) messageHandler(w http.ResponseWriter, r *http.Request, handle func(context.Context, *request) *response) {
	// Handle CORS preflight for browser-based clients.
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sid := r.URL.Query().Get("session_id")
	t.mu.Lock()
	ch, ok := t.sessions[sid]
	t.mu.Unlock()
	if !ok {
		http.Error(w, "unknown session", http.StatusBadRequest)
		return
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	slog.DebugContext(r.Context(), "recv", "method", req.Method, "id", string(req.ID), "session", sid)
	w.WriteHeader(http.StatusAccepted)

	if resp := handle(r.Context(), &req); resp != nil {
		select {
		case ch <- resp:
		default:
			slog.WarnContext(r.Context(), "session channel full, dropping response", "session", sid, "id", string(resp.ID))
		}
	}
}
