package mcpserver_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/mcpserver"
)

// TestObserve checks the span and the duration metric of each kind of outcome of a tool call.
func TestObserve(t *testing.T) {
	t.Parallel()
	notFound := mcpserver.DefineError("thing_not_found", "The thing does not exist.")
	s := serve(t, func(reg *mcpserver.Registry) {
		echoTool(reg)
		reg.Add(mcpserver.Tool{Name: "test_missing", Description: "Fails.", Kind: mcpserver.Read}, failing(notFound))
		reg.Add(mcpserver.Tool{Name: "test_fail", Description: "Fails.", Kind: mcpserver.Read},
			failing(errors.New("connection refused")))
		reg.Add(mcpserver.Tool{Name: "test_panic", Description: "Panics.", Kind: mcpserver.Read},
			func(context.Context, uuid.UUID, noInput) (mcpserver.DoneOutput, error) { panic("boom") })
	})
	call := func(tool string) map[string]string {
		return map[string]string{
			"mcp.method.name": "tools/call", "gen_ai.tool.name": tool, "gen_ai.operation.name": "execute_tool",
		}
	}
	with := func(attrs map[string]string, key, value string) map[string]string {
		attrs = maps.Clone(attrs)
		attrs[key] = value
		return attrs
	}
	discover := map[string]string{"mcp.method.name": "server/discover"}

	s.mcp.Call("test_echo", `{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4","name":"ada"}`)
	s.mcp.Call("test_echo", `{}`)
	s.mcp.Call("test_missing", "")
	s.mcp.Call("test_fail", "")
	s.mcp.Call("test_nope", "")
	s.post(t, "application/json", "", callBody("test_panic"))

	tests := []struct {
		span   string
		want   map[string]string
		status sdktrace.Status
	}{
		{span: "server/discover", want: with(discover, "jsonrpc.request.id", "1")},
		{span: "tools/call test_echo", want: with(call("test_echo"), "jsonrpc.request.id", "2")},
		{
			span:   "tools/call test_echo",
			want:   with(with(call("test_echo"), "jsonrpc.request.id", "3"), "error.type", "tool_error"),
			status: sdktrace.Status{Code: codes.Error, Description: "tool_error"},
		},
		{
			span:   "tools/call test_missing",
			want:   with(with(call("test_missing"), "jsonrpc.request.id", "4"), "error.type", "thing_not_found"),
			status: sdktrace.Status{Code: codes.Error, Description: "thing_not_found"},
		},
		{
			span:   "tools/call test_fail",
			want:   with(with(call("test_fail"), "jsonrpc.request.id", "5"), "error.type", "internal_error"),
			status: sdktrace.Status{Code: codes.Error, Description: "connection refused"},
		},
		{
			span:   "tools/call _OTHER",
			want:   with(with(call("_OTHER"), "jsonrpc.request.id", "6"), "error.type", "-32602"),
			status: sdktrace.Status{Code: codes.Error, Description: "-32602"},
		},
		{
			span:   "tools/call test_panic",
			want:   with(with(call("test_panic"), "jsonrpc.request.id", "7"), "error.type", "internal_error"),
			status: sdktrace.Status{Code: codes.Error, Description: "panic: boom"},
		},
	}

	spans := map[string]sdktrace.ReadOnlySpan{}
	parents := map[trace.SpanID]bool{}
	for _, span := range s.telemetry.Spans() {
		if span.Name() == "parent" {
			parents[span.SpanContext().SpanID()] = true
			continue
		}
		spans[span.Name()+" "+stringAttrs(span.Attributes())["jsonrpc.request.id"]] = span
	}
	require.Len(t, spans, len(tests))
	wantPoints := map[attribute.Distinct]uint64{}
	for _, tt := range tests {
		span, ok := spans[tt.span+" "+tt.want["jsonrpc.request.id"]]
		if !assert.Truef(t, ok, "no span %s for request %s", tt.span, tt.want["jsonrpc.request.id"]) {
			continue
		}
		assert.Equal(t, tt.want, stringAttrs(span.Attributes()), tt.span)
		assert.Equal(t, tt.status, span.Status(), tt.span)
		assert.Equal(t, trace.SpanKindServer, span.SpanKind(), tt.span)
		assert.True(t, parents[span.Parent().SpanID()], "%s is the child of the request's span", tt.span)

		metricAttrs := maps.Clone(tt.want)
		delete(metricAttrs, "jsonrpc.request.id")
		set := toSet(metricAttrs)
		wantPoints[set.Equivalent()]++
	}

	gotPoints := map[attribute.Distinct]uint64{}
	for _, point := range s.telemetry.HistogramPoints[float64](t, "mcp.server.operation.duration") {
		gotPoints[point.Attributes.Equivalent()] = point.Count
		assert.Positive(t, point.Sum)
	}
	assert.Equal(t, wantPoints, gotPoints)
}

// stringAttrs returns attrs as a map from their keys to their values, as strings.
func stringAttrs(attrs []attribute.KeyValue) map[string]string {
	m := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		m[string(attr.Key)] = attr.Value.String()
	}

	return m
}

// toSet returns the attribute set of the strings in m.
func toSet(m map[string]string) attribute.Set {
	var kvs []attribute.KeyValue
	for key, value := range m {
		kvs = append(kvs, attribute.String(key, value))
	}

	return attribute.NewSet(kvs...)
}
