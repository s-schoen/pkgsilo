package api

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/core/rules"
	"pkgsilo/feature/apikey/service"
)

// CreateKeyRequest is the body of a request that creates an API key.
type CreateKeyRequest struct {
	Name string `json:"name"`
	// ExpiresAt is absent for an API key that never expires.
	ExpiresAt *time.Time `json:"expiresAt,omitzero"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r CreateKeyRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name,
			validation.Required,
			rules.NotBlank,
			rules.NoControlCharacters,
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

// KeyResponse is the representation of an API key in responses. A field that the caller may not read is
// absent.
type KeyResponse struct {
	ID         uuid.UUID                 `json:"id"`
	IdentityID envelope.Field[uuid.UUID] `json:"identityId,omitzero"`
	Name       envelope.Field[string]    `json:"name,omitzero"`
	// KeySuffix is the key's last characters, which tell the holder of a key which API key it is.
	KeySuffix envelope.Field[string] `json:"keySuffix,omitzero"`
	// ExpiresAt is null for an API key that never expires, and absent if it's hidden.
	ExpiresAt envelope.Field[*time.Time] `json:"expiresAt,omitzero"`
	// LastUsedAt is null for an API key that has not been used yet, and absent if it's hidden.
	LastUsedAt     envelope.Field[*time.Time]  `json:"lastUsedAt,omitzero"`
	CreatedBy      envelope.Field[uuid.UUID]   `json:"createdBy,omitzero"`
	CreatedAt      envelope.Field[time.Time]   `json:"createdAt,omitzero"`
	AllowedActions envelope.Field[[]KeyAction] `json:"allowedActions,omitzero" jsonschema:"The actions that the policies allow the caller to do to it: apikey:delete. Absent only from a new API key that the caller may not read."`
}

// KeyAction is the name of an action that the caller may do to an API key, as KeyResponse lists
// them: one of service.Actions().Allowable().
type KeyAction string

// EnumValues returns the names of the actions that KeyResponse can list, the enum of its schema.
func (KeyAction) EnumValues() []string {
	return service.Actions().Allowable()
}

// CreatedKeyResponse is the representation of a new API key: the only one that holds its key, which is
// never hidden, but an envelope.Field because every member of a representation is.
type CreatedKeyResponse struct {
	KeyResponse

	Key envelope.Field[string] `json:"key,omitzero"`
}

func toKeyResponse(k service.APIKey) KeyResponse {
	return KeyResponse{
		ID:             k.ID,
		IdentityID:     envelope.NewField(k.IdentityID.Get()),
		Name:           envelope.NewField(k.Name.Get()),
		KeySuffix:      envelope.NewField(k.KeySuffix.Get()),
		ExpiresAt:      envelope.NewField(k.ExpiresAt.Get()),
		LastUsedAt:     envelope.NewField(k.LastUsedAt.Get()),
		CreatedBy:      envelope.NewField(k.CreatedBy.Get()),
		CreatedAt:      envelope.NewField(k.CreatedAt.Get()),
		AllowedActions: envelope.NewActions[KeyAction](k.AllowedActions),
	}
}

func toCreatedKeyResponse(k service.CreatedAPIKey) CreatedKeyResponse {
	return CreatedKeyResponse{KeyResponse: toKeyResponse(k.APIKey), Key: envelope.Visible(k.Key)}
}
