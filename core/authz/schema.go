package authz

import (
	"slices"
	"strconv"
	"strings"

	"github.com/cedar-policy/cedar-go/types"
)

// Declaration declares a resource type and the actions on it, so that policies can use them. Each feature
// declares its own, and app assembles the schema of all of them with Schema.
type Declaration struct {
	// Type is the Cedar entity type of the resources, such as Identity.
	Type string
	// Actions are the actions on the resources that aren't field actions.
	Actions []Action
	// Fields are the field actions of the writes of the resources.
	Fields []FieldActions
}

// Schema returns the Cedar schema, in the human-readable format, that policies are validated against when
// they're saved: the entity types Group and Identity, which is a member of groups, the declared types, and
// the declared actions, each of which applies to an Identity as the principal and its declaration's type
// as the resource. The action groups that the actions are members of are declared as actions that apply to
// nothing, since they're never requested themselves.
//
// No entity type has attributes, since no resource of a decision carries any: an attribute that the schema
// declared would pass validation and then fail every decision.
func Schema(declarations ...Declaration) string {
	var b strings.Builder
	writeEntityTypes(&b, declarations)

	for _, group := range actionGroups(declarations) {
		b.WriteString("action " + strconv.Quote(string(group.ID)) + ";\n")
	}

	for _, declaration := range declarations {
		for _, action := range declaration.actions() {
			writeAction(&b, action, declaration.Type)
		}
	}

	return b.String()
}

// writeEntityTypes writes the entity types of the schema to b: Group, Identity and each declared type once.
func writeEntityTypes(b *strings.Builder, declarations []Declaration) {
	b.WriteString("entity " + GroupType + ";\n")
	b.WriteString("entity " + IdentityType + " in [" + GroupType + "];\n")

	written := []string{GroupType, IdentityType}
	for _, declaration := range declarations {
		if !slices.Contains(written, declaration.Type) {
			written = append(written, declaration.Type)
			b.WriteString("entity " + declaration.Type + ";\n")
		}
	}
}

// actionGroups returns the action groups that the declared actions are members of, without those that are
// declared actions themselves, in the order of their first use.
func actionGroups(declarations []Declaration) []types.EntityUID {
	var actions, groups []types.EntityUID
	for _, declaration := range declarations {
		for _, action := range declaration.actions() {
			actions = append(actions, action.entity.UID)
			for _, parent := range action.parents() {
				if !slices.Contains(groups, parent) {
					groups = append(groups, parent)
				}
			}
		}
	}

	return slices.DeleteFunc(groups, func(group types.EntityUID) bool {
		return slices.Contains(actions, group)
	})
}

// writeAction writes the declaration of the action on resources of the given type to b.
func writeAction(b *strings.Builder, action Action, resourceType string) {
	b.WriteString("action " + strconv.Quote(string(action.entity.UID.ID)))
	if parents := action.parents(); len(parents) > 0 {
		names := make([]string, len(parents))
		for i, parent := range parents {
			names[i] = strconv.Quote(string(parent.ID))
		}
		b.WriteString(" in [" + strings.Join(names, ", ") + "]")
	}
	b.WriteString(" appliesTo { principal: " + IdentityType + ", resource: " + resourceType + ", context: {} };\n")
}

// actions returns all actions of the declaration, the field actions included.
func (d Declaration) actions() []Action {
	actions := slices.Clone(d.Actions)
	for _, fields := range d.Fields {
		actions = append(actions, fields.actions...)
	}

	return actions
}

// parents returns the action groups that the action is a member of, sorted by name, so that the schema
// doesn't change from one call to the next.
func (a Action) parents() []types.EntityUID {
	parents := a.entity.Parents.Slice()
	slices.SortFunc(parents, func(x, y types.EntityUID) int {
		return strings.Compare(string(x.ID), string(y.ID))
	})

	return parents
}
