package authz

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"uuid"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"

	"pkgsilo/core/database"
)

// Engine is the Authorizer that decides by the Cedar policies in the database.
//
// Each request reads the policies' revision and parses them only if it changed. Every instance on the
// database thus applies a policy change on its next request, without a background process.
type Engine struct {
	db database.Querier
	// loaded is the latest parsed policies. Concurrent requests may parse them more than once, and an
	// older result may replace a newer one, which the next request corrects.
	loaded atomic.Pointer[revisionedPolicies]
}

var _ Authorizer = (*Engine)(nil)

// revisionedPolicies holds the parsed policies of a revision.
type revisionedPolicies struct {
	revision int64
	policies *cedar.PolicySet
}

// policyRecord is a row of the policies table, as far as decisions need it.
type policyRecord struct {
	ID     uuid.UUID `db:"id"`
	Source string    `db:"source"`
}

// New returns an Engine that reads the policies and the actors' groups from db.
func New(db database.Querier) *Engine {
	return &Engine{db: db}
}

// For returns the Decider for the actor, with the current policies and the actor's groups.
func (e *Engine) For(ctx context.Context, actor uuid.UUID) (Decider, error) {
	policies, err := e.currentPolicies(ctx)
	if err != nil {
		return nil, err
	}

	var groups []uuid.UUID
	err = e.db.SelectContext(ctx, &groups, `SELECT group_id FROM group_members WHERE identity_id = $1`, actor)
	if err != nil {
		return nil, fmt.Errorf("load groups of actor: %w", err)
	}

	return &decider{policies: policies, principal: toEntity(IdentityType, actor, groups)}, nil
}

// currentPolicies returns the policies of the current revision, parsing them if that's not the loaded one.
func (e *Engine) currentPolicies(ctx context.Context) (*cedar.PolicySet, error) {
	// Read before the policies: policies newer than their revision only cause another parse, while policies
	// older than their revision would stay in use.
	var revision int64
	if err := e.db.GetContext(ctx, &revision, `SELECT revision FROM policy_revision`); err != nil {
		return nil, fmt.Errorf("read policy revision: %w", err)
	}

	if loaded := e.loaded.Load(); loaded != nil && loaded.revision == revision {
		return loaded.policies, nil
	}

	var recs []policyRecord
	if err := e.db.SelectContext(ctx, &recs, `SELECT id, source FROM policies`); err != nil {
		return nil, fmt.Errorf("load policies: %w", err)
	}

	policies, err := parsePolicies(recs)
	if err != nil {
		return nil, err
	}
	e.loaded.Store(&revisionedPolicies{revision: revision, policies: policies})

	return policies, nil
}

// parsePolicies returns the statements of the policies as one policy set. Each statement's ID is
// <policy id>#<n>, so that evaluation errors name the policy and the statement in it.
func parsePolicies(recs []policyRecord) (*cedar.PolicySet, error) {
	policies := cedar.NewPolicySet()
	for _, rec := range recs {
		statements, err := cedar.NewPolicyListFromBytes(rec.ID.String(), []byte(rec.Source))
		if err != nil {
			return nil, fmt.Errorf("parse policy %s: %w", rec.ID, err)
		}
		for i, statement := range statements {
			policies.Add(cedar.PolicyID(fmt.Sprintf("%s#%d", rec.ID, i)), statement)
		}
	}

	return policies, nil
}

// decider decides by the policies for one principal.
type decider struct {
	policies  *cedar.PolicySet
	principal types.Entity
}

var _ Decider = (*decider)(nil)

// Can reports whether the policies allow the principal the action on the resource.
func (d *decider) Can(action Action, resource Resource) (bool, error) {
	entity := toEntity(resource.Type, resource.ID, resource.Groups)

	decision, diagnostic := cedar.Authorize(d.policies,
		requestEntities{d.principal, action.entity, entity},
		cedar.Request{Principal: d.principal.UID, Action: action.entity.UID, Resource: entity.UID},
	)
	if len(diagnostic.Errors) > 0 {
		errs := make([]error, len(diagnostic.Errors))
		for i, diagnosticErr := range diagnostic.Errors {
			errs[i] = errors.New(diagnosticErr.String())
		}

		return false, fmt.Errorf("decide %s on %s: %w", action.entity.UID, entity.UID, errors.Join(errs...))
	}

	return decision == cedar.Allow, nil
}

// requestEntities holds the entities of one decision, for Cedar to look up. Building it instead of a map
// keeps checking the many rows of a list cheap.
type requestEntities [3]types.Entity

// Get returns the entity with the given ID.
func (e requestEntities) Get(uid types.EntityUID) (types.Entity, bool) {
	for _, entity := range e {
		if entity.UID == uid {
			return entity, true
		}
	}

	return types.Entity{}, false
}

// toEntity returns the Cedar entity of the given type and ID, as a member of the given groups.
func toEntity(entityType string, id uuid.UUID, groups []uuid.UUID) types.Entity {
	parents := make([]types.EntityUID, len(groups))
	for i, group := range groups {
		parents[i] = types.NewEntityUID(groupType, types.String(group.String()))
	}

	return types.Entity{
		UID:     types.NewEntityUID(types.EntityType(entityType), types.String(id.String())),
		Parents: types.NewEntityUIDSet(parents...),
	}
}
