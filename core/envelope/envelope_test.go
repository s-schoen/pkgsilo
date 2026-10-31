package envelope

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusServiceUnavailable, user{ID: "user-42", Name: "Ada"})

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"id":"user-42","name":"Ada"}`, rec.Body.String())
}

func TestWriteJSON_PanicsBeforeWritingUnencodableData(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	assert.Panics(t, func() { WriteJSON(rec, http.StatusOK, make(chan int)) })
	assert.Empty(t, rec.Header())
	assert.Empty(t, rec.Body.String())
}
