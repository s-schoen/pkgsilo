package envelope

import "net/http"

// HealthStatus is the state reported for a service or one of its components.
type HealthStatus string

const (
	// HealthHealthy means the service is operating normally.
	HealthHealthy HealthStatus = "healthy"
	// HealthDegraded means the service is available, but with concerns.
	HealthDegraded HealthStatus = "degraded"
	// HealthUnhealthy means the service cannot perform its intended work.
	HealthUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth is the health of a component or dependency used by a service.
type ComponentHealth struct {
	Name   string       `json:"name"`
	Status HealthStatus `json:"status"`
	Detail string       `json:"detail,omitempty"`
}

// Health is the aggregate health of a service and, optionally, its components.
//
// Names and details must not expose credentials, internal addresses or other sensitive data.
type Health struct {
	Status     HealthStatus      `json:"status"`
	Service    string            `json:"service,omitempty"`
	Components []ComponentHealth `json:"components,omitempty"`
}

// HTTPStatus returns the response status for h: 200 when healthy or degraded, 503 otherwise.
// Only the aggregate status counts; component statuses are diagnostic context.
func (h Health) HTTPStatus() int {
	switch h.Status {
	case HealthHealthy, HealthDegraded:
		return http.StatusOK
	default:
		return http.StatusServiceUnavailable
	}
}

// WriteHealth writes h as a JSON response with the status matching its aggregate health.
func WriteHealth(w http.ResponseWriter, h Health) {
	writeJSON(w, h.HTTPStatus(), jsonMediaType, h)
}
