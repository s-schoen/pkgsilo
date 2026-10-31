package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/logger/loggertest"
	"pkgsilo/core/middleware"
)

func TestRequestLogger(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		handler    http.HandlerFunc
		wantStatus int
		wantLevel  string
	}{
		"explicit status": {
			handler:    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) },
			wantStatus: http.StatusCreated,
			wantLevel:  "INFO",
		},
		"implicit 200": {
			handler:    func(http.ResponseWriter, *http.Request) {},
			wantStatus: http.StatusOK,
			wantLevel:  "INFO",
		},
		"server error": {
			handler:    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) },
			wantStatus: http.StatusBadGateway,
			wantLevel:  "ERROR",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			logger, buf := loggertest.NewJSON()
			h := middleware.RequestID(middleware.ClientIP(nil)(middleware.RequestLogger(logger)(tt.handler)))

			req := httptest.NewRequest(http.MethodPost, "/things", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			req.Header.Set(middleware.RequestIDHeader, "req-1")
			// Not from a trusted proxy, so it must not be logged as the client IP.
			req.Header.Set("X-Forwarded-For", "6.6.6.6")
			h.ServeHTTP(httptest.NewRecorder(), req)

			entries := loggertest.Entries(t, buf)
			require.Len(t, entries, 1)
			entry := entries[0]

			assert.Equal(t, "request", entry["msg"])
			assert.Equal(t, tt.wantLevel, entry["level"])
			assert.Equal(t, "POST", entry["method"])
			assert.Equal(t, "/things", entry["path"])
			assert.EqualValues(t, tt.wantStatus, entry["status"])
			assert.Equal(t, "10.0.0.1", entry["client_ip"])
			assert.Equal(t, "req-1", entry["request_id"])
			assert.Contains(t, entry, "duration")
		})
	}
}
