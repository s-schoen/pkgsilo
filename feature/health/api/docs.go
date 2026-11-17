package api

import (
	"net/http"

	"pkgsilo/core/openapi"
)

// Docs describes the health endpoint of Routes in the OpenAPI document.
func Docs() openapi.Section {
	return openapi.Section{
		Name:        "Health",
		Description: "The health of pkgsilo, for load balancers and monitoring. It needs no authentication.",
		Operations: []openapi.Operation{
			{
				Method: http.MethodGet, Pattern: "/", ID: "checkHealth", Summary: "Check the health",
				Description: "Reports whether pkgsilo can do its work: healthy, degraded or unhealthy.",
				Responses: []openapi.Response{
					openapi.JSON[HealthResponse](http.StatusOK, "pkgsilo is healthy or degraded."),
					openapi.JSON[HealthResponse](http.StatusServiceUnavailable, "pkgsilo is unhealthy."),
				},
			},
		},
	}
}
