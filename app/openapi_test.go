package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/router"
	sessionservice "pkgsilo/feature/session/service"
	"pkgsilo/test/loggertest"
)

// TestDocs pins the OpenAPI document, the public contract of the HTTP API. A change of it must be
// deliberate, by rewriting the golden file with -update, and shows up in its diff.
func TestDocs(t *testing.T) {
	t.Parallel()
	golden := filepath.Join("testdata", "openapi.json")

	got := Docs().JSON()

	if *update {
		// The trailing newline keeps the golden file stable under the end-of-file-fixer hook.
		require.NoError(t, os.WriteFile(golden, append(slices.Clone(got), '\n'), 0o600))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(got), "rewrite the golden file with go test ./app -run TestDocs -update")
}

// TestDocs_Valid checks that the OpenAPI document is valid OpenAPI 3.1.
func TestDocs_Valid(t *testing.T) {
	t.Parallel()
	doc, err := libopenapi.NewDocument(Docs().JSON())
	require.NoError(t, err)
	v, errs := validator.NewValidator(doc)
	require.Empty(t, errs)

	ok, validationErrs := v.ValidateDocument()

	for _, e := range validationErrs {
		t.Errorf("%s: %s", e.Message, e.Reason)
		for _, s := range e.SchemaValidationErrors {
			t.Errorf("  %s: %s", s.FieldPath, s.Reason)
		}
	}
	assert.True(t, ok)
}

// TestDocs_Routes checks that the OpenAPI document describes every route of the API, and no other, so that a
// route can't be added, moved or removed without its description. Only the document itself isn't in it.
func TestDocs_Routes(t *testing.T) {
	t.Parallel()
	handler := NewHandler(router.Options{}, nil, nil, sessionservice.Lifetimes{},
		loggertest.Discard(), noop.NewMeterProvider(),
		tracenoop.NewTracerProvider())
	routes, ok := handler.(chi.Routes)
	require.True(t, ok, "the handler is a chi router")

	var served []string
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(strings.ReplaceAll(route, "/*/", "/"), "/")
		if route == apiPath+specPath || route == mcpPath {
			return nil
		}
		served = append(served, method+" "+route)
		return nil
	})
	require.NoError(t, err)

	var documented []string
	for _, r := range Docs().Routes() {
		documented = append(documented, fmt.Sprintf("%s %s", r.Method, r.Path))
	}
	slices.Sort(served)
	slices.Sort(documented)
	assert.Equal(t, served, documented)
}
