// Package authz decides what an authenticated identity may do, by the Cedar policies in the database.
//
// It knows no features. Each feature declares its resource type with DeclareType, which derives the actions
// on it from the authz tags of the feature's types, and enforces them in its service: it gets a Decider for
// the request's actor from an Authorizer and asks it about each resource. The fields of a readable object
// have read actions of their own, and a service reveals only those that the actor may read, as a Field,
// with a Mapper from the object's record. Each object also tells which of the other actions on it the actor
// may do, its AllowedActions, which only guide a client: they're decided as hints, not enforced. Everything
// that no policy permits is denied.
//
// Its model has two types of its own: Identity, the principal of every decision and the owner of owned
// resources, and Group, which identities and other resources are members of. Their features declare their
// actions like any other, but core names the types, since every decision and the schema use them. Every
// other resource type is a feature's own and never appears here.
//
// # Accepted risks
//
//   - Timing: a list checks all rows of its type, so its response time reveals a rough total row count of
//     that type, though not where the hidden rows sit. Likewise, a hidden object takes slightly longer to
//     answer than a missing one, which only helps a caller who already holds its ID. Never add a filter or
//     search parameter on a field the caller can't read: it turns this into "does this exact value exist?",
//     which has been attacked remotely against row-level security
//     (https://doi.org/10.1145/3588943).
//   - Uniqueness conflicts: a taken unique value, such as a username, reveals that it exists even if the
//     caller can't see the object.
//   - Lockout: a bad policy, or removing the last administrator, can lock everyone out. There's no bypass,
//     last-member guard or dry run. Recovery means fixing the data directly in the database.
package authz

import (
	"context"
	"fmt"
	"uuid"

	"github.com/cedar-policy/cedar-go/types"
)

const (
	// IdentityType is the Cedar entity type of identities. Every decision's principal is one. The identity
	// feature declares its actions with DeclareType, and stored policies use the name, so it can't change.
	IdentityType = "Identity"
	// GroupType is the Cedar entity type of groups, which identities are members of. A group as a resource
	// is the same entity as a group of the principal, so that `principal in resource` matches its members.
	// The group feature declares its actions with DeclareType, and stored policies use the name, so it
	// can't change.
	GroupType = "Group"
	// ActionGroupRead is the action group that bundles all reading actions, for policies such as a
	// read-only audit group.
	ActionGroupRead = "read"
	// OwnerAttribute is the attribute of the resources of a type declared as Owned that holds the identity
	// they belong to, for policies such as `resource.owner == principal`.
	OwnerAttribute = "owner"

	actionType types.EntityType = "Action"
)

// Action is what a caller wants to do to a resource.
//
// It carries its Cedar entity, since action groups only work if Cedar can find the action's parents. That
// way, no decision can miss them, which would make a forbid written with `action in` allow what it should
// deny.
type Action struct {
	entity types.Entity
}

// NewAction returns the action with the given name, such as identity:read, as a member of the given
// action groups. Action groups only bundle other actions, and are never requested themselves.
func NewAction(name string, groups ...string) Action {
	parents := make([]types.EntityUID, len(groups))
	for i, group := range groups {
		parents[i] = types.NewEntityUID(actionType, types.String(group))
	}

	return Action{entity: types.Entity{
		UID:     types.NewEntityUID(actionType, types.String(name)),
		Parents: types.NewEntityUIDSet(parents...),
	}}
}

// Name returns the action's name, such as identity:read.
func (a Action) Name() string {
	return string(a.entity.UID.ID)
}

// Resource is what an action is done to.
type Resource struct {
	// Type is the resource's Cedar entity type.
	Type string
	ID   uuid.UUID
	// Groups holds the groups that the resource is a member of. A resource that can be a group member
	// must always carry its groups: without them, `resource in Group::"…"` silently doesn't match, and
	// a forbid written that way allows what it should deny.
	Groups []uuid.UUID
	// Owner is the identity that the resource belongs to, which a resource of a type declared as Owned must
	// always carry, and others never: a policy that refers to an owner that a resource lacks fails every
	// decision on it.
	Owner *Owner
}

// Owner is the identity that a resource belongs to. It must always carry the identity's groups, for the
// same reason as Resource.Groups: without them, `resource.owner in Group::"…"` silently doesn't match.
type Owner struct {
	ID     uuid.UUID
	Groups []uuid.UUID
}

// Authorizer hands out the decisions for a request.
type Authorizer interface {
	// For returns the Decider for the actor, for the rest of the request. It loads the actor's groups
	// once, so that checking many resources doesn't take many queries.
	For(ctx context.Context, actor uuid.UUID) (Decider, error)
}

// Decider decides what one actor may do.
type Decider interface {
	// Can reports whether the actor may do the action to the resource. A forbid that fails to evaluate
	// results in an error and never in access, since Cedar skips such a statement, so that it would
	// otherwise allow what it should deny. A permit that fails to evaluate grants nothing, without an error,
	// so that one broken permit can't fail every decision.
	Can(action Action, resource Resource) (bool, error)
	// Hint decides like Can, for an answer that only guides a client about what it may do, such as the
	// allowed actions of an object, and is counted as a hint rather than as a check that a request passes
	// or fails by. It must never decide whether a request is allowed.
	//
	// A caller counts an error as not allowed rather than failing on it: the request that it hints at fails
	// the same way, and the decider counts the error.
	Hint(action Action, resource Resource) (bool, error)
	// CanSome reports whether the actor may do the action to some resource of the type, such as "may the
	// actor delete identities?", for a client that has no object in hand. It's decided as a hint, like
	// Hint, and must never decide whether a request is allowed: it isn't exact.
	//
	// It errs only toward yes. A permit whose condition refers to the resource counts as matching some
	// resource, and so does a forbid that is scoped to some resources, by `resource ==`, `resource in` or
	// `resource is` another type, or whose condition refers to the resource: neither can deny every
	// resource of the type. Only an unscoped forbid and one scoped to `resource is <the type>` without a
	// condition on the resource deny. The principal's scope and conditions are evaluated exactly. As for
	// Can, a forbid that fails to evaluate results in an error, and a permit that fails grants nothing. A
	// caller counts an error as not allowed, as for Hint: such a forbid fails the action on every resource.
	CanSome(action Action, resourceType string) (bool, error)
}

// AllowAll returns an Authorizer that allows everything to everyone. It's only for code that runs without
// a caller, such as the first start. It's chosen when the code is wired up, so that no actor value can
// bypass the checks of the running application.
func AllowAll() Authorizer {
	return allowAll{}
}

type allowAll struct{}

func (allowAll) For(context.Context, uuid.UUID) (Decider, error) {
	return allowAll{}, nil
}

func (allowAll) Can(Action, Resource) (bool, error) {
	return true, nil
}

func (allowAll) Hint(Action, Resource) (bool, error) {
	return true, nil
}

func (allowAll) CanSome(Action, string) (bool, error) {
	return true, nil
}

// Filter returns the rows on which the decider allows the action, in their order, for a list. It checks
// every row, so that the time it takes doesn't depend on where the hidden rows sit. Between rows, it checks
// ctx, since a decision can't be canceled.
func Filter[T any](
	ctx context.Context, decider Decider, action Action, rows []T, resource func(T) Resource,
) ([]T, error) {
	var allowed []T
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("check rows: %w", err)
		}

		can, err := decider.Can(action, resource(row))
		if err != nil {
			return nil, fmt.Errorf("check rows: %w", err)
		}
		if can {
			allowed = append(allowed, row)
		}
	}

	return allowed, nil
}
