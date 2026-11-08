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
	ID   string        `json:"id"`
	Name Field[string] `json:"name"`
}

// encode returns the JSON encoding of v.
func encode(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)

	return string(b)
}

func TestNewResource(t *testing.T) {
	t.Parallel()
	r := NewResource(user{ID: "user-42", Name: Visible("Ada")})

	assert.JSONEq(t, `{"data":{"id":"user-42","name":"Ada"}}`, encode(t, r))
}

func TestWriteResource(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteResource(rec, user{ID: "user-42", Name: Visible("Ada")})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"data":{"id":"user-42","name":"Ada"}}`, rec.Body.String())
}

func TestWriteCreated(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteCreated(rec, "/users/user-42", user{ID: "user-42", Name: Visible("Ada")})

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "/users/user-42", rec.Header().Get("Location"))
	assert.JSONEq(t, `{"data":{"id":"user-42","name":"Ada"}}`, rec.Body.String())
}

func TestWriteResource_PanicsBeforeWritingUnencodableData(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	assert.Panics(t, func() { WriteResource(rec, make(chan int)) })
	assert.Empty(t, rec.Header())
	assert.Empty(t, rec.Body.String())
}

func TestWriteCreated_PanicsBeforeSettingLocation(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	assert.Panics(t, func() { WriteCreated(rec, "/users/user-42", make(chan int)) })
	assert.Empty(t, rec.Header())
}
