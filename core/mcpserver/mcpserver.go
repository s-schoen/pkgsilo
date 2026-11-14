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
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"runtime/debug"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
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
// and ask before a deletion. It sets the tool's annotations.
type Kind int

// The kinds of tools.
const (
	// Read is a tool that changes nothing, such as one that gets or lists objects.
	Read Kind = iota + 1
	// Write is a tool that creates or changes an object.
	Write
	// Delete is a tool that removes an object, which can't be undone.
	Delete
)

// annotations returns the MCP annotations of a tool of kind k. It panics on an unknown kind.
func (k Kind) annotations() *mcp.ToolAnnotations {
	switch k {
	case Read:
		return &mcp.ToolAnnotations{
			ReadOnlyHint: true, DestructiveHint: new(false), IdempotentHint: true, OpenWorldHint: new(false),
		}
	case Write:
		return &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)}
	case Delete:
		return &mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(false)}
	default:
		panic(fmt.Sprintf("mcpserver: unknown tool kind %d", k))
	}
}

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
}

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
// Members tagged omitempty or omitzero are optional, and all others are required.
//
// For each call, the SDK checks the arguments against the input schema and decodes them, Add validates
// them with In's Validate, which the type constraint requires (a tool without arguments takes an empty
// struct whose Validate returns nil), and then calls handler with the actor of the request, as
// authn.Actor returns it to an HTTP handler, which passes it to the service. Invalid arguments are answered
// with the error invalid_arguments, which names each invalid member and why, and handler isn't called.
//
// handler returns the tool's output, or an error, which Add answers as a tool error: an error of
// DefineError with its code and message, an *authz.FieldsDeniedError with fields_denied, authz.ErrDenied
// with forbidden, and any other error with internal_error and the request ID, after logging it.
//
// Each member of Out but the ID is an envelope.Field[T] tagged omitzero, as in a response of the HTTP API,
// filled with envelope.NewField: a hidden field is absent from the output, and a visible null stays null.
// Its schema is that of T, and it's optional.
//
// Add panics if the tool's name isn't of the form <type>_<verb> or is taken, if its description is empty,
// if its kind is unknown, or if a schema can't be built from In or Out, such as for an envelope.Field
// without omitzero: these are bugs of the feature, found when the server is built.
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
	inputSchema, err := schemaFor(reflect.TypeFor[In](), false)
	if err != nil {
		panic(fmt.Sprintf("mcpserver: input schema of tool %q: %v", tool.Name, err))
	}
	outputSchema, err := schemaFor(reflect.TypeFor[Out](), true)
	if err != nil {
		panic(fmt.Sprintf("mcpserver: output schema of tool %q: %v", tool.Name, err))
	}

	mcp.AddTool(r.server, &mcp.Tool{
		Name:         tool.Name,
		Description:  tool.Description,
		Annotations:  tool.Kind.annotations(),
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var zero Out
		if err := validateArguments(in); err != nil {
			return nil, zero, err
		}
		out, err := handler(ctx, authn.PrincipalFrom(ctx).IdentityID, in)
		if err != nil {
			return nil, zero, r.toolError(ctx, tool.Name, err)
		}

		return nil, out, nil
	})
	r.tools[tool.Name] = true
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
