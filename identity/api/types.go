package api

import (
	"regexp"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/identity/service"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)

// Rules of the identity fields, shared by the request bodies that set them.
var (
	usernameRules = []validation.Rule{
		validation.Required,
		validation.Length(3, 64),
		validation.Match(usernamePattern).
			Error("must consist of lowercase letters and digits, optionally separated by single '.', '_' or '-'"),
	}
	displayNameRules = []validation.Rule{
		validation.Required,
		validation.RuneLength(1, 128),
	}
)

// CreateIdentityRequest is the body of a request that creates an identity.
type CreateIdentityRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

func (r CreateIdentityRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Username, usernameRules...),
		validation.Field(&r.DisplayName, displayNameRules...),
	)
}

func (r CreateIdentityRequest) toInput() service.IdentityInput {
	return service.IdentityInput{Username: r.Username, DisplayName: r.DisplayName}
}

// ReplaceIdentityRequest is the body of a request that replaces all settable fields of an identity.
type ReplaceIdentityRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

func (r ReplaceIdentityRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Username, usernameRules...),
		validation.Field(&r.DisplayName, displayNameRules...),
	)
}

func (r ReplaceIdentityRequest) toInput() service.IdentityInput {
	return service.IdentityInput{Username: r.Username, DisplayName: r.DisplayName}
}

// IdentityResponse is the representation of an identity in responses.
type IdentityResponse struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	CreatedBy   uuid.UUID `json:"createdBy"`
	UpdatedBy   uuid.UUID `json:"updatedBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func toIdentityResponse(i service.Identity) IdentityResponse {
	return IdentityResponse{
		ID:          i.ID,
		Username:    i.Username,
		DisplayName: i.DisplayName,
		CreatedBy:   i.CreatedBy,
		UpdatedBy:   i.UpdatedBy,
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
	}
}
