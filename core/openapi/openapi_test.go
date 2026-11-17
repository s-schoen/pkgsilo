package openapi_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	"pkgsilo/core/openapi"
	"pkgsilo/core/request"
)

type thingRequest struct {
	Name string `json:"name" jsonschema:"The name."`
}

type thingPatch struct {
	Name request.Optional[string] `json:"name,omitzero"`
}

type thingResponse struct {
	ID   uuid.UUID              `json:"id"`
	Name envelope.Field[string] `json:"name,omitzero"`
}

type healthResponse struct {
	Status string `json:"status"`
}

var thingNotFound = envelope.DefineProblemType("thing-not-found", "Thing not found", http.StatusNotFound)

// things returns a section with an operation of every shape.
func things() openapi.Section {
	id := openapi.PathID("id", "The ID of the thing.")

	return openapi.Section{
		Name: "Things",
		Operations: []openapi.Operation{
			{
				Method: http.MethodGet, Pattern: "/", ID: "listThings", Summary: "List things",
				Responses: []openapi.Response{openapi.Collection[thingResponse]("A page of things.")},
			},
			{
				Method: http.MethodPost, Pattern: "/", ID: "createThing", Summary: "Create a thing",
				Body:      openapi.JSONBody[thingRequest]("The new thing."),
				Responses: []openapi.Response{openapi.Created[thingResponse]("The new thing.")},
			},
			{
				Method: http.MethodPatch, Pattern: "/{id}", ID: "patchThing", Summary: "Change a thing",
				Parameters: []openapi.Parameter{id},
				Body:       openapi.MergePatchBody[thingPatch]("The fields to change."),
				Responses:  []openapi.Response{openapi.Resource[thingResponse]("The thing.")},
				Problems:   []envelope.ProblemType{thingNotFound, envelope.ForbiddenProblem},
			},
			{
				Method: http.MethodDelete, Pattern: "/{id}", ID: "deleteThing", Summary: "Delete a thing",
				Parameters: []openapi.Parameter{id},
				Responses:  []openapi.Response{openapi.NoContent("The thing is deleted.")},
				Problems:   []envelope.ProblemType{thingNotFound},
			},
		},
	}
}

func newSpec() *openapi.Spec {
	return &openapi.Spec{
		Info:     openapi.Info{Title: "Test", Version: "1"},
		Security: openapi.SecurityScheme{Name: "apiKey", Header: "X-API-Key"},
	}
}

// build returns the document of spec, decoded.
func build(t *testing.T, spec *openapi.Spec) map[string]any {
	t.Helper()
	doc, err := spec.Build()
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(doc.JSON(), &decoded))

	return decoded
}

// lookup returns the member of v at the path of member names.
func lookup(t *testing.T, v any, path ...string) any {
	t.Helper()
	for _, name := range path {
		object, ok := v.(map[string]any)
		require.True(t, ok, "%s isn't in an object", name)
		v, ok = object[name]
		require.True(t, ok, "no member %s", name)
	}

	return v
}

// marshal returns v in JSON, for comparing a part of a document with assert.JSONEq.
func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)

	return string(b)
}

// TestBuild_Routes checks that the operations are at their mount path, without a trailing slash.
func TestBuild_Routes(t *testing.T) {
	t.Parallel()
	spec := newSpec()
	spec.Mount("/api/things", things())

	doc, err := spec.Build()

	require.NoError(t, err)
	assert.Equal(t, []openapi.Route{
		{Method: http.MethodGet, Path: "/api/things"},
		{Method: http.MethodPost, Path: "/api/things"},
		{Method: http.MethodPatch, Path: "/api/things/{id}"},
		{Method: http.MethodDelete, Path: "/api/things/{id}"},
	}, doc.Routes())
}

// TestBuild_Collection checks that a collection is in its envelope and adds the pagination's parameters and
// problem.
func TestBuild_Collection(t *testing.T) {
	t.Parallel()
	spec := newSpec()
	spec.Mount("/things", things())

	doc := build(t, spec)

	list := lookup(t, doc, "paths", "/things", "get")
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"data": {"type": "array", "items": {"$ref": "#/components/schemas/thingResponse"}},
			"pagination": {"$ref": "#/components/schemas/Pagination"}
		},
		"required": ["data", "pagination"],
		"additionalProperties": false
	}`, marshal(t, lookup(t, list, "responses", "200", "content", "application/json", "schema")))
	assert.JSONEq(t, `[
		{
			"name": "offset", "in": "query", "description": "The zero-based position of the first resource of the page.",
			"schema": {"type": "integer", "minimum": 0, "default": 0}
		},
		{
			"name": "limit", "in": "query", "description": "The maximum number of resources in the page.",
			"schema": {"type": "integer", "minimum": 1, "maximum": 100, "default": 50}
		}
	]`, marshal(t, lookup(t, list, "parameters")))
	assert.JSONEq(t, `["https://pkgsilo/problems/invalid-query"]`,
		marshal(t, lookup(t, list, "responses", "400", "content", "application/problem+json", "schema", "properties",
			"type", "enum")))
}

// TestBuild_Body checks that a request body refers to the schema of its type and adds the problems of
// decoding it, and that a resource is in its envelope.
func TestBuild_Body(t *testing.T) {
	t.Parallel()
	spec := newSpec()
	spec.Mount("/things", things())

	doc := build(t, spec)

	patch := lookup(t, doc, "paths", "/things/{id}", "patch")
	assert.JSONEq(t, `{
		"description": "The fields to change.",
		"required": true,
		"content": {"application/merge-patch+json": {"schema": {"$ref": "#/components/schemas/thingPatch"}}}
	}`, marshal(t, lookup(t, patch, "requestBody")))
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {"data": {"$ref": "#/components/schemas/thingResponse"}},
		"required": ["data"],
		"additionalProperties": false
	}`, marshal(t, lookup(t, patch, "responses", "200", "content", "application/json", "schema")))
	responses, ok := lookup(t, patch, "responses").(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"200", "400", "401", "403", "404", "413", "415", "422", "500"},
		keys(responses))
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {"name": {"type": "string"}},
		"additionalProperties": false
	}`, marshal(t, lookup(t, doc, "components", "schemas", "thingPatch")))
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {"id": {"type": "string", "format": "uuid"}, "name": {"type": "string"}},
		"required": ["id"],
		"additionalProperties": false
	}`, marshal(t, lookup(t, doc, "components", "schemas", "thingResponse")))
}

func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}

	return ks
}

// TestBuild_Problems checks that the problems of a status are one response that allows only their types.
func TestBuild_Problems(t *testing.T) {
	t.Parallel()
	spec := newSpec()
	spec.Mount("/things", things())

	doc := build(t, spec)

	assert.JSONEq(t, `{
		"description": "A problem of one of these types:\n\n- Thing not found: `+
		"`https://pkgsilo/problems/thing-not-found`"+`",
		"content": {"application/problem+json": {"schema": {
			"allOf": [{"$ref": "#/components/schemas/Problem"}],
			"properties": {"type": {"enum": ["https://pkgsilo/problems/thing-not-found"]}, "status": {"const": 404}}
		}}}
	}`, marshal(t, lookup(t, doc, "paths", "/things/{id}", "delete", "responses", "404")))
	assert.JSONEq(t, `["https://pkgsilo/problems/invalid-body"]`,
		marshal(t, lookup(t, doc, "paths", "/things/{id}", "patch", "responses", "422", "content",
			"application/problem+json", "schema", "properties", "type", "enum")))
}

// TestBuild_Created checks that a created resource has a Location header.
func TestBuild_Created(t *testing.T) {
	t.Parallel()
	spec := newSpec()
	spec.Mount("/things", things())

	doc := build(t, spec)

	assert.JSONEq(t, `{
		"description": "The path of the new resource.",
		"required": true,
		"schema": {"type": "string", "format": "uri-reference"}
	}`, marshal(t, lookup(t, doc, "paths", "/things", "post", "responses", "201", "headers", "Location")))
}

// TestBuild_Security checks that the routes need the security scheme, unless they're public.
func TestBuild_Security(t *testing.T) {
	t.Parallel()
	spec := newSpec()
	spec.Mount("/things", things())
	spec.MountPublic("/health", openapi.Section{Name: "Health", Operations: []openapi.Operation{{
		Method: http.MethodGet, Pattern: "/", ID: "checkHealth", Summary: "Check the health",
		Responses: []openapi.Response{openapi.JSON[healthResponse](http.StatusOK, "Healthy.")},
	}}})

	doc := build(t, spec)

	assert.JSONEq(t, `[{"apiKey": []}]`, marshal(t, lookup(t, doc, "security")))
	assert.JSONEq(t, `{"apiKey": {"type": "apiKey", "name": "X-API-Key", "in": "header"}}`,
		marshal(t, lookup(t, doc, "components", "securitySchemes")))
	health := lookup(t, doc, "paths", "/health", "get")
	assert.JSONEq(t, `[]`, marshal(t, lookup(t, health, "security")))
	responses, ok := lookup(t, health, "responses").(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"200", "500"}, keys(responses), "no 401 for a public route")
	assert.NotContains(t, lookup(t, doc, "paths", "/things", "get"), "security")
}

// TestBuild_Invalid checks that Build fails for an operation that it can't describe.
func TestBuild_Invalid(t *testing.T) {
	t.Parallel()
	ok := openapi.Operation{
		Method: http.MethodGet, Pattern: "/{id}", ID: "getThing", Summary: "Get a thing",
		Parameters: []openapi.Parameter{openapi.PathID("id", "The ID.")},
		Responses:  []openapi.Response{openapi.Resource[thingResponse]("The thing.")},
	}
	tests := []struct {
		name   string
		change func(op *openapi.Operation)
	}{
		{name: "unknown method", change: func(op *openapi.Operation) { op.Method = "FETCH" }},
		{name: "no ID", change: func(op *openapi.Operation) { op.ID = "" }},
		{name: "no summary", change: func(op *openapi.Operation) { op.Summary = "" }},
		{name: "no responses", change: func(op *openapi.Operation) { op.Responses = nil }},
		{name: "undeclared path parameter", change: func(op *openapi.Operation) { op.Parameters = nil }},
		{name: "path parameter not in the path", change: func(op *openapi.Operation) { op.Pattern = "/" }},
		{name: "regular expression", change: func(op *openapi.Operation) { op.Pattern = "/{id:[0-9]+}" }},
		{
			name: "problem with a successful status",
			change: func(op *openapi.Operation) {
				op.Problems = []envelope.ProblemType{envelope.DefineProblemType("odd", "Odd", http.StatusOK)}
			},
		},
		{
			name: "field in the body",
			change: func(op *openapi.Operation) {
				op.Method, op.Body = http.MethodPut, openapi.JSONBody[thingResponse]("A body.")
			},
		},
		{
			name: "two responses with a status",
			change: func(op *openapi.Operation) {
				op.Responses = append(op.Responses, openapi.Resource[thingRequest]("Another."))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			op := ok
			op.Parameters = append([]openapi.Parameter(nil), ok.Parameters...)
			tt.change(&op)
			spec := newSpec()
			spec.Mount("/things", openapi.Section{Name: "Things", Operations: []openapi.Operation{op}})

			_, err := spec.Build()

			assert.Error(t, err)
		})
	}
}

// TestBuild_Twice checks that Build fails for a route or an operation ID that two operations have.
func TestBuild_Twice(t *testing.T) {
	t.Parallel()
	spec := newSpec()
	spec.Mount("/things", things())
	spec.Mount("/other-things", openapi.Section{Name: "Other things", Operations: things().Operations})

	_, err := spec.Build()

	assert.Error(t, err)
}

// TestDocument_ServeHTTP checks that the document is served in JSON, and not again to a client that has it.
func TestDocument_ServeHTTP(t *testing.T) {
	t.Parallel()
	spec := newSpec()
	spec.Mount("/things", things())
	doc, err := spec.Build()
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	doc.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, string(doc.JSON()), rec.Body.String())
	etag := rec.Header().Get("ETag")
	require.NotEmpty(t, etag)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	doc.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code)
}
