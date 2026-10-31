package apitest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
	"pkgsilo/core/request"
	"pkgsilo/test/tbtest"
)

type item struct {
	Name string `json:"name"`
}

func (i item) Validate() error {
	return validation.ValidateStruct(&i, validation.Field(&i.Name, validation.Required))
}

var (
	ada     = item{Name: "ada"}
	adaJSON = `{"name":"ada"}`

	itemGoneType    = envelope.DefineProblemType("item-gone", "Item gone", http.StatusGone)
	itemGoneProblem = itemGoneType.New(envelope.Occurrence{Detail: "The item is gone."})
)

// newTestClient returns a client for routes that answer in every format the assertions cover,
// mounted under /items on the application router.
func newTestClient(t *testing.T) *Client {
	t.Helper()

	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		page, ok := request.ParsePagination(w, req)
		if ok {
			envelope.WriteCollection(w, envelope.NewPagedCollection([]item{ada}, page.Offset, page.Limit, true))
		}
	})
	r.Post("/", func(w http.ResponseWriter, req *http.Request) {
		var body item
		if request.DecodeJSON(w, req, &body) {
			envelope.WriteCreated(w, "/items/"+body.Name, body)
		}
	})
	r.Get("/ada", func(w http.ResponseWriter, _ *http.Request) { envelope.WriteResource(w, ada) })
	r.Delete("/ada", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Get("/gone", func(w http.ResponseWriter, _ *http.Request) { envelope.WriteProblem(w, itemGoneProblem) })
	r.Get("/status", func(w http.ResponseWriter, _ *http.Request) {
		envelope.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down"})
	})
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("boom") })
	// Echoes the request, for checking what Send built.
	r.HandleFunc("/echo", func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		envelope.WriteJSON(w, http.StatusOK, map[string]string{
			"method": req.Method, "contentType": req.Header.Get("Content-Type"), "body": string(body),
		})
	})

	return NewClient(t, Mount("/items", r))
}

func TestClient_Send(t *testing.T) {
	t.Parallel()
	api := newTestClient(t)

	api.Post("/items/echo", `{"a":1}`).AssertJSON(http.StatusOK,
		`{"method":"POST","contentType":"application/json","body":"{\"a\":1}"}`)
	api.Put("/items/echo", `{"a":1}`).AssertJSON(http.StatusOK,
		`{"method":"PUT","contentType":"application/json","body":"{\"a\":1}"}`)
	// Without a body, no content type is claimed.
	api.Get("/items/echo").AssertJSON(http.StatusOK, `{"method":"GET","contentType":"","body":""}`)
	api.Delete("/items/echo").AssertJSON(http.StatusOK, `{"method":"DELETE","contentType":"","body":""}`)
	api.Send(http.MethodPatch, "/items/echo", "").AssertJSON(http.StatusOK,
		`{"method":"PATCH","contentType":"","body":""}`)
}

func TestClient_Do(t *testing.T) {
	t.Parallel()
	api := newTestClient(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items/echo", strings.NewReader("ada"))
	req.Header.Set("Content-Type", "text/plain")

	api.Do(req).AssertJSON(http.StatusOK, `{"method":"POST","contentType":"text/plain","body":"ada"}`)
}

func TestClient_For(t *testing.T) {
	t.Parallel()
	api := newTestClient(t)

	rec := tbtest.Run(t, func(tb *tbtest.Recorder) {
		api.For(tb).Get("/items/ada").AssertStatus(http.StatusTeapot)
	})

	assert.True(t, rec.Failed(), "the failure must be reported to the client's new test")
}

func TestMount(t *testing.T) {
	t.Parallel()
	api := newTestClient(t)

	unmatched := api.Get("/items/missing")

	unmatched.AssertProblem(envelope.NotFoundProblem.New(envelope.Occurrence{}))
	assert.NotEmpty(t, unmatched.Header().Get(middleware.RequestIDHeader), "the shared middleware must run")
	api.Get("/items/panic").AssertInternalError()
}

// TestResponse_Passes runs every assertion on a response that it matches.
func TestResponse_Passes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		assert func(api *Client)
	}{
		{name: "status", assert: func(api *Client) { api.Get("/items/ada").AssertStatus(http.StatusOK) }},
		{name: "resource", assert: func(api *Client) { api.Get("/items/ada").AssertResource(adaJSON) }},
		{name: "created", assert: func(api *Client) { api.Post("/items", adaJSON).AssertCreated("/items/ada", adaJSON) }},
		{name: "collection", assert: func(api *Client) {
			api.Get("/items?offset=1&limit=1").AssertCollection(`[`+adaJSON+`]`,
				envelope.Pagination{Offset: 1, Limit: 1, NextOffset: new(2)})
		}},
		{name: "no content", assert: func(api *Client) { api.Delete("/items/ada").AssertNoContent() }},
		{name: "json", assert: func(api *Client) {
			api.Get("/items/status").AssertJSON(http.StatusServiceUnavailable, `{"status":"down"}`)
		}},
		{name: "problem", assert: func(api *Client) { api.Get("/items/gone").AssertProblem(itemGoneProblem) }},
		{name: "problem type", assert: func(api *Client) { api.Get("/items/gone").AssertProblemType(itemGoneType) }},
		{name: "invalid body", assert: func(api *Client) {
			api.Post("/items", `{}`).AssertInvalidBody(envelope.ValidationError{Pointer: "#/name", Detail: "cannot be blank"})
		}},
		{name: "invalid query", assert: func(api *Client) {
			api.Get("/items?limit=0").AssertInvalidQuery(
				envelope.ValidationError{Parameter: "limit", Detail: "must be no less than 1"})
		}},
		{name: "internal error", assert: func(api *Client) { api.Get("/items/panic").AssertInternalError() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api := newTestClient(t)

			rec := tbtest.Run(t, func(tb *tbtest.Recorder) { tt.assert(api.For(tb)) })

			assert.False(t, rec.Failed(), rec.Messages())
		})
	}
}

// TestResponse_Fails runs every assertion on a response that it doesn't match, and checks that the
// failure message tells why.
func TestResponse_Fails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		assert func(api *Client)
		// wantMessage is part of the reported failure.
		wantMessage string
	}{
		{
			name:        "status shows the body",
			assert:      func(api *Client) { api.Get("/items/gone").AssertStatus(http.StatusOK) },
			wantMessage: "The item is gone.",
		},
		{
			name:        "resource data",
			assert:      func(api *Client) { api.Get("/items/ada").AssertResource(`{"name":"grace"}`) },
			wantMessage: "grace",
		},
		{
			name:        "resource of a problem",
			assert:      func(api *Client) { api.Get("/items/gone").AssertResource(adaJSON) },
			wantMessage: "status of the response",
		},
		{
			name:        "created location",
			assert:      func(api *Client) { api.Post("/items", adaJSON).AssertCreated("/items/grace", adaJSON) },
			wantMessage: "Location header",
		},
		{
			name: "collection pagination",
			assert: func(api *Client) {
				api.Get("/items").AssertCollection(`[`+adaJSON+`]`, envelope.Pagination{Offset: 0, Limit: 50})
			},
			wantMessage: "nextOffset",
		},
		{
			name:        "no content with a body",
			assert:      func(api *Client) { api.Get("/items/ada").AssertNoContent() },
			wantMessage: "status of the response",
		},
		{
			name:        "json content type",
			assert:      func(api *Client) { api.Get("/items/gone").AssertJSON(http.StatusGone, `{}`) },
			wantMessage: "Content-Type header",
		},
		{
			name: "problem detail",
			assert: func(api *Client) {
				api.Get("/items/gone").AssertProblem(itemGoneType.New(envelope.Occurrence{Detail: "Other."}))
			},
			wantMessage: "Other.",
		},
		{
			name: "problem type",
			assert: func(api *Client) {
				api.Get("/items/gone").AssertProblemType(envelope.DefineProblemType("item-lost", "Item gone", http.StatusGone))
			},
			wantMessage: "item-lost",
		},
		{
			name:        "invalid body errors",
			assert:      func(api *Client) { api.Post("/items", `{}`).AssertInvalidBody() },
			wantMessage: "cannot be blank",
		},
		{
			name:        "internal error of a success",
			assert:      func(api *Client) { api.Get("/items/ada").AssertInternalError() },
			wantMessage: "status of the response",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api := newTestClient(t)

			rec := tbtest.Run(t, func(tb *tbtest.Recorder) { tt.assert(api.For(tb)) })

			require.True(t, rec.Failed(), "the mismatch must be reported")
			assert.Contains(t, rec.Messages(), tt.wantMessage)
		})
	}
}

func TestDecodeResource(t *testing.T) {
	t.Parallel()
	api := newTestClient(t)
	resp := api.Post("/items", adaJSON)

	assert.Equal(t, ada, DecodeResource[item](resp, http.StatusCreated))
	// Decoding leaves the body for further assertions.
	resp.AssertCreated("/items/ada", adaJSON)
}

func TestDecodeCollection(t *testing.T) {
	t.Parallel()
	api := newTestClient(t)

	got := DecodeCollection[item](api.Get("/items?limit=1"), http.StatusOK)

	assert.Equal(t, envelope.Collection[item]{
		Data:       []item{ada},
		Pagination: envelope.Pagination{Offset: 0, Limit: 1, NextOffset: new(1)},
	}, got)
}

func TestDecode_Fails(t *testing.T) {
	t.Parallel()
	type unrelated struct {
		Other string `json:"other"`
	}
	tests := []struct {
		name        string
		decode      func(api *Client)
		wantMessage string
	}{
		{
			name:        "status",
			decode:      func(api *Client) { DecodeResource[item](api.Get("/items/gone"), http.StatusOK) },
			wantMessage: "The item is gone.",
		},
		{
			name:        "unknown member",
			decode:      func(api *Client) { DecodeResource[unrelated](api.Get("/items/ada"), http.StatusOK) },
			wantMessage: `unknown field "name"`,
		},
		{
			name:        "content type",
			decode:      func(api *Client) { DecodeCollection[item](api.Get("/items/gone"), http.StatusGone) },
			wantMessage: "Content-Type header",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api := newTestClient(t)
			reached := false

			rec := tbtest.Run(t, func(tb *tbtest.Recorder) {
				tt.decode(api.For(tb))
				reached = true
			})

			require.True(t, rec.Failed(), "the mismatch must be reported")
			assert.Contains(t, rec.Messages(), tt.wantMessage)
			assert.False(t, reached, "a failed decode must stop the test")
		})
	}
}
