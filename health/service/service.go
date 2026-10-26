package service

import (
	"context"
	"fmt"
	"time"

	"pkgsilo/health/repository"
)

// checkTimeout bounds each component check, so a hanging dependency is reported as unhealthy
// instead of stalling the health check.
const checkTimeout = 2 * time.Second

// HealthService checks the health of pkgsilo.
type HealthService interface {
	// Check checks every component. A failing component is a result, not an error.
	Check(ctx context.Context) Health
}

type healthService struct {
	repo repository.HealthRepository
}

// NewHealthService returns a HealthService that reaches the components through repo.
func NewHealthService(repo repository.HealthRepository) HealthService {
	return &healthService{repo: repo}
}

func (s *healthService) Check(ctx context.Context) Health {
	health := Health{
		Status:     StatusHealthy,
		Components: []Component{s.checkDatabase(ctx)},
	}
	for _, c := range health.Components {
		if c.Status != StatusHealthy {
			health.Status = StatusUnhealthy
		}
	}

	return health
}

func (s *healthService) checkDatabase(ctx context.Context) Component {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	if err := s.repo.Ping(ctx); err != nil {
		return Component{Name: "database", Status: StatusUnhealthy, Cause: fmt.Errorf("check database: %w", err)}
	}

	return Component{Name: "database", Status: StatusHealthy}
}
