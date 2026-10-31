package api

import (
	"net/http"

	"pkgsilo/health/service"
)

// serviceName identifies pkgsilo in health responses.
const serviceName = "pkgsilo"

// HealthStatus is the state reported for pkgsilo or one of its components.
type HealthStatus string

const (
	// HealthHealthy means the service is operating normally.
	HealthHealthy HealthStatus = "healthy"
	// HealthDegraded means the service is available, but with concerns.
	HealthDegraded HealthStatus = "degraded"
	// HealthUnhealthy means the service cannot perform its intended work.
	HealthUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealthResponse is the health of a component or dependency used by pkgsilo.
type ComponentHealthResponse struct {
	Name   string       `json:"name"`
	Status HealthStatus `json:"status"`
	Detail string       `json:"detail,omitempty"`
}

// HealthResponse is the aggregate health of pkgsilo and, optionally, its components.
//
// Names and details must not expose credentials, internal addresses or other sensitive data.
type HealthResponse struct {
	Status     HealthStatus              `json:"status"`
	Service    string                    `json:"service,omitempty"`
	Components []ComponentHealthResponse `json:"components,omitempty"`
}

// httpStatus returns the response status for h: 200 when healthy or degraded, 503 otherwise.
// Only the aggregate status counts; component statuses are diagnostic context.
func (h HealthResponse) httpStatus() int {
	switch h.Status {
	case HealthHealthy, HealthDegraded:
		return http.StatusOK
	default:
		return http.StatusServiceUnavailable
	}
}

func toHealthResponse(h service.Health) HealthResponse {
	components := make([]ComponentHealthResponse, len(h.Components))
	for i, c := range h.Components {
		components[i] = ComponentHealthResponse{Name: c.Name, Status: toHealthStatus(c.Status)}
	}

	return HealthResponse{Status: toHealthStatus(h.Status), Service: serviceName, Components: components}
}

// toHealthStatus reports any status other than healthy as unhealthy, so an unknown one fails safe.
func toHealthStatus(s service.Status) HealthStatus {
	if s == service.StatusHealthy {
		return HealthHealthy
	}

	return HealthUnhealthy
}
