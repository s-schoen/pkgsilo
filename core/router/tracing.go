package router

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// serverTracerProvider is the tracer provider of otelhttp, whose tracers start the server span of a request
// (see serverTracer.Start).
type serverTracerProvider struct {
	trace.TracerProvider
}

// Tracer returns the tracer of provider's TracerProvider, which starts its spans as serverTracer.Start says.
func (p serverTracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return serverTracer{p.TracerProvider.Tracer(name, opts...)}
}

// serverTracer is a tracer of serverTracerProvider.
type serverTracer struct {
	trace.Tracer
}

// Start starts the server span of a request, as a serverSpan if it records.
//
// A tracer that starts no span of its own, such as the no-op one with tracing off, returns the span context
// of the request's traceparent instead, which no span of pkgsilo continues. Start leaves the request's
// context without a span context then, since the SDK's exemplar filter would otherwise attach the trace that
// the client chose to the request's metrics, and the client could have it sampled too.
func (t serverTracer) Start(
	ctx context.Context, name string, opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	ctx, span := t.Tracer.Start(ctx, name, opts...)
	if span.SpanContext().IsRemote() {
		ctx = trace.ContextWithSpanContext(ctx, trace.SpanContext{})
		return ctx, trace.SpanFromContext(ctx)
	}
	if !span.IsRecording() {
		return ctx, span
	}

	server := &serverSpan{Span: span}

	return trace.ContextWithSpan(ctx, server), server
}

// serverSpan is the server span of a request, which keeps the description of its Error status.
type serverSpan struct {
	trace.Span

	mu sync.Mutex
	// described reports whether the span's status was set to Error with a description.
	described bool
}

// SetStatus sets the span's status, unless the status is Error without a description while the span's
// status already is Error with one. otelhttp sets that status for every 5xx response once the request has
// been handled, which would erase the description of the error that the logger recorded on the span.
func (s *serverSpan) SetStatus(code codes.Code, description string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if code == codes.Error {
		if description == "" && s.described {
			return
		}
		s.described = s.described || description != ""
	}
	s.Span.SetStatus(code, description)
}
