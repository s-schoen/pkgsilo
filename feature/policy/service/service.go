package service

import (
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"pkgsilo/core/authz"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/policy/repository"
)

// PolicyService manages policies. Changes are attributed to the given actor, and apply to the next
// request on every instance.
//
// Saving a policy checks its source first, and returns an *InvalidPolicyError, without storing anything,
// if it has a mistake: if it's too large or can't be parsed, if it holds no statements or would take all
// policies over 1,000 statements, if a statement doesn't match the schema or tests for an undeclared
// attribute with has, or if it references a group that doesn't exist. The source is checked after the
// actor was allowed to write, so that it reveals no groups to others.
//
// Every policy that it returns holds only the fields that the actor may read, and the ID.
type PolicyService interface {
	// Create stores a new policy with the given input. It returns an *authz.FieldsDeniedError, without
	// storing anything, if the actor may not set any of its fields. A new policy that the actor may not
	// read is returned with only its ID.
	Create(ctx context.Context, actor uuid.UUID, in PolicyInput) (Policy, error)
	// Get returns the policy with the given ID. A policy that the actor may not read is reported like a
	// missing one.
	Get(ctx context.Context, actor, id uuid.UUID) (Policy, error)
	// List returns the page of up to limit policies that skips the first offset, out of those that the
	// actor may read, ordered by creation time. Only the last page can be short.
	List(ctx context.Context, actor uuid.UUID, offset, limit int) (pagination.Page[Policy], error)
	// Replace overwrites all caller-set fields of the policy with the given ID. A policy that the actor may
	// not read is reported like a missing one. It returns an *authz.FieldsDeniedError, without writing
	// anything, if the actor may not set any of the fields, even to their current values.
	Replace(ctx context.Context, actor, id uuid.UUID, in PolicyInput) (Policy, error)
	// Patch overwrites the fields that the patch sets of the policy with the given ID, and keeps the others,
	// even if they change concurrently. A policy that the actor may not read is reported like a missing one.
	// It returns an *authz.FieldsDeniedError, without writing anything, if the actor may not set any of the
	// fields that the patch sets. The source that results from the patch is checked like a saved one, even
	// if the patch keeps it. A patch that sets no field writes nothing, so that the policy is returned as it
	// is.
	Patch(ctx context.Context, actor, id uuid.UUID, patch PolicyPatch) (Policy, error)
	// Delete removes the policy with the given ID. A policy that the actor may not read is reported like a
	// missing one, and one that it may read but not delete with authz.ErrDenied.
	Delete(ctx context.Context, actor, id uuid.UUID) error
}

type policyService struct {
	repo       repository.PolicyRepository
	authorizer authz.Authorizer
	checker    *checker
	now        func() time.Time
}

// NewPolicyService returns a PolicyService that stores policies in repo, asks authorizer what an actor may
// do, and validates policies against the Cedar schema, in the human-readable format, such as authz.Schema
// returns. It panics if the schema is invalid, which is a bug in the declarations.
func NewPolicyService(repo repository.PolicyRepository, authorizer authz.Authorizer, schema string) PolicyService {
	return newPolicyService(repo, authorizer, schema, time.Now)
}

func newPolicyService(
	repo repository.PolicyRepository, authorizer authz.Authorizer, schema string, now func() time.Time,
) *policyService {
	checker, err := newChecker(schema)
	if err != nil {
		panic(fmt.Errorf("policy service: %w", err))
	}

	return &policyService{repo: repo, authorizer: authorizer, checker: checker, now: now}
}

func (s *policyService) Create(ctx context.Context, actor uuid.UUID, in PolicyInput) (Policy, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Policy{}, fmt.Errorf("create policy: %w", err)
	}

	id := uuid.NewV4()
	if err := createFields.Check(decider, toResource(id)); err != nil {
		return Policy{}, fmt.Errorf("create policy: %w", err)
	}

	source, err := s.checkSource(ctx, in.Source)
	if err != nil {
		return Policy{}, fmt.Errorf("create policy: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := readFields.ReadableIf(decider, actionRead, toResource(id))
	if err != nil {
		return Policy{}, fmt.Errorf("create policy: %w", err)
	}

	now := s.now()
	var rec repository.PolicyRecord
	err = s.repo.Write(ctx, func(w repository.PolicyWriter) error {
		if err := checkTotal(ctx, w, id, source.statements); err != nil {
			return err
		}

		stored, err := w.Insert(ctx, repository.PolicyRecord{
			ID:        id,
			Name:      in.Name,
			Source:    in.Source,
			CreatedBy: actor,
			UpdatedBy: actor,
			CreatedAt: now,
			UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("store: %w", err)
		}
		rec = stored

		return nil
	})
	if err != nil {
		return Policy{}, fmt.Errorf("create policy: %w", translateError(err))
	}

	return toPolicy(rec, fields), nil
}

func (s *policyService) Get(ctx context.Context, actor, id uuid.UUID) (Policy, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Policy{}, fmt.Errorf("get policy: %w", err)
	}

	rec, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return Policy{}, fmt.Errorf("get policy: %w", err)
	}

	fields, err := readFields.Readable(decider, toResource(id))
	if err != nil {
		return Policy{}, fmt.Errorf("get policy: %w", err)
	}

	return toPolicy(rec, fields), nil
}

func (s *policyService) List(ctx context.Context, actor uuid.UUID, offset, limit int) (pagination.Page[Policy], error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return pagination.Page[Policy]{}, fmt.Errorf("list policies: %w", err)
	}

	recs, err := s.repo.ListAll(ctx)
	if err != nil {
		return pagination.Page[Policy]{}, fmt.Errorf("list policies: %w", err)
	}

	readable, err := authz.Filter(ctx, decider, actionRead, recs, func(rec repository.PolicyRecord) authz.Resource {
		return toResource(rec.ID)
	})
	if err != nil {
		return pagination.Page[Policy]{}, fmt.Errorf("list policies: %w", err)
	}

	// Only the fields of the rows on the page are checked.
	page := pagination.FromAll(readable, offset, limit)
	policies := make([]Policy, len(page.Items))
	for i, rec := range page.Items {
		fields, err := readFields.Readable(decider, toResource(rec.ID))
		if err != nil {
			return pagination.Page[Policy]{}, fmt.Errorf("list policies: %w", err)
		}
		policies[i] = toPolicy(rec, fields)
	}

	return pagination.Page[Policy]{Items: policies, HasMore: page.HasMore}, nil
}

func (s *policyService) Replace(ctx context.Context, actor, id uuid.UUID, in PolicyInput) (Policy, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Policy{}, fmt.Errorf("replace policy: %w", err)
	}

	if _, err := s.getReadable(ctx, decider, id); err != nil {
		return Policy{}, fmt.Errorf("replace policy: %w", err)
	}

	// Replacing sets every field, so each one is checked, even if its value doesn't change.
	if err := updateFields.Check(decider, toResource(id)); err != nil {
		return Policy{}, fmt.Errorf("replace policy: %w", err)
	}

	source, err := s.checkSource(ctx, in.Source)
	if err != nil {
		return Policy{}, fmt.Errorf("replace policy: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := readFields.Readable(decider, toResource(id))
	if err != nil {
		return Policy{}, fmt.Errorf("replace policy: %w", err)
	}

	var rec repository.PolicyRecord
	err = s.repo.Write(ctx, func(w repository.PolicyWriter) error {
		if err := checkTotal(ctx, w, id, source.statements); err != nil {
			return err
		}

		stored, err := w.Update(ctx, repository.PolicyRecord{
			ID:        id,
			Name:      in.Name,
			Source:    in.Source,
			UpdatedBy: actor,
			UpdatedAt: s.now(),
		})
		if err != nil {
			return fmt.Errorf("store: %w", err)
		}
		rec = stored

		return nil
	})
	if err != nil {
		return Policy{}, fmt.Errorf("replace policy: %w", translateError(err))
	}

	return toPolicy(rec, fields), nil
}

func (s *policyService) Patch(ctx context.Context, actor, id uuid.UUID, patch PolicyPatch) (Policy, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Policy{}, fmt.Errorf("patch policy: %w", err)
	}

	rec, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return Policy{}, fmt.Errorf("patch policy: %w", err)
	}

	// Patching sets only the fields in the patch, so only those are checked.
	if err := updateFields.Only(patch.fields()...).Check(decider, toResource(id)); err != nil {
		return Policy{}, fmt.Errorf("patch policy: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := readFields.Readable(decider, toResource(id))
	if err != nil {
		return Policy{}, fmt.Errorf("patch policy: %w", err)
	}

	if len(patch.fields()) == 0 {
		return toPolicy(rec, fields), nil
	}

	err = s.repo.Write(ctx, func(w repository.PolicyWriter) error {
		if err := s.checkPatched(ctx, w, id, patch); err != nil {
			return err
		}

		stored, err := w.Patch(ctx, repository.PolicyPatch{
			ID:        id,
			Name:      patch.Name,
			Source:    patch.Source,
			UpdatedBy: actor,
			UpdatedAt: s.now(),
		})
		if err != nil {
			return fmt.Errorf("store: %w", err)
		}
		rec = stored

		return nil
	})
	if err != nil {
		return Policy{}, fmt.Errorf("patch policy: %w", translateError(err))
	}

	return toPolicy(rec, fields), nil
}

func (s *policyService) Delete(ctx context.Context, actor, id uuid.UUID) error {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}

	if _, err := s.getReadable(ctx, decider, id); err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}

	can, err := decider.Can(actionDelete, toResource(id))
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}
	if !can {
		return fmt.Errorf("delete policy: %w", authz.ErrDenied)
	}

	// Deleting the administrators policy is allowed too, which can lock everyone out: an accepted risk.
	err = s.repo.Write(ctx, func(w repository.PolicyWriter) error {
		return w.Delete(ctx, id)
	})
	if err != nil {
		return fmt.Errorf("delete policy: %w", translateError(err))
	}

	return nil
}

// getReadable returns the record of the policy with the given ID. It reports a policy that the decider
// doesn't allow to read like a missing one, so that every endpoint hides it before checking anything else.
func (s *policyService) getReadable(
	ctx context.Context, decider authz.Decider, id uuid.UUID,
) (repository.PolicyRecord, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return repository.PolicyRecord{}, translateError(err)
	}

	can, err := decider.Can(actionRead, toResource(id))
	if err != nil {
		return repository.PolicyRecord{}, fmt.Errorf("check read: %w", err)
	}
	if !can {
		return repository.PolicyRecord{}, ErrPolicyNotFound
	}

	return rec, nil
}

// checkSource runs the checks of the source that don't depend on the other policies, including that the
// groups it references exist.
func (s *policyService) checkSource(ctx context.Context, source string) (checkedSource, error) {
	checked, err := s.checker.check(source)
	if err != nil {
		return checkedSource{}, err
	}
	if len(checked.groups) == 0 {
		return checked, nil
	}

	existing, err := s.repo.ListExistingGroups(ctx, checked.groups)
	if err != nil {
		return checkedSource{}, fmt.Errorf("check groups: %w", err)
	}
	for _, group := range checked.groups {
		if !slices.Contains(existing, group) {
			return checkedSource{}, missingGroup(group.String())
		}
	}

	return checked, nil
}

// checkPatched runs the checks of a saved policy on the source of the policy with the given ID that results
// from the patch, even if the patch keeps it. It runs in the write, which holds every other policy write until
// the commit, so that the source it reads is the one that's stored.
func (s *policyService) checkPatched(
	ctx context.Context, w repository.PolicyWriter, id uuid.UUID, patch PolicyPatch,
) error {
	current, err := w.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}
	source := current.Source
	if patch.Source != nil {
		source = *patch.Source
	}

	checked, err := s.checkSource(ctx, source)
	if err != nil {
		return err
	}

	return checkTotal(ctx, w, id, checked.statements)
}

// checkTotal checks that the policies, with the policy with the given ID holding the given number of
// statements, hold no more than maxStatements together. It runs in the write, which keeps other writes
// from changing the total until the commit.
func checkTotal(ctx context.Context, w repository.PolicyWriter, id uuid.UUID, statements int) error {
	recs, err := w.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("count statements: %w", err)
	}

	total := statements
	for _, rec := range recs {
		if rec.ID == id {
			continue
		}

		others, err := parseStatements(rec.Source)
		if err != nil {
			return fmt.Errorf("count statements of policy %s: %w", rec.ID, err)
		}
		total += len(others)
	}

	if total > maxStatements {
		return tooManyStatements(total)
	}

	return nil
}

// toResource returns the policy with the given ID as a resource of a decision.
func toResource(id uuid.UUID) authz.Resource {
	return authz.Resource{Type: policyType, ID: id}
}

// toPolicy returns the policy of the record with its ID and the readable fields. Every other field stays
// hidden, so that one that's missing here can't be revealed by mistake.
func toPolicy(rec repository.PolicyRecord, fields authz.ReadableFields) Policy {
	return Policy{
		ID:        rec.ID,
		Name:      authz.Reveal(fields, "name", rec.Name),
		Source:    authz.Reveal(fields, "source", rec.Source),
		CreatedBy: authz.Reveal(fields, "createdBy", rec.CreatedBy),
		UpdatedBy: authz.Reveal(fields, "updatedBy", rec.UpdatedBy),
		CreatedAt: authz.Reveal(fields, "createdAt", rec.CreatedAt),
		UpdatedAt: authz.Reveal(fields, "updatedAt", rec.UpdatedAt),
	}
}
