package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2Params are the parameters of an Argon2id hash.
type argon2Params struct {
	// memory is the memory that a hash uses, in KiB.
	memory uint32
	// time is the number of passes over the memory.
	time uint32
	// threads is the degree of parallelism.
	threads uint8
	// saltLength and keyLength are the lengths of the salt and of the hash output, in bytes.
	saltLength, keyLength uint32
}

// currentParams are the parameters of new hashes: the first of OWASP's recommended Argon2id configurations,
// with a 16-byte salt and a 32-byte output. A stored hash with other parameters is rehashed with these once
// its password is verified. They're constants, and there is no pepper: it would be a secret that couldn't be
// rotated without resetting every password.
var currentParams = argon2Params{memory: 19 * 1024, time: 2, threads: 1, saltLength: 16, keyLength: 32}

// hashSlots holds a slot for each hash that may run at once: one per core that Go uses, since each hash
// keeps a core busy and holds its memory until it's done. A shared cap, since the password service and the
// verifier hash on the same cores.
var hashSlots = make(chan struct{}, runtime.GOMAXPROCS(0))

// errMalformedHash is returned when a stored hash isn't an Argon2id PHC string.
var errMalformedHash = errors.New("malformed password hash")

// hashPassword returns the hash of password with currentParams and a new random salt, as a PHC string. It
// waits for a free slot, and gives up with ctx's error when ctx is done first.
func hashPassword(ctx context.Context, password string) (string, error) {
	salt := make([]byte, currentParams.saltLength)
	// rand.Read never returns an error.
	_, _ = rand.Read(salt)

	key, err := deriveKey(ctx, password, salt, currentParams)
	if err != nil {
		return "", err
	}

	return encodeHash(currentParams, salt, key), nil
}

// verifyPassword reports whether password matches the PHC string hash, and whether hash should be replaced
// by one with currentParams. It waits for a free slot like hashPassword.
func verifyPassword(ctx context.Context, password, hash string) (match, outdated bool, err error) {
	stored, err := decodeHash(hash)
	if err != nil {
		return false, false, err
	}

	got, err := deriveKey(ctx, password, stored.salt, stored.params)
	if err != nil {
		return false, false, err
	}

	return subtle.ConstantTimeCompare(got, stored.key) == 1, stored.params != currentParams, nil
}

// deriveKey returns the Argon2id hash of password with the salt and params, once a slot is free. A request
// that's done already doesn't hash, even if a slot is free.
func deriveKey(ctx context.Context, password string, salt []byte, params argon2Params) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("wait for a password hash slot: %w", context.Cause(ctx))
	}

	select {
	case hashSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for a password hash slot: %w", context.Cause(ctx))
	}
	defer func() { <-hashSlots }()

	return argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, params.keyLength), nil
}

// encodeHash returns the PHC string of an Argon2id hash: $argon2id$v=19$m=<memory>,t=<time>,p=<threads>$<salt>$<key>,
// with the salt and key in unpadded standard base64.
func encodeHash(params argon2Params, salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, params.memory, params.time,
		params.threads, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// phcHash is an Argon2id hash as its PHC string holds it.
type phcHash struct {
	params    argon2Params
	salt, key []byte
}

// decodeHash returns the parameters, salt and key of the PHC string of an Argon2id hash.
func decodeHash(hash string) (phcHash, error) {
	// The string starts with $, so the first part is empty.
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return phcHash{}, errMalformedHash
	}

	params, err := decodeParams(parts[2], parts[3])
	if err != nil {
		return phcHash{}, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return phcHash{}, errMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return phcHash{}, errMalformedHash
	}
	params.saltLength, params.keyLength = uint32(len(salt)), uint32(len(key)) //nolint:gosec // Short strings.

	return phcHash{params: params, salt: salt, key: key}, nil
}

// decodeParams returns the parameters of the version and parameter parts of a PHC string, such as v=19 and
// m=19456,t=2,p=1, without the lengths, which the salt and key tell.
func decodeParams(version, params string) (argon2Params, error) {
	var v int
	if _, err := fmt.Sscanf(version, "v=%d", &v); err != nil || v != argon2.Version {
		return argon2Params{}, errMalformedHash
	}

	var p argon2Params
	_, err := fmt.Sscanf(params, "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads)
	if err != nil || p.time == 0 || p.threads == 0 {
		return argon2Params{}, errMalformedHash
	}

	return p, nil
}

// dummyHash is a hash with currentParams that no password matches in practice, since its key is all zeros.
// Verifying a password against it takes as long as against a real hash, for an identity that is unknown or
// has no password, so that the response time doesn't reveal which.
var dummyHash = encodeHash(currentParams, make([]byte, currentParams.saltLength), make([]byte, currentParams.keyLength))
