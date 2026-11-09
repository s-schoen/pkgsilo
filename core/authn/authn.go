// Package authn authenticates requests: it establishes which identity a request acts as.
//
// It knows no authentication method itself. Each method implements Authenticator in its own feature,
// and the application passes all of them to Require.
package authn

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"uuid"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
)

var (
	// ErrNoCredentials means that a request holds no credential of the method, so that the next
	// method is tried.
	ErrNoCredentials = errors.New("no credentials")
	// ErrInvalidCredentials means that a request holds a credential of the method and the method
	// rejects it, so that no other method is tried. A method that knows why wraps one of the reason
	// errors below instead, which are ErrInvalidCredentials too: the metrics and the logs name the
	// reason, and a bare ErrInvalidCredentials as "invalid".
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrMalformedCredentials is an ErrInvalidCredentials for a credential that doesn't have the
	// method's format.
	ErrMalformedCredentials error = &reasonError{reason: reasonMalformed}
	// ErrUnknownCredentials is an ErrInvalidCredentials for a credential that belongs to no one.
	ErrUnknownCredentials error = &reasonError{reason: reasonUnknown}
	// ErrExpiredCredentials is an ErrInvalidCredentials for a credential that has expired.
	ErrExpiredCredentials error = &reasonError{reason: reasonExpired}
)

const (
	reasonMalformed = "malformed"
	reasonUnknown   = "unknown"
	reasonExpired   = "expired"
	// reasonInvalid is the reason of an ErrInvalidCredentials that wraps none of the reason errors.
	reasonInvalid = "invalid"
)

// reasonError is an ErrInvalidCredentials with a reason that the metrics and the logs can name.
type reasonError struct {
	reason string
}

func (e *reasonError) Error() string {
	return e.reason + " credentials"
}

// Is reports that a reasonError is an ErrInvalidCredentials.
func (*reasonError) Is(target error) bool {
	return target == ErrInvalidCredentials
}

// rejectionReason returns the reason of an error that wraps ErrInvalidCredentials.
func rejectionReason(err error) string {
	if reasonErr, ok := errors.AsType[*reasonError](err); ok {
		return reasonErr.reason
	}

	return reasonInvalid
}

// Principal is the caller that a request is authenticated as.
type Principal struct {
	IdentityID uuid.UUID
	// Method is the name of the authentication method that accepted the request.
	Method string
}

// Authenticator checks the credentials of one authentication method on a request.
type Authenticator interface {
	// Method returns the name of the authentication method, for the Principal, the logs and the
	// metrics. Require reads it once, when it's built.
	Method() string
	// Authenticate returns the principal that the request's credential belongs to. Require sets the
	// principal's Method, so that an implementation only sets IdentityID.
	//
	// Telling whether the request holds a credential of the method must be a cheap check of its
	// format: Require asks every method in turn until one recognizes the credential. It returns
	// ErrNoCredentials if the request holds none, an error wrapping ErrInvalidCredentials with the
	// reason if it rejects the credential, and any other error if it can't tell. Errors are logged,
	// so they must not contain the credential.
	Authenticate(r *http.Request) (Principal, error)
	// Challenge returns the method's challenge for the WWW-Authenticate header of a 401 response.
	// Require reads it once, when it's built.
	Challenge() string
}

type principalKey struct{}

// PrincipalFrom returns the principal of a request that passed Require. It panics if there is none:
// a route that needs a principal but isn't behind Require is a wiring bug.
func PrincipalFrom(ctx context.Context) Principal {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	if !ok {
		panic("authn: no principal in the context: the route is not behind Require")
	}

	return principal
}

// Actor returns the identity that a request that passed Require is authenticated as, which its handler
// passes to the service as the actor. A service takes the actor as a parameter and never reads it from
// the context, so that callers without a request, such as the first start, pass their own. It panics
// like PrincipalFrom if there is no principal.
func Actor(r *http.Request) uuid.UUID {
	return PrincipalFrom(r.Context()).IdentityID
}

// Require returns middleware that passes on only requests with a valid credential of one of the
// given authentication methods, with the principal for PrincipalFrom in the context and for the logs
// of the request. It panics if there are no methods.
//
// The methods are tried in the given order, and the first that recognizes the credential decides.
// Requests with a missing or rejected credential get the same 401, which never says why and
// challenges for every method. Rejections are logged at info level with their reason (see
// ErrInvalidCredentials) and error. Any other error of a method is logged at error level and answered
// with a 500. Every request is counted with meterProvider in pkgsilo.authn.attempts, by outcome, method
// and reason.
//
// The methods run in the span pkgsilo.authn.authenticate from tracerProvider, so that their queries nest
// under it. It has the attributes of pkgsilo.authn.attempts, and its status is Error only when a method
// fails. A request that passes sets enduser.id to its identity's ID on the span that its context held
// before, the request's server span.
func Require(
	logger *slog.Logger, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider,
	authenticators ...Authenticator,
) func(http.Handler) http.Handler {
	if len(authenticators) == 0 {
		panic("authn: Require needs at least one authenticator")
	}
	methods := make([]string, len(authenticators))
	challenges := make([]string, len(authenticators))
	for i, authenticator := range authenticators {
		methods[i] = authenticator.Method()
		challenges[i] = authenticator.Challenge()
	}
	base := requireHandler{
		logger:         logger,
		authenticators: authenticators,
		methods:        methods,
		challenges:     challenges,
		attempts:       newAttempts(meterProvider, methods),
		tracer:         tracerProvider.Tracer(tracerName),
	}

	return func(next http.Handler) http.Handler {
		h := base
		h.next = next

		return h
	}
}

// requireHandler is the middleware of Require in front of next.
type requireHandler struct {
	logger         *slog.Logger
	authenticators []Authenticator
	// methods and challenges hold what each authenticator returned, at its index.
	methods    []string
	challenges []string
	attempts   attempts
	tracer     trace.Tracer
	next       http.Handler
}

func (h requireHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Each outcome ends the span before it responds, so that the span covers the authentication only.
	spanCtx, span := h.tracer.Start(ctx, authenticateSpanName)
	spanReq := r.WithContext(spanCtx)

	for i, authenticator := range h.authenticators {
		principal, err := authenticator.Authenticate(spanReq)

		switch {
		case err == nil:
			principal.Method = h.methods[i]
			h.attempts.succeeded(ctx, span, i)
			span.End()
			trace.SpanFromContext(ctx).SetAttributes(semconv.EnduserID(principal.IdentityID.String()))
			middleware.AddLogAttrs(ctx,
				slog.String("identity_id", principal.IdentityID.String()),
				slog.String("auth_method", principal.Method),
			)
			h.next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, principalKey{}, principal)))

			return
		case errors.Is(err, ErrNoCredentials):
			continue
		case errors.Is(err, ErrInvalidCredentials):
			reason := rejectionReason(err)
			h.attempts.rejected(ctx, span, i, reason)
			span.End()
			h.logger.InfoContext(ctx, "authentication rejected",
				"auth_method", h.methods[i], "reason", reason, "error", err)
			writeUnauthorized(w, h.challenges)

			return
		default:
			h.attempts.failed(ctx, span, i)
			span.SetStatus(codes.Error, "authentication failed")
			span.End()
			// Logged with the request's context, so that the error reaches its server span.
			h.logger.ErrorContext(ctx, "authentication failed", "auth_method", h.methods[i], "error", err)
			envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))

			return
		}
	}

	h.attempts.missing(ctx, span)
	span.End()
	writeUnauthorized(w, h.challenges)
}

// writeUnauthorized answers a request without a valid credential.
func writeUnauthorized(w http.ResponseWriter, challenges []string) {
	for _, challenge := range challenges {
		w.Header().Add("WWW-Authenticate", challenge)
	}
	envelope.WriteProblem(w, envelope.UnauthorizedProblem.New(envelope.Occurrence{}))
}
