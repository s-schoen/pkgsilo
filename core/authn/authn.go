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

	"go.opentelemetry.io/otel/metric"

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
func Require(
	logger *slog.Logger, meterProvider metric.MeterProvider, authenticators ...Authenticator,
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
	attempts := newAttempts(meterProvider, methods)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for i, authenticator := range authenticators {
				principal, err := authenticator.Authenticate(r)

				switch {
				case err == nil:
					principal.Method = methods[i]
					attempts.succeeded(r.Context(), i)
					middleware.AddLogAttrs(r.Context(),
						slog.String("identity_id", principal.IdentityID.String()),
						slog.String("auth_method", principal.Method),
					)
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))

					return
				case errors.Is(err, ErrNoCredentials):
					continue
				case errors.Is(err, ErrInvalidCredentials):
					reason := rejectionReason(err)
					attempts.rejected(r.Context(), i, reason)
					logger.InfoContext(r.Context(), "authentication rejected",
						"auth_method", methods[i], "reason", reason, "error", err)
					writeUnauthorized(w, challenges)

					return
				default:
					attempts.failed(r.Context(), i)
					logger.ErrorContext(r.Context(), "authentication failed",
						"auth_method", methods[i], "error", err)
					envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))

					return
				}
			}

			attempts.missing(r.Context())
			writeUnauthorized(w, challenges)
		})
	}
}

// writeUnauthorized answers a request without a valid credential.
func writeUnauthorized(w http.ResponseWriter, challenges []string) {
	for _, challenge := range challenges {
		w.Header().Add("WWW-Authenticate", challenge)
	}
	envelope.WriteProblem(w, envelope.UnauthorizedProblem.New(envelope.Occurrence{}))
}
