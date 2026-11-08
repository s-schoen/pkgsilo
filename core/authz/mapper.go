package authz

import (
	"fmt"
	"reflect"
	"slices"
)

// Mapper builds the domain model M of a resource type, as an actor sees it, from its record R. It copies
// each member of M from the member of R with the same name: a Field[T] from a member of type T, visible
// only if the actor may read the field of its tag, and any other member, the ID, from a member of the same
// type. The members of R that M lacks, such as a key hash, are left out.
//
// Every member of M must map, so that a field can't be forgotten, and stay visible or hidden by mistake. A
// model that doesn't map member by member onto its record is built by hand, with Reveal.
type Mapper[R, M any] struct {
	// members maps each member of M, by its index, to the member of R that it's copied from.
	members []mappedMember
	// names are the names of the read fields of M, in the order of ReadableFields.
	names []string
}

// mappedMember is a member of the model, copied from a member of the record.
type mappedMember struct {
	model, record int
	// field is the index of the member's field in the read fields, or -1 for a member that isn't a Field.
	field int
}

// NewMapper returns the Mapper from R to the Model M of the resource type of actions. It panics if M isn't
// that Model, or if a member of M has no member of R with the same name and type, which is a bug that a
// feature's package initialization reveals.
func NewMapper[R, M any](actions TypeActions) Mapper[R, M] {
	model, record := reflect.TypeFor[M](), reflect.TypeFor[R]()
	if model != actions.model {
		panic(fmt.Sprintf("authz: %s is not the model %v of the type %s", model, actions.model, actions.Type()))
	}
	checkStruct(record)

	mapper := Mapper[R, M]{names: actions.ReadFields().Fields()}
	for member := range model.Fields() {
		mapped := mappedMember{model: member.Index[0], field: -1}
		want := member.Type
		if isField(member.Type) {
			mapped.field = slices.Index(mapper.names, member.Tag.Get(fieldTag))
			want = member.Type.Field(0).Type
		}

		source, ok := record.FieldByName(member.Name)
		if !ok || len(source.Index) != 1 || source.Type != want {
			panic(fmt.Sprintf("authz: member %s of %s has no member %s of type %s in %s",
				member.Name, model, member.Name, want, record))
		}
		mapped.record = source.Index[0]
		mapper.members = append(mapper.members, mapped)
	}

	return mapper
}

// Map returns the model of the record, with the fields that are readable visible and all others hidden. It
// panics on fields that aren't read fields of the model, which is a bug in the caller.
func (m Mapper[R, M]) Map(rec R, fields ReadableFields) M {
	if !slices.Equal(fields.fields, m.names) {
		panic(fmt.Sprintf("authz: the readable fields %v aren't the read fields %v of %s",
			fields.fields, m.names, reflect.TypeFor[M]()))
	}

	var model M
	target, source := reflect.ValueOf(&model).Elem(), reflect.ValueOf(rec)
	for _, member := range m.members {
		value := source.Field(member.record)
		switch {
		case member.field < 0:
			target.Field(member.model).Set(value)
		case fields.readable[member.field]:
			field := target.Field(member.model)
			field.Field(0).Set(value)
			field.Field(1).SetBool(true)
		}
	}

	return model
}
