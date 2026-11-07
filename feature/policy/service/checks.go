package service

import (
	"fmt"
	"slices"
	"uuid"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/cedar-policy/cedar-go/x/exp/eval"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/cedar-policy/cedar-go/x/exp/schema/validate"
)

// The limits of the policies. Deciding can't be canceled, so they cap the time that a decision takes.
const (
	// maxSourceBytes is the largest source of one policy.
	maxSourceBytes = 16 << 10
	// maxStatements is the most statements that all policies together may hold.
	maxStatements = 1000
)

// groupType is the Cedar entity type of groups, which policies reference by ID.
const groupType types.EntityType = "Group"

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
	// groups holds the IDs of the groups that the source references, which must exist.
	groups []uuid.UUID
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

	var groups []string
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
		groups = appendGroups(groups, root)
	}

	ids, err := parseGroupIDs(groups)
	if err != nil {
		return checkedSource{}, err
	}

	return checkedSource{statements: len(statements), groups: ids}, nil
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

// appendGroups appends the IDs of the groups that the node references, in the scope or in a condition, to
// groups.
func appendGroups(groups []string, root xast.Node) []string {
	xast.Inspect(root, func(n xast.IsNode) bool {
		if value, ok := n.(xast.NodeValue); ok {
			groups = appendGroupsOfValue(groups, value.Value)
		}

		return true
	})

	return groups
}

// appendGroupsOfValue appends the IDs of the groups that the value is or holds to groups.
func appendGroupsOfValue(groups []string, value types.Value) []string {
	switch v := value.(type) {
	case types.EntityUID:
		if v.Type == groupType && !slices.Contains(groups, string(v.ID)) {
			groups = append(groups, string(v.ID))
		}
	case types.Set:
		for element := range v.All() {
			groups = appendGroupsOfValue(groups, element)
		}
	case types.Record:
		for _, element := range v.All() {
			groups = appendGroupsOfValue(groups, element)
		}
	}

	return groups
}

// parseGroupIDs returns the group IDs as UUIDs. Every group has a UUID, so an ID that isn't one is reported
// as a group that doesn't exist.
func parseGroupIDs(groups []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(groups))
	for i, group := range groups {
		id, err := uuid.Parse(group)
		if err != nil {
			return nil, missingGroup(group)
		}
		ids[i] = id
	}

	return ids, nil
}

// parseStatements returns the statements of the source, as the authorization engine parses them.
func parseStatements(source string) (cedar.PolicyList, error) {
	statements, err := cedar.NewPolicyListFromBytes("policy", []byte(source))
	if err != nil {
		return nil, fmt.Errorf("the source can't be parsed: %w", err)
	}

	return statements, nil
}

// missingGroup reports a reference to a group that doesn't exist. Cedar can't tell it from a group without
// members, so that a forbid with a mistyped group ID would silently never match.
func missingGroup(id string) *InvalidPolicyError {
	return &InvalidPolicyError{Reason: fmt.Sprintf("the group %q does not exist", id)}
}

// tooManyStatements reports that the policies would hold more statements than they may.
func tooManyStatements(total int) *InvalidPolicyError {
	return &InvalidPolicyError{Reason: fmt.Sprintf(
		"the policies would hold %d statements, more than the %d that they may hold together", total, maxStatements)}
}
