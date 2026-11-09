// Package logger creates the application's structured logger.
package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"
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

	return slog.New(contextHandler{Handler: h})
}

// contextHandler adds request-scoped values from the context to every record, and records errors on the
// context's span.
type contextHandler struct {
	slog.Handler

	// exceptionAttrs are the attributes that With added under the keys that recordException reads, so that
	// an error added with With is recorded like one of the record's own.
	exceptionAttrs []slog.Attr
}

// The keys of the attributes that recordException reads: AGENTS.md has errors logged under errorKey, and
// middleware.Recoverer logs the panic value and its stack trace under panicKey and stackKey.
const (
	errorKey = "error"
	panicKey = "panic"
	stackKey = "stack"
)

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
			recordException(span, h.exceptionAttrs, r)
		}
	}

	return h.Handler.Handle(ctx, r) //nolint:wrapcheck // A decorating handler passes errors through unchanged.
}

// recordException adds an exception event for the record to span and sets the span's status to Error,
// described by the exception's message. The event holds the type (see exceptionType) and text of the
// record's error attribute, or the Recoverer's panic value and stack trace, or else the record's message.
// An attribute of the record overrides one of withAttrs, the attributes that With added.
//
// It's the only place that records exceptions on spans, so that no error is recorded twice.
func recordException(span trace.Span, withAttrs []slog.Attr, r slog.Record) {
	var (
		err        error
		panicValue string
		stack      string
		isPanic    bool
	)
	read := func(a slog.Attr) bool {
		switch a.Key {
		case errorKey:
			err, _ = a.Value.Resolve().Any().(error)
		case panicKey:
			isPanic = true
			panicValue = a.Value.Resolve().String()
		case stackKey:
			stack = a.Value.Resolve().String()
		}

		return true
	}
	for _, a := range withAttrs {
		read(a)
	}
	r.Attrs(read)

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
		attrs = append(attrs, semconv.ExceptionType(exceptionType(err)), semconv.ExceptionMessage(message))
	default:
		attrs = append(attrs, semconv.ExceptionMessage(message))
	}

	span.AddEvent(semconv.ExceptionEventName, trace.WithAttributes(attrs...), trace.WithTimestamp(r.Time))
	span.SetStatus(codes.Error, message)
}

// wrapErrorType is the type of the errors with which fmt.Errorf wraps a single error with %w.
var wrapErrorType = reflect.TypeOf(fmt.Errorf("%w", io.EOF))

// exceptionType returns the type of err for exception.type. The errors with which fmt.Errorf wraps a single
// error, which nearly every logged error is, add context but no type of their own, so it's the type of the
// first error in err's chain that isn't one.
func exceptionType(err error) string {
	for reflect.TypeOf(err) == wrapErrorType {
		err = errors.Unwrap(err)
	}

	return fmt.Sprintf("%T", err)
}

// WithAttrs and WithGroup must re-wrap the derived handler, otherwise loggers created via
// With or WithGroup would silently fall back to the bare handler and drop the request ID.
func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Clipped, so that appending copies rather than shares the array with the handlers derived from h.
	exceptionAttrs := slices.Clip(h.exceptionAttrs)
	for _, a := range attrs {
		if a.Key == errorKey || a.Key == panicKey || a.Key == stackKey {
			exceptionAttrs = append(exceptionAttrs, a)
		}
	}

	return contextHandler{Handler: h.Handler.WithAttrs(attrs), exceptionAttrs: exceptionAttrs}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{Handler: h.Handler.WithGroup(name), exceptionAttrs: h.exceptionAttrs}
}
