package authz

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testRecord is the record of testModel, with a member that the model lacks.
type testRecord struct {
	ID        string
	Secret    []byte
	Name      string
	OwnerID   string
	CreatedAt time.Time
}

var (
	testActions = DeclareType(TypeSpec{Type: "Thing", Model: reflect.TypeFor[testModel]()})
	createdAt   = time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	storedThing = testRecord{
		ID: "t1", Secret: []byte("hash"), Name: "Ada", OwnerID: "o1", CreatedAt: createdAt,
	}
)

// readable returns the read fields of testActions, of which only those with the given names are readable.
func readable(names ...string) ReadableFields {
	fields := testActions.ReadFields().NoneReadable()
	for i, name := range fields.fields {
		fields.readable[i] = slices.Contains(names, name)
	}

	return fields
}

func TestMapper_Map(t *testing.T) {
	t.Parallel()
	mapper := NewMapper[testRecord, testModel](testActions)
	tests := []struct {
		name     string
		readable []string
		want     testModel
	}{
		{
			name:     "all",
			readable: []string{"name", "ownerId", "createdAt"},
			want: testModel{
				ID: "t1", Name: Visible("Ada"), OwnerID: Visible("o1"), CreatedAt: Visible(createdAt),
			},
		},
		{
			name:     "some",
			readable: []string{"ownerId"},
			want:     testModel{ID: "t1", OwnerID: Visible("o1")},
		},
		// The ID is no field, so it's always copied.
		{name: "none", want: testModel{ID: "t1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, mapper.Map(storedThing, readable(tt.readable...)))
		})
	}
}

// TestMapper_Map_ZeroValue checks that a readable field with the zero value is visible, unlike a hidden one.
func TestMapper_Map_ZeroValue(t *testing.T) {
	t.Parallel()
	mapper := NewMapper[testRecord, testModel](testActions)

	got := mapper.Map(testRecord{ID: "t1"}, readable("name"))

	assert.Equal(t, Visible(""), got.Name)
}

// TestMapper_Map_OtherFields checks that the readable fields of another type are a bug that fails loudly,
// instead of revealing the fields at the same positions.
func TestMapper_Map_OtherFields(t *testing.T) {
	t.Parallel()
	mapper := NewMapper[testRecord, testModel](testActions)
	other := NewReadFieldActions("thing", "name", "createdAt", "ownerId").NoneReadable()

	assert.Panics(t, func() { mapper.Map(storedThing, other) })
}

// TestNewMapper_Invalid checks that a model that doesn't map onto its record member by member panics, so
// that the feature's package fails to initialize.
func TestNewMapper_Invalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		new  func()
		want string
	}{
		{
			name: "other model",
			new: func() {
				NewMapper[testRecord, testInput](testActions)
			},
			want: "not the model",
		},
		{
			name: "record that's no struct",
			new: func() {
				NewMapper[string, testModel](testActions)
			},
			want: "no struct type",
		},
		{
			name: "missing member",
			new: func() {
				NewMapper[struct {
					ID, Name  string
					CreatedAt time.Time
				}, testModel](testActions)
			},
			want: "has no member OwnerID of type string",
		},
		{
			name: "member of another type",
			new: func() {
				NewMapper[struct {
					ID, Name, OwnerID string
					CreatedAt         string
				}, testModel](testActions)
			},
			want: "has no member CreatedAt of type time.Time",
		},
		{
			name: "ID of another type",
			new: func() {
				NewMapper[struct {
					ID            int
					Name, OwnerID string
					CreatedAt     time.Time
				}, testModel](testActions)
			},
			want: "has no member ID of type string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				msg, _ := recover().(string)
				assert.Contains(t, msg, tt.want)
			}()

			tt.new()
		})
	}
}
