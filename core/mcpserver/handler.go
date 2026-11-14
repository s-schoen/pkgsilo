package mcpserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

// handler serves the SDK's streamable HTTP handler, mcp, and keeps what the SDK hides from the methods
// it calls in the request's call (see call).
type handler struct {
	mcp http.Handler
}

// ServeHTTP serves the request with the SDK, which reads its body and then calls the MCP method on a
// goroutine of its own. A panic of the method, which observe recovers there, panics again here, on the
// request's goroutine, so that the Recoverer of the router logs it and answers with a 500.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := &call{}
	if r.Body != nil {
		r.Body = &recordingBody{ReadCloser: r.Body, call: c}
	}

	h.mcp.ServeHTTP(&callWriter{ResponseWriter: w, call: c}, r.WithContext(context.WithValue(r.Context(), callKey{}, c)))

	if p := c.panic.Load(); p != nil {
		panic(p)
	}
}

type callKey struct{}

// call is the state of one request to the MCP endpoint that the SDK doesn't pass to the MCP methods. It's
// in the context of the request and of the methods.
type call struct {
	mu sync.Mutex
	// body holds the bytes of the request body that the SDK has read: all of it before any method is called.
	body []byte
	// panic is the panic of a method, set before the SDK writes the method's response.
	panic atomic.Pointer[methodPanicError]
}

// panicked reports whether a method of the call panicked.
func (c *call) panicked() bool {
	return c.panic.Load() != nil
}

// callFrom returns the call of the request whose context, or the context of whose method, ctx is, or nil if
// the request didn't come through handler.
func callFrom(ctx context.Context) *call {
	c, _ := ctx.Value(callKey{}).(*call)
	return c
}

// requestID returns the ID of the JSON-RPC request in the body, or false if the body holds none, such as a
// notification or a batch. The SDK keeps the ID of the request that it calls a method for to itself.
func (c *call) requestID() (jsonrpc.ID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg, err := jsonrpc.DecodeMessage(c.body)
	if err != nil {
		return jsonrpc.ID{}, false
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok || !req.ID.IsValid() {
		return jsonrpc.ID{}, false
	}

	return req.ID, true
}

// methodPanicError is the panic of an MCP method, recovered on the goroutine of the method, with the stack
// trace of that goroutine, which the Recoverer can't see.
type methodPanicError struct {
	method string
	value  any
	stack  []byte
}

func newMethodPanic(method string, value any) *methodPanicError {
	return &methodPanicError{method: method, value: value, stack: debug.Stack()}
}

// Error describes the panic for the log of the Recoverer, with the stack trace of the method's goroutine.
func (p *methodPanicError) Error() string {
	return fmt.Sprintf("MCP method %s panicked: %v\n\n%s", p.method, p.value, p.stack)
}

// Unwrap returns the panic's value if it's an error.
func (p *methodPanicError) Unwrap() error {
	err, _ := p.value.(error)
	return err
}

// recordingBody records the bytes of a request body that are read in its call.
type recordingBody struct {
	io.ReadCloser

	call *call
}

func (b *recordingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.call.mu.Lock()
	b.call.body = append(b.call.body, p[:n]...)
	b.call.mu.Unlock()

	return n, err //nolint:wrapcheck // A reader's io.EOF must stay unwrapped.
}

// callWriter writes the response of a call, unless a method panicked: then it drops what the SDK writes,
// so that the Recoverer can answer with a 500.
type callWriter struct {
	http.ResponseWriter

	call *call
}

// Header returns the response's header, or a header that's dropped once a method panicked.
func (w *callWriter) Header() http.Header {
	if w.call.panicked() {
		return http.Header{}
	}

	return w.ResponseWriter.Header()
}

func (w *callWriter) WriteHeader(status int) {
	if !w.call.panicked() {
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *callWriter) Write(b []byte) (int, error) {
	if w.call.panicked() {
		return len(b), nil
	}

	return w.ResponseWriter.Write(b) //nolint:wrapcheck // The writer's own error, for the SDK.
}

// Flush flushes the response, unless a method panicked. callWriter has no Unwrap method, so that an
// http.ResponseController can't reach around it.
func (w *callWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok && !w.call.panicked() {
		flusher.Flush()
	}
}
