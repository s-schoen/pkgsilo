// Package service implements the health domain: whether pkgsilo and the components it depends on can do their work.
package service

// Status is the state of pkgsilo or one of its components.
type Status string

const (
	// StatusHealthy means it can do its work.
	StatusHealthy Status = "healthy"
	// StatusUnhealthy means it can't do its work.
	StatusUnhealthy Status = "unhealthy"
)

// Health is the aggregate health of pkgsilo: healthy only when all its components are.
type Health struct {
	Status     Status
	Components []Component
}

// Component is the health of a dependency of pkgsilo.
type Component struct {
	Name   string
	Status Status
	// Cause is why the component isn't healthy, or nil. It may contain sensitive details,
	// such as internal addresses, so it's meant for logs only.
	Cause error
}
