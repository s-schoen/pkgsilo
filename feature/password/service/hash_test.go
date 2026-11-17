package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/argon2"
)

// outdatedParams are cheaper parameters than currentParams, as a hash from before a change of them has.
var outdatedParams = argon2Params{memory: 8 * 1024, time: 1, threads: 1, saltLength: 16, keyLength: 32}

// hashWith returns the PHC string of the hash of currentPassword with params.
func hashWith(params argon2Params) string {
	const password = currentPassword

	salt := make([]byte, params.saltLength)
	key := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, params.keyLength)

	return encodeHash(params, salt, key)
}

func TestHashPassword(t *testing.T) {
	t.Parallel()

	hash, err := hashPassword(t.Context(), "correct horse")
	require.NoError(t, err)

	assert.Regexp(t, `^\$argon2id\$v=19\$m=19456,t=2,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`, hash)
	other, err := hashPassword(t.Context(), "correct horse")
	require.NoError(t, err)
	assert.NotEqual(t, hash, other, "every hash must have a salt of its own")
}

func TestVerifyPassword(t *testing.T) {
	t.Parallel()
	current, err := hashPassword(t.Context(), "correct horse")
	require.NoError(t, err)
	outdated := hashWith(outdatedParams)

	tests := []struct {
		name         string
		password     string
		hash         string
		wantMatch    bool
		wantOutdated bool
	}{
		{name: "match", password: "correct horse", hash: current, wantMatch: true},
		{name: "mismatch", password: "correct horse ", hash: current},
		{name: "empty", password: "", hash: current},
		{name: "outdated match", password: "correct horse", hash: outdated, wantMatch: true, wantOutdated: true},
		{name: "outdated mismatch", password: "wrong", hash: outdated, wantOutdated: true},
		{name: "dummy", password: "", hash: dummyHash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			match, outdated, err := verifyPassword(t.Context(), tt.password, tt.hash)
			require.NoError(t, err)

			assert.Equal(t, tt.wantMatch, match)
			assert.Equal(t, tt.wantOutdated, outdated)
		})
	}
}

func TestVerifyPassword_OtherLengthsAreOutdated(t *testing.T) {
	t.Parallel()
	params := currentParams
	params.saltLength, params.keyLength = 8, 16

	match, outdated, err := verifyPassword(t.Context(), "correct horse", hashWith(params))
	require.NoError(t, err)

	assert.True(t, match)
	assert.True(t, outdated)
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"empty":         "",
		"bcrypt":        "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234",
		"argon2i":       "$argon2i$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"old version":   "$argon2id$v=16$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"no params":     "$argon2id$v=19$$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"zero time":     "$argon2id$v=19$m=19456,t=0,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"bad salt":      "$argon2id$v=19$m=19456,t=2,p=1$!!!$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"no key":        "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$",
		"missing parts": "$argon2id$v=19$m=19456,t=2,p=1",
	}
	for name, hash := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, err := verifyPassword(t.Context(), "correct horse", hash)

			assert.ErrorIs(t, err, errMalformedHash)
		})
	}
}

// TestHashPassword_WaitsForASlot checks that no more hashes run at once than there are slots, and that a
// caller that waits for one gives up when its context is canceled. It isn't parallel, since it takes every
// slot.
func TestHashPassword_WaitsForASlot(t *testing.T) { //nolint:paralleltest // It takes every hash slot.
	for range cap(hashSlots) {
		hashSlots <- struct{}{}
	}
	released := false
	release := func() {
		if !released {
			for range cap(hashSlots) {
				<-hashSlots
			}
			released = true
		}
	}
	t.Cleanup(release)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err := hashPassword(ctx, "correct horse")
	require.ErrorIs(t, err, context.DeadlineExceeded)

	done := make(chan error, 1)
	go func() {
		_, err := hashPassword(t.Context(), "correct horse")
		done <- err
	}()
	select {
	case err := <-done:
		require.Failf(t, "hashed without a free slot", "error: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	release()
	assert.NoError(t, <-done)
}
