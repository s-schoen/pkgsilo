package envelope

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type keyBody struct {
	ID        string            `json:"id"`
	Name      Field[string]     `json:"name,omitzero"`
	ExpiresAt Field[*time.Time] `json:"expiresAt,omitzero"`
}

func TestField_MarshalJSON(t *testing.T) {
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
			want: `{"id":"key-1","name":"ci","expiresAt":"2026-11-08T12:00:00Z"}`,
		},
		{
			name: "visible null",
			body: keyBody{ID: "key-1", Name: Visible("ci"), ExpiresAt: Visible[*time.Time](nil)},
			want: `{"id":"key-1","name":"ci","expiresAt":null}`,
		},
		{
			name: "visible zero value",
			body: keyBody{ID: "key-1", Name: Visible("")},
			want: `{"id":"key-1","name":""}`,
		},
		{
			name: "hidden",
			body: keyBody{ID: "key-1"},
			want: `{"id":"key-1"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.JSONEq(t, tt.want, encode(t, tt.body))
		})
	}
}

func TestField_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	var got keyBody
	require.NoError(t, json.Unmarshal([]byte(`{"id":"key-1","expiresAt":null}`), &got))

	assert.Equal(t, keyBody{ID: "key-1", ExpiresAt: Visible[*time.Time](nil)}, got,
		"a null member must be visible, and an absent one hidden")
}

func TestField_UnmarshalJSON_Invalid(t *testing.T) {
	t.Parallel()

	var got keyBody
	require.Error(t, json.Unmarshal([]byte(`{"name":1}`), &got))
}

func TestNewField(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Visible("ci"), NewField("ci", true))
	assert.Equal(t, Field[string]{}, NewField("ci", false), "a hidden field must not hold its value")
}
