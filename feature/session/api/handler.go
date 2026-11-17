// Package api serves the session endpoints of the HTTP API, which log browsers in with a password, and
// authenticates requests by their session cookie.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path"
	"uuid"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
	"pkgsilo/core/request"
	"pkgsilo/feature/session/service"
)

// currentPattern is the pattern of the route of the request's own session.
const currentPattern = "/current"

// Handler serves the session endpoints.
//
// None of them checks a policy, which is a deliberate exception to denying what no policy permits. A login is
// authentication, like presenting an API key, so there's no actor to check yet. The current session is the
// one that the request is authenticated with, so reading it reveals nothing that the caller doesn't hold
// already, and ending it must always work: a policy must never block a logout.
type Handler struct {
	service service.SessionService
	logins  *authn.LoginRecorder
	logger  *slog.Logger
}

// NewHandler returns a Handler that logs in and manages sessions through svc, records logins with logins and
// logs to logger.
func NewHandler(svc service.SessionService, logins *authn.LoginRecorder, logger *slog.Logger) *Handler {
	return &Handler{service: svc, logins: logins, logger: logger}
}

// Routes returns the session routes: the login, which is public, and the routes of the current session,
// behind authenticate, which must accept the requests of an Authenticator.
func (h *Handler) Routes(authenticate func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.login)
	r.With(authenticate).Get(currentPattern, h.getCurrent)
	r.With(authenticate).Delete(currentPattern, h.deleteCurrent)

	return r
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body LoginRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	var presented string
	if cookie, err := r.Cookie(cookieName); err == nil {
		presented = cookie.Value
	}
	in := body.toInput(presented, middleware.ClientIPFromContext(r.Context()), r.UserAgent())

	var created service.CreatedSession
	err := h.logins.Record(r.Context(), func(ctx context.Context) error {
		var err error
		created, err = h.service.Login(ctx, in)

		return toAuthnError(err)
	})
	if rejected, ok := errors.AsType[*service.RejectedError](err); ok {
		h.logRejected(r.Context(), rejected)
		envelope.WriteProblem(w, unauthorizedProblem)

		return
	}
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	middleware.AddLogAttrs(r.Context(),
		slog.String("identity_id", created.IdentityID.String()),
		slog.String("session_id", created.ID.String()),
	)
	if created.RehashErr != nil {
		// A warning: the login succeeded, and the next one tries again.
		h.logger.WarnContext(r.Context(), "password rehash failed", "error", created.RehashErr)
	}
	setSessionCookie(w, created.Token)
	envelope.WriteCreated(w, path.Join(r.URL.Path, currentPattern), toSessionResponse(created.Session))
}

// logRejected logs a rejected login with its reason, and with its identity if one has the username. It never
// logs the username, which may be a password typed into the wrong field, or the password.
func (h *Handler) logRejected(ctx context.Context, rejected *service.RejectedError) {
	reason := "invalid"
	if errors.Is(rejected.Reason, service.ErrUnknownCredentials) {
		reason = "unknown"
	}
	attrs := []any{"auth_method", "password", "reason", reason}
	if rejected.IdentityID != uuid.Nil() {
		attrs = append(attrs, "identity_id", rejected.IdentityID.String())
	}
	h.logger.InfoContext(ctx, "login rejected", attrs...)
}

func (h *Handler) getCurrent(w http.ResponseWriter, r *http.Request) {
	id, ok := currentSession(w, r)
	if !ok {
		return
	}

	session, err := h.service.Get(r.Context(), authn.Actor(r), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toSessionResponse(session))
}

func (h *Handler) deleteCurrent(w http.ResponseWriter, r *http.Request) {
	id, ok := currentSession(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), authn.Actor(r), id); err != nil {
		h.writeError(w, r, err)
		return
	}

	expireSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// currentSession returns the ID of the session that the request is authenticated with. It answers a request
// that's authenticated otherwise, such as with an API key, with session-not-found.
func currentSession(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	principal := authn.PrincipalFrom(r.Context())
	if principal.Method != method {
		envelope.WriteProblem(w, sessionNotFoundProblem)
		return uuid.Nil(), false
	}

	return principal.CredentialID, true
}
