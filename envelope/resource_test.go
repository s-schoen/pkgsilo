package envelope

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type user struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// encode returns the JSON encoding of v.
func encode(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)

	return string(b)
}

func TestNewResource(t *testing.T) {
	r := NewResource(user{ID: "user-42", Name: "Ada"})

	assert.JSONEq(t, `{"data":{"id":"user-42","name":"Ada"}}`, encode(t, r))
}
