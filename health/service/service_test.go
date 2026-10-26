package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pkgsilo/health/repository/mocks"
)

func TestHealthService_Check_Healthy(t *testing.T) {
	repo := mocks.NewMockHealthRepository(t)
	repo.EXPECT().Ping(mock.Anything).Return(nil)

	got := NewHealthService(repo).Check(t.Context())

	assert.Equal(t, Health{
		Status:     StatusHealthy,
		Components: []Component{{Name: "database", Status: StatusHealthy}},
	}, got)
}

func TestHealthService_Check_DatabaseUnreachable(t *testing.T) {
	repo := mocks.NewMockHealthRepository(t)
	errDB := errors.New("connection refused")
	repo.EXPECT().Ping(mock.Anything).Return(errDB)

	got := NewHealthService(repo).Check(t.Context())

	assert.Equal(t, StatusUnhealthy, got.Status)
	assert.Len(t, got.Components, 1)
	assert.Equal(t, "database", got.Components[0].Name)
	assert.Equal(t, StatusUnhealthy, got.Components[0].Status)
	assert.ErrorIs(t, got.Components[0].Cause, errDB)
}

func TestHealthService_Check_BoundsPing(t *testing.T) {
	repo := mocks.NewMockHealthRepository(t)
	// The deadline was set before the matcher runs, so it can't be later than checkTimeout from now.
	repo.EXPECT().Ping(mock.MatchedBy(func(ctx context.Context) bool {
		deadline, ok := ctx.Deadline()
		return ok && !deadline.After(time.Now().Add(checkTimeout))
	})).Return(nil)

	NewHealthService(repo).Check(t.Context())
}
