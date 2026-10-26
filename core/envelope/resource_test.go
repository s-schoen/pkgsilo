package envelope

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestWriteResource(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteResource(rec, user{ID: "user-42", Name: "Ada"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"data":{"id":"user-42","name":"Ada"}}`, rec.Body.String())
}

func TestWriteCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteCreated(rec, "/users/user-42", user{ID: "user-42", Name: "Ada"})

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "/users/user-42", rec.Header().Get("Location"))
	assert.JSONEq(t, `{"data":{"id":"user-42","name":"Ada"}}`, rec.Body.String())
}
