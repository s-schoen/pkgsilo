package logger_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/middleware"
	"pkgsilo/test/loggertest"
	"pkgsilo/test/telemetrytest"
)

// requestContext returns the context middleware.RequestID gives a request carrying id.
func requestContext(t *testing.T, id string) context.Context {
	t.Helper()

	var handled *http.Request
	h := middleware.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handled = r
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, id)
	h.ServeHTTP(httptest.NewRecorder(), req)

	return handled.Context()
}

func TestLogger_AddsRequestIDFromContext(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	ctx := requestContext(t, "req-1")

	logger.InfoContext(ctx, "plain")
	logger.With("component", "test").InfoContext(ctx, "derived via With")
	logger.WithGroup("grp").InfoContext(ctx, "derived via WithGroup")
	logger.InfoContext(t.Context(), "no request")

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 4)

	assert.Equal(t, "req-1", entries[0]["request_id"])
	assert.Equal(t, "req-1", entries[1]["request_id"])
	assert.Equal(t, map[string]any{"request_id": "req-1"}, entries[2]["grp"])
	assert.NotContains(t, entries[3], "request_id")
}

func TestLogger_AddsLogAttrsFromContext(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()

	var handled *http.Request
	h := middleware.RequestLogger(loggertest.Discard())(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handled = r
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	ctx := handled.Context()
	middleware.AddLogAttrs(ctx, slog.String("identity_id", "ada"))

	logger.InfoContext(ctx, "plain")
	logger.With("component", "test").InfoContext(ctx, "derived via With")
	logger.InfoContext(t.Context(), "no request")

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 3)

	assert.Equal(t, "ada", entries[0]["identity_id"])
	assert.Equal(t, "ada", entries[1]["identity_id"])
	assert.NotContains(t, entries[2], "identity_id")
}

// spanContext returns a context that holds a recording span of tel, which ends at the end of the test
// unless the test ends it before.
func spanContext(t *testing.T, tel *telemetrytest.Recorder) (context.Context, trace.Span) {
	t.Helper()

	ctx, span := tel.TracerProvider().Tracer("test").Start(t.Context(), "operation")
	t.Cleanup(func() { span.End() })

	return ctx, span
}

func TestLogger_AddsTraceAndSpanIDOfRecordingSpan(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	ctx, span := spanContext(t, telemetrytest.New(t))
	traceID, spanID := span.SpanContext().TraceID().String(), span.SpanContext().SpanID().String()

	logger.InfoContext(ctx, "plain")
	logger.With("component", "test").InfoContext(ctx, "derived via With")
	logger.WithGroup("grp").InfoContext(ctx, "derived via WithGroup")

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 3)

	assert.Equal(t, traceID, entries[0]["trace_id"])
	assert.Equal(t, spanID, entries[0]["span_id"])
	assert.Equal(t, traceID, entries[1]["trace_id"])
	assert.Equal(t, spanID, entries[1]["span_id"])
	assert.Equal(t, map[string]any{"trace_id": traceID, "span_id": spanID}, entries[2]["grp"])
}

func TestLogger_AddsNoIDsWithoutRecordingSpan(t *testing.T) {
	t.Parallel()

	remote := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x0a, 0xf7, 0x65, 0x19},
		SpanID:     trace.SpanID{0xb7, 0xad, 0x6b, 0x71},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	tests := map[string]context.Context{
		"no span":     t.Context(),
		"remote span": trace.ContextWithRemoteSpanContext(t.Context(), remote),
	}
	for name, ctx := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			logger, buf := loggertest.NewJSON()

			logger.InfoContext(ctx, "plain")
			logger.ErrorContext(ctx, "failed", "error", errors.New("boom"))

			entries := loggertest.Entries(t, buf)
			require.Len(t, entries, 2)
			for _, entry := range entries {
				assert.NotContains(t, entry, "trace_id")
				assert.NotContains(t, entry, "span_id")
			}
		})
	}
}

// pathError is an error type of its own, so that the test sees the type that an exception names.
type pathError struct{ path string }

func (e *pathError) Error() string { return "no such file: " + e.path }

func TestLogger_RecordsErrorOnSpan(t *testing.T) {
	t.Parallel()
	tel := telemetrytest.New(t)
	logger, _ := loggertest.NewJSON()
	ctx, span := spanContext(t, tel)

	logger.ErrorContext(ctx, "read failed", "error", fmt.Errorf("read config: %w", &pathError{path: "/etc/x"}))
	span.End()

	spans := tel.Spans()
	require.Len(t, spans, 1)
	assert.Equal(t, sdktrace.Status{Code: codes.Error, Description: "read config: no such file: /etc/x"},
		spans[0].Status())
	require.Len(t, spans[0].Events(), 1)
	event := spans[0].Events()[0]
	assert.Equal(t, semconv.ExceptionEventName, event.Name)
	assert.ElementsMatch(t, []attribute.KeyValue{
		semconv.ExceptionType("*logger_test.pathError"),
		semconv.ExceptionMessage("read config: no such file: /etc/x"),
	}, event.Attributes)
}

// TestLogger_RecordsExceptionTypeBehindWrapping checks that exception.type skips the errors with which
// fmt.Errorf wraps a single error, which name no type of their own, and only those.
func TestLogger_RecordsExceptionTypeBehindWrapping(t *testing.T) {
	t.Parallel()
	cause := &pathError{path: "/etc/x"}

	tests := map[string]struct {
		err  error
		want string
	}{
		"unwrapped":        {err: cause, want: "*logger_test.pathError"},
		"wrapped twice":    {err: fmt.Errorf("a: %w", fmt.Errorf("b: %w", cause)), want: "*logger_test.pathError"},
		"wrapped sentinel": {err: fmt.Errorf("a: %w", io.EOF), want: "*errors.errorString"},
		"not wrapped":      {err: fmt.Errorf("a: %v", cause), want: "*errors.errorString"}, //nolint:errorlint // On purpose.
		"wrapping two":     {err: fmt.Errorf("a: %w, %w", cause, io.EOF), want: "*fmt.wrapErrors"},
		"wrapped join":     {err: fmt.Errorf("a: %w", errors.Join(cause, io.EOF)), want: "*errors.joinError"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tel := telemetrytest.New(t)
			logger, _ := loggertest.NewJSON()
			ctx, span := spanContext(t, tel)

			logger.ErrorContext(ctx, "failed", "error", tt.err)
			span.End()

			spans := tel.Spans()
			require.Len(t, spans, 1)
			require.Len(t, spans[0].Events(), 1)
			attrs := attribute.NewSet(spans[0].Events()[0].Attributes...)
			got, _ := attrs.Value(semconv.ExceptionTypeKey)
			assert.Equal(t, tt.want, got.AsString())
		})
	}
}

// TestLogger_RecordsErrorAddedWithWith checks that an error added with With is recorded like one of the
// record's own, which overrides it.
func TestLogger_RecordsErrorAddedWithWith(t *testing.T) {
	t.Parallel()
	added := &pathError{path: "/etc/added"}

	tests := map[string]struct {
		log  func(*slog.Logger, context.Context)
		want error
	}{
		"with": {
			log:  func(l *slog.Logger, ctx context.Context) { l.With("error", added).ErrorContext(ctx, "failed") },
			want: added,
		},
		"with, then a group": {
			log: func(l *slog.Logger, ctx context.Context) {
				l.With("error", added).WithGroup("grp").ErrorContext(ctx, "failed")
			},
			want: added,
		},
		"the record's own": {
			log: func(l *slog.Logger, ctx context.Context) {
				l.With("error", added).ErrorContext(ctx, "failed", "error", io.EOF)
			},
			want: io.EOF,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tel := telemetrytest.New(t)
			logger, _ := loggertest.NewJSON()
			ctx, span := spanContext(t, tel)

			tt.log(logger, ctx)
			span.End()

			spans := tel.Spans()
			require.Len(t, spans, 1)
			assert.Equal(t, sdktrace.Status{Code: codes.Error, Description: tt.want.Error()}, spans[0].Status())
			require.Len(t, spans[0].Events(), 1)
			assert.ElementsMatch(t, []attribute.KeyValue{
				semconv.ExceptionType(fmt.Sprintf("%T", tt.want)),
				semconv.ExceptionMessage(tt.want.Error()),
			}, spans[0].Events()[0].Attributes)
		})
	}
}

// TestLogger_DerivedLoggersKeepTheirOwnErrors checks that loggers derived from one logger with With don't
// share the errors that each added.
func TestLogger_DerivedLoggersKeepTheirOwnErrors(t *testing.T) {
	t.Parallel()
	tel := telemetrytest.New(t)
	logger, _ := loggertest.NewJSON()
	base := logger.With("component", "test")
	first := base.With("error", &pathError{path: "/first"})
	second := base.With("error", &pathError{path: "/second"})

	firstCtx, firstSpan := spanContext(t, tel)
	first.ErrorContext(firstCtx, "failed")
	firstSpan.End()
	secondCtx, secondSpan := spanContext(t, tel)
	second.ErrorContext(secondCtx, "failed")
	secondSpan.End()
	baseCtx, baseSpan := spanContext(t, tel)
	base.ErrorContext(baseCtx, "failed")
	baseSpan.End()

	spans := tel.Spans()
	require.Len(t, spans, 3)
	assert.Equal(t, "no such file: /first", spans[0].Status().Description)
	assert.Equal(t, "no such file: /second", spans[1].Status().Description)
	assert.Equal(t, "failed", spans[2].Status().Description)
}

func TestLogger_RecordsMessageOnSpanWithoutError(t *testing.T) {
	t.Parallel()
	tel := telemetrytest.New(t)
	logger, _ := loggertest.NewJSON()
	ctx, span := spanContext(t, tel)

	logger.ErrorContext(ctx, "something broke")
	span.End()

	spans := tel.Spans()
	require.Len(t, spans, 1)
	assert.Equal(t, sdktrace.Status{Code: codes.Error, Description: "something broke"}, spans[0].Status())
	require.Len(t, spans[0].Events(), 1)
	assert.Equal(t, []attribute.KeyValue{semconv.ExceptionMessage("something broke")},
		spans[0].Events()[0].Attributes)
}

func TestLogger_RecordsRecoveredPanicOnSpan(t *testing.T) {
	t.Parallel()
	tel := telemetrytest.New(t)
	logger, _ := loggertest.NewJSON()
	h := middleware.Recoverer(logger, false)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	ctx, span := spanContext(t, tel)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
	span.End()

	spans := tel.Spans()
	require.Len(t, spans, 1)
	assert.Equal(t, sdktrace.Status{Code: codes.Error, Description: "boom"}, spans[0].Status())
	require.Len(t, spans[0].Events(), 1)
	event := spans[0].Events()[0]
	assert.Equal(t, semconv.ExceptionEventName, event.Name)
	attrs := attribute.NewSet(event.Attributes...)
	assert.Equal(t, 2, attrs.Len())
	message, _ := attrs.Value(semconv.ExceptionMessageKey)
	assert.Equal(t, "boom", message.AsString())
	stack, _ := attrs.Value(semconv.ExceptionStacktraceKey)
	assert.Contains(t, stack.AsString(), "logger_test.go")
}

func TestLogger_RecordsNothingOnSpanBelowErrorLevel(t *testing.T) {
	t.Parallel()
	tel := telemetrytest.New(t)
	logger, _ := loggertest.NewJSON()
	ctx, span := spanContext(t, tel)

	logger.WarnContext(ctx, "rejected", "error", errors.New("bad credentials"))
	span.End()

	spans := tel.Spans()
	require.Len(t, spans, 1)
	assert.Empty(t, spans[0].Events())
	assert.Equal(t, sdktrace.Status{Code: codes.Unset}, spans[0].Status())
}

// TestLogger_RecordsNoExceptionForRequestRecord checks that RequestLogger's record of a 500, logged at
// error level, adds no exception: the error that caused it is recorded where it's logged.
func TestLogger_RecordsNoExceptionForRequestRecord(t *testing.T) {
	t.Parallel()
	tel := telemetrytest.New(t)
	logger, buf := loggertest.NewJSON()
	h := middleware.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	ctx, span := spanContext(t, tel)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
	span.End()

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "ERROR", entries[0]["level"])
	assert.Equal(t, span.SpanContext().TraceID().String(), entries[0]["trace_id"])
	spans := tel.Spans()
	require.Len(t, spans, 1)
	assert.Empty(t, spans[0].Events())
}
