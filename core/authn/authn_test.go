package authn_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/authn"
	"pkgsilo/core/authn/mocks"
	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
	"pkgsilo/test/loggertest"
	"pkgsilo/test/telemetrytest"
)

const unauthorizedBody = `{"type":"about:blank","title":"Unauthorized","status":401}`

// newAuthenticator returns a mock of the given method, whose challenge is the method's name. Calling
// its Authenticate fails the test unless the test expects it.
func newAuthenticator(t *testing.T, method string) *mocks.MockAuthenticator {
	t.Helper()

	authenticator := mocks.NewMockAuthenticator(t)
	authenticator.EXPECT().Challenge().Return(method)
	authenticator.EXPECT().Method().Return(method).Maybe()

	return authenticator
}

// served is what serve observed of one request.
type served struct {
	rec       *httptest.ResponseRecorder
	principal *authn.Principal
	telemetry *telemetrytest.Recorder
	// parent is the span that the request's context held, as its server span would be.
	parent trace.SpanContext
	// handlerSpan is the span in the context of the handler, if it ran.
	handlerSpan trace.SpanContext
}

// serve sends a request in a parent span through Require to a handler, and returns the response and the
// principal that the handler got, which is nil if it didn't run.
func serve(t *testing.T, logger *slog.Logger, authenticators ...authn.Authenticator) served {
	t.Helper()

	got := served{rec: httptest.NewRecorder(), telemetry: telemetrytest.New(t)}
	h := authn.Require(logger, got.telemetry.MeterProvider(), got.telemetry.TracerProvider(), authenticators...)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := authn.PrincipalFrom(r.Context())
			got.principal = &principal
			got.handlerSpan = trace.SpanFromContext(r.Context()).SpanContext()
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	ctx, parent := got.telemetry.TracerProvider().Tracer("pkgsilo/test").Start(t.Context(), "parent")
	got.parent = parent.SpanContext()
	h.ServeHTTP(got.rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
	parent.End()

	return got
}

// span returns the ended span with the given name, failing the test if there's not exactly one.
func (s served) span(t *testing.T, name string) sdktrace.ReadOnlySpan {
	t.Helper()

	var found []sdktrace.ReadOnlySpan
	for _, span := range s.telemetry.Spans() {
		if span.Name() == name {
			found = append(found, span)
		}
	}
	require.Len(t, found, 1, name)

	return found[0]
}

// authnSpan returns the span pkgsilo.authn.authenticate, a child of the parent span.
func (s served) authnSpan(t *testing.T) sdktrace.ReadOnlySpan {
	t.Helper()

	span := s.span(t, "pkgsilo.authn.authenticate")
	assert.Equal(t, "pkgsilo/core/authn", span.InstrumentationScope().Name)
	assert.Equal(t, s.parent.SpanID(), span.Parent().SpanID(), "the span is a child of the request's span")

	return span
}

// enduserID returns the attribute enduser.id of the parent span, and whether it's set.
func (s served) enduserID(t *testing.T) (string, bool) {
	t.Helper()

	attrs := attribute.NewSet(s.span(t, "parent").Attributes()...)
	value, ok := attrs.Value(semconv.EnduserIDKey)

	return value.AsString(), ok
}

// stringAttrs returns the attributes as strings, by key.
func stringAttrs(kvs []attribute.KeyValue) map[string]string {
	attrs := map[string]string{}
	for _, kv := range kvs {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}

	return attrs
}

// assertAttempts asserts that pkgsilo.authn.attempts counted exactly one request, with the attributes want,
// and that the span pkgsilo.authn.authenticate has the same attributes.
func assertAttempts(t *testing.T, got served, want map[string]string) {
	t.Helper()

	points := telemetrytest.SumPoints[int64](t, got.telemetry, "pkgsilo.authn.attempts")
	require.Len(t, points, 1)
	assert.Equal(t, int64(1), points[0].Value)
	assert.Equal(t, want, stringAttrs(points[0].Attributes.ToSlice()), "the metric's attributes")
	assert.Equal(t, want, stringAttrs(got.authnSpan(t).Attributes()), "the span's attributes")
}

// assertNotPassed asserts that the span pkgsilo.authn.authenticate of a request that didn't pass has the
// status code want, and that the parent span names no end user.
func assertNotPassed(t *testing.T, got served, want codes.Code) {
	t.Helper()

	assert.Equal(t, want, got.authnSpan(t).Status().Code)
	_, ok := got.enduserID(t)
	assert.False(t, ok, "enduser.id is set")
}

func TestRequire_FirstMethodThatRecognizesTheCredentialDecides(t *testing.T) {
	t.Parallel()
	id := uuid.NewV7()
	first := newAuthenticator(t, "first")
	first.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, authn.ErrNoCredentials)
	second := newAuthenticator(t, "second")
	var authenticated trace.SpanContext
	second.EXPECT().Authenticate(mock.Anything).RunAndReturn(func(r *http.Request) (authn.Principal, error) {
		authenticated = trace.SpanFromContext(r.Context()).SpanContext()

		return authn.Principal{IdentityID: id}, nil
	})
	third := newAuthenticator(t, "third")

	got := serve(t, loggertest.Discard(), first, second, third)

	assert.Equal(t, http.StatusNoContent, got.rec.Code)
	require.NotNil(t, got.principal)
	assert.Equal(t, authn.Principal{IdentityID: id, Method: "second"}, *got.principal)
	assertAttempts(t, got, map[string]string{
		"pkgsilo.authn.outcome": "success",
		"pkgsilo.authn.method":  "second",
	})
	span := got.authnSpan(t)
	assert.Equal(t, codes.Unset, span.Status().Code)
	assert.Equal(t, span.SpanContext(), authenticated, "the method runs in the span, so that its queries nest under it")
	assert.Equal(t, got.parent, got.handlerSpan, "the handler runs in the request's span")
	enduserID, ok := got.enduserID(t)
	assert.True(t, ok, "enduser.id is set")
	assert.Equal(t, id.String(), enduserID)
}

func TestRequire_AnswersMissingCredentialsWith401(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	first := newAuthenticator(t, "first")
	first.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, authn.ErrNoCredentials)
	second := newAuthenticator(t, "second")
	second.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, authn.ErrNoCredentials)

	got := serve(t, logger, first, second)

	assert.Equal(t, http.StatusUnauthorized, got.rec.Code)
	assert.Equal(t, envelope.ProblemJSONMediaType, got.rec.Header().Get("Content-Type"))
	assert.JSONEq(t, unauthorizedBody, got.rec.Body.String())
	assert.Equal(t, []string{"first", "second"}, got.rec.Header().Values("WWW-Authenticate"))
	assert.Nil(t, got.principal)
	assert.Empty(t, buf.String())
	assertAttempts(t, got, map[string]string{"pkgsilo.authn.outcome": "missing"})
	assertNotPassed(t, got, codes.Unset)
}

func TestRequire_AnswersRejectedCredentialsWith401(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	first := newAuthenticator(t, "first")
	first.EXPECT().Authenticate(mock.Anything).
		Return(authn.Principal{}, fmt.Errorf("%w: key 1 expired", authn.ErrExpiredCredentials))
	second := newAuthenticator(t, "second")

	got := serve(t, logger, first, second)

	assert.Equal(t, http.StatusUnauthorized, got.rec.Code)
	assert.JSONEq(t, unauthorizedBody, got.rec.Body.String(), "the response doesn't say why")
	assert.Equal(t, []string{"first", "second"}, got.rec.Header().Values("WWW-Authenticate"))
	assert.Nil(t, got.principal)

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "INFO", entries[0]["level"])
	assert.Equal(t, "authentication rejected", entries[0]["msg"])
	assert.Equal(t, "first", entries[0]["auth_method"])
	assert.Equal(t, "expired", entries[0]["reason"])
	assert.Equal(t, "expired credentials: key 1 expired", entries[0]["error"])
	assertAttempts(t, got, map[string]string{
		"pkgsilo.authn.outcome": "rejected",
		"pkgsilo.authn.method":  "first",
		"pkgsilo.authn.reason":  "expired",
	})
	assertNotPassed(t, got, codes.Unset)
}

func TestRequire_NamesTheReasonOfARejection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err    error
		reason string
	}{
		{authn.ErrMalformedCredentials, "malformed"},
		{authn.ErrUnknownCredentials, "unknown"},
		{fmt.Errorf("wrapped: %w", authn.ErrExpiredCredentials), "expired"},
		{authn.ErrInvalidCredentials, "invalid"},
		{fmt.Errorf("%w: revoked", authn.ErrInvalidCredentials), "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			t.Parallel()
			logger, buf := loggertest.NewJSON()
			authenticator := newAuthenticator(t, "key")
			authenticator.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, tt.err)

			got := serve(t, logger, authenticator)

			assert.Equal(t, http.StatusUnauthorized, got.rec.Code)
			entries := loggertest.Entries(t, buf)
			require.Len(t, entries, 1)
			assert.Equal(t, tt.reason, entries[0]["reason"])
			assertAttempts(t, got, map[string]string{
				"pkgsilo.authn.outcome": "rejected",
				"pkgsilo.authn.method":  "key",
				"pkgsilo.authn.reason":  tt.reason,
			})
			assertNotPassed(t, got, codes.Unset)
		})
	}
}

func TestReasonErrors_AreInvalidCredentials(t *testing.T) {
	t.Parallel()

	for _, err := range []error{authn.ErrMalformedCredentials, authn.ErrUnknownCredentials, authn.ErrExpiredCredentials} {
		require.ErrorIs(t, err, authn.ErrInvalidCredentials)
		require.NotErrorIs(t, err, authn.ErrNoCredentials)
	}
	require.NotErrorIs(t, authn.ErrMalformedCredentials, authn.ErrUnknownCredentials)
	require.EqualError(t, authn.ErrMalformedCredentials, "malformed credentials")
	require.EqualError(t, authn.ErrUnknownCredentials, "unknown credentials")
	assert.EqualError(t, authn.ErrExpiredCredentials, "expired credentials")
}

func TestRequire_AnswersOtherErrorsWith500(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	first := newAuthenticator(t, "first")
	first.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, errors.New("database down"))
	second := newAuthenticator(t, "second")

	got := serve(t, logger, first, second)

	assert.Equal(t, http.StatusInternalServerError, got.rec.Code)
	assert.JSONEq(t, `{"type":"about:blank","title":"Internal Server Error","status":500}`, got.rec.Body.String())
	assert.Empty(t, got.rec.Header().Values("WWW-Authenticate"))
	assert.Nil(t, got.principal)

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "ERROR", entries[0]["level"])
	assert.Equal(t, "authentication failed", entries[0]["msg"])
	assert.Equal(t, "first", entries[0]["auth_method"])
	assert.Equal(t, "database down", entries[0]["error"])
	assertAttempts(t, got, map[string]string{
		"pkgsilo.authn.outcome": "error",
		"pkgsilo.authn.method":  "first",
	})
	assertNotPassed(t, got, codes.Error)
	assert.Equal(t, "authentication failed", got.authnSpan(t).Status().Description)
}

func TestRequire_AddsThePrincipalToTheLogsOfTheRequest(t *testing.T) {
	t.Parallel()
	id := uuid.NewV7()
	logger, buf := loggertest.NewJSON()
	authenticator := newAuthenticator(t, "first")
	authenticator.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{IdentityID: id}, nil)

	authenticate := authn.Require(logger, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), authenticator)
	h := middleware.RequestLogger(logger)(authenticate(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			logger.InfoContext(r.Context(), "handled")
		}),
	))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 2)
	for _, entry := range entries {
		assert.Equal(t, id.String(), entry["identity_id"], entry["msg"])
		assert.Equal(t, "first", entry["auth_method"], entry["msg"])
	}
}

func TestRequire_PanicsWithoutAuthenticators(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "authn: Require needs at least one authenticator", func() {
		authn.Require(loggertest.Discard(), noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	})
}

func TestPrincipalFrom_PanicsWithoutPrincipal(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "authn: no principal in the context: the route is not behind Require", func() {
		authn.PrincipalFrom(t.Context())
	})
}

func TestActor_ReturnsThePrincipalsIdentity(t *testing.T) {
	t.Parallel()
	id := uuid.NewV7()
	authenticator := newAuthenticator(t, "key")
	authenticator.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{IdentityID: id}, nil)

	var got uuid.UUID
	h := authn.Require(loggertest.Discard(), noop.NewMeterProvider(), tracenoop.NewTracerProvider(), authenticator)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = authn.Actor(r)
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, id, got)
}

func TestActor_PanicsWithoutPrincipal(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "authn: no principal in the context: the route is not behind Require", func() {
		authn.Actor(httptest.NewRequest(http.MethodGet, "/", nil))
	})
}
