package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"

	"pkgsilo/envelope"
)

// Recoverer recovers panics in downstream handlers, logs them and responds with a 500 problem.
//
// The stack trace is attached to the log record, unless prettyStack is set (meant for development):
// then it is printed to stderr in a readable form instead, since log handlers escape it onto one line.
func Recoverer(logger *slog.Logger, prettyStack bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					// Deliberate abort: let net/http drop the connection silently.
					panic(rec)
				}

				attrs := []slog.Attr{slog.Any("panic", rec)}
				if !prettyStack {
					attrs = append(attrs, slog.String("stack", string(debug.Stack())))
				}
				logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered", attrs...)

				if prettyStack {
					middleware.PrintPrettyStack(rec)
				}

				// The response may already be underway; its status can't be changed anymore.
				// The panic value is not exposed to the client.
				if ww.Status() == 0 {
					envelope.WriteProblem(ww, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
