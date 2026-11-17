package authz

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"pkgsilo/core/database"
)

// ErrOwnerNotFound is returned when the identity that owns a resource doesn't exist, or the actor may not read
// it: the two cases look the same.
var ErrOwnerNotFound = errors.New("owner not found")

// OwnerLoader loads the identities that own the resources of types declared as Owned, for the services of
// those types.
type OwnerLoader interface {
	// ReadableOwner returns the identity with the given ID as an owner, with its groups, if the decider allows
	// reading it. It returns ErrOwnerNotFound if the identity doesn't exist, or if the decider doesn't allow
	// reading it, so that no endpoint of an owned resource reveals whether its owner exists.
	ReadableOwner(ctx context.Context, decider Decider, identityID uuid.UUID) (*Owner, error)
}

// Owners is the OwnerLoader that reads owners from the database.
//
// It reads tables that features own, like Engine: identities of the identity feature and group_members of the
// group feature. Changing them means changing Owners, which its tests cover.
type Owners struct {
	db database.Querier
	// read is the action of reading an identity.
	read Action
}

var _ OwnerLoader = (*Owners)(nil)

// NewOwners returns Owners that reads owners from db, and lets a decider read one if it allows read, the
// identity feature's action of reading an identity.
func NewOwners(db database.Querier, read Action) *Owners {
	return &Owners{db: db, read: read}
}

// ReadableOwner returns the identity with the given ID as an owner, with its groups, if the decider allows
// reading it. It decides nothing for an identity that doesn't exist.
func (o *Owners) ReadableOwner(ctx context.Context, decider Decider, identityID uuid.UUID) (*Owner, error) {
	groups, err := o.groups(ctx, identityID)
	if err != nil {
		return nil, err
	}

	can, err := decider.Can(o.read, Resource{Type: IdentityType, ID: identityID, Groups: groups})
	if err != nil {
		return nil, fmt.Errorf("check owner read: %w", err)
	}
	if !can {
		return nil, ErrOwnerNotFound
	}

	return &Owner{ID: identityID, Groups: groups}, nil
}

// groups returns the IDs of the groups that the identity with the given ID is a member of, in no particular
// order, or ErrOwnerNotFound if the identity doesn't exist. One query tells both, so that the identity can't
// be deleted between them.
func (o *Owners) groups(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error) {
	// One row per group, or one row without a group for an identity in none.
	var rows []struct {
		GroupID *uuid.UUID `db:"group_id"`
	}
	err := o.db.SelectContext(ctx, &rows, `
		SELECT m.group_id FROM identities i
		LEFT JOIN group_members m ON m.identity_id = i.id
		WHERE i.id = $1`,
		identityID,
	)
	if err != nil {
		return nil, fmt.Errorf("load owner groups: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrOwnerNotFound
	}

	var groups []uuid.UUID
	for _, row := range rows {
		if row.GroupID != nil {
			groups = append(groups, *row.GroupID)
		}
	}

	return groups, nil
}
