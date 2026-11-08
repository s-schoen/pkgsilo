package request

import (
	"encoding/json"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptional_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want Optional[string]
	}{
		{name: "absent", body: `{}`, want: Optional[string]{}},
		{name: "null", body: `{"name":null}`, want: Optional[string]{Set: true, Null: true}},
		{name: "value", body: `{"name":"Ada"}`, want: Optional[string]{Value: "Ada", Set: true}},
		{name: "empty value", body: `{"name":""}`, want: Optional[string]{Set: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got struct {
				Name Optional[string] `json:"name"`
			}

			require.NoError(t, json.Unmarshal([]byte(tt.body), &got))
			assert.Equal(t, tt.want, got.Name)
		})
	}
}

func TestOptional_UnmarshalJSON_Pointer(t *testing.T) {
	t.Parallel()
	var got struct {
		Count Optional[*int] `json:"count"`
	}

	require.NoError(t, json.Unmarshal([]byte(`{"count":3}`), &got))
	require.NotNil(t, got.Count.Value)
	assert.Equal(t, 3, *got.Count.Value)
	assert.False(t, got.Count.Null)
}

func TestOptional_Pointer(t *testing.T) {
	t.Parallel()
	assert.Nil(t, Optional[string]{}.Pointer(), "absent")
	assert.Nil(t, Optional[string]{Set: true, Null: true}.Pointer(), "null")
	assert.Equal(t, new("Ada"), Optional[string]{Value: "Ada", Set: true}.Pointer())
}

func TestIfSet(t *testing.T) {
	t.Parallel()
	rule := IfSet(validation.Required, validation.Length(3, 5))
	tests := []struct {
		name  string
		value Optional[string]
		// wantErr is the message of the validation error, empty for a valid value.
		wantErr string
	}{
		{name: "absent", value: Optional[string]{}},
		{name: "valid", value: Optional[string]{Value: "Ada", Set: true}},
		{name: "null", value: Optional[string]{Set: true, Null: true}, wantErr: "cannot be null"},
		{name: "empty", value: Optional[string]{Set: true}, wantErr: "cannot be blank"},
		{
			name: "invalid", value: Optional[string]{Value: "Ada Lovelace", Set: true},
			wantErr: "the length must be between 3 and 5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validation.Validate(tt.value, rule)

			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			var valueErr validation.Error
			require.ErrorAs(t, err, &valueErr)
			assert.Equal(t, tt.wantErr, valueErr.Error())
		})
	}
}

// TestIfSet_NamesMember checks that a struct reports an invalid Optional under the member's name, like a
// plain member.
func TestIfSet_NamesMember(t *testing.T) {
	t.Parallel()
	patch := namePatch{Name: Optional[string]{Set: true, Null: true}}

	err := patch.Validate()

	var errs validation.Errors
	require.ErrorAs(t, err, &errs)
	assert.Equal(t, []string{"name"}, keys(errs))
}

func TestIfSet_NotOptional(t *testing.T) {
	t.Parallel()
	err := validation.Validate("Ada", IfSet(validation.Required))

	var internalErr validation.InternalError
	assert.ErrorAs(t, err, &internalErr, "a misconfigured rule must fail internally")
}

// keys returns the members that errs reports.
func keys(errs validation.Errors) []string {
	var members []string
	for member := range errs {
		members = append(members, member)
	}

	return members
}
