package envelope

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealth_HTTPStatus(t *testing.T) {
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
			h := Health{Status: tt.status, Components: []ComponentHealth{{Name: "db", Status: HealthUnhealthy}}}

			assert.Equal(t, tt.want, h.HTTPStatus())
		})
	}
}

func TestWriteHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteHealth(rec, Health{
		Status:  HealthDegraded,
		Service: "orders-api",
		Components: []ComponentHealth{
			{Name: "database", Status: HealthHealthy},
			{Name: "email", Status: HealthDegraded, Detail: "Delivery may be delayed."},
		},
	})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{
		"status": "degraded",
		"service": "orders-api",
		"components": [
			{"name":"database","status":"healthy"},
			{"name":"email","status":"degraded","detail":"Delivery may be delayed."}
		]
	}`, rec.Body.String())
}
