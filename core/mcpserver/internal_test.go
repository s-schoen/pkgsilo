package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/middleware"
	"pkgsilo/test/loggertest"
)

// requestContext returns the context of a request that middleware.RequestID gave the request ID id.
func requestContext(t *testing.T, id string) context.Context {
	t.Helper()

	var ctx context.Context
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
	req.Header.Set(middleware.RequestIDHeader, id)
	middleware.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context() //nolint:fatcontext // Captures the request's context, nothing nests.
	})).ServeHTTP(httptest.NewRecorder(), req)

	return ctx
}

// observed returns the method handler that observe wraps around a method that fails with err, and the
// entries that it has logged so far.
func observed(t *testing.T, err error) (handler mcp.MethodHandler, logs func() []map[string]any) {
	t.Helper()

	logger, buf := loggertest.NewJSON()
	handler = observe(logger, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), map[string]bool{})(
		func(context.Context, string, mcp.Request) (mcp.Result, error) { return nil, err })

	return handler, func() []map[string]any { return loggertest.Entries(t, buf) }
}

// TestObserve_ReplacesUnexpectedErrors checks that a plain error of a method is logged and replaced by a
// JSON-RPC error that says only internal error and the request ID.
func TestObserve_ReplacesUnexpectedErrors(t *testing.T) {
	t.Parallel()
	handler, logs := observed(t, errors.New("connection refused"))

	_, err := handler(requestContext(t, "req-1"), "tools/list", &mcp.ListToolsRequest{})

	assert.Equal(t, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error (request ID req-1)"}, err)
	entries := logs()
	require.Len(t, entries, 1)
	assert.Equal(t, "mcp method failed", entries[0]["msg"])
	assert.Equal(t, "tools/list", entries[0]["mcp_method"])
	assert.Equal(t, "connection refused", entries[0]["error"])
}

// TestObserve_KeepsJSONRPCErrors checks that the JSON-RPC errors that the SDK answers with pass unchanged.
func TestObserve_KeepsJSONRPCErrors(t *testing.T) {
	t.Parallel()
	wireErr := &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: `unknown tool "nope"`}
	handler, logs := observed(t, wireErr)

	_, err := handler(requestContext(t, "req-1"), "tools/call", &mcp.CallToolRequest{})

	assert.Same(t, wireErr, err)
	assert.Empty(t, logs())
}

// TestObserve_PanicsWithoutHandler checks that a panic of a method that isn't served by handler, which
// would pass it on, panics again.
func TestObserve_PanicsWithoutHandler(t *testing.T) {
	t.Parallel()
	handler := observe(loggertest.Discard(), noop.NewMeterProvider(), tracenoop.NewTracerProvider(), nil)(
		func(context.Context, string, mcp.Request) (mcp.Result, error) { panic("boom") })

	p, ok := recovered(func() { _, _ = handler(t.Context(), "tools/list", &mcp.ListToolsRequest{}) }).(*methodPanicError)

	require.True(t, ok, "panics with a *methodPanicError")
	assert.Equal(t, "boom", p.value)
	assert.Equal(t, "tools/list", p.method)
	assert.Contains(t, p.Error(), "MCP method tools/list panicked: boom\n\ngoroutine ")
}

// recovered returns what f panics with, or nil if it doesn't panic.
func recovered(f func()) (value any) {
	defer func() { value = recover() }()
	f()

	return nil
}

// TestDecodeRequestID checks that the ID of a JSON-RPC request is found wherever it is in the body, without
// decoding the other members, and that a body without one has none.
func TestDecodeRequestID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		// want is the ID as the span names it, or "" for none.
		want string
	}{
		{name: "number", body: `{"jsonrpc":"2.0","id":7,"method":"ping"}`, want: "7"},
		{name: "string", body: `{"jsonrpc":"2.0","id":"abc","method":"ping"}`, want: "abc"},
		{
			name: "after the params",
			body: `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"x","arguments":{"id":"inner"}},"id":8}`,
			want: "8",
		},
		{name: "notification", body: `{"jsonrpc":"2.0","method":"notifications/initialized"}`},
		{name: "null", body: `{"jsonrpc":"2.0","id":null,"method":"ping"}`},
		{name: "object", body: `{"jsonrpc":"2.0","id":{},"method":"ping"}`},
		{name: "batch", body: `[{"jsonrpc":"2.0","id":7,"method":"ping"}]`},
		{name: "empty", body: ``},
		{name: "malformed", body: `{"jsonrpc":`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id, ok := decodeRequestID([]byte(tt.body))

			if tt.want == "" {
				assert.False(t, ok, "found the ID %v", id.Raw())
				return
			}
			require.True(t, ok)
			assert.Equal(t, tt.want, fmt.Sprint(id.Raw()))
		})
	}
}

// TestMethodPanic_Unwrap checks that the panic of a method unwraps to its value if that's an error.
func TestMethodPanic_Unwrap(t *testing.T) {
	t.Parallel()
	err := errors.New("boom")

	require.ErrorIs(t, newMethodPanic("tools/call", err), err)
	assert.NoError(t, newMethodPanic("tools/call", "boom").Unwrap())
}
