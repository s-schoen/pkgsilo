package apitest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/openapi"
)

// docs checks responses against an OpenAPI document.
type docs struct {
	model     *v3.Document
	validator validator.Validator
	options   *config.ValidationOptions
}

// docsCache holds the docs of each document, by its JSON, so that the clients of every test share them.
var docsCache sync.Map

// WithDocs returns a client for the same handler that also checks every response against the operations of
// section, whose routes are mounted at path, as in the OpenAPI document of the API. A response that the
// operation of its request doesn't describe, such as one with another status, problem type or body, fails
// the test. Responses to requests that no operation describes, such as to unknown routes, aren't checked.
func (c *Client) WithDocs(path string, section openapi.Section) *Client {
	c.tb.Helper()
	spec := openapi.Spec{
		Info:     openapi.Info{Title: "Test", Version: "test"},
		Security: []openapi.SecurityScheme{{Name: "apiKey", Header: "X-API-Key"}},
	}
	spec.Mount(path, section)
	doc, err := spec.Build()
	require.NoError(c.tb, err, "build the OpenAPI document")

	return c.WithDocument(doc.JSON())
}

// WithDocument returns a client for the same handler that also checks every response against the OpenAPI
// document in JSON, like WithDocs.
func (c *Client) WithDocument(document []byte) *Client {
	c.tb.Helper()
	d, err := loadDocs(document)
	require.NoError(c.tb, err, "load the OpenAPI document")

	client := *c
	client.docs = d

	return &client
}

func loadDocs(document []byte) (*docs, error) {
	if d, ok := docsCache.Load(string(document)); ok {
		return d.(*docs), nil //nolint:forcetypeassert // The cache holds only docs.
	}

	doc, err := libopenapi.NewDocument(document)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("build the model: %w", err)
	}
	options := []config.Option{config.WithFormatAssertions()}
	v, errs := validator.NewValidator(doc, options...)
	if len(errs) > 0 {
		return nil, fmt.Errorf("create the validator: %v", errs)
	}
	d := &docs{model: &model.Model, validator: v, options: config.NewValidationOptions(options...)}
	actual, _ := docsCache.LoadOrStore(string(document), d)

	return actual.(*docs), nil //nolint:forcetypeassert // The cache holds only docs.
}

// check reports to tb how the response rec to req differs from what the document describes, unless no
// operation describes req.
func (d *docs) check(tb testing.TB, req *http.Request, rec *httptest.ResponseRecorder) {
	tb.Helper()
	item, errs, _ := paths.FindPath(req, d.model, d.options)
	if item == nil || len(errs) > 0 || helpers.ExtractOperation(req, item) == nil {
		return
	}

	res := rec.Result()
	// The body of a recorded response holds nothing to release.
	defer func() { _ = res.Body.Close() }()
	if ok, errs := d.validator.ValidateHttpResponse(req, res); !ok {
		var messages strings.Builder
		for _, e := range errs {
			fmt.Fprintf(&messages, "\n  %s: %s", e.Message, e.Reason)
			for _, s := range e.SchemaValidationErrors {
				fmt.Fprintf(&messages, "\n    %s: %s", s.FieldPath, s.Reason)
			}
		}
		tb.Errorf("the %d response to %s %s doesn't match the OpenAPI document:%s\nbody: %s",
			rec.Code, req.Method, req.URL.Path, messages.String(), rec.Body.String())
	}
}
