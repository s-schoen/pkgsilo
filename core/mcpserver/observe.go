package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/middleware"
)

const (
	// scopeName is the instrumentation scope of the MCP span and metric.
	scopeName = "pkgsilo/core/mcpserver"
	// durationName is the duration of the MCP methods that the server handles, of the MCP semantic
	// conventions (semantic-conventions-genai, docs/gen-ai/mcp.md).
	durationName = "mcp.server.operation.duration"

	// methodNameKey, toolNameKey and operationNameKey are the attributes of the MCP semantic conventions
	// that semconv has no constants for yet.
	methodNameKey    = attribute.Key("mcp.method.name")
	toolNameKey      = attribute.Key("gen_ai.tool.name")
	operationNameKey = attribute.Key("gen_ai.operation.name")

	// operationExecuteTool is the gen_ai.operation.name of tools/call.
	operationExecuteTool = "execute_tool"
	// methodCallTool is the MCP method that calls a tool.
	methodCallTool = "tools/call"
	// other replaces a method or tool name that the server doesn't know, so that the attributes stay
	// bounded whatever a client sends.
	other = "_OTHER"
	// errorTypeTool is the error.type of a tool result that is an error without a ToolError, such as the
	// SDK's answer to arguments that don't match the input schema.
	errorTypeTool = "tool_error"
)

// durationBuckets are the bucket boundaries of mcp.server.operation.duration that the semantic conventions
// recommend, in seconds.
var durationBuckets = []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30, 60, 120, 300}

// methods are the methods that a client may call on an MCP server: those of the MCP specification that
// the SDK handles. Any other method is recorded as _OTHER.
var methods = map[string]bool{
	"completion/complete": true, "server/discover": true, "initialize": true, "ping": true,
	"prompts/list": true, "prompts/get": true, "tools/list": true, methodCallTool: true,
	"resources/list": true, "resources/templates/list": true, "resources/read": true,
	"resources/subscribe": true, "resources/unsubscribe": true, "subscriptions/listen": true,
	"logging/setLevel": true, "notifications/initialized": true,
	"notifications/cancelled":          true, //nolint:misspell // The method's name in the MCP specification.
	"notifications/roots/list_changed": true, "notifications/progress": true,
}

// observer is the receiving middleware of observe.
type observer struct {
	logger   *slog.Logger
	tracer   trace.Tracer
	duration metric.Float64Histogram
	// tools holds the names of the registered tools.
	tools map[string]bool
}

// observe returns the receiving middleware that every MCP method that a client calls goes through. It
// gives the method a span from tracerProvider, the child of the request's server span, named
// "{mcp.method.name} {gen_ai.tool.name}" for tools/call and "{mcp.method.name}" otherwise, and records its
// duration in mcp.server.operation.duration with meterProvider. The span has mcp.method.name,
// gen_ai.tool.name and gen_ai.operation.name for tools/call, jsonrpc.request.id and error.type, and the
// metric those but jsonrpc.request.id. A method that the specification doesn't name and a tool that tools
// doesn't name are recorded as _OTHER.
//
// error.type is absent on success. It's the code of the ToolError of a tool result that is an error,
// tool_error for such a result without one, internal_error for an unexpected error, and the code of a
// JSON-RPC error that the SDK answers with, such as -32601 for an unknown method.
//
// A method's unexpected error, a plain error rather than the *jsonrpc.Error that the SDK answers with
// itself, is logged to logger and replaced by a JSON-RPC error that says only internal error and the
// request ID, so that no internal message reaches the client. A panic of the method is recovered, recorded
// as internal_error and passed on to handler.ServeHTTP, which panics with it on the request's goroutine.
//
// It panics if the instrument can't be created, which only a bug in its definition can cause.
func observe(
	logger *slog.Logger, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider, tools map[string]bool,
) mcp.Middleware {
	duration, err := meterProvider.Meter(scopeName).Float64Histogram(durationName, metric.WithUnit("s"),
		metric.WithDescription("The duration of the MCP requests that the server handled."),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	if err != nil {
		panic(fmt.Sprintf("mcpserver: create %s: %v", durationName, err))
	}
	o := &observer{logger: logger, tracer: tracerProvider.Tracer(scopeName), duration: duration, tools: tools}

	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			return o.handle(ctx, next, method, req)
		}
	}
}

// handle calls the method with next in its span, and records it.
func (o *observer) handle(
	ctx context.Context, next mcp.MethodHandler, method string, req mcp.Request,
) (result mcp.Result, err error) {
	start := time.Now()
	attrs := o.attributes(method, req)
	name := attrs[0].Value.AsString()
	if len(attrs) > 1 {
		name += " " + attrs[1].Value.AsString()
	}
	spanAttrs := attrs
	if id, ok := o.requestID(ctx); ok {
		spanAttrs = append(slices.Clip(spanAttrs), id)
	}
	ctx, span := o.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(spanAttrs...))
	defer span.End()

	defer func() {
		if rec := recover(); rec != nil {
			o.record(ctx, start, span, attrs, internalErrorCode)
			result, err = nil, o.passOn(ctx, span, method, rec)
		}
	}()

	result, err = next(ctx, method, req)
	errorType, err := o.classify(ctx, span, method, result, err)
	o.record(ctx, start, span, attrs, errorType)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// classify returns the error.type of a method that returned result and err, and the error to answer it
// with: err itself, unless it's unexpected, which classify logs with the span's context, so that the span's
// status is Error with the error, and replaces with the JSON-RPC error that says only internal error.
func (o *observer) classify(
	ctx context.Context, span trace.Span, method string, result mcp.Result, err error,
) (errorType string, _ error) {
	wireErr, isWire := errors.AsType[*jsonrpc.Error](err)
	switch {
	case err == nil:
		return resultErrorType(result), nil
	case isWire && wireErr.Code == jsonrpc.CodeInternalError:
		span.SetStatus(codes.Error, wireErr.Message)
		return internalErrorCode, err
	case isWire:
		return strconv.FormatInt(wireErr.Code, 10), err
	default:
		o.logger.ErrorContext(ctx, "mcp method failed", "mcp_method", method, "error", err)
		return internalErrorCode, internalJSONRPCError(middleware.RequestIDFromContext(ctx))
	}
}

// passOn sets the status of the span to Error with the panic rec of the method, and passes it on to
// handler.ServeHTTP, which panics with it on the request's goroutine. It returns the error that the SDK
// answers the method with, which handler.ServeHTTP drops. Without a handler.ServeHTTP to pass it on to, it
// panics again, as without observe.
func (*observer) passOn(ctx context.Context, span trace.Span, method string, rec any) error {
	span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", rec))
	p := newMethodPanic(method, rec)
	c := callFrom(ctx)
	if c == nil {
		panic(p)
	}
	c.panic.Store(p)

	return internalJSONRPCError(middleware.RequestIDFromContext(ctx))
}

// attributes returns the attributes of the method that the span and the metric share: mcp.method.name
// first, then, for tools/call, gen_ai.tool.name and gen_ai.operation.name.
func (o *observer) attributes(method string, req mcp.Request) []attribute.KeyValue {
	if !methods[method] {
		return []attribute.KeyValue{methodNameKey.String(other)}
	}
	if method != methodCallTool {
		return []attribute.KeyValue{methodNameKey.String(method)}
	}

	tool := other
	if callReq, ok := req.(*mcp.CallToolRequest); ok && callReq.Params != nil && o.tools[callReq.Params.Name] {
		tool = callReq.Params.Name
	}

	return []attribute.KeyValue{
		methodNameKey.String(method), toolNameKey.String(tool), operationNameKey.String(operationExecuteTool),
	}
}

// requestID returns jsonrpc.request.id for the JSON-RPC request of the call in ctx, if it has one.
func (*observer) requestID(ctx context.Context) (attribute.KeyValue, bool) {
	c := callFrom(ctx)
	if c == nil {
		return attribute.KeyValue{}, false
	}
	id, ok := c.requestID()
	if !ok {
		return attribute.KeyValue{}, false
	}

	return semconv.JSONRPCRequestID(fmt.Sprint(id.Raw())), true
}

// record records the method's duration since start with attrs and errorType, which is empty on success, and
// sets errorType on span. With an errorType, it sets the span's status to Error with errorType as its
// description, unless that's internal_error: the logger sets the status of a logged internal error to its
// message, which the caller sets otherwise.
func (o *observer) record(
	ctx context.Context, start time.Time, span trace.Span, attrs []attribute.KeyValue, errorType string,
) {
	if errorType != "" {
		errorAttr := semconv.ErrorTypeKey.String(errorType)
		attrs = append(slices.Clip(attrs), errorAttr)
		span.SetAttributes(errorAttr)
		if errorType != internalErrorCode {
			span.SetStatus(codes.Error, errorType)
		}
	}
	o.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
}

// resultErrorType returns the error.type of a successful method's result: that of a tool result that is an
// error (see observe), or "".
func resultErrorType(result mcp.Result) string {
	toolResult, ok := result.(*mcp.CallToolResult)
	if !ok || toolResult == nil || !toolResult.IsError {
		return ""
	}
	if toolErr, ok := errors.AsType[*ToolError](toolResult.GetError()); ok {
		return toolErr.Code
	}

	return errorTypeTool
}
