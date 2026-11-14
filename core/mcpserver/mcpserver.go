// Package mcpserver serves the tools of the features over the Model Context Protocol (MCP), with the
// official Go SDK, so that agents can work with pkgsilo.
//
// It is the only package that imports the SDK. A feature offers its tools in a package of its own,
// feature/<feature>/tools, whose Tools type is a Toolset: NewHandler passes it a Registry, and it adds each
// tool with Registry.Add, which hands the tool's handler the actor and its validated input, and never the
// SDK's types. A tool's errors are defined with DefineError.
//
// The handler that NewHandler returns is stateless: every request is an MCP session of its own, built from
// the request's context, so that a tool's handler sees the principal of authn.Require, the request ID and
// the request's server span. It answers in application/json, never in an event stream, and sends no
// requests or notifications to the client.
package mcpserver

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"runtime/debug"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/authn"
	"pkgsilo/core/request"
)

// serverName is the name that the server gives itself to clients.
const serverName = "pkgsilo"

// toolNamePattern is the form of a tool's name, <type>_<verb> in snake case, such as identity_get.
var toolNamePattern = regexp.MustCompile(`^[a-z]+(_[a-z]+)+$`)

// Kind tells what a tool does to the objects it touches, so that clients can approve reads without asking
// and ask before a change that takes something away. With Tool.Additive and Tool.Idempotent, it sets the
// tool's annotations.
type Kind int

// The kinds of tools.
const (
	// Read is a tool that changes nothing, such as one that gets or lists objects.
	Read Kind = iota + 1
	// Write is a tool that creates or changes objects. It's destructive, so that clients ask before calling
	// it, unless Tool.Additive says that it only adds.
	Write
	// Delete is a tool that removes an object, which can't be undone. It's destructive and idempotent.
	Delete
)

// Tool describes a tool to clients. An agent chooses a tool and fills in its arguments from the
// descriptions alone, so they say when to use it, the domain terms it relies on, what its errors mean and
// how to page.
type Tool struct {
	// Name is the tool's name, <type>_<verb> in snake case, with the type as in the type's authz actions,
	// such as identity_get or group_add_member.
	Name string
	// Description tells an agent when and how to use the tool.
	Description string
	// Kind tells what the tool does to the objects it touches.
	Kind Kind
	// Additive marks a Write that only adds, such as one that creates an object: it overwrites no value,
	// removes nothing and can't take away access, which a policy with a forbid or a group under one can.
	// Clients may call it without asking. Only a Write may set it.
	Additive bool
	// Idempotent marks a Write that changes nothing more when it's called again with the same arguments,
	// such as one that sets fields or adds a member that may be one already. Only a Write may set it.
	Idempotent bool
}

// annotations returns the MCP annotations of the tool: a Read is read-only, a Write is destructive unless
// it's Additive and idempotent if it's Idempotent, and a Delete is destructive and idempotent. None reaches
// beyond pkgsilo. It panics on an unknown kind, and on Additive or Idempotent for a kind but Write.
func (t Tool) annotations() *mcp.ToolAnnotations {
	if t.Kind != Write && (t.Additive || t.Idempotent) {
		panic(fmt.Sprintf("mcpserver: tool %q sets Additive or Idempotent, which only a Write may", t.Name))
	}

	switch t.Kind {
	case Read:
		return &mcp.ToolAnnotations{
			ReadOnlyHint: true, DestructiveHint: new(false), IdempotentHint: true, OpenWorldHint: new(false),
		}
	case Write:
		return &mcp.ToolAnnotations{
			DestructiveHint: new(!t.Additive), IdempotentHint: t.Idempotent, OpenWorldHint: new(false),
		}
	case Delete:
		return &mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(false)}
	default:
		panic(fmt.Sprintf("mcpserver: tool %q has the unknown kind %d", t.Name, t.Kind))
	}
}

// DoneOutput is the output of a tool that reports only that it succeeded, such as one that deletes an
// object: an empty object.
type DoneOutput struct{}

// Toolset is the set of tools that a feature offers, the Tools type of its tools package.
type Toolset interface {
	// Register adds the feature's tools to reg with Registry.Add.
	Register(reg *Registry)
}

// Registry is what a Toolset adds its tools to, on the server that NewHandler builds.
type Registry struct {
	server *mcp.Server
	logger *slog.Logger
	// tools holds the names of the tools added so far.
	tools map[string]bool
}

// Add adds a tool whose arguments are decoded into In and whose result is Out, both structs, to reg. Its
// schemas are inferred from their json tags, and the jsonschema tag of a member is its description.
// Members tagged omitempty or omitzero are optional, and all others are required. No member of In may be
// null, even a pointer, which is how an input marks an optional member: null would mean the same as
// leaving it out, which an agent might mistake for clearing it.
//
// For each call, Add decodes the arguments into In and validates them with In's Validate, which the type
// constraint requires (a tool without arguments takes an empty struct whose Validate returns nil), and then
// calls handler with the actor of the request, as authn.Actor returns it to an HTTP handler, which passes
// it to the service. Arguments that don't match the input schema, such as a missing or unknown member, a
// null or a malformed UUID, and arguments that Validate rejects are answered with the error
// invalid_arguments, which names each invalid member and why, and handler isn't called.
//
// handler returns the tool's output, or an error, which Add answers as a tool error: an error of
// DefineError with its code and message, an *authz.FieldsDeniedError with fields_denied, authz.ErrDenied
// with forbidden, and any other error with internal_error and the request ID, after logging it. The output
// is the result's structured content, and its text content too, as JSON that leaves <, > and & as they are,
// so that a model that reads only the text reads them as they are, such as in the source of a policy.
//
// Each member of Out but the ID is an envelope.Field[T] tagged omitzero, as in a response of the HTTP API,
// filled with envelope.NewField: a hidden field is absent from the output, and a visible null stays null.
// Its schema is that of T, and it's optional.
//
// Add panics if the tool's name isn't of the form <type>_<verb> or is taken, if its description is empty,
// if its kind is unknown or doesn't allow Tool.Additive or Tool.Idempotent, or if a schema can't be built
// from In or Out, such as for an envelope.Field without omitzero: these are bugs of the feature, found when
// the server is built. A call panics if its output can't be encoded or doesn't match the output schema.
func (r *Registry) Add[In validation.Validatable, Out any](
	tool Tool, handler func(ctx context.Context, actor uuid.UUID, in In) (Out, error),
) {
	if !toolNamePattern.MatchString(tool.Name) {
		panic(fmt.Sprintf("mcpserver: tool name %q is not of the form <type>_<verb>", tool.Name))
	}
	if r.tools[tool.Name] {
		panic(fmt.Sprintf("mcpserver: two tools named %q", tool.Name))
	}
	if tool.Description == "" {
		panic(fmt.Sprintf("mcpserver: tool %q has no description", tool.Name))
	}
	annotations := tool.annotations()
	inputSchema, err := schemaFor(reflect.TypeFor[In](), false)
	if err != nil {
		panic(fmt.Sprintf("mcpserver: input schema of tool %q: %v", tool.Name, err))
	}
	outputSchema, err := schemaFor(reflect.TypeFor[Out](), true)
	if err != nil {
		panic(fmt.Sprintf("mcpserver: output schema of tool %q: %v", tool.Name, err))
	}
	output, err := outputSchema.Resolve(nil)
	if err != nil {
		panic(fmt.Sprintf("mcpserver: output schema of tool %q: %v", tool.Name, err))
	}
	args := newArguments(inputSchema)

	r.server.AddTool(&mcp.Tool{
		Name:         tool.Name,
		Description:  tool.Description,
		Annotations:  annotations,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in In
		if toolErr := args.decode(req.Params.Arguments, &in); toolErr != nil {
			return errorResult(toolErr), nil
		}
		if toolErr := validateArguments(in); toolErr != nil {
			return errorResult(toolErr), nil
		}
		out, err := handler(ctx, authn.PrincipalFrom(ctx).IdentityID, in)
		if err != nil {
			return errorResult(r.toolError(ctx, tool.Name, err)), nil
		}

		return outputResult(tool.Name, out, output), nil
	})
	r.tools[tool.Name] = true
}

// errorResult returns the tool result that answers err, whose text is "<code>: <message>".
func errorResult(err *ToolError) *mcp.CallToolResult {
	result := &mcp.CallToolResult{}
	result.SetError(err)

	return result
}

// outputResult returns the result of the tool with the given name whose output is out: its JSON, as the
// structured content and as the text content, for clients that read no structured content. Unlike
// encoding/json, which the SDK would encode it with, encoding/json/v2 leaves <, > and & as they are. It
// panics if out can't be encoded or doesn't match its schema, a bug of the tool.
func outputResult(tool string, out any, schema *jsonschema.Resolved) *mcp.CallToolResult {
	encoded, err := json.Marshal(out)
	if err != nil {
		panic(fmt.Errorf("mcpserver: encode the output of tool %q: %w", tool, err))
	}
	var instance any
	if err := json.Unmarshal(encoded, &instance); err != nil {
		panic(fmt.Errorf("mcpserver: decode the output of tool %q: %w", tool, err))
	}
	if err := schema.Validate(instance); err != nil {
		panic(fmt.Errorf("mcpserver: the output of tool %q doesn't match its schema: %w", tool, err))
	}

	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
		StructuredContent: jsontext.Value(encoded),
	}
}

// NewHandler returns the MCP endpoint, which serves the tools of toolsets. It must be served behind
// authn.Require and the shared middleware of router.New, whose Recoverer answers a panic of a tool with a
// 500. Unexpected errors are logged to logger. Every MCP method that a client calls gets a span from
// tracerProvider, the child of the request's server span, and is measured with meterProvider (see
// observe).
//
// Only POST is served: every other method is answered with 405. The body must be application/json and
// not exceed request.MaxBodyBytes, and Accept must name both application/json and text/event-stream.
// There's no localhost protection, which would refuse every request behind a reverse proxy on a loopback
// address, and needs none: every request is authenticated, and a browser can't send another site's API
// key.
//
// It panics if two tools share a name, or if a toolset adds a tool that Registry.Add rejects.
func NewHandler(
	logger *slog.Logger, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider,
	toolsets ...Toolset,
) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: version()}, &mcp.ServerOptions{
		// Tools only, without the list-changed notifications of the SDK's default, which a stateless server
		// can't send, and without logging to the client.
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})
	reg := &Registry{server: server, logger: logger, tools: map[string]bool{}}
	for _, toolset := range toolsets {
		toolset.Register(reg)
	}
	server.AddReceivingMiddleware(observe(logger, meterProvider, tracerProvider, reg.tools))

	return &handler{mcp: mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:                  true,
			JSONResponse:               true,
			DisableLocalhostProtection: true,
			MaxRequestBodyBytes:        request.MaxBodyBytes,
		})}
}

// version returns the version of the main module, as the build recorded it, or "" if it recorded none.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	return info.Main.Version
}
