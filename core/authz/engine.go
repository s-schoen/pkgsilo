package authz

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"sync/atomic"
	"uuid"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	"github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/cedar-policy/cedar-go/x/exp/batch"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/database"
)

// Engine is the Authorizer that decides by the Cedar policies in the database.
//
// Each request reads the policies' revision and parses them only if it changed. Every instance on the
// database thus applies a policy change on its next request, without a background process.
//
// It reads tables that features own: policies and policy_revision of the policy feature, and group_members
// of the group feature. Changing them means changing Engine, which their tests cover.
type Engine struct {
	db database.Querier
	// loaded is the latest parsed policies. Concurrent requests may parse them more than once, and an
	// older result may replace a newer one, which the next request corrects.
	loaded atomic.Pointer[revisionedPolicies]
	// decisions counts the decisions of every Decider of the engine.
	decisions decisions
	// tracer creates the span of For.
	tracer trace.Tracer
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

// New returns an Engine that reads the policies and the actors' groups from db, and counts its decisions
// with meterProvider in pkgsilo.authz.decisions, by purpose and result. For runs its queries in a span from
// tracerProvider.
func New(db database.Querier, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider) *Engine {
	return &Engine{db: db, decisions: newDecisions(meterProvider), tracer: tracerProvider.Tracer(tracerName)}
}

// For returns the Decider for the actor, with the current policies and the actor's groups.
//
// Its queries nest under the span pkgsilo.authz.load, whose attribute pkgsilo.authz.policies.reloaded tells
// whether it parsed the policies again. The span's status is Error when a query or the parse fails.
func (e *Engine) For(ctx context.Context, actor uuid.UUID) (Decider, error) {
	ctx, span := e.tracer.Start(ctx, loadSpanName)
	defer span.End()

	decider, err := e.load(ctx, span, actor)
	if err != nil {
		span.SetStatus(codes.Error, "load policies and groups")

		return nil, err
	}

	return decider, nil
}

// load returns the Decider for the actor, and sets on span whether it parsed the policies again.
func (e *Engine) load(ctx context.Context, span trace.Span, actor uuid.UUID) (Decider, error) {
	policies, err := e.currentPolicies(ctx, span)
	if err != nil {
		return nil, err
	}

	var groups []uuid.UUID
	err = e.db.SelectContext(ctx, &groups, `SELECT group_id FROM group_members WHERE identity_id = $1`, actor)
	if err != nil {
		return nil, fmt.Errorf("load groups of actor: %w", err)
	}

	return &decider{policies: policies, principal: toEntity(IdentityType, actor, groups), decisions: e.decisions}, nil
}

// currentPolicies returns the policies of the current revision, parsing them if that's not the loaded one,
// and sets on span whether it did.
func (e *Engine) currentPolicies(ctx context.Context, span trace.Span) (*cedar.PolicySet, error) {
	// Read before the policies: policies newer than their revision only cause another parse, while policies
	// older than their revision would stay in use.
	var revision int64
	if err := e.db.GetContext(ctx, &revision, `SELECT revision FROM policy_revision`); err != nil {
		return nil, fmt.Errorf("read policy revision: %w", err)
	}

	if loaded := e.loaded.Load(); loaded != nil && loaded.revision == revision {
		span.SetAttributes(reloadedKey.Bool(false))

		return loaded.policies, nil
	}
	span.SetAttributes(reloadedKey.Bool(true))

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
	decisions decisions
}

var _ Decider = (*decider)(nil)

// Can reports whether the policies allow the principal the action on the resource, and counts the decision
// as enforced.
func (d *decider) Can(action Action, resource Resource) (bool, error) {
	allowed, err := d.decide(action, resource)
	d.decisions.record(purposeEnforce, allowed, err)

	return allowed, err
}

// Hint reports whether the policies allow the principal the action on the resource, and counts the decision
// as a hint.
func (d *decider) Hint(action Action, resource Resource) (bool, error) {
	allowed, err := d.decide(action, resource)
	d.decisions.record(purposeHint, allowed, err)

	return allowed, err
}

// CanSome reports whether the policies allow the principal the action on some resource of the type, and
// counts the decision as a hint.
func (d *decider) CanSome(action Action, resourceType string) (bool, error) {
	allowed, err := d.decideSome(action, resourceType)
	d.decisions.record(purposeHint, allowed, err)

	return allowed, err
}

// decide reports whether the policies allow the principal the action on the resource.
func (d *decider) decide(action Action, resource Resource) (bool, error) {
	entities := requestEntities{list: [4]types.Entity{d.principal, action.entity}, n: 3}
	entity := toEntity(resource.Type, resource.ID, resource.Groups)
	if owner := resource.Owner; owner != nil {
		// The owner is an entity of its own, so that Cedar finds its groups.
		ownerEntity := toEntity(IdentityType, owner.ID, owner.Groups)
		entity.Attributes = types.NewRecord(types.RecordMap{OwnerAttribute: ownerEntity.UID})
		entities.list[3] = ownerEntity
		entities.n = 4
	}
	entities.list[2] = entity

	decision, diagnostic := cedar.Authorize(d.policies, entities,
		cedar.Request{Principal: d.principal.UID, Action: action.entity.UID, Resource: entity.UID},
	)
	if err := d.forbidErrors(diagnostic.Errors); err != nil {
		return false, fmt.Errorf("decide %s on %s: %w", action.entity.UID, entity.UID, err)
	}

	return decision == cedar.Allow, nil
}

// decideSome reports whether the policies allow the principal the action on some resource of the type.
//
// It relies on cedar-go's experimental batch package, whose ignored resource drops the permits' conditions
// on the resource and the forbids that have one, but lets every scope clause on the resource match, a
// forbid's too. A forbid scoped to `resource == Identity::"<admin>"` would thus deny every identity. The
// forbids that are scoped to some resources are therefore left out before, so that the answer errs only
// toward yes. The engine tests pin that behavior, so that an upgrade that changes it fails them.
func (d *decider) decideSome(action Action, resourceType string) (bool, error) {
	entities := requestEntities{list: [4]types.Entity{d.principal, action.entity}, n: 2}
	request := batch.Request{
		Principal: d.principal.UID,
		Action:    action.entity.UID,
		Resource:  batch.Ignore(),
		Context:   types.Record{},
	}

	var result batch.Result
	err := batch.Authorize(context.Background(), somePolicies{d.policies, types.EntityType(resourceType)}, entities,
		request, func(r batch.Result) error {
			result = batch.Result{Decision: r.Decision, Diagnostic: r.Diagnostic}

			return nil
		})
	if err != nil {
		return false, fmt.Errorf("decide %s on some %s: %w", action.entity.UID, resourceType, err)
	}
	if err := d.forbidErrors(result.Diagnostic.Errors); err != nil {
		return false, fmt.Errorf("decide %s on some %s: %w", action.entity.UID, resourceType, err)
	}

	return result.Decision == cedar.Allow, nil
}

// somePolicies iterates over the statements that can decide about every resource of a type: the permits,
// and the forbids that are scoped to any resource or to `resource is <the type>`.
//
// Filtering per decision is no more work than batch.Authorize does itself per decision, so the result isn't
// cached per revision and type.
type somePolicies struct {
	policies     *cedar.PolicySet
	resourceType types.EntityType
}

// All returns an iterator over the statements.
func (p somePolicies) All() iter.Seq2[cedar.PolicyID, *cedar.Policy] {
	return func(yield func(cedar.PolicyID, *cedar.Policy) bool) {
		for id, statement := range p.policies.All() {
			if p.decidesEvery(statement) && !yield(id, statement) {
				return
			}
		}
	}
}

// decidesEvery reports whether the statement can decide about every resource of the type.
func (p somePolicies) decidesEvery(statement *cedar.Policy) bool {
	if statement.Effect() == cedar.Permit {
		return true
	}
	switch scope := (*ast.Policy)(statement.AST()).Resource.(type) {
	case ast.ScopeTypeAll:
		return true
	case ast.ScopeTypeIs:
		return scope.Type == p.resourceType
	default:
		return false
	}
}

// forbidErrors joins the errors of the forbid statements that failed to evaluate. Cedar skips a statement
// that fails, which is safe for a permit, since it then grants nothing, but lets a forbid's denial pass. The
// errors of permits are thus dropped, so that one broken permit doesn't fail every decision, including
// those needed to delete it.
func (d *decider) forbidErrors(diagnosticErrs []types.DiagnosticError) error {
	var errs []error
	for _, diagnosticErr := range diagnosticErrs {
		if statement := d.policies.Get(diagnosticErr.PolicyID); statement != nil && statement.Effect() == cedar.Permit {
			continue
		}
		errs = append(errs, errors.New(diagnosticErr.String()))
	}

	return errors.Join(errs...)
}

// requestEntities holds the entities of one decision, for Cedar to look up: the principal, the action, the
// resource and, if it has one, the resource's owner. Building it instead of a map keeps checking the many
// rows of a list cheap.
type requestEntities struct {
	list [4]types.Entity
	// n is the number of entities in list.
	n int
}

// Get returns the entity with the given ID. If the principal owns the resource, both entities have the
// same ID and groups, so it doesn't matter which one it returns.
func (e requestEntities) Get(uid types.EntityUID) (types.Entity, bool) {
	for _, entity := range e.list[:e.n] {
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
		parents[i] = types.NewEntityUID(GroupType, types.String(group.String()))
	}

	return types.Entity{
		UID:     types.NewEntityUID(types.EntityType(entityType), types.String(id.String())),
		Parents: types.NewEntityUIDSet(parents...),
	}
}
