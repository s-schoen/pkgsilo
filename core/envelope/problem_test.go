package envelope

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

var validationProblem = DefineProblemType("validation", "Request validation failed", 422)

func TestProblem_MarshalJSONTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		problem Problem
		want    string
	}{
		{
			name:    "about:blank without occurrence details",
			problem: NotFoundProblem.New(Occurrence{}),
			want:    `{"type":"about:blank","title":"Not Found","status":404}`,
		},
		{
			name: "detail, instance and extensions",
			problem: validationProblem.New(Occurrence{
				Detail:     "The profile contains invalid fields.",
				Instance:   "urn:uuid:0192e0a4-0000-7000-8000-000000000000",
				Extensions: map[string]any{"errors": []map[string]string{{"detail": "Must be at least 18", "pointer": "#/age"}}},
			}),
			want: `{
				"type": "https://pkgsilo/problems/validation",
				"title": "Request validation failed",
				"status": 422,
				"detail": "The profile contains invalid fields.",
				"instance": "urn:uuid:0192e0a4-0000-7000-8000-000000000000",
				"errors": [{"detail":"Must be at least 18","pointer":"#/age"}]
			}`,
		},
		{
			name: "extensions don't override standard members",
			problem: validationProblem.New(Occurrence{
				Extensions: map[string]any{"type": "x", "title": "x", "status": 200, "detail": "x", "instance": "x"},
			}),
			want: `{"type":"https://pkgsilo/problems/validation","title":"Request validation failed","status":422}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.JSONEq(t, tt.want, encode(t, tt.problem))
		})
	}
}

func TestProblem_MarshalJSONTo_OrdersStandardMembersFirst(t *testing.T) {
	t.Parallel()
	p := BadRequestProblem.New(Occurrence{Detail: "d", Instance: "urn:x", Extensions: map[string]any{
		"f": 6, "b": 2, "h": 8, "a": 1, "e": 5, "c": 3, "g": 7, "d": 4, "status": 999,
	}})

	// Go iterates over a map in a random order each time, so a sorting that only happened by chance would
	// fail some of the encodings.
	for range 20 {
		//nolint:testifylint // JSONEq would ignore the member order under test.
		assert.Equal(t, `{"type":"about:blank","title":"Bad Request","status":400,"detail":"d","instance":"urn:x",`+
			`"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8}`, encode(t, p))
	}
}

func TestDefineProblemType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ProblemType{
		Type:   "https://pkgsilo/problems/username-taken",
		Title:  "Username taken",
		Status: http.StatusConflict,
	}, DefineProblemType("username-taken", "Username taken", http.StatusConflict))
}

func TestDefineProblemType_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { DefineProblemType("too-low", "Too low", 99) })
	assert.Panics(t, func() { DefineProblemType("too-high", "Too high", 600) })
	assert.Panics(t, func() { DefineProblemType("no-title", "", 400) })

	invalidSlugs := []string{"", "about:blank", "https://x.test/y", "Username", "username_taken", "-x", "x-", "a--b"}
	for _, slug := range invalidSlugs {
		t.Run(slug, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() { DefineProblemType(slug, "Invalid slug", 400) })
		})
	}
}

func TestWriteProblem(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteProblem(rec, ConflictProblem.New(Occurrence{Detail: "Already exists."}))

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, ProblemJSONMediaType, rec.Header().Get("Content-Type")) //nolint:testifylint // A media type, not JSON.
	assert.JSONEq(t, `{"type":"about:blank","title":"Conflict","status":409,"detail":"Already exists."}`,
		rec.Body.String())
}

func TestValidationError_MarshalJSON(t *testing.T) {
	t.Parallel()
	p := validationProblem.New(Occurrence{Extensions: map[string]any{
		ValidationErrorsExtension: []ValidationError{
			{Detail: "cannot be blank", Pointer: "#/name"},
			{Detail: "must be an integer", Parameter: "limit"},
		},
	}})

	assert.JSONEq(t, `{
		"type": "https://pkgsilo/problems/validation",
		"title": "Request validation failed",
		"status": 422,
		"errors": [
			{"detail":"cannot be blank","pointer":"#/name"},
			{"detail":"must be an integer","parameter":"limit"}
		]
	}`, encode(t, p))
}
