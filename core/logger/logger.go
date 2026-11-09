// Package logger creates the application's structured logger.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/lmittmann/tint"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/middleware"
)

// Options configures the logger.
type Options struct {
	// Dev selects colored, human-readable output instead of JSON.
	Dev   bool
	Level slog.Level
}

// New returns a colored, human-readable logger in development and a JSON logger otherwise.
// Records logged with a request context automatically include its request ID and the attributes
// added with middleware.AddLogAttrs.
//
// A record whose context holds a recording span names its trace and span with trace_id and span_id. A
// record at error level or above also adds an exception event to that span and sets its status to Error,
// so that every logged error and recovered panic shows up in the trace (see recordException).
func New(w io.Writer, opts Options) *slog.Logger {
	var h slog.Handler
	if opts.Dev {
		h = tint.NewTextHandler(w, &tint.Options{
			Level:      opts.Level,
			TimeFormat: time.Kitchen,
		})
	} else {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: opts.Level})
	}

	return slog.New(contextHandler{h})
}

// contextHandler adds request-scoped values from the context to every record, and records errors on the
// context's span.
type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := middleware.RequestIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id)) //nolint:sloglint // The one place that adds the request ID.
	}
	r.AddAttrs(middleware.LogAttrsFromContext(ctx)...)

	// A span that doesn't record, with tracing off, in a trace that isn't sampled or extracted from a
	// client's traceparent, has IDs that lead nowhere.
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		sc := span.SpanContext()
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
		if r.Level >= slog.LevelError && !middleware.IsRequestRecord(ctx) {
			recordException(span, r)
		}
	}

	return h.Handler.Handle(ctx, r) //nolint:wrapcheck // A decorating handler passes errors through unchanged.
}

// recordException adds an exception event for the record to span and sets the span's status to Error,
// described by the exception's message. The event holds the type and text of the record's error
// attribute, or the Recoverer's panic value and stack trace, or else the record's message.
//
// It's the only place that records exceptions on spans, so that no error is recorded twice.
func recordException(span trace.Span, r slog.Record) {
	var (
		err        error
		panicValue string
		stack      string
		isPanic    bool
	)
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "error":
			err, _ = a.Value.Resolve().Any().(error)
		case "panic":
			isPanic = true
			panicValue = a.Value.Resolve().String()
		case "stack":
			stack = a.Value.Resolve().String()
		}

		return true
	})

	var attrs []attribute.KeyValue
	message := r.Message
	switch {
	case isPanic:
		message = panicValue
		attrs = append(attrs, semconv.ExceptionMessage(message))
		if stack != "" {
			attrs = append(attrs, semconv.ExceptionStacktrace(stack))
		}
	case err != nil:
		message = err.Error()
		attrs = append(attrs, semconv.ExceptionType(fmt.Sprintf("%T", err)), semconv.ExceptionMessage(message))
	default:
		attrs = append(attrs, semconv.ExceptionMessage(message))
	}

	span.AddEvent(semconv.ExceptionEventName, trace.WithAttributes(attrs...), trace.WithTimestamp(r.Time))
	span.SetStatus(codes.Error, message)
}

// WithAttrs and WithGroup must re-wrap the derived handler, otherwise loggers created via
// With or WithGroup would silently fall back to the bare handler and drop the request ID.
func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
