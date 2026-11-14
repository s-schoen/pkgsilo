// Package mcptest calls the tools of the MCP endpoint in tests, with the official SDK's client over HTTP,
// and asserts on their results.
//
// New serves toolsets through the production handler of mcpserver.NewHandler on an httptest server, so
// that every call goes through HTTP, the JSON-RPC framing and the stateless mode of the endpoint, and none
// through the SDK's in-memory transport. Connect connects to any endpoint, such as the whole application
// that apptest serves.
//
// It must not import app or the features' packages: the internal tests of the features' tools packages use
// it, so that would be an import cycle.
package mcptest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/mcpserver"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

// Path is the path at which Serve serves the endpoint, as the application does.
const Path = "/mcp"

// internalErrorText matches the text of the internal_error result that answers an unexpected error.
var internalErrorText = regexp.MustCompile(`^internal_error: internal error \(request ID [A-Za-z0-9._-]+\)$`)

// New returns a client for toolsets, served with mcpserver.NewHandler, without telemetry and logs, as Serve
// serves it. Every call is authenticated as apitest.Actor. It fails the test unless every tool and every
// member of its input and output has a description (see Client.AssertDescriptions).
func New(tb testing.TB, toolsets ...mcpserver.Toolset) *Client {
	tb.Helper()

	handler := mcpserver.NewHandler(loggertest.Discard(), noop.NewMeterProvider(), tracenoop.NewTracerProvider(),
		toolsets...)
	c := Connect(tb, Serve(tb, handler).URL+Path, nil)
	c.AssertDescriptions()

	return c
}

// Serve serves h, an MCP endpoint, at Path on an httptest server on a loopback address, behind the shared
// middleware of the application router, as apitest.Mount mounts it: every request is authenticated as
// apitest.Actor. The server is closed at the end of the test.
func Serve(tb testing.TB, h http.Handler) *httptest.Server {
	tb.Helper()

	server := httptest.NewServer(apitest.Mount(Path, h))
	tb.Cleanup(server.Close)

	return server
}

// Client calls the tools of an MCP endpoint and reports failed assertions to its test. It connects on its
// first call, so that a client whose requests are refused, such as one without a credential, can be
// created too: Connect then returns the error.
//
// A client belongs to the test it was created for. Inside a subtest, use the client that For returns, so
// that failures are reported on the subtest.
type Client struct {
	tb   testing.TB
	conn *connection
}

// connection is the connection that a client and the clients of its For share.
type connection struct {
	endpoint string
	header   http.Header

	once    sync.Once
	session *mcp.ClientSession
	err     error
}

// Connect returns a client for the MCP endpoint at the URL endpoint that sends header with every request,
// such as the credential of a test's caller. The connection is closed at the end of the test.
func Connect(tb testing.TB, endpoint string, header http.Header) *Client {
	tb.Helper()

	conn := &connection{endpoint: endpoint, header: header.Clone()}
	tb.Cleanup(func() {
		if conn.session != nil {
			assert.NoError(tb, conn.session.Close())
		}
	})

	return &Client{tb: tb, conn: conn}
}

// For returns a client on the same connection that reports to tb.
func (c *Client) For(tb testing.TB) *Client {
	tb.Helper()

	return &Client{tb: tb, conn: c.conn}
}

// Connect connects to the endpoint, unless it did before, and returns the error of connecting, such as
// for a request that the endpoint refused.
func (c *Client) Connect() error {
	c.tb.Helper()
	conn := c.conn
	conn.once.Do(func() {
		client := mcp.NewClient(&mcp.Implementation{Name: "mcptest", Version: "test"}, nil)
		transport := &mcp.StreamableClientTransport{
			Endpoint:   conn.endpoint,
			HTTPClient: &http.Client{Transport: headerTransport{header: conn.header, next: http.DefaultTransport}},
			// The endpoint is stateless and serves no event stream.
			DisableStandaloneSSE: true,
		}
		conn.session, conn.err = client.Connect(c.tb.Context(), transport, nil)
	})

	return conn.err
}

// headerTransport adds header to every request that next sends.
type headerTransport struct {
	header http.Header
	next   http.RoundTripper
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for name, values := range t.header {
		req.Header[name] = slices.Clone(values)
	}

	return t.next.RoundTrip(req) //nolint:wrapcheck // The transport's error, for the SDK client.
}

// ServerInfo returns the name and version that the server gave in the handshake.
func (c *Client) ServerInfo() (name, version string) {
	c.tb.Helper()
	info := c.session().InitializeResult().ServerInfo

	return info.Name, info.Version
}

// Tools returns the tools that the endpoint lists, sorted by name.
func (c *Client) Tools() []*mcp.Tool {
	c.tb.Helper()
	var tools []*mcp.Tool
	for tool, err := range c.session().Tools(c.tb.Context(), nil) {
		require.NoError(c.tb, err, "list the tools")
		tools = append(tools, tool)
	}
	slices.SortFunc(tools, func(a, b *mcp.Tool) int { return strings.Compare(a.Name, b.Name) })

	return tools
}

// Hints returns what the annotations of each tool of the endpoint tell clients, by the tool's name:
// "read-only" for a tool that changes nothing, and otherwise "destructive" or "additive", followed by
// ", idempotent" for one that changes nothing more when it's called again.
func (c *Client) Hints() map[string]string {
	c.tb.Helper()
	hints := map[string]string{}
	for _, tool := range c.Tools() {
		a := tool.Annotations
		if !assert.NotNilf(c.tb, a, "annotations of tool %s", tool.Name) {
			continue
		}

		switch {
		case a.ReadOnlyHint:
			hints[tool.Name] = "read-only"
			continue
		// A missing destructiveHint means true.
		case a.DestructiveHint == nil || *a.DestructiveHint:
			hints[tool.Name] = "destructive"
		default:
			hints[tool.Name] = "additive"
		}
		if a.IdempotentHint {
			hints[tool.Name] += ", idempotent"
		}
	}

	return hints
}

// ToolNames returns the names of the tools that the endpoint lists, sorted.
func (c *Client) ToolNames() []string {
	c.tb.Helper()
	var names []string
	for _, tool := range c.Tools() {
		names = append(names, tool.Name)
	}

	return names
}

// AssertDescriptions asserts that every tool of the endpoint has a description, and so does every member of
// its input and of its output, nested ones included, such as those of the items of a list, since an agent
// chooses a tool, fills in its arguments and reads its output from the descriptions alone.
func (c *Client) AssertDescriptions() {
	c.tb.Helper()
	for _, tool := range c.Tools() {
		assert.NotEmpty(c.tb, tool.Description, "description of tool %s", tool.Name)

		for part, schema := range map[string]any{"input": tool.InputSchema, "output": tool.OutputSchema} {
			var s jsonschema.Schema
			remarshal(c.tb, schema, &s)
			walkMembers(&s, "", func(path string, member *jsonschema.Schema) {
				assert.NotEmpty(c.tb, member.Description,
					"description of member %s of the %s of tool %s: tag it with jsonschema", path, part, tool.Name)
			})
		}
	}
}

// walkMembers calls visit with every member of the objects in schema, nested ones included, and its path,
// such as items.id, under the prefix path.
func walkMembers(schema *jsonschema.Schema, path string, visit func(path string, member *jsonschema.Schema)) {
	if schema == nil {
		return
	}

	for name, member := range schema.Properties {
		visit(path+name, member)
		walkMembers(member, path+name+".", visit)
	}
	walkMembers(schema.Items, path, visit)
	walkMembers(schema.AdditionalProperties, path, visit)
}

// Call calls the tool with the given name with args, a JSON object, or no arguments if args is empty. Each
// member is sent as it's written, so that a number keeps its digits.
func (c *Client) Call(name, args string) *Result {
	c.tb.Helper()
	params := &mcp.CallToolParams{Name: name}
	if args != "" {
		var arguments map[string]jsontext.Value
		require.NoError(c.tb, json.Unmarshal([]byte(args), &arguments), "decode the arguments")
		params.Arguments = arguments
	}

	result, err := c.session().CallTool(c.tb.Context(), params)

	return &Result{tb: c.tb, result: result, err: err}
}

// session returns the session of the client's connection, connecting first if needed. It stops the test
// if connecting fails.
func (c *Client) session() *mcp.ClientSession {
	c.tb.Helper()
	require.NoError(c.tb, c.Connect(), "connect to the MCP endpoint")

	return c.conn.session
}

// Result is the result of a tool call. Its assertions report to the test of the client that called the
// tool and, like those of testify's assert package, don't stop the test.
type Result struct {
	tb     testing.TB
	result *mcp.CallToolResult
	// err is the error of the call, such as a JSON-RPC error that the server answered with.
	err error
}

// AssertOutput asserts a successful result whose structured output is the JSON want, and whose text content
// is the same JSON, for clients that read no structured output.
func (r *Result) AssertOutput(want string) {
	r.tb.Helper()
	if !r.succeeded() {
		return
	}

	got := marshal(r.tb, r.result.StructuredContent)
	if assert.JSONEq(r.tb, want, got, "structured output") {
		assert.JSONEq(r.tb, want, r.Text(), "text content")
	}
}

// Decode returns the structured output of a successful result, decoded into T. It stops the test unless the
// result succeeded and its output has no members that T lacks.
func (r *Result) Decode[T any]() T {
	r.tb.Helper()
	if !r.succeeded() {
		r.tb.FailNow()
	}

	var v T
	require.NoError(r.tb, json.Unmarshal([]byte(marshal(r.tb, r.result.StructuredContent)), &v,
		json.RejectUnknownMembers(true)), "decode the structured output")

	return v
}

// AssertToolError asserts a result that is the tool error want, such as one of a feature's tools/errors.go or
// mcpserver.ErrForbidden.
func (r *Result) AssertToolError(want *mcpserver.ToolError) {
	r.tb.Helper()
	r.AssertErrorText(want.Error())
}

// AssertInternalError asserts the internal_error result that answers an unexpected error, which names the
// request ID only.
func (r *Result) AssertInternalError() {
	r.tb.Helper()
	if r.isError() {
		assert.Regexp(r.tb, internalErrorText, r.Text(), "text of the error result")
	}
}

// AssertErrorText asserts a result that is an error with the text want, such as a tool error whose message
// depends on the call, "<code>: <message>".
func (r *Result) AssertErrorText(want string) {
	r.tb.Helper()
	if r.isError() {
		assert.Equal(r.tb, want, r.Text(), "text of the error result")
		assert.Nil(r.tb, r.result.StructuredContent, "structured output of the error result")
	}
}

// AssertProtocolError asserts that the call failed with a JSON-RPC error of the given code and message, such
// as the SDK's answer to an unknown tool, rather than with a result.
func (r *Result) AssertProtocolError(code int64, message string) {
	r.tb.Helper()
	wireErr, ok := errors.AsType[*jsonrpc.Error](r.err)
	if !assert.Truef(r.tb, ok, "want a JSON-RPC error, got the result %+v and the error %v", r.result, r.err) {
		return
	}
	assert.Equal(r.tb, code, wireErr.Code, "code of the JSON-RPC error")
	assert.Equal(r.tb, message, wireErr.Message, "message of the JSON-RPC error")
}

// Text returns the text of the result's only content, as a model that reads no structured output reads it.
// It asserts that the result has a single text content, and returns "" otherwise.
func (r *Result) Text() string {
	r.tb.Helper()
	if !assert.Len(r.tb, r.result.Content, 1, "content of the result") {
		return ""
	}
	text, ok := r.result.Content[0].(*mcp.TextContent)
	if !assert.Truef(r.tb, ok, "content of the result is a %T, no text", r.result.Content[0]) {
		return ""
	}

	return text.Text
}

// succeeded asserts that the call returned a result that isn't an error. It stops the test if the call
// failed without a result.
func (r *Result) succeeded() bool {
	r.tb.Helper()
	require.NoError(r.tb, r.err, "call the tool")

	return assert.Falsef(r.tb, r.result.IsError, "the result is an error: %s", r.Text())
}

// isError asserts that the call returned a result that is an error. It stops the test if the call failed
// without a result.
func (r *Result) isError() bool {
	r.tb.Helper()
	require.NoError(r.tb, r.err, "call the tool")

	return assert.Truef(r.tb, r.result.IsError, "the result is no error: %s", r.Text())
}

// marshal returns v as JSON.
func marshal(tb testing.TB, v any) string {
	tb.Helper()
	b, err := json.Marshal(v)
	require.NoError(tb, err)

	return string(b)
}

// remarshal decodes the JSON of v into dst.
func remarshal(tb testing.TB, v, dst any) {
	tb.Helper()
	require.NoError(tb, json.Unmarshal([]byte(marshal(tb, v)), dst))
}

// AssertReadableMembers asserts that every member of the output type except id is an envelope.Field tagged
// omitzero, so that a hidden field is absent from the output, and that these members are exactly the
// readable fields, which each have a read action of their own in the service, as
// apitest.AssertReadableMembers asserts for a response body.
func AssertReadableMembers(tb testing.TB, output reflect.Type, readable []string) {
	tb.Helper()
	apitest.AssertReadableMembers(tb, output, readable)
	for field := range output.Fields() {
		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name != "id" && !slices.Contains(strings.Split(options, ","), "omitzero") {
			tb.Errorf("member %q of %s isn't tagged omitzero, so a hidden field would fail the call", name, output)
		}
	}
}

// AssertDescriptionContains asserts that the description of the member of t with the given JSON name, its
// jsonschema tag, contains each of want, such as the limits of the member's rules, written from their
// constants: a tag can't refer to a constant, so when one changes, the test fails until the description
// names the new value.
func AssertDescriptionContains(tb testing.TB, t reflect.Type, member string, want ...string) {
	tb.Helper()
	for field := range t.Fields() {
		if name, _, _ := strings.Cut(field.Tag.Get("json"), ","); name != member {
			continue
		}

		description := field.Tag.Get("jsonschema")
		for _, w := range want {
			assert.Contains(tb, description, w, "description of member %s of %s", member, t)
		}
		return
	}

	tb.Errorf("%s has no member %s", t, member)
}

// AssertSettableMembers asserts that the members of the input type of a tool that creates or changes an
// object, except id, are exactly the fields that have an action for setting them in the service, such as
// Actions().CreateFields().Fields(). A member without one would be written without being authorized, and
// a field denial couldn't name a member that doesn't exist.
func AssertSettableMembers(tb testing.TB, input reflect.Type, settable []string) {
	tb.Helper()
	var members []string
	for field := range input.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name != "id" {
			members = append(members, name)
		}
	}

	assert.ElementsMatch(tb, settable, members, "the members of %s except id must be the settable fields", input)
}
