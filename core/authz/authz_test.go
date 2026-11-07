package authz

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deciderFunc is a Decider that decides by the function.
type deciderFunc func(Action, Resource) (bool, error)

func (f deciderFunc) Can(action Action, resource Resource) (bool, error) {
	return f(action, resource)
}

var testAction = NewAction("identity:read", ActionGroupRead)

func TestAllowAll(t *testing.T) {
	t.Parallel()

	decider, err := AllowAll().For(t.Context(), uuid.NewV4())
	require.NoError(t, err)

	can, err := decider.Can(testAction, Resource{Type: IdentityType, ID: uuid.NewV4()})
	require.NoError(t, err)
	assert.True(t, can)
}

// toRowResource returns the resource of a row of the tests of Filter: its ID ends in the row's value.
func toRowResource(row int) Resource {
	return Resource{Type: IdentityType, ID: uuid.UUID{15: byte(row)}}
}

func TestFilter(t *testing.T) {
	t.Parallel()
	var checked []int
	decider := deciderFunc(func(action Action, resource Resource) (bool, error) {
		assert.Equal(t, testAction, action)
		row := int(resource.ID[15])
		checked = append(checked, row)

		return row%2 == 0, nil
	})

	allowed, err := Filter(t.Context(), decider, testAction, []int{1, 2, 3, 4, 5}, toRowResource)
	require.NoError(t, err)

	assert.Equal(t, []int{2, 4}, allowed, "allowed rows must keep their order")
	assert.Equal(t, []int{1, 2, 3, 4, 5}, checked, "every row must be checked, so that timing reveals no position")
}

func TestFilter_DecisionFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")
	decider := deciderFunc(func(_ Action, resource Resource) (bool, error) {
		if resource.ID[15] == 2 {
			return false, errDecision
		}

		return true, nil
	})

	allowed, err := Filter(t.Context(), decider, testAction, []int{1, 2, 3}, toRowResource)

	require.ErrorIs(t, err, errDecision)
	assert.Nil(t, allowed, "a failed decision must not return the rows allowed so far")
}

func TestFilter_ChecksContextBetweenRows(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	checks := 0
	decider := deciderFunc(func(Action, Resource) (bool, error) {
		checks++
		cancel()

		return true, nil
	})

	allowed, err := Filter(ctx, decider, testAction, []int{1, 2, 3}, toRowResource)

	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, allowed)
	assert.Equal(t, 1, checks, "no row may be checked after the request was canceled")
}
