package core

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter_MiddlewareAppliesToUnmatchedRoutes(t *testing.T) {
	r := NewRouter(Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// chi only builds the middleware chain once at least one route exists.
	r.Get("/", func(http.ResponseWriter, *http.Request) {})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/does-not-exist", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(RequestIDHeader))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

func TestNewRouter_LogsClientIPFromTrustedProxy(t *testing.T) {
	logger, buf := newJSONLogger()
	var trusted TrustedProxies
	require.NoError(t, trusted.UnmarshalText([]byte("10.0.0.0/8")))

	r := NewRouter(Config{TrustedProxies: trusted}, logger)
	r.Get("/", func(http.ResponseWriter, *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	r.ServeHTTP(httptest.NewRecorder(), req)

	entries := logEntries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "203.0.113.7", entries[0]["client_ip"])
}

func TestNewRouter_RecoveredPanicIsLoggedAs500(t *testing.T) {
	logger, buf := newJSONLogger()
	r := NewRouter(Config{}, logger)
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("boom") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	entries := logEntries(t, buf)
	require.Len(t, entries, 2)
	panicEntry, requestEntry := entries[0], entries[1]

	assert.Equal(t, "panic recovered", panicEntry["msg"])
	assert.Equal(t, "request", requestEntry["msg"])
	assert.Equal(t, float64(http.StatusInternalServerError), requestEntry["status"])
	assert.Equal(t, "ERROR", requestEntry["level"])
	assert.Equal(t, requestEntry["request_id"], panicEntry["request_id"])
}
