package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	"pkgsilo/core/mcpserver"
	"pkgsilo/core/request"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
	"pkgsilo/test/mcptest"
	"pkgsilo/test/telemetrytest"
)

// registerFunc is a Toolset that registers its tools by calling itself.
type registerFunc func(reg *mcpserver.Registry)

func (f registerFunc) Register(reg *mcpserver.Registry) {
	f(reg)
}

// echoInput is the input of the echo tool.
type echoInput struct {
	ID   uuid.UUID `json:"id" jsonschema:"The ID to echo."`
	Name string    `json:"name" jsonschema:"The name to echo."`
	Note *string   `json:"note,omitempty" jsonschema:"The note to echo, if any."`
}

func (in echoInput) Validate() error {
	return validation.ValidateStruct(&in, validation.Field(&in.Name, validation.Required, validation.Length(3, 10)))
}

// echoOutput is the output of the echo tool. It hides the note unless the input sets one.
type echoOutput struct {
	ID    uuid.UUID                 `json:"id"`
	Actor envelope.Field[uuid.UUID] `json:"actor,omitzero" jsonschema:"The actor."`
	Name  envelope.Field[string]    `json:"name,omitzero"`
	Note  envelope.Field[*string]   `json:"note,omitzero"`
	At    envelope.Field[time.Time] `json:"at,omitzero"`
}

var at = time.Date(2026, time.November, 14, 12, 0, 0, 0, time.UTC)

// echo returns its input and the actor.
func echo(_ context.Context, actor uuid.UUID, in echoInput) (echoOutput, error) {
	return echoOutput{
		ID: in.ID, Actor: envelope.Visible(actor), Name: envelope.Visible(in.Name),
		Note: envelope.NewField(in.Note, in.Note != nil), At: envelope.Visible(at),
	}, nil
}

// noInput is the input of a tool without arguments.
type noInput struct{}

func (noInput) Validate() error {
	return nil
}

// failing returns a tool handler that fails with err.
func failing(err error) func(context.Context, uuid.UUID, noInput) (mcpserver.DoneOutput, error) {
	return func(context.Context, uuid.UUID, noInput) (mcpserver.DoneOutput, error) {
		return mcpserver.DoneOutput{}, err
	}
}

// echoTool registers the echo tool.
func echoTool(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{Name: "test_echo", Description: "Echoes its input.", Kind: mcpserver.Read}, echo)
}

// server is an MCP endpoint served for a test, with the telemetry and the log that it records.
type server struct {
	url       string
	mcp       *mcptest.Client
	telemetry *telemetrytest.Recorder
	logs      *bytes.Buffer
}

// serve serves the tools that register adds through mcpserver.NewHandler, inside a span named parent, as the
// application router would serve it inside the request's server span.
func serve(t *testing.T, register func(reg *mcpserver.Registry)) server {
	t.Helper()

	logger, logs := loggertest.NewJSON()
	tel := telemetrytest.New(t)
	h := mcpserver.NewHandler(logger, tel.MeterProvider(), tel.TracerProvider(), registerFunc(register))
	parent := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tel.TracerProvider().Tracer("pkgsilo/test").Start(r.Context(), "parent")
		defer span.End()
		h.ServeHTTP(w, r.WithContext(ctx))
	})
	url := mcptest.Serve(t, parent).URL + mcptest.Path

	return server{url: url, mcp: mcptest.Connect(t, url, nil), telemetry: tel, logs: logs}
}

// response is a response of the endpoint to a request of post or send.
type response struct {
	status int
	header http.Header
}

// post sends a JSON-RPC message to the endpoint as a client would, with the given content type and Host.
func (s server) post(t *testing.T, contentType, host, body string) response {
	t.Helper()

	return send(t, s.newRequest(t, contentType, host, body))
}

// postBody sends the JSON-RPC message body to the endpoint as a client would, and returns the body of the
// response, which must be 200 OK.
func (s server) postBody(t *testing.T, body string) string {
	t.Helper()

	resp, err := http.DefaultClient.Do(s.newRequest(t, "application/json", "", body))
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(b)
}

// newRequest returns the request of post.
func (s server) newRequest(t *testing.T, contentType, host, body string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, s.url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json, text/event-stream")
	if host != "" {
		req.Host = host
	}

	return req
}

// send sends req and returns its response, whose body it drops.
func send(t *testing.T, req *http.Request) response {
	t.Helper()

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	return response{status: resp.StatusCode, header: resp.Header}
}

// callBody is the JSON-RPC request that calls the tool with the given name without arguments.
func callBody(tool string) string {
	return `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"` + tool + `","arguments":{}}}`
}

func TestNewHandler_ServerInfo(t *testing.T) {
	t.Parallel()
	s := serve(t, echoTool)

	name, _ := s.mcp.ServerInfo()

	assert.Equal(t, "pkgsilo", name)
}

func TestAdd_PassesActorAndArguments(t *testing.T) {
	t.Parallel()
	s := serve(t, echoTool)

	s.mcp.Call("test_echo", `{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4","name":"ada"}`).AssertOutput(`{
		"id": "0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4",
		"actor": "` + apitest.Actor.String() + `",
		"name": "ada",
		"at": "2026-11-14T12:00:00Z"
	}`)
}

// TestAdd_TextContent checks that the text content leaves <, > and & as they are, which encoding/json
// escapes, so that a model that reads only the text reads them as they are.
func TestAdd_TextContent(t *testing.T) {
	t.Parallel()
	s := serve(t, echoTool)

	result := s.mcp.Call("test_echo", `{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4","name":"a<b&c>","note":"&&"}`)

	result.AssertOutput(`{
		"id": "0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4",
		"actor": "` + apitest.Actor.String() + `",
		"name": "a<b&c>",
		"note": "&&",
		"at": "2026-11-14T12:00:00Z"
	}`)
	assert.Contains(t, result.Text(), `"name":"a<b&c>","note":"&&"`)
}

// TestAdd_VisibleNull checks that a visible field whose value is null is null in the output.
func TestAdd_VisibleNull(t *testing.T) {
	t.Parallel()
	s := serve(t, func(reg *mcpserver.Registry) {
		reg.Add(mcpserver.Tool{Name: "test_null", Description: "Returns a null note.", Kind: mcpserver.Read},
			func(context.Context, uuid.UUID, noInput) (echoOutput, error) {
				return echoOutput{Note: envelope.Visible[*string](nil)}, nil
			})
	})

	s.mcp.Call("test_null", "").AssertOutput(`{"id":"00000000-0000-0000-0000-000000000000","note":null}`)
}

// TestAdd_Annotations checks the annotations of each kind of tool: a write is destructive unless it's
// additive, and idempotent only if it says so.
func TestAdd_Annotations(t *testing.T) {
	t.Parallel()
	s := serve(t, func(reg *mcpserver.Registry) {
		for _, tool := range []mcpserver.Tool{
			{Name: "test_read", Kind: mcpserver.Read},
			{Name: "test_write", Kind: mcpserver.Write},
			{Name: "test_create", Kind: mcpserver.Write, Additive: true},
			{Name: "test_update", Kind: mcpserver.Write, Idempotent: true},
			{Name: "test_add", Kind: mcpserver.Write, Additive: true, Idempotent: true},
			{Name: "test_delete", Kind: mcpserver.Delete},
		} {
			tool.Description = "A tool."
			reg.Add(tool, failing(nil))
		}
	})

	got := map[string]string{}
	for _, tool := range s.mcp.Tools() {
		got[tool.Name] = marshal(t, tool.Annotations)
	}

	assert.Equal(t, map[string]string{
		"test_read":   `{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`,
		"test_write":  `{"destructiveHint":true,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`,
		"test_create": `{"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`,
		"test_update": `{"destructiveHint":true,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":false}`,
		"test_add":    `{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":false}`,
		"test_delete": `{"destructiveHint":true,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":false}`,
	}, got)
}

// TestAdd_Schemas checks the schemas of a tool: a UUID is a string, a pointer in the input is optional but
// not null, and an envelope.Field has the schema of its value, null included for a pointer, and is optional.
func TestAdd_Schemas(t *testing.T) {
	t.Parallel()
	s := serve(t, echoTool)

	tools := s.mcp.Tools()

	require.Len(t, tools, 1)
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"id": {"type": "string", "format": "uuid", "description": "The ID to echo."},
			"name": {"type": "string", "description": "The name to echo."},
			"note": {"type": "string", "description": "The note to echo, if any."}
		},
		"required": ["id", "name"],
		"additionalProperties": false
	}`, marshal(t, tools[0].InputSchema))
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"id": {"type": "string", "format": "uuid"},
			"actor": {"type": "string", "format": "uuid", "description": "The actor."},
			"name": {"type": "string"},
			"note": {"type": ["null", "string"]},
			"at": {"type": "string"}
		},
		"required": ["id"],
		"additionalProperties": false
	}`, marshal(t, tools[0].OutputSchema))
}

func TestAdd_InvalidArguments(t *testing.T) {
	t.Parallel()
	s := serve(t, echoTool)

	s.mcp.Call("test_echo", `{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4","name":"ab"}`).AssertToolError(
		mcpserver.InvalidArgumentsError(validation.Errors{"name": errors.New("the length must be between 3 and 10")}))
}

// TestAdd_ArgumentsNotMatchingTheSchema checks that arguments that don't match the input schema are answered
// with invalid_arguments, naming each member and why, before they're validated and the tool is called.
func TestAdd_ArgumentsNotMatchingTheSchema(t *testing.T) {
	t.Parallel()
	const id = `"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"`
	tests := []struct {
		name string
		args string
		want validation.Errors
	}{
		{
			name: "unknown member", args: `{` + id + `,"name":"ada","extra":1}`,
			want: validation.Errors{"extra": errors.New("is not an argument of this tool")},
		},
		{
			name: "missing members", args: `{"note":"x"}`,
			want: validation.Errors{"id": errors.New("is required"), "name": errors.New("is required")},
		},
		{
			name: "null", args: `{` + id + `,"name":"ada","note":null}`,
			want: validation.Errors{"note": errors.New("cannot be null")},
		},
		// Checked before the input's Validate, which would report a name that's too short.
		{
			name: "null of a required member", args: `{` + id + `,"name":null}`,
			want: validation.Errors{"name": errors.New("cannot be null")},
		},
		{
			name: "malformed UUID", args: `{"id":"abc","name":"ada"}`,
			want: validation.Errors{"id": errors.New("must be a UUID")},
		},
		{
			name: "empty UUID", args: `{"id":"","name":"ada"}`,
			want: validation.Errors{"id": errors.New("must be a UUID")},
		},
		{
			name: "wrong type", args: `{` + id + `,"name":7}`,
			want: validation.Errors{"name": errors.New("must be a string")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := serve(t, echoTool)

			result := s.mcp.Call("test_echo", tt.args)

			result.AssertToolError(mcpserver.InvalidArgumentsError(tt.want))
		})
	}
}

// TestAdd_MalformedArguments checks that arguments that the SDK's client can't send, since it decodes them
// first, are answered with invalid_arguments: arguments that aren't a JSON object, and a member that appears
// twice.
func TestAdd_MalformedArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args string
		want *mcpserver.ToolError
	}{
		{
			name: "not an object", args: `[1]`,
			want: mcpserver.DefineError("invalid_arguments", "The arguments must be a JSON object."),
		},
		{
			name: "duplicate member", args: `{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4","name":"ada","name":"bob"}`,
			want: mcpserver.InvalidArgumentsError(validation.Errors{"name": errors.New("appears more than once")}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := serve(t, echoTool)

			body := s.postBody(t,
				`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_echo","arguments":`+tt.args+`}}`)

			assert.JSONEq(t, `{
				"jsonrpc": "2.0",
				"id": 1,
				"result": {"content": [{"type": "text", "text": `+marshal(t, tt.want.Error())+`}], "isError": true}
			}`, body)
		})
	}
}

func TestAdd_Errors(t *testing.T) {
	t.Parallel()
	notFound := mcpserver.DefineError("thing_not_found", "The thing does not exist.")
	tests := []struct {
		name string
		err  error
		want *mcpserver.ToolError
	}{
		{name: "tool error", err: fmt.Errorf("get thing: %w", notFound), want: notFound},
		{
			name: "fields denied",
			err:  fmt.Errorf("create thing: %w", &authz.FieldsDeniedError{Fields: []string{"name", "note"}}),
			want: mcpserver.FieldsDeniedError("name", "note"),
		},
		{name: "denied", err: fmt.Errorf("delete thing: %w", authz.ErrDenied), want: mcpserver.ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := serve(t, func(reg *mcpserver.Registry) {
				reg.Add(mcpserver.Tool{Name: "test_fail", Description: "Fails.", Kind: mcpserver.Write}, failing(tt.err))
			})

			s.mcp.Call("test_fail", "").AssertToolError(tt.want)
			assert.Empty(t, s.logs.String(), "log")
		})
	}
}

func TestToolErrors_Text(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "fields_denied: The policies do not allow setting these arguments: name, note. Nothing was changed.",
		mcpserver.FieldsDeniedError("name", "note").Error())
	assert.Equal(t, "forbidden: The policies do not allow this action.", mcpserver.ErrForbidden.Error())
	assert.Equal(t, "invalid_arguments: The arguments are invalid: a: cannot be blank; b: is invalid.",
		mcpserver.InvalidArgumentsError(validation.Errors{
			"b": errors.New("is invalid"), "a": errors.New("cannot be blank"),
		}).Error())
	assert.Equal(t, "internal_error: internal error (request ID 42)", mcpserver.InternalError("42").Error())
}

// TestAdd_UnexpectedError checks that an unexpected error is logged and answered with the request ID only.
func TestAdd_UnexpectedError(t *testing.T) {
	t.Parallel()
	s := serve(t, func(reg *mcpserver.Registry) {
		reg.Add(mcpserver.Tool{Name: "test_fail", Description: "Fails.", Kind: mcpserver.Read},
			failing(errors.New("connection refused")))
	})

	result := s.mcp.Call("test_fail", "")

	result.AssertInternalError()
	entries := loggertest.Entries(t, s.logs)
	require.Len(t, entries, 1)
	assert.Equal(t, "mcp tool call failed", entries[0]["msg"])
	assert.Equal(t, "test_fail", entries[0]["tool"])
	assert.Equal(t, "connection refused", entries[0]["error"])
	result.AssertErrorText(mcpserver.InternalError(fmt.Sprint(entries[0]["request_id"])).Error())
}

func TestAdd_UnknownTool(t *testing.T) {
	t.Parallel()
	s := serve(t, echoTool)

	s.mcp.Call("test_nope", "").AssertProtocolError(-32602, `unknown tool "test_nope"`)
}

func TestAdd_Rejects(t *testing.T) {
	t.Parallel()
	type hiddenWithoutOmitzero struct {
		Name envelope.Field[string] `json:"name"`
	}
	tests := []struct {
		name string
		add  func(reg *mcpserver.Registry)
	}{
		{name: "empty name", add: addNamed("")},
		{name: "one word", add: addNamed("identity")},
		{name: "upper case", add: addNamed("identity_Get")},
		{name: "trailing underscore", add: addNamed("identity_")},
		{name: "digit", add: addNamed("identity_get2")},
		{name: "duplicate", add: func(reg *mcpserver.Registry) { addNamed("test_dup")(reg); addNamed("test_dup")(reg) }},
		{
			name: "empty description",
			add: func(reg *mcpserver.Registry) {
				reg.Add(mcpserver.Tool{Name: "test_tool", Kind: mcpserver.Read}, failing(nil))
			},
		},
		{
			name: "unknown kind",
			add: func(reg *mcpserver.Registry) {
				reg.Add(mcpserver.Tool{Name: "test_tool", Description: "A tool."}, failing(nil))
			},
		},
		{
			name: "additive read",
			add: func(reg *mcpserver.Registry) {
				reg.Add(mcpserver.Tool{Name: "test_tool", Description: "A tool.", Kind: mcpserver.Read, Additive: true},
					failing(nil))
			},
		},
		{
			name: "idempotent delete",
			add: func(reg *mcpserver.Registry) {
				reg.Add(mcpserver.Tool{
					Name: "test_tool", Description: "A tool.", Kind: mcpserver.Delete, Idempotent: true,
				}, failing(nil))
			},
		},
		{
			name: "field without omitzero",
			add: func(reg *mcpserver.Registry) {
				reg.Add(mcpserver.Tool{Name: "test_tool", Description: "A tool.", Kind: mcpserver.Read},
					func(context.Context, uuid.UUID, noInput) (hiddenWithoutOmitzero, error) {
						return hiddenWithoutOmitzero{}, nil
					})
			},
		},
		{
			name: "field in input",
			add: func(reg *mcpserver.Registry) {
				reg.Add(mcpserver.Tool{Name: "test_tool", Description: "A tool.", Kind: mcpserver.Read},
					func(context.Context, uuid.UUID, fieldInput) (mcpserver.DoneOutput, error) {
						return mcpserver.DoneOutput{}, nil
					})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				mcpserver.NewHandler(loggertest.Discard(), noop.NewMeterProvider(), tracenoop.NewTracerProvider(),
					registerFunc(tt.add))
			})
		})
	}
}

// fieldInput is an input with an envelope.Field, which only an output may hold.
type fieldInput struct {
	Name envelope.Field[string] `json:"name,omitzero"`
}

func (fieldInput) Validate() error {
	return nil
}

// addNamed returns a function that adds a valid tool with the given name.
func addNamed(name string) func(reg *mcpserver.Registry) {
	return func(reg *mcpserver.Registry) {
		reg.Add(mcpserver.Tool{Name: name, Description: "A tool.", Kind: mcpserver.Read}, failing(nil))
	}
}

func TestDefineError_Rejects(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, code, message string }{
		{name: "empty code", message: "A message."},
		{name: "kebab case", code: "not-found", message: "A message."},
		{name: "upper case", code: "Not_found", message: "A message."},
		{name: "empty message", code: "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() { _ = mcpserver.DefineError(tt.code, tt.message) })
		})
	}
}

func TestNewHandler_HTTP(t *testing.T) {
	t.Parallel()
	s := serve(t, echoTool)
	ping := `{"jsonrpc":"2.0","id":1,"method":"ping"}`

	t.Run("GET", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, s.url, http.NoBody)
		require.NoError(t, err)

		resp := send(t, req)

		assert.Equal(t, http.StatusMethodNotAllowed, resp.status)
		assert.Equal(t, http.MethodPost, resp.header.Get("Allow"))
	})
	t.Run("content type", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusUnsupportedMediaType, s.post(t, "text/plain", "", ping).status)
	})
	// A reverse proxy on a loopback address passes the Host of the client's request, which the SDK's
	// localhost protection would refuse.
	t.Run("non-loopback host", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusOK, s.post(t, "application/json", "pkgsilo.example", ping).status)
	})
	t.Run("body too large", func(t *testing.T) {
		t.Parallel()
		body := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"_meta":{"pad":"` +
			strings.Repeat("a", request.MaxBodyBytes) + `"}}}`

		assert.Equal(t, http.StatusRequestEntityTooLarge, s.post(t, "application/json", "", body).status)
	})
}

// TestAdd_Panic checks that a panic of a tool reaches the router's Recoverer, which answers with a 500,
// instead of crashing the process on the SDK's goroutine.
func TestAdd_Panic(t *testing.T) {
	t.Parallel()
	s := serve(t, func(reg *mcpserver.Registry) {
		reg.Add(mcpserver.Tool{Name: "test_panic", Description: "Panics.", Kind: mcpserver.Read},
			func(context.Context, uuid.UUID, noInput) (mcpserver.DoneOutput, error) { panic("boom") })
	})

	resp := s.post(t, "application/json", "", callBody("test_panic"))

	assert.Equal(t, http.StatusInternalServerError, resp.status)
	assert.Equal(t, envelope.ProblemJSONMediaType, resp.header.Get("Content-Type"))
}

// marshal returns v as JSON.
func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)

	return string(b)
}
