package api

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/request"
	"pkgsilo/feature/apikey/service"
)

// CreateKeyRequest is the body of a request that creates an API key.
type CreateKeyRequest struct {
	Name string `json:"name"`
	// ExpiresAt is absent for an API key that never expires.
	ExpiresAt *time.Time `json:"expiresAt"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r CreateKeyRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name,
			validation.Required,
			request.NotBlank,
			request.NoControlCharacters,
			validation.RuneLength(1, 128),
		),
		validation.Field(&r.ExpiresAt,
			// Without ValidateEmpty, the rule skips the zero time as empty.
			validation.Min(time.Now()).Exclusive().ValidateEmpty().Error("must be in the future"),
		),
	)
}

func (r CreateKeyRequest) toInput() service.APIKeyInput {
	return service.APIKeyInput{Name: r.Name, ExpiresAt: r.ExpiresAt}
}

// KeyResponse is the representation of an API key in responses.
type KeyResponse struct {
	ID         uuid.UUID `json:"id"`
	IdentityID uuid.UUID `json:"identityId"`
	Name       string    `json:"name"`
	// KeySuffix is the key's last characters, which tell the holder of a key which API key it is.
	KeySuffix string `json:"keySuffix"`
	// ExpiresAt is null for an API key that never expires.
	ExpiresAt *time.Time `json:"expiresAt"`
	// LastUsedAt is null for an API key that has not been used yet.
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedBy  uuid.UUID  `json:"createdBy"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// CreatedKeyResponse is the representation of a new API key: the only one that holds its key.
type CreatedKeyResponse struct {
	KeyResponse

	Key string `json:"key"`
}

func toKeyResponse(k service.APIKey) KeyResponse {
	return KeyResponse{
		ID:         k.ID,
		IdentityID: k.IdentityID,
		Name:       k.Name,
		KeySuffix:  k.KeySuffix,
		ExpiresAt:  k.ExpiresAt,
		LastUsedAt: k.LastUsedAt,
		CreatedBy:  k.CreatedBy,
		CreatedAt:  k.CreatedAt,
	}
}

func toCreatedKeyResponse(k service.CreatedAPIKey) CreatedKeyResponse {
	return CreatedKeyResponse{KeyResponse: toKeyResponse(k.APIKey), Key: k.Key}
}
