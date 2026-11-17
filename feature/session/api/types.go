package api

import (
	"net/netip"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/session/service"
)

// LoginRequest is the body of a login.
type LoginRequest struct {
	Username string `json:"username" jsonschema:"The username of the identity to log in as."`
	Password string `json:"password" jsonschema:"The identity's password, exactly as it was set."`
}

// Validate reports the invalid members of r as validation.Errors.
func (r LoginRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Username, service.UsernameRules...),
		validation.Field(&r.Password, service.PasswordRules...),
	)
}

func (r LoginRequest) toInput(presentedToken string, clientIP netip.Addr, userAgent string) service.LoginInput {
	return service.LoginInput{
		Username: r.Username, Password: r.Password, PresentedToken: presentedToken, ClientIP: clientIP,
		UserAgent: userAgent,
	}
}

// SessionResponse is the representation of a session in responses. It never holds the session's token,
// which only its cookie does. No policy hides its fields, since only its own identity ever sees it.
type SessionResponse struct {
	ID         uuid.UUID                 `json:"id"                  jsonschema:"The session's ID."`
	IdentityID envelope.Field[uuid.UUID] `json:"identityId,omitzero" jsonschema:"The identity that the session is logged in as."`
	CreatedAt  envelope.Field[time.Time] `json:"createdAt,omitzero"  jsonschema:"When the session was created by a login."`
	ExpiresAt  envelope.Field[time.Time] `json:"expiresAt,omitzero"  jsonschema:"When the session ends at the latest, however often it's used. It ends earlier if it's idle for too long."`
}

func toSessionResponse(s service.Session) SessionResponse {
	return SessionResponse{
		ID:         s.ID,
		IdentityID: envelope.Visible(s.IdentityID),
		CreatedAt:  envelope.Visible(s.CreatedAt),
		ExpiresAt:  envelope.Visible(s.ExpiresAt),
	}
}
