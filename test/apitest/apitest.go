// Package apitest sends requests to HTTP handlers in tests and asserts on their responses in the
// formats of core/envelope.
//
// It must not import app or the features' packages: the internal tests of the features' api
// packages use it, so that would be an import cycle.
package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/core/router"
	"pkgsilo/test/loggertest"
)

// jsonMediaType is the media type of request bodies and of successful JSON responses.
const jsonMediaType = "application/json"

// Actor is the identity that requests to the routes of Mount are authenticated as.
var Actor = uuid.MustParse("5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b")

// Mount returns the application router, as router.New builds it, with routes mounted at prefix, so
// that a feature's routes are served behind the shared middleware and 404 and 405 handlers. Every
// request to them is authenticated as Actor.
func Mount(prefix string, routes http.Handler) http.Handler {
	logger := loggertest.Discard()
	r := router.New(router.Options{}, logger)
	r.With(authn.Require(logger, actorAuthenticator{})).Mount(prefix, routes)

	return r
}

// actorAuthenticator authenticates every request as Actor.
type actorAuthenticator struct{}

var _ authn.Authenticator = actorAuthenticator{}

// Method returns the name of the authentication method.
func (actorAuthenticator) Method() string {
	return "test"
}

// Authenticate returns the principal of Actor.
func (actorAuthenticator) Authenticate(*http.Request) (authn.Principal, error) {
	return authn.Principal{IdentityID: Actor}, nil
}

// Challenge returns a challenge that no response uses, since every request is authenticated.
func (actorAuthenticator) Challenge() string {
	return "Test"
}

// Client sends requests to a handler and reports failed assertions on the responses to its test.
//
// A client belongs to the test it was created for. Inside a subtest, use the client that For
// returns, so that failures are reported on the subtest.
type Client struct {
	tb      testing.TB
	handler http.Handler
	// header holds the headers of every request, unless the request sets them itself.
	header http.Header
}

// NewClient returns a client that sends requests to h and reports to tb.
func NewClient(tb testing.TB, h http.Handler) *Client {
	tb.Helper()

	return &Client{tb: tb, handler: h}
}

// For returns a client for the same handler that reports to tb.
func (c *Client) For(tb testing.TB) *Client {
	tb.Helper()

	return &Client{tb: tb, handler: c.handler, header: c.header}
}

// WithHeader returns a client for the same handler that also sets the given header on every request,
// such as the credential of a test's caller.
func (c *Client) WithHeader(name, value string) *Client {
	header := c.header.Clone()
	if header == nil {
		header = http.Header{}
	}
	header.Set(name, value)

	return &Client{tb: c.tb, handler: c.handler, header: header}
}

// WithHandler returns a client that sends requests with the same headers to h, such as another instance of
// the same application.
func (c *Client) WithHandler(h http.Handler) *Client {
	return &Client{tb: c.tb, handler: h, header: c.header}
}

// Anonymous returns a client for the same handler that sets no headers on its requests of its own, so
// that they hold no credential.
func (c *Client) Anonymous() *Client {
	return &Client{tb: c.tb, handler: c.handler}
}

// Get sends a GET request to target.
func (c *Client) Get(target string) *Response {
	return c.Send(http.MethodGet, target, "")
}

// Post sends a POST request with the JSON body to target.
func (c *Client) Post(target, body string) *Response {
	return c.Send(http.MethodPost, target, body)
}

// Put sends a PUT request with the JSON body to target.
func (c *Client) Put(target, body string) *Response {
	return c.Send(http.MethodPut, target, body)
}

// Patch sends a PATCH request with the JSON Merge Patch body to target.
func (c *Client) Patch(target, body string) *Response {
	return c.Send(http.MethodPatch, target, body)
}

// Delete sends a DELETE request to target.
func (c *Client) Delete(target string) *Response {
	return c.Send(http.MethodDelete, target, "")
}

// Send sends a request with method to target. A non-empty body is sent as application/json, or, with
// PATCH, as a JSON Merge Patch in application/merge-patch+json.
func (c *Client) Send(method, target, body string) *Response {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(c.tb.Context(), method, target, reader)
	switch {
	case body == "":
	case method == http.MethodPatch:
		req.Header.Set("Content-Type", request.MergePatchMediaType)
	default:
		req.Header.Set("Content-Type", jsonMediaType)
	}

	return c.Do(req)
}

// Do sends req, for requests that Send can't build, such as ones with other content types or
// headers.
func (c *Client) Do(req *http.Request) *Response {
	for name, values := range c.header {
		if _, set := req.Header[name]; !set {
			req.Header[name] = values
		}
	}

	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)

	return &Response{ResponseRecorder: rec, tb: c.tb}
}

// Response is a recorded response. Its assertions report to the test of the client that received
// it and, like those of testify's assert package, don't stop the test.
type Response struct {
	*httptest.ResponseRecorder

	tb testing.TB
}

// AssertStatus asserts that the response has the given status. The failure message holds the body,
// which tells why a request failed.
func (r *Response) AssertStatus(status int) {
	r.tb.Helper()
	r.status(status)
}

// AssertResource asserts a resource response with status 200 whose data is the JSON wantData.
func (r *Response) AssertResource(wantData string) {
	r.tb.Helper()
	r.body(http.StatusOK, jsonMediaType, `{"data":`+wantData+`}`)
}

// AssertCreated asserts a resource response with status 201 whose data is the JSON wantData and
// whose Location header is wantLocation.
func (r *Response) AssertCreated(wantLocation, wantData string) {
	r.tb.Helper()
	if r.body(http.StatusCreated, jsonMediaType, `{"data":`+wantData+`}`) {
		assert.Equal(r.tb, wantLocation, r.Header().Get("Location"), "Location header")
	}
}

// AssertCollection asserts a collection response with status 200 whose data is the JSON array
// wantData and whose pagination is want.
func (r *Response) AssertCollection(wantData string, want envelope.Pagination) {
	r.tb.Helper()
	pagination, err := json.Marshal(want)
	require.NoError(r.tb, err)
	r.body(http.StatusOK, jsonMediaType, `{"data":`+wantData+`,"pagination":`+string(pagination)+`}`)
}

// AssertNoContent asserts a response with status 204 and an empty body.
func (r *Response) AssertNoContent() {
	r.tb.Helper()
	if r.status(http.StatusNoContent) {
		assert.Empty(r.tb, r.Body.String(), "body")
	}
}

// AssertJSON asserts a JSON response with the given status and the body want, for bodies with a
// format of their own that are written without an envelope.
func (r *Response) AssertJSON(status int, want string) {
	r.tb.Helper()
	r.body(status, jsonMediaType, want)
}

// AssertProblem asserts a Problem Details response that equals want, its status included.
func (r *Response) AssertProblem(want envelope.Problem) {
	r.tb.Helper()
	b, err := json.Marshal(want)
	require.NoError(r.tb, err)
	r.body(want.Status, envelope.ProblemJSONMediaType, string(b))
}

// AssertProblemType asserts a Problem Details response of the problem type want, whatever the
// members that are specific to the occurrence.
func (r *Response) AssertProblemType(want envelope.ProblemType) {
	r.tb.Helper()
	if !r.status(want.Status) || !r.contentType(envelope.ProblemJSONMediaType) {
		return
	}

	var got struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &got); err != nil {
		r.tb.Errorf("decode problem: %v\nbody: %s", err, r.Body.String())
		return
	}
	assert.Equal(r.tb, want, envelope.ProblemType{Type: got.Type, Title: got.Title, Status: got.Status})
}

// AssertInvalidBody asserts the problem that request.DecodeJSON answers a body with invalid members
// with, listing errs in the order of their pointers.
func (r *Response) AssertInvalidBody(errs ...envelope.ValidationError) {
	r.tb.Helper()
	r.AssertProblem(request.InvalidBodyProblem(errs...))
}

// AssertInvalidQuery asserts the problem that request.ParsePagination answers invalid query
// parameters with, listing errs in the order of their parameters.
func (r *Response) AssertInvalidQuery(errs ...envelope.ValidationError) {
	r.tb.Helper()
	r.AssertProblem(request.InvalidQueryProblem(errs...))
}

// AssertInternalError asserts the generic 500 problem that unexpected errors are answered with.
func (r *Response) AssertInternalError() {
	r.tb.Helper()
	r.AssertProblem(envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
}

// DecodeResource returns the data of the resource response r, decoded into T. It stops the test
// unless r has the given status and its data has no members that T lacks.
func DecodeResource[T any](r *Response, status int) T {
	r.tb.Helper()

	return decode[envelope.Resource[T]](r, status).Data
}

// DecodeCollection returns the collection response r, its data decoded into T. It stops the test
// unless r has the given status and its data has no members that T lacks.
func DecodeCollection[T any](r *Response, status int) envelope.Collection[T] {
	r.tb.Helper()

	return decode[envelope.Collection[T]](r, status)
}

func decode[T any](r *Response, status int) T {
	r.tb.Helper()
	if !r.status(status) || !r.contentType(jsonMediaType) {
		r.tb.FailNow()
	}

	// Decoded from a copy, so that the body stays readable for later assertions.
	dec := json.NewDecoder(bytes.NewReader(r.Body.Bytes()))
	dec.DisallowUnknownFields()
	var v T
	if err := dec.Decode(&v); err != nil {
		r.tb.Fatalf("decode body: %v\nbody: %s", err, r.Body.String())
	}

	return v
}

// body asserts the status, content type and JSON body of the response and reports whether they all
// match. The body is only compared when the status matches, since a body of another status says why
// it doesn't.
func (r *Response) body(status int, contentType, want string) bool {
	r.tb.Helper()
	if !r.status(status) || !r.contentType(contentType) {
		return false
	}

	return assert.JSONEq(r.tb, want, r.Body.String())
}

func (r *Response) status(want int) bool {
	r.tb.Helper()

	return assert.Equal(r.tb, want, r.Code, "status of the response with body:\n%s", r.Body.String())
}

func (r *Response) contentType(want string) bool {
	r.tb.Helper()

	return assert.Equal(r.tb, want, r.Header().Get("Content-Type"), "Content-Type header")
}
