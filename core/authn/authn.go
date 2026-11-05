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

	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
)

var (
	// ErrNoCredentials means that a request holds no credential of the method, so that the next
	// method is tried.
	ErrNoCredentials = errors.New("no credentials")
	// ErrInvalidCredentials means that a request holds a credential of the method and the method
	// rejects it, so that no other method is tried.
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// Principal is the caller that a request is authenticated as.
type Principal struct {
	IdentityID uuid.UUID
	// Method is the name of the authentication method that accepted the request.
	Method string
}

// Authenticator checks the credentials of one authentication method on a request.
type Authenticator interface {
	// Method returns the name of the authentication method, for the Principal and the logs.
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

// Require returns middleware that passes on only requests with a valid credential of one of the
// given authentication methods, with the principal for PrincipalFrom in the context and for the logs
// of the request. It panics if there are no methods.
//
// The methods are tried in the given order, and the first that recognizes the credential decides.
// Requests with a missing or rejected credential get the same 401, which never says why and
// challenges for every method. Rejections are logged at info level with the method's reason. Any
// other error of a method is logged at error level and answered with a 500.
func Require(logger *slog.Logger, authenticators ...Authenticator) func(http.Handler) http.Handler {
	if len(authenticators) == 0 {
		panic("authn: Require needs at least one authenticator")
	}

	challenges := make([]string, len(authenticators))
	for i, authenticator := range authenticators {
		challenges[i] = authenticator.Challenge()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, authenticator := range authenticators {
				principal, err := authenticator.Authenticate(r)

				switch {
				case err == nil:
					principal.Method = authenticator.Method()
					middleware.AddLogAttrs(r.Context(),
						slog.String("identity_id", principal.IdentityID.String()),
						slog.String("auth_method", principal.Method),
					)
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))

					return
				case errors.Is(err, ErrNoCredentials):
					continue
				case errors.Is(err, ErrInvalidCredentials):
					logger.InfoContext(r.Context(), "authentication rejected",
						"auth_method", authenticator.Method(), "reason", err)
					writeUnauthorized(w, challenges)

					return
				default:
					logger.ErrorContext(r.Context(), "authentication failed",
						"auth_method", authenticator.Method(), "error", err)
					envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))

					return
				}
			}

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
