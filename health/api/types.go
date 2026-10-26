package api

import (
	"pkgsilo/core/envelope"
	"pkgsilo/health/service"
)

// serviceName identifies pkgsilo in health responses.
const serviceName = "pkgsilo"

func toHealthResponse(h service.Health) envelope.Health {
	components := make([]envelope.ComponentHealth, len(h.Components))
	for i, c := range h.Components {
		components[i] = envelope.ComponentHealth{Name: c.Name, Status: toHealthStatus(c.Status)}
	}

	return envelope.Health{Status: toHealthStatus(h.Status), Service: serviceName, Components: components}
}

// toHealthStatus reports any status other than healthy as unhealthy, so an unknown one fails safe.
func toHealthStatus(s service.Status) envelope.HealthStatus {
	if s == service.StatusHealthy {
		return envelope.HealthHealthy
	}

	return envelope.HealthUnhealthy
}
