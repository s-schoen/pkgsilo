package service

import (
	"errors"

	"pkgsilo/feature/apikey/repository"
)

var (
	// ErrAPIKeyNotFound is returned when the identity has no API key with the requested ID.
	ErrAPIKeyNotFound = errors.New("api key not found")
	// ErrIdentityNotFound is returned when the identity of an API key doesn't exist.
	ErrIdentityNotFound = errors.New("identity not found")
	// ErrMalformedKey is returned when a presented key doesn't have the format of a key.
	ErrMalformedKey = errors.New("malformed key")
	// ErrUnknownKey is returned when a presented key belongs to no API key.
	ErrUnknownKey = errors.New("unknown key")
	// ErrExpiredKey is returned when a presented key belongs to an API key that has expired.
	ErrExpiredKey = errors.New("expired key")
)

// translateError replaces repository errors with their domain errors.
func translateError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrAPIKeyNotFound
	case errors.Is(err, repository.ErrIdentityNotFound):
		return ErrIdentityNotFound
	default:
		return err
	}
}
