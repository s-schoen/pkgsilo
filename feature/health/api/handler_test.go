package api

import (
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/feature/health/service"
	"pkgsilo/feature/health/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

// newTestClient returns a client for a handler that is backed by a mock service and mounted on the
// application router.
func newTestClient(t *testing.T, logger *slog.Logger) (*apitest.Client, *mocks.MockHealthService) {
	t.Helper()

	svc := mocks.NewMockHealthService(t)

	return apitest.NewClient(t, apitest.Mount("/health", NewHandler(svc, logger).Routes())), svc
}

func TestHandler_Check_Healthy(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Check(mock.Anything).Return(service.Health{
		Status:     service.StatusHealthy,
		Components: []service.Component{{Name: "database", Status: service.StatusHealthy}},
	})

	api.Get("/health").AssertJSON(http.StatusOK, `{
		"status": "healthy",
		"service": "pkgsilo",
		"components": [{"name":"database","status":"healthy"}]
	}`)
}

func TestHandler_Check_Unhealthy(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	api, svc := newTestClient(t, logger)
	svc.EXPECT().Check(mock.Anything).Return(service.Health{
		Status: service.StatusUnhealthy,
		Components: []service.Component{{
			Name:   "database",
			Status: service.StatusUnhealthy,
			Cause:  errors.New("dial tcp 10.0.0.7:5432: connection refused"),
		}},
	})

	api.Get("/health").AssertJSON(http.StatusServiceUnavailable, `{
		"status": "unhealthy",
		"service": "pkgsilo",
		"components": [{"name":"database","status":"unhealthy"}]
	}`)

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "health component unhealthy", entries[0]["msg"])
	assert.Equal(t, "database", entries[0]["component"])
	assert.Equal(t, "dial tcp 10.0.0.7:5432: connection refused", entries[0]["error"])
}

func TestToHealthStatus_UnknownIsUnhealthy(t *testing.T) {
	t.Parallel()
	assert.Equal(t, HealthUnhealthy, toHealthStatus(service.Status("unknown")))
}

func TestHealthResponse_HTTPStatus(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			// Component statuses don't affect the aggregate.
			h := HealthResponse{Status: tt.status, Components: []ComponentHealthResponse{{Name: "db", Status: HealthUnhealthy}}}

			assert.Equal(t, tt.want, h.httpStatus())
		})
	}
}
