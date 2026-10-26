package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/config"
	"pkgsilo/core/logger/loggertest"
	"pkgsilo/core/middleware"
	"pkgsilo/envelope"
)

func TestNew_MiddlewareAppliesToUnmatchedRoutes(t *testing.T) {
	r := New(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// chi only builds the middleware chain once at least one route exists.
	r.Get("/", func(http.ResponseWriter, *http.Request) {})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/does-not-exist", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(middleware.RequestIDHeader))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404}`, rec.Body.String())
}

func TestNew_MethodNotAllowed(t *testing.T) {
	r := New(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	noop := func(http.ResponseWriter, *http.Request) {}
	r.Get("/root", noop)
	r.Delete("/root", noop)
	// Mounted after New, so the subrouter inherits the 404 and 405 handlers.
	sub := chi.NewRouter()
	sub.Put("/", noop)
	sub.Get("/item", noop)
	r.Mount("/sub", sub)
	r.Mount("/param/{id}", sub)

	tests := []struct {
		path      string
		wantAllow []string
	}{
		{path: "/root", wantAllow: []string{http.MethodGet, http.MethodDelete}},
		{path: "/sub", wantAllow: []string{http.MethodPut}},
		{path: "/sub/", wantAllow: []string{http.MethodPut}},
		{path: "/sub/item", wantAllow: []string{http.MethodGet}},
		{path: "/param/42", wantAllow: []string{http.MethodPut}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tt.path, nil))

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
			assert.Equal(t, tt.wantAllow, rec.Header().Values("Allow"))
			assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
			assert.JSONEq(t, `{"type":"about:blank","title":"Method Not Allowed","status":405}`, rec.Body.String())
		})
	}
}

func TestNew_MountedSubrouterNotFound(t *testing.T) {
	r := New(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	sub := chi.NewRouter()
	sub.Get("/", func(http.ResponseWriter, *http.Request) {})
	r.Mount("/sub", sub)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sub/missing", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404}`, rec.Body.String())
}

func TestNew_LogsClientIPFromTrustedProxy(t *testing.T) {
	logger, buf := loggertest.NewJSON()
	var trusted middleware.TrustedProxies
	require.NoError(t, trusted.UnmarshalText([]byte("10.0.0.0/8")))

	r := New(config.Config{TrustedProxies: trusted}, logger)
	r.Get("/", func(http.ResponseWriter, *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	r.ServeHTTP(httptest.NewRecorder(), req)

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "203.0.113.7", entries[0]["client_ip"])
}

func TestNew_RecoveredPanicIsLoggedAs500(t *testing.T) {
	logger, buf := loggertest.NewJSON()
	r := New(config.Config{}, logger)
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("boom") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"type":"about:blank","title":"Internal Server Error","status":500}`, rec.Body.String())

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 2)
	panicEntry, requestEntry := entries[0], entries[1]

	assert.Equal(t, "panic recovered", panicEntry["msg"])
	assert.Equal(t, "request", requestEntry["msg"])
	assert.Equal(t, float64(http.StatusInternalServerError), requestEntry["status"])
	assert.Equal(t, "ERROR", requestEntry["level"])
	assert.Equal(t, requestEntry["request_id"], panicEntry["request_id"])
}
