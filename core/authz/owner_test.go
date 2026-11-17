package authz

import (
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/test/databasetest"
)

// readIdentity is the action of reading an identity, as the identity feature declares it.
var readIdentity = NewAction("identity:read", ActionGroupRead)

// recordingDecider answers every decision with can and err, and records the resources it was asked about.
type recordingDecider struct {
	can   bool
	err   error
	asked []Resource
}

func (d *recordingDecider) Can(action Action, resource Resource) (bool, error) {
	if action.entity.UID != readIdentity.entity.UID {
		panic(fmt.Sprintf("unexpected action %s", action.entity.UID))
	}
	d.asked = append(d.asked, resource)

	return d.can, d.err
}

func TestOwners_ReadableOwner(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	ada, grace := insertIdentity(t, db), insertIdentity(t, db)
	maintainers, reviewers := insertGroup(t, db, ada), insertGroup(t, db, ada, grace)
	// Another identity's group, which must not be loaded.
	insertGroup(t, db, grace)
	decider := &recordingDecider{can: true}

	owner, err := NewOwners(db, readIdentity).ReadableOwner(t.Context(), decider, ada)
	require.NoError(t, err)

	assert.Equal(t, ada, owner.ID)
	assert.ElementsMatch(t, []uuid.UUID{maintainers, reviewers}, owner.Groups)
	require.Len(t, decider.asked, 1)
	assert.Equal(t, IdentityType, decider.asked[0].Type)
	assert.Equal(t, ada, decider.asked[0].ID)
	assert.ElementsMatch(t, []uuid.UUID{maintainers, reviewers}, decider.asked[0].Groups,
		"the decision must see the identity's groups, so that a forbid on a group applies")
}

func TestOwners_ReadableOwner_NoGroups(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	insertGroup(t, db)
	ada := insertIdentity(t, db)

	owner, err := NewOwners(db, readIdentity).ReadableOwner(t.Context(), &recordingDecider{can: true}, ada)
	require.NoError(t, err)

	assert.Equal(t, &Owner{ID: ada}, owner)
}

// TestOwners_ReadableOwner_NotFound checks that a hidden identity looks like a missing one, and that a missing
// one is reported without a decision.
func TestOwners_ReadableOwner_NotFound(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	owners := NewOwners(db, readIdentity)

	t.Run("hidden", func(t *testing.T) {
		t.Parallel()
		_, err := owners.ReadableOwner(t.Context(), &recordingDecider{can: false}, insertIdentity(t, db))
		assert.ErrorIs(t, err, ErrOwnerNotFound)
	})
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		decider := &recordingDecider{can: true}

		_, err := owners.ReadableOwner(t.Context(), decider, uuid.NewV4())

		require.ErrorIs(t, err, ErrOwnerNotFound)
		assert.Empty(t, decider.asked)
	})
}

// TestOwners_ReadableOwner_DecisionFails checks that failing to decide is an error and never a missing owner,
// which would hide a broken policy from the logs.
func TestOwners_ReadableOwner_DecisionFails(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	errDecision := errors.New("policy failed")

	_, err := NewOwners(db, readIdentity).ReadableOwner(t.Context(), &recordingDecider{err: errDecision},
		insertIdentity(t, db))

	require.ErrorIs(t, err, errDecision)
	assert.NotErrorIs(t, err, ErrOwnerNotFound)
}
