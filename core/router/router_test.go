package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
	"pkgsilo/test/loggertest"
	"pkgsilo/test/telemetrytest"
)

func TestNew_MiddlewareAppliesToUnmatchedRoutes(t *testing.T) {
	t.Parallel()
	r := New(Options{}, loggertest.Discard(), noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	// chi only builds the middleware chain once at least one route exists.
	r.Get("/", func(http.ResponseWriter, *http.Request) {})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/does-not-exist", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(middleware.RequestIDHeader))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404}`, rec.Body.String())
}

func TestNew_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	r := New(Options{}, loggertest.Discard(), noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	nothing := func(http.ResponseWriter, *http.Request) {}
	r.Get("/root", nothing)
	r.Delete("/root", nothing)
	// Mounted after New, so the subrouter inherits the 404 and 405 handlers.
	sub := chi.NewRouter()
	sub.Put("/", nothing)
	sub.Get("/item", nothing)
	r.Mount("/sub", sub)
	r.Mount("/param/{id}", sub)
	r.Route("/nested", func(n chi.Router) { n.Mount("/sub", sub) })

	tests := []struct {
		path      string
		wantAllow []string
	}{
		{path: "/root", wantAllow: []string{http.MethodGet, http.MethodDelete}},
		{path: "/sub", wantAllow: []string{http.MethodPut}},
		{path: "/sub/", wantAllow: []string{http.MethodPut}},
		{path: "/sub/item", wantAllow: []string{http.MethodGet}},
		{path: "/param/42", wantAllow: []string{http.MethodPut}},
		{path: "/nested/sub", wantAllow: []string{http.MethodPut}},
		{path: "/nested/sub/", wantAllow: []string{http.MethodPut}},
		{path: "/nested/sub/item", wantAllow: []string{http.MethodGet}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tt.path, nil))

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
			assert.Equal(t, tt.wantAllow, rec.Header().Values("Allow"))
			assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
			assert.JSONEq(t, `{"type":"about:blank","title":"Method Not Allowed","status":405}`, rec.Body.String())
		})
	}
}

func TestNew_MountedSubrouterNotFound(t *testing.T) {
	t.Parallel()
	r := New(Options{}, loggertest.Discard(), noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	sub := chi.NewRouter()
	sub.Get("/", func(http.ResponseWriter, *http.Request) {})
	r.Mount("/sub", sub)
	r.Route("/nested", func(n chi.Router) { n.Mount("/sub", sub) })

	for _, path := range []string{"/sub/missing", "/nested/sub/missing"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404}`, rec.Body.String())
		})
	}
}

func TestNew_LogsClientIPFromTrustedProxy(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	r := New(Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}, logger,
		noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	r.Get("/", func(http.ResponseWriter, *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	r.ServeHTTP(httptest.NewRecorder(), req)

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "203.0.113.7", entries[0]["client_ip"])
}

func TestNew_RecoveredPanicIsLoggedAs500(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	r := New(Options{}, logger, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("boom") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"type":"about:blank","title":"Internal Server Error","status":500}`, rec.Body.String())

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 2)
	panicEntry, requestEntry := entries[0], entries[1]

	assert.Equal(t, "panic recovered", panicEntry["msg"])
	assert.Equal(t, "request", requestEntry["msg"])
	assert.EqualValues(t, http.StatusInternalServerError, requestEntry["status"])
	assert.Equal(t, "ERROR", requestEntry["level"])
	assert.Equal(t, requestEntry["request_id"], panicEntry["request_id"])
}

// TestNew_RecoveredPanicIsOneExceptionOfServerSpan checks that a panic reaches the server span once, through
// the Recoverer's record, and not again through the error-level record of the 500 that RequestLogger writes.
func TestNew_RecoveredPanicIsOneExceptionOfServerSpan(t *testing.T) {
	t.Parallel()
	tel := telemetrytest.New(t)
	logger, buf := loggertest.NewJSON()
	r := New(Options{}, logger, tel.MeterProvider(), tel.TracerProvider())
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("boom") })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil))

	spans := tel.Spans()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status().Code)
	events := spans[0].Events()
	require.Len(t, events, 1)
	assert.Equal(t, semconv.ExceptionEventName, events[0].Name)
	attrs := attribute.NewSet(events[0].Attributes...)
	message, _ := attrs.Value(semconv.ExceptionMessageKey)
	assert.Equal(t, "boom", message.AsString())

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 2)
	for _, entry := range entries {
		assert.Equal(t, spans[0].SpanContext().TraceID().String(), entry["trace_id"])
		assert.Equal(t, spans[0].SpanContext().SpanID().String(), entry["span_id"])
	}
}

func TestNew_EncodingFailureIsLoggedWithRequestID(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	r := New(Options{}, logger, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	r.Get("/unencodable", func(w http.ResponseWriter, _ *http.Request) {
		envelope.WriteResource(w, make(chan int))
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/unencodable", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"type":"about:blank","title":"Internal Server Error","status":500}`, rec.Body.String())

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 2)
	panicEntry, requestEntry := entries[0], entries[1]

	assert.Equal(t, "panic recovered", panicEntry["msg"])
	assert.Contains(t, panicEntry["panic"], "envelope: encode response")
	assert.NotEmpty(t, panicEntry["request_id"])
	assert.Equal(t, requestEntry["request_id"], panicEntry["request_id"])
}

// TestNew_RecordsHTTPServerTelemetry checks the metrics and the server span of each request.
func TestNew_RecordsHTTPServerTelemetry(t *testing.T) {
	t.Parallel()
	sub := chi.NewRouter()
	sub.Get("/{itemID}", func(http.ResponseWriter, *http.Request) {})
	sub.Get("/missing", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	// Answers every request with a 401 before the subrouter mounted behind it routes it, as authn.Require does.
	unauthorized := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantRoute  string
	}{
		{
			name: "nested mounted route", method: http.MethodGet, path: "/api/v1/things/7/items/42",
			wantStatus: http.StatusOK, wantRoute: "/api/v1/things/{thingID}/items/{itemID}",
		},
		{
			name: "404 of a matched route", method: http.MethodGet, path: "/api/v1/things/7/items/missing",
			wantStatus: http.StatusNotFound, wantRoute: "/api/v1/things/{thingID}/items/missing",
		},
		{name: "no route", method: http.MethodGet, path: "/does-not-exist", wantStatus: http.StatusNotFound},
		{
			name: "no route in a subrouter", method: http.MethodGet, path: "/api/v1/things/7/items/42/more",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "no route at the path of a mount", method: http.MethodGet, path: "/api/v1/things/7/items",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "answered before a mounted subrouter", method: http.MethodGet, path: "/api/v1/guarded/7/items/42",
			wantStatus: http.StatusUnauthorized, wantRoute: "/api/v1/guarded/{thingID}/items/{itemID}",
		},
		{
			name: "answered before a mounted subrouter at its path", method: http.MethodGet,
			path: "/api/v1/guarded/7/items", wantStatus: http.StatusUnauthorized,
			wantRoute: "/api/v1/guarded/{thingID}/items",
		},
		{
			name: "answered before a mounted subrouter without a route", method: http.MethodGet,
			path: "/api/v1/guarded/7/items/42/more", wantStatus: http.StatusUnauthorized,
		},
		{
			name: "wildcard route", method: http.MethodGet, path: "/files/a/b",
			wantStatus: http.StatusOK, wantRoute: "/files/*",
		},
		{
			name: "method not allowed", method: http.MethodPost, path: "/api/v1/things/7/items/42",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name: "unknown method", method: "BREW", path: "/api/v1/things/7/items/42",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name: "panic", method: http.MethodGet, path: "/panic",
			wantStatus: http.StatusInternalServerError, wantRoute: "/panic",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tel := telemetrytest.New(t)
			r := New(Options{}, loggertest.Discard(), tel.MeterProvider(), tel.TracerProvider())
			r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("boom") })
			r.Get("/files/*", func(http.ResponseWriter, *http.Request) {})
			r.Route("/api/v1", func(r chi.Router) {
				r.Mount("/things/{thingID}/items", sub)
				r.With(unauthorized).Mount("/guarded/{thingID}/items", sub)
			})

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil))
			require.Equal(t, tt.wantStatus, rec.Code)

			wantMethod, wantSpanName := tt.method, tt.method
			if tt.method == "BREW" {
				wantMethod, wantSpanName = "_OTHER", "HTTP"
			}
			want := []attribute.KeyValue{
				semconv.HTTPRequestMethodKey.String(wantMethod),
				semconv.HTTPResponseStatusCode(tt.wantStatus),
				semconv.NetworkProtocolName("http"),
				semconv.NetworkProtocolVersion("1.1"),
				semconv.URLScheme("http"),
			}
			if tt.wantRoute != "" {
				want = append(want, semconv.HTTPRoute(tt.wantRoute))
				wantSpanName += " " + tt.wantRoute
			}
			duration := telemetrytest.HistogramPoints[float64](t, tel, "http.server.request.duration")
			require.Len(t, duration, 1)
			assert.ElementsMatch(t, want, duration[0].Attributes.ToSlice())
			assert.EqualValues(t, 1, duration[0].Count)
			for _, name := range []string{"http.server.request.body.size", "http.server.response.body.size"} {
				points := telemetrytest.HistogramPoints[int64](t, tel, name)
				require.Len(t, points, 1, name)
				assert.ElementsMatch(t, want, points[0].Attributes.ToSlice(), name)
				assert.EqualValues(t, 1, points[0].Count, name)
			}

			spans := tel.Spans()
			require.Len(t, spans, 1)
			span := spans[0]
			assert.Equal(t, wantSpanName, span.Name())
			assert.Equal(t, trace.SpanKindServer, span.SpanKind())
			attrs := attribute.NewSet(span.Attributes()...)
			for _, kv := range want[:2] {
				got, ok := attrs.Value(kv.Key)
				if assert.True(t, ok, kv.Key) {
					assert.Equal(t, kv.Value, got, kv.Key)
				}
			}
			route, ok := attrs.Value(semconv.HTTPRouteKey)
			assert.Equal(t, tt.wantRoute != "", ok)
			assert.Equal(t, tt.wantRoute, route.AsString())
			wantCode := codes.Unset
			if tt.wantStatus >= http.StatusInternalServerError {
				wantCode = codes.Error
			}
			assert.Equal(t, wantCode, span.Status().Code)
		})
	}
}

func TestNew_RecordsBodySizes(t *testing.T) {
	t.Parallel()
	metrics := telemetrytest.New(t)
	r := New(Options{}, loggertest.Discard(), metrics.MeterProvider(), tracenoop.NewTracerProvider())
	r.Post("/echo", func(w http.ResponseWriter, req *http.Request) { _, _ = io.Copy(w, req.Body) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/echo", strings.NewReader("hello")))
	require.Equal(t, http.StatusOK, rec.Code)

	for _, name := range []string{"http.server.request.body.size", "http.server.response.body.size"} {
		points := telemetrytest.HistogramPoints[int64](t, metrics, name)
		require.Len(t, points, 1, name)
		assert.EqualValues(t, 5, points[0].Sum, name)
	}
}

func TestNew_SkipsUnmeasuredPaths(t *testing.T) {
	t.Parallel()
	health := chi.NewRouter()
	health.Get("/", func(http.ResponseWriter, *http.Request) {})

	tests := []struct {
		path         string
		wantStatus   int
		wantMeasured bool
	}{
		{path: "/api/v1/health", wantStatus: http.StatusOK},
		{path: "/api/v1/health/", wantStatus: http.StatusOK},
		// Decoded, the path is unmeasured, but chi routes it by its escaped form, to no route.
		{path: "/api/v1/%68ealth", wantStatus: http.StatusNotFound, wantMeasured: true},
		{path: "/api/v1/health/more", wantStatus: http.StatusNotFound, wantMeasured: true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			tel := telemetrytest.New(t)
			r := New(Options{UnmeasuredPaths: []string{"/api/v1/health"}}, loggertest.Discard(), tel.MeterProvider(),
				tel.TracerProvider())
			r.Mount("/api/v1/health", health)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))
			require.Equal(t, tt.wantStatus, rec.Code)

			_, measured := tel.Metric(t, "http.server.request.duration")
			assert.Equal(t, tt.wantMeasured, measured)
			assert.Equal(t, tt.wantMeasured, len(tel.Spans()) == 1, "span recorded")
		})
	}
}

// TestNew_ContinuesTracesOfTrustedProxiesOnly checks that the server span continues the trace of a
// traceparent sent by a trusted proxy, and starts a new trace linked to the sent span otherwise.
func TestNew_ContinuesTracesOfTrustedProxiesOnly(t *testing.T) {
	t.Parallel()
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	sent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: true,
	})

	tests := map[string]struct {
		remoteAddr  string
		wantTrusted bool
	}{
		"untrusted peer": {remoteAddr: "198.51.100.1:1234"},
		"trusted peer":   {remoteAddr: "10.0.0.1:1234", wantTrusted: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tel := telemetrytest.New(t)
			r := New(Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}},
				loggertest.Discard(), tel.MeterProvider(), tel.TracerProvider())
			var gotBaggage baggage.Baggage
			r.Get("/", func(_ http.ResponseWriter, r *http.Request) { gotBaggage = baggage.FromContext(r.Context()) })

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("Traceparent", "00-"+sent.TraceID().String()+"-"+sent.SpanID().String()+"-01")
			req.Header.Set("Baggage", "tenant=spoofed")
			r.ServeHTTP(httptest.NewRecorder(), req)

			assert.Zero(t, gotBaggage.Len(), "baggage isn't propagated")
			spans := tel.Spans()
			require.Len(t, spans, 1)
			span := spans[0]
			if tt.wantTrusted {
				assert.Equal(t, sent.TraceID(), span.SpanContext().TraceID())
				assert.True(t, span.Parent().Equal(sent), "parent is the sent span")
				assert.Empty(t, span.Links())
			} else {
				assert.NotEqual(t, sent.TraceID(), span.SpanContext().TraceID())
				assert.False(t, span.Parent().IsValid(), "no parent")
				require.Len(t, span.Links(), 1)
				assert.True(t, span.Links()[0].SpanContext.Equal(sent), "links to the sent span")
			}
		})
	}
}

// TestNew_SpanClientAddressIgnoresUntrustedForwardedFor checks that a client that isn't a trusted proxy
// can't set the span's client.address with X-Forwarded-For, and that a trusted proxy can.
func TestNew_SpanClientAddressIgnoresUntrustedForwardedFor(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		remoteAddr  string
		wantAddress string
	}{
		"untrusted peer": {remoteAddr: "198.51.100.1:1234", wantAddress: "198.51.100.1"},
		"trusted peer":   {remoteAddr: "10.0.0.1:1234", wantAddress: "203.0.113.7"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tel := telemetrytest.New(t)
			r := New(Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}},
				loggertest.Discard(), tel.MeterProvider(), tel.TracerProvider())
			r.Get("/", func(http.ResponseWriter, *http.Request) {})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Forwarded-For", "203.0.113.7")
			r.ServeHTTP(httptest.NewRecorder(), req)

			spans := tel.Spans()
			require.Len(t, spans, 1)
			attrs := attribute.NewSet(spans[0].Attributes()...)
			address, ok := attrs.Value(semconv.ClientAddressKey)
			require.True(t, ok, "client.address is set")
			assert.Equal(t, tt.wantAddress, address.AsString())
		})
	}
}

func TestNew_SpanHoldsRequestID(t *testing.T) {
	t.Parallel()
	tel := telemetrytest.New(t)
	r := New(Options{}, loggertest.Discard(), tel.MeterProvider(), tel.TracerProvider())
	r.Get("/", func(http.ResponseWriter, *http.Request) {})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	spans := tel.Spans()
	require.Len(t, spans, 1)
	attrs := attribute.NewSet(spans[0].Attributes()...)
	id, ok := attrs.Value("pkgsilo.request.id")
	require.True(t, ok, "pkgsilo.request.id is set")
	assert.NotEmpty(t, id.AsString())
	assert.Equal(t, rec.Header().Get(middleware.RequestIDHeader), id.AsString())
}

func TestNew_PanicsWithoutMeterProvider(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "router: New needs a meter provider", func() {
		New(Options{}, loggertest.Discard(), nil, tracenoop.NewTracerProvider())
	})
}

func TestNew_PanicsWithoutTracerProvider(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "router: New needs a tracer provider", func() {
		New(Options{}, loggertest.Discard(), noop.NewMeterProvider(), nil)
	})
}
