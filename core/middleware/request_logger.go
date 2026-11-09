package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// requestRecordKey marks the context with which RequestLogger logs a request.
type requestRecordKey struct{}

// IsRequestRecord reports whether ctx is the context with which RequestLogger logged a request. That
// record only repeats the response's status, which the request's span holds already, so the logger
// records no exception on the span for it, even at error level.
func IsRequestRecord(ctx context.Context) bool {
	marked, _ := ctx.Value(requestRecordKey{}).(bool)
	return marked
}

// RequestLogger logs every request once it has been handled. Responses with a 5xx status
// are logged at error level. The request ID and the attributes of AddLogAttrs are added by the
// logger's context handler. The record is no exception of the request's span (see IsRequestRecord):
// the error that caused a 5xx is logged, and so recorded, where it happened.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			// chi's wrapper keeps optional interfaces like http.Flusher and http.Hijacker intact.
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			ctx := withLogAttrs(r.Context())

			next.ServeHTTP(ww, r.WithContext(ctx))

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			level := slog.LevelInfo
			if status >= http.StatusInternalServerError {
				level = slog.LevelError
			}

			logger.LogAttrs(context.WithValue(ctx, requestRecordKey{}, true), level, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
				slog.Any("client_ip", ClientIPFromContext(ctx)),
			)
		})
	}
}
