package envelope

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	type health struct {
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	WriteJSON(rec, http.StatusServiceUnavailable, health{Status: "unhealthy"})

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"unhealthy","detail":""}`, rec.Body.String(),
		"a body without an envelope keeps its zero members")
}

func TestWriteJSON_PanicsBeforeWritingUnencodableData(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	assert.Panics(t, func() { WriteJSON(rec, http.StatusOK, make(chan int)) })
	assert.Empty(t, rec.Header())
	assert.Empty(t, rec.Body.String())
}

func TestEnvelope_RejectsMembersThatAreNoFields(t *testing.T) {
	t.Parallel()
	type withPlainMember struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	type withEmbeddedPlainMember struct {
		user

		Enabled bool `json:"enabled"`
	}
	tests := map[string]any{
		"resource":                NewResource(withPlainMember{ID: "user-42", Enabled: true}),
		"collection":              NewCollection([]withPlainMember{{ID: "user-42"}}, Pagination{Limit: 1}),
		"empty collection":        NewCollection[withPlainMember](nil, Pagination{Limit: 1}),
		"embedded representation": NewResource(withEmbeddedPlainMember{ID: "user-42"}),
		"no struct":               NewResource("user-42"),
	}
	for name, envelope := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := json.Marshal(envelope)

			require.ErrorContains(t, err, "envelope: ", "a member that's no Field would vanish when it's zero")
		})
	}
}

func TestEnvelope_EmbeddedRepresentation(t *testing.T) {
	t.Parallel()
	type created struct {
		user

		Secret Field[string] `json:"secret"`
	}

	assert.JSONEq(t, `{"data":{"id":"user-42","secret":"s3cr3t"}}`,
		encode(t, NewResource(created{ID: "user-42", Secret: Visible("s3cr3t")})),
		"the members of an embedded struct must be inlined, and their hidden fields left out")
}
