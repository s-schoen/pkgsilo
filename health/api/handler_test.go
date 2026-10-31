package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/logger/loggertest"
	"pkgsilo/core/router"
	"pkgsilo/health/service"
	"pkgsilo/health/service/mocks"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestRouter mounts a handler backed by a mock service on the application router.
func newTestRouter(t *testing.T, logger *slog.Logger) (http.Handler, *mocks.MockHealthService) {
	svc := mocks.NewMockHealthService(t)
	r := router.New(router.Options{}, discardLogger())
	r.Mount("/health", NewHandler(svc, logger).Routes())

	return r, svc
}

func serve(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	return rec
}

func TestHandler_Check_Healthy(t *testing.T) {
	r, svc := newTestRouter(t, discardLogger())
	svc.EXPECT().Check(mock.Anything).Return(service.Health{
		Status:     service.StatusHealthy,
		Components: []service.Component{{Name: "database", Status: service.StatusHealthy}},
	})

	rec := serve(r)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{
		"status": "healthy",
		"service": "pkgsilo",
		"components": [{"name":"database","status":"healthy"}]
	}`, rec.Body.String())
}

func TestHandler_Check_Unhealthy(t *testing.T) {
	logger, buf := loggertest.NewJSON()
	r, svc := newTestRouter(t, logger)
	svc.EXPECT().Check(mock.Anything).Return(service.Health{
		Status: service.StatusUnhealthy,
		Components: []service.Component{{
			Name:   "database",
			Status: service.StatusUnhealthy,
			Cause:  errors.New("dial tcp 10.0.0.7:5432: connection refused"),
		}},
	})

	rec := serve(r)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{
		"status": "unhealthy",
		"service": "pkgsilo",
		"components": [{"name":"database","status":"unhealthy"}]
	}`, rec.Body.String())

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "health component unhealthy", entries[0]["msg"])
	assert.Equal(t, "database", entries[0]["component"])
	assert.Equal(t, "dial tcp 10.0.0.7:5432: connection refused", entries[0]["error"])
}

func TestToHealthStatus_UnknownIsUnhealthy(t *testing.T) {
	assert.Equal(t, HealthUnhealthy, toHealthStatus(service.Status("unknown")))
}

func TestHealthResponse_HTTPStatus(t *testing.T) {
	tests := []struct {
		status HealthStatus
		want   int
	}{
		{HealthHealthy, http.StatusOK},
		{HealthDegraded, http.StatusOK},
		{HealthUnhealthy, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			// Component statuses don't affect the aggregate.
			h := HealthResponse{Status: tt.status, Components: []ComponentHealthResponse{{Name: "db", Status: HealthUnhealthy}}}

			assert.Equal(t, tt.want, h.httpStatus())
		})
	}
}
