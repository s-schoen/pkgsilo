package service

import (
	"context"
	"fmt"
	"slices"
	"uuid"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/cedar-policy/cedar-go/x/exp/eval"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/cedar-policy/cedar-go/x/exp/schema/validate"

	"pkgsilo/core/authz"
	"pkgsilo/feature/policy/repository"
)

// The limits of the policies. Deciding can't be canceled, so they cap the time that a decision takes.
const (
	// maxSourceBytes is the largest source of one policy.
	maxSourceBytes = 16 << 10
	// maxStatements is the most statements that all policies together may hold.
	maxStatements = 1000
)

// The Cedar entity types of the objects that policies reference by ID, which must exist.
const (
	groupType    types.EntityType = authz.GroupType
	identityType types.EntityType = authz.IdentityType
)

// referencedTypes names the entity types of the objects that policies reference by ID, for the reasons of
// their mistakes.
var referencedTypes = map[types.EntityType]string{groupType: "group", identityType: "identity"}

// checker checks the source of a policy before it's saved, so that a policy with a mistake is rejected with
// a reason instead of silently never matching.
//
// It's the only user of cedar-go's x/exp packages, which may break at any time: they're used when a policy
// is saved, and never to decide.
type checker struct {
	validator *validate.Validator
	// attributes holds the names of the attributes that any entity type declares.
	attributes map[types.String]bool
}

// checkedSource is a source that passed the checks that need no database.
type checkedSource struct {
	// statements is the number of statements in the source.
	statements int
	// groups and identities hold the IDs of the groups and identities that the source references, which
	// must exist.
	groups, identities []uuid.UUID
}

// newChecker returns a checker that validates policies against the Cedar schema, in the human-readable
// format.
func newChecker(cedarSchema string) (*checker, error) {
	var s schema.Schema
	if err := s.UnmarshalCedar([]byte(cedarSchema)); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}

	resolved, err := s.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve schema: %w", err)
	}

	attributes := map[types.String]bool{}
	for _, entity := range resolved.Entities {
		for name := range entity.Shape {
			attributes[name] = true
		}
	}

	return &checker{validator: validate.New(resolved, validate.WithStrict()), attributes: attributes}, nil
}

// check checks the source, as far as that needs no database: its size, that it parses into at least one
// statement, and that every statement matches the schema and tests only for declared attributes with has.
// It reports a mistake with an *InvalidPolicyError. The size limit keeps a source far below maxStatements,
// which the total of all policies is checked against when it's written.
func (c *checker) check(source string) (checkedSource, error) {
	if len(source) > maxSourceBytes {
		return checkedSource{}, &InvalidPolicyError{
			Reason: fmt.Sprintf("the source is larger than %d bytes", maxSourceBytes),
		}
	}

	statements, err := parseStatements(source)
	if err != nil {
		return checkedSource{}, &InvalidPolicyError{Reason: err.Error()}
	}
	if len(statements) == 0 {
		return checkedSource{}, &InvalidPolicyError{Reason: "the source holds no statements"}
	}

	var references []types.EntityUID
	for i, statement := range statements {
		policy := (*xast.Policy)(statement.AST())
		name := fmt.Sprintf("statement %d", i+1)

		if err := c.validator.Policy(name, policy); err != nil {
			return checkedSource{}, &InvalidPolicyError{Reason: "the source does not match the schema: " + err.Error()}
		}

		root := eval.PolicyToNode(policy)
		if err := c.checkHas(name, root); err != nil {
			return checkedSource{}, err
		}
		references = appendReferences(references, root)
	}

	groups, identities, err := parseReferences(references)
	if err != nil {
		return checkedSource{}, err
	}

	return checkedSource{statements: len(statements), groups: groups, identities: identities}, nil
}

// checkHas reports a has test for an attribute that no entity type declares. The validator accepts it,
// since it types the test as false, so that a statement such as `forbid (…) when { resource has secrte &&
// resource.secrte }` would silently never match.
func (c *checker) checkHas(name string, root xast.Node) error {
	var err error
	xast.Inspect(root, func(n xast.IsNode) bool {
		if has, ok := n.(xast.NodeTypeHas); ok && err == nil && !c.attributes[has.Value] {
			err = &InvalidPolicyError{Reason: fmt.Sprintf(
				"%s tests for the attribute %q with has, which no entity type declares", name, has.Value)}
		}

		return err == nil
	})

	return err
}

// appendReferences appends the groups and identities that the node references, in the scope or in a
// condition, to references.
func appendReferences(references []types.EntityUID, root xast.Node) []types.EntityUID {
	xast.Inspect(root, func(n xast.IsNode) bool {
		if value, ok := n.(xast.NodeValue); ok {
			references = appendReferencesOfValue(references, value.Value)
		}

		return true
	})

	return references
}

// appendReferencesOfValue appends the groups and identities that the value is or holds to references.
func appendReferencesOfValue(references []types.EntityUID, value types.Value) []types.EntityUID {
	switch v := value.(type) {
	case types.EntityUID:
		if _, ok := referencedTypes[v.Type]; ok && !slices.Contains(references, v) {
			references = append(references, v)
		}
	case types.Set:
		for element := range v.All() {
			references = appendReferencesOfValue(references, element)
		}
	case types.Record:
		for _, element := range v.All() {
			references = appendReferencesOfValue(references, element)
		}
	}

	return references
}

// parseReferences returns the IDs of the referenced groups and identities as UUIDs, in their order. Every
// group and identity has a UUID, so an ID that isn't one is reported as an object that doesn't exist. The
// engine writes the IDs in lowercase with hyphens, and Cedar compares them as strings, so an ID that's
// written another way, such as in uppercase, is reported too, since it would never match.
func parseReferences(references []types.EntityUID) (groups, identities []uuid.UUID, err error) {
	for _, reference := range references {
		kind := referencedTypes[reference.Type]
		written := string(reference.ID)
		id, err := uuid.Parse(written)
		if err != nil {
			return nil, nil, missingReference(kind, written)
		}
		if id.String() != written {
			return nil, nil, &InvalidPolicyError{Reason: fmt.Sprintf(
				"the %s ID %q never matches, since IDs are written in lowercase with hyphens, as %q",
				kind, written, id.String())}
		}

		if reference.Type == groupType {
			groups = append(groups, id)
		} else {
			identities = append(identities, id)
		}
	}

	return groups, identities, nil
}

// parseStatements returns the statements of the source, as the authorization engine parses them.
func parseStatements(source string) (cedar.PolicyList, error) {
	statements, err := cedar.NewPolicyListFromBytes("policy", []byte(source))
	if err != nil {
		return nil, fmt.Errorf("the source can't be parsed: %w", err)
	}

	return statements, nil
}

// missingReference reports a reference to a group or identity, by the name of its kind, that doesn't exist.
// Cedar can't tell it from a group without members or an identity that never asks, so that a forbid with a
// mistyped ID would silently never match.
func missingReference(kind, id string) *InvalidPolicyError {
	return &InvalidPolicyError{Reason: fmt.Sprintf("the %s %q does not exist", kind, id)}
}

// tooManyStatements reports that the policies would hold more statements than they may.
func tooManyStatements(total int) *InvalidPolicyError {
	return &InvalidPolicyError{Reason: fmt.Sprintf(
		"the policies would hold %d statements, more than the %d that they may hold together", total, maxStatements)}
}

// checkWithReferences runs check on the source, and checks that the groups and identities that it
// references exist, with lister.
func (c *checker) checkWithReferences(
	ctx context.Context, lister repository.ReferenceLister, source string,
) (checkedSource, error) {
	checked, err := c.check(source)
	if err != nil {
		return checkedSource{}, err
	}

	err = checkExisting(ctx, referencedTypes[groupType], checked.groups, lister.ListExistingGroups)
	if err != nil {
		return checkedSource{}, err
	}
	err = checkExisting(ctx, referencedTypes[identityType], checked.identities, lister.ListExistingIdentities)
	if err != nil {
		return checkedSource{}, err
	}

	return checked, nil
}

// checkExisting reports the first of the referenced objects, of the kind with the given name, that list
// doesn't return. It doesn't list anything if there are none.
func checkExisting(
	ctx context.Context, kind string, ids []uuid.UUID, list func(context.Context, []uuid.UUID) ([]uuid.UUID, error),
) error {
	if len(ids) == 0 {
		return nil
	}

	existing, err := list(ctx, ids)
	if err != nil {
		return fmt.Errorf("check %s references: %w", kind, err)
	}
	for _, id := range ids {
		if !slices.Contains(existing, id) {
			return missingReference(kind, id.String())
		}
	}

	return nil
}
