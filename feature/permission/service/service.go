// Package service implements the permissions domain: what the policies allow the actor to do, for a client
// that has no object in hand, such as a web UI that hides what would be denied. It has no repository: the
// answers are the policies' decisions on the resource types that the other features declare.
package service

import (
	"context"
	"fmt"
	"slices"
	"uuid"

	"pkgsilo/core/authz"
)

// PermissionService tells an actor what the policies allow it to do. Its answers only guide a client, and
// count as hints: every request is still checked on its own.
type PermissionService interface {
	// List returns the actions that the policies allow the actor to do. identityID must be the actor's own:
	// another identity is reported with ErrIdentityNotFound if the actor may not read it, like a missing one,
	// and with authz.ErrDenied otherwise.
	//
	// Without owner, it lists every declared action that the actor may do to some resource of its type: the
	// creates of a type that isn't owned exactly, on a new resource as a create checks it, and every other
	// action, the creates of owned types too, by authz.Decider.CanSome, which errs only toward yes.
	//
	// With owner, it lists exactly the actions of the owned types for that identity: the creates of each
	// owned type on a new resource of the owner, and every action of a type that is OnePerOwner on the
	// owner's one resource. An owner that the actor may not read is reported with ErrIdentityNotFound.
	List(ctx context.Context, actor, identityID uuid.UUID, owner *uuid.UUID) (Permissions, error)
}

type permissionService struct {
	types      []authz.TypeActions
	authorizer authz.Authorizer
	owners     authz.OwnerLoader
}

// NewPermissionService returns a PermissionService that lists the actions of types, every resource type that
// the features declare, asks authorizer what an actor may do, and loads identities with owners.
func NewPermissionService(
	types []authz.TypeActions, authorizer authz.Authorizer, owners authz.OwnerLoader,
) PermissionService {
	return &permissionService{types: slices.Clone(types), authorizer: authorizer, owners: owners}
}

func (s *permissionService) List(
	ctx context.Context, actor, identityID uuid.UUID, owner *uuid.UUID,
) (Permissions, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Permissions{}, fmt.Errorf("list permissions: %w", err)
	}

	if identityID != actor {
		// Only to answer an identity that the actor may not read like a missing one: no actor may list the
		// permissions of another.
		if _, err := s.owners.ReadableOwner(ctx, decider, identityID); err != nil {
			return Permissions{}, fmt.Errorf("list permissions: %w", translateError(err))
		}

		return Permissions{}, fmt.Errorf("list permissions: %w", authz.ErrDenied)
	}

	var allowed authz.AllowedActions
	if owner == nil {
		allowed, err = s.allowedOnSome(ctx, decider)
	} else {
		allowed, err = s.allowedForOwner(ctx, decider, *owner)
	}
	if err != nil {
		return Permissions{}, fmt.Errorf("list permissions: %w", err)
	}

	return Permissions{ID: identityID, AllowedActions: allowed}, nil
}

// allowedOnSome returns the names of the actions that the decider allows on some resource of their type, never
// nil.
func (s *permissionService) allowedOnSome(ctx context.Context, decider authz.Decider) (authz.AllowedActions, error) {
	allowed := authz.AllowedActions{}
	for _, t := range s.types {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("check actions: %w", err)
		}

		// What a create of a type that isn't owned checks: a new resource, with a fresh ID and in no group.
		fresh := authz.Resource{Type: t.Type(), ID: uuid.NewV4()}
		for _, action := range t.Actions() {
			var can bool
			var err error
			if !t.Owned() && isCreate(t, action) {
				can, err = decider.Hint(action, fresh)
			} else {
				can, err = decider.CanSome(action, t.Type())
			}
			if err != nil {
				return nil, fmt.Errorf("check %s: %w", action.Name(), err)
			}
			if can {
				allowed = append(allowed, action.Name())
			}
		}
	}

	return allowed, nil
}

// allowedForOwner returns the names of the actions of the owned types that the decider allows for the
// identity with the given ID as their owner, never nil, or authz.ErrOwnerNotFound translated if the decider
// doesn't allow reading the identity.
func (s *permissionService) allowedForOwner(
	ctx context.Context, decider authz.Decider, ownerID uuid.UUID,
) (authz.AllowedActions, error) {
	owner, err := s.owners.ReadableOwner(ctx, decider, ownerID)
	if err != nil {
		return nil, translateError(err)
	}

	allowed := authz.AllowedActions{}
	for _, t := range s.types {
		if !t.Owned() {
			continue
		}

		resource := t.NewResource(owner)
		for _, action := range t.Actions() {
			if !t.OnePerOwner() && !isCreate(t, action) {
				continue
			}

			can, err := decider.Hint(action, resource)
			if err != nil {
				return nil, fmt.Errorf("check %s: %w", action.Name(), err)
			}
			if can {
				allowed = append(allowed, action.Name())
			}
		}
	}

	return allowed, nil
}

// isCreate reports whether the action is one of the field actions of creating a resource of the type.
func isCreate(t authz.TypeActions, action authz.Action) bool {
	return slices.ContainsFunc(t.CreateFields().Actions(), func(create authz.Action) bool {
		return create.Name() == action.Name()
	})
}
