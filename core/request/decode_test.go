package request

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
)

type profile struct {
	Name    string  `json:"name"`
	Age     int     `json:"age"`
	Address address `json:"address"`
}

func (p profile) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.Name, validation.Required),
		validation.Field(&p.Age, validation.Min(18)),
		// Validated by address.Validate.
		validation.Field(&p.Address),
	)
}

type address struct {
	City string `json:"city"`
}

func (a address) Validate() error {
	return validation.ValidateStruct(&a, validation.Field(&a.City, validation.Required))
}

// namePatch is a JSON Merge Patch of a profile's name, which must not be null.
type namePatch struct {
	Name Optional[string] `json:"name"`
}

func (p namePatch) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.Name, IfSet(validation.Required)),
	)
}

// brokenRules fails validation with an internal error, as a misconfigured rule does.
type brokenRules struct{}

func (brokenRules) Validate() error {
	return validation.NewInternalError(errors.New("misconfigured rule"))
}

func newJSONRequest(contentType, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return req
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	var p profile

	req := newJSONRequest("application/json; charset=utf-8", `{"name":"Ada","age":36,"address":{"city":"London"}}`)
	ok := DecodeJSON(rec, req, &p)

	require.True(t, ok)
	assert.Equal(t, profile{Name: "Ada", Age: 36, Address: address{City: "London"}}, p)
	assert.Zero(t, rec.Body.Len(), "nothing must be written on success")
}

func TestDecodeJSON_Problems(t *testing.T) {
	t.Parallel()
	const valid = `{"name":"Ada","age":36,"address":{"city":"London"}}`

	tests := []struct {
		name        string
		contentType string
		body        string
		want        string
	}{
		{
			name: "missing content type",
			body: valid,
			want: `{"type":"about:blank","title":"Unsupported Media Type","status":415,
				"detail":"The request body must be application/json."}`,
		},
		{
			name:        "other content type",
			contentType: "text/plain",
			body:        valid,
			want: `{"type":"about:blank","title":"Unsupported Media Type","status":415,
				"detail":"The request body must be application/json."}`,
		},
		{
			name:        "empty body",
			contentType: "application/json",
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body must not be empty."}`,
		},
		{
			name:        "truncated JSON",
			contentType: "application/json",
			body:        `{"name":`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body is not valid JSON."}`,
		},
		{
			name:        "invalid JSON",
			contentType: "application/json",
			body:        `{"name" "Ada"}`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body is not valid JSON."}`,
		},
		{
			name:        "wrong member type",
			contentType: "application/json",
			body:        `{"name":42}`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The member \"name\" must be of type string."}`,
		},
		{
			name:        "unknown member",
			contentType: "application/json",
			body:        `{"name":"Ada","nickname":"ada"}`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The member \"nickname\" is not allowed."}`,
		},
		{
			name:        "not an object",
			contentType: "application/json",
			body:        `["Ada"]`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body must be a single JSON object."}`,
		},
		{
			name:        "null",
			contentType: "application/json",
			body:        `null`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body must be a single JSON object."}`,
		},
		{
			name:        "trailing value",
			contentType: "application/json",
			body:        valid + ` {}`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body must be a single JSON object."}`,
		},
		{
			name:        "trailing garbage",
			contentType: "application/json",
			body:        valid + ` x`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body is not valid JSON."}`,
		},
		{
			name:        "too large",
			contentType: "application/json",
			body:        `{"name":"` + strings.Repeat("a", MaxBodyBytes) + `"}`,
			want: `{"type":"about:blank","title":"Request Entity Too Large","status":413,
				"detail":"The request body must not exceed 1048576 bytes."}`,
		},
		{
			name:        "invalid members",
			contentType: "application/json",
			body:        `{"age":17,"address":{}}`,
			want: `{"type":"https://pkgsilo/problems/invalid-body","title":"Invalid request body","status":422,
				"detail":"The request body contains invalid members.",
				"errors":[
					{"detail":"cannot be blank","pointer":"#/address/city"},
					{"detail":"must be no less than 18","pointer":"#/age"},
					{"detail":"cannot be blank","pointer":"#/name"}
				]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()

			ok := DecodeJSON(rec, newJSONRequest(tt.contentType, tt.body), &profile{})

			assert.False(t, ok)
			assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.want, rec.Body.String())
		})
	}
}

func TestDecodeJSON_InternalErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dst  validation.Validatable
	}{
		{name: "validation fails internally", dst: &brokenRules{}},
		{name: "destination is not a pointer", dst: profile{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()

			assert.Panics(t, func() { DecodeJSON(rec, newJSONRequest("application/json", `{}`), tt.dst) })
			assert.Empty(t, rec.Body.String())
		})
	}
}

func TestDecodeMergePatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want namePatch
	}{
		{name: "value", body: `{"name":"Ada"}`, want: namePatch{Name: Optional[string]{Value: "Ada", Set: true}}},
		{name: "empty", body: ` {}`, want: namePatch{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			var got namePatch

			ok := DecodeMergePatch(rec, newJSONRequest("application/merge-patch+json; charset=utf-8", tt.body), &got)

			require.True(t, ok)
			assert.Equal(t, tt.want, got)
			assert.Zero(t, rec.Body.Len(), "nothing must be written on success")
		})
	}
}

func TestDecodeMergePatch_Problems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		body        string
		want        string
	}{
		{
			name:        "JSON",
			contentType: "application/json",
			body:        `{"name":"Ada"}`,
			want: `{"type":"about:blank","title":"Unsupported Media Type","status":415,
				"detail":"The request body must be application/merge-patch+json."}`,
		},
		{
			name:        "empty body",
			contentType: "application/merge-patch+json",
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body must not be empty."}`,
		},
		{
			name:        "null",
			contentType: "application/merge-patch+json",
			body:        ` null`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body must be a single JSON object."}`,
		},
		{
			name:        "wrong member type",
			contentType: "application/merge-patch+json",
			body:        `{"name":5}`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The member \"name\" must be of type string."}`,
		},
		{
			name:        "unknown member",
			contentType: "application/merge-patch+json",
			body:        `{"name":"Ada","nickname":"ada"}`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The member \"nickname\" is not allowed."}`,
		},
		{
			name:        "trailing value",
			contentType: "application/merge-patch+json",
			body:        `{} {}`,
			want: `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body must be a single JSON object."}`,
		},
		{
			name:        "too large",
			contentType: "application/merge-patch+json",
			body:        `{"name":"` + strings.Repeat("a", MaxBodyBytes) + `"}`,
			want: `{"type":"about:blank","title":"Request Entity Too Large","status":413,
				"detail":"The request body must not exceed 1048576 bytes."}`,
		},
		{
			name:        "null member",
			contentType: "application/merge-patch+json",
			body:        `{"name":null}`,
			want: `{"type":"https://pkgsilo/problems/invalid-body","title":"Invalid request body","status":422,
				"detail":"The request body contains invalid members.",
				"errors":[{"detail":"cannot be null","pointer":"#/name"}]}`,
		},
		{
			name:        "invalid member",
			contentType: "application/merge-patch+json",
			body:        `{"name":""}`,
			want: `{"type":"https://pkgsilo/problems/invalid-body","title":"Invalid request body","status":422,
				"detail":"The request body contains invalid members.",
				"errors":[{"detail":"cannot be blank","pointer":"#/name"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()

			ok := DecodeMergePatch(rec, newJSONRequest(tt.contentType, tt.body), &namePatch{})

			assert.False(t, ok)
			assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.want, rec.Body.String())
		})
	}
}

// TestDecodeMergePatch_AcceptPatch checks that a body of another media type is answered with the one that
// the endpoint takes, as RFC 5789 recommends.
func TestDecodeMergePatch_AcceptPatch(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	DecodeMergePatch(rec, newJSONRequest("application/json", `{}`), &namePatch{})

	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
	assert.Equal(t, "application/merge-patch+json", rec.Header().Get("Accept-Patch"))
}

func TestEnsureEmptyBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		body   io.Reader
		wantOK bool
	}{
		{name: "no body", wantOK: true},
		{name: "empty body", body: strings.NewReader(""), wantOK: true},
		{name: "JSON object", body: strings.NewReader(`{"role":"admin"}`)},
		{name: "empty JSON object", body: strings.NewReader(`{}`)},
		{name: "whitespace", body: strings.NewReader("\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()

			ok := EnsureEmptyBody(rec, httptest.NewRequest(http.MethodPut, "/", tt.body))

			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Zero(t, rec.Body.Len(), "nothing must be written on success")
				return
			}
			assert.JSONEq(t, `{"type":"about:blank","title":"Bad Request","status":400,
				"detail":"The request body must be empty."}`, rec.Body.String())
		})
	}
}

func TestJSONPointer(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "#", jsonPointer(nil))
	assert.Equal(t, "#/address/city", jsonPointer([]string{"address", "city"}))
	assert.Equal(t, "#/a~1b/c~0d", jsonPointer([]string{"a/b", "c~d"}))
}
