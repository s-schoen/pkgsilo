package envelope

import (
	"encoding/json/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type keyBody struct {
	ID        string            `json:"id"`
	Name      Field[string]     `json:"name"`
	ExpiresAt Field[*time.Time] `json:"expiresAt"`
}

func TestField_MarshalJSONTo_Resource(t *testing.T) {
	t.Parallel()
	expiresAt := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		body keyBody
		want string
	}{
		{
			name: "visible",
			body: keyBody{ID: "key-1", Name: Visible("ci"), ExpiresAt: Visible(&expiresAt)},
			want: `{"data":{"id":"key-1","name":"ci","expiresAt":"2026-11-08T12:00:00Z"}}`,
		},
		{
			name: "visible null",
			body: keyBody{ID: "key-1", Name: Visible("ci"), ExpiresAt: Visible[*time.Time](nil)},
			want: `{"data":{"id":"key-1","name":"ci","expiresAt":null}}`,
		},
		{
			name: "visible zero value",
			body: keyBody{ID: "key-1", Name: Visible("")},
			want: `{"data":{"id":"key-1","name":""}}`,
		},
		{
			name: "hidden",
			body: keyBody{ID: "key-1"},
			want: `{"data":{"id":"key-1"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.JSONEq(t, tt.want, encode(t, NewResource(tt.body)))
		})
	}
}

func TestField_MarshalJSONTo_Collection(t *testing.T) {
	t.Parallel()
	c := NewCollection([]keyBody{
		{ID: "key-1", Name: Visible("ci")},
		{ID: "key-2", ExpiresAt: Visible[*time.Time](nil)},
	}, Pagination{Offset: 0, Limit: 2})

	assert.JSONEq(t, `{
		"data": [{"id":"key-1","name":"ci"},{"id":"key-2","expiresAt":null}],
		"pagination": {"offset":0,"limit":2,"nextOffset":null}
	}`, encode(t, c), "hidden fields must be absent, and the pagination must keep its zero members")
}

func TestField_MarshalJSONTo_ZeroMembersOfValue(t *testing.T) {
	t.Parallel()
	type window struct {
		From int  `json:"from"`
		To   *int `json:"to"`
	}
	type body struct {
		ID     string        `json:"id"`
		Window Field[window] `json:"window"`
	}

	assert.JSONEq(t, `{"data":{"id":"w-1","window":{"from":0,"to":null}}}`,
		encode(t, NewResource(body{ID: "w-1", Window: Visible(window{})})),
		"the value of a visible field must keep its zero members")
}

func TestField_MarshalJSONTo_HiddenOutsideEnvelope(t *testing.T) {
	t.Parallel()

	_, err := json.Marshal(keyBody{ID: "key-1"})

	require.ErrorIs(t, err, errHiddenFieldEncoded, "a hidden field must not be encoded as null")
}

func TestField_UnmarshalJSONFrom(t *testing.T) {
	t.Parallel()

	var got keyBody
	require.NoError(t, json.Unmarshal([]byte(`{"id":"key-1","expiresAt":null}`), &got))

	assert.Equal(t, keyBody{ID: "key-1", ExpiresAt: Visible[*time.Time](nil)}, got,
		"a null member must be visible, and an absent one hidden")
}

func TestField_UnmarshalJSONFrom_Invalid(t *testing.T) {
	t.Parallel()

	var got keyBody
	require.Error(t, json.Unmarshal([]byte(`{"name":1}`), &got))
}

func TestNewField(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Visible("ci"), NewField("ci", true))
	assert.Equal(t, Field[string]{}, NewField("ci", false), "a hidden field must not hold its value")
}
