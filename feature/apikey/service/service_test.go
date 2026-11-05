package service

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/pagination"
	"pkgsilo/feature/apikey/repository"
	"pkgsilo/feature/apikey/repository/mocks"
	"pkgsilo/test/servicetest"
)

var (
	now        = time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	actor      = uuid.MustParse("6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b")
	identityID = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")
	pepper     = []byte("test-pepper-of-at-least-32-characters")
	expiresAt  = now.Add(24 * time.Hour)
	lastUsedAt = now.Add(-30 * time.Second)

	// stored is a record as the repository returns it, with a distinct value in every field.
	stored = repository.APIKeyRecord{
		ID:         uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		IdentityID: identityID,
		Name:       "ci",
		KeyHash:    []byte("hash"),
		KeySuffix:  "WXYZ",
		ExpiresAt:  &expiresAt,
		LastUsedAt: &lastUsedAt,
		CreatedBy:  uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		CreatedAt:  now.Add(-time.Hour),
	}
	// storedKey is stored, translated to the domain model.
	storedKey = APIKey{
		ID:         stored.ID,
		IdentityID: identityID,
		Name:       "ci",
		KeySuffix:  "WXYZ",
		ExpiresAt:  &expiresAt,
		LastUsedAt: &lastUsedAt,
		CreatedBy:  stored.CreatedBy,
		CreatedAt:  stored.CreatedAt,
	}

	errDB = errors.New("connection refused")
)

func newTestService(t *testing.T) (*apiKeyService, *mocks.MockAPIKeyRepository) {
	t.Helper()

	repo := mocks.NewMockAPIKeyRepository(t)

	return newAPIKeyService(repo, pepper, func() time.Time { return now }), repo
}

func TestAPIKeyService_Create(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	var inserted repository.APIKeyRecord
	repo.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)

	got, err := svc.Create(t.Context(), actor, identityID, APIKeyInput{Name: "ci", ExpiresAt: &expiresAt})
	require.NoError(t, err)

	assert.Equal(t, byte(4), inserted.ID[6]>>4, "ID must be a version 4 UUID")
	assert.Regexp(t, `^psk_[A-Z2-7]{52}$`, got.Key)
	assert.Equal(t, repository.APIKeyRecord{
		ID:         inserted.ID,
		IdentityID: identityID,
		Name:       "ci",
		KeyHash:    svc.hash(got.Key),
		KeySuffix:  got.Key[len(got.Key)-4:],
		ExpiresAt:  &expiresAt,
		CreatedBy:  actor,
		CreatedAt:  now,
	}, inserted)
	// The API key as stored is returned, not the one sent to the repository.
	assert.Equal(t, storedKey, got.APIKey)
}

func TestAPIKeyService_Create_GeneratesUniqueKeysAndIDs(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	ids, keys := map[uuid.UUID]bool{}, map[string]bool{}
	repo.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rec repository.APIKeyRecord) (repository.APIKeyRecord, error) {
			ids[rec.ID] = true
			return rec, nil
		},
	).Times(2)

	for range 2 {
		got, err := svc.Create(t.Context(), actor, identityID, APIKeyInput{Name: "ci"})
		require.NoError(t, err)
		keys[got.Key] = true
	}

	assert.Len(t, ids, 2)
	assert.Len(t, keys, 2)
}

func TestAPIKeyService_Create_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		cause error
		want  error
	}{
		{name: "identity not found", cause: repository.ErrIdentityNotFound, want: ErrIdentityNotFound},
		{name: "unexpected", cause: errDB, want: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			repo.EXPECT().Insert(mock.Anything, mock.Anything).
				Return(repository.APIKeyRecord{}, fmt.Errorf("insert api key: %w", tt.cause))

			_, err := svc.Create(t.Context(), actor, identityID, APIKeyInput{Name: "ci"})

			servicetest.AssertTranslated(t, err, tt.want, tt.cause)
		})
	}
}

func TestAPIKeyService_Hash(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService(t)

	// Computed with openssl. A changed algorithm invalidates every stored key, so it must fail here.
	assert.Equal(t,
		"5436c1cb28ef90614916fb6e6002b683bd716970e1cf0cfe7190b98213db5756",
		hex.EncodeToString(svc.hash("psk_TESTKEY")),
	)

	other := newAPIKeyService(mocks.NewMockAPIKeyRepository(t), []byte("another-pepper-of-at-least-32-chars"), time.Now)
	assert.NotEqual(t, svc.hash("psk_TESTKEY"), other.hash("psk_TESTKEY"), "the hash must depend on the pepper")
}

func TestAPIKeyService_Get(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(stored, nil)

	got, err := svc.Get(t.Context(), identityID, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, storedKey, got)
}

func TestAPIKeyService_Get_Errors(t *testing.T) {
	t.Parallel()
	notFound := fmt.Errorf("get api key: %w", repository.ErrNotFound)
	tests := []struct {
		name   string
		cause  error
		exists func(*mocks.MockAPIKeyRepository)
		want   error
	}{
		{
			name:  "api key not found",
			cause: notFound,
			exists: func(repo *mocks.MockAPIKeyRepository) {
				repo.EXPECT().IdentityExists(mock.Anything, identityID).Return(true, nil)
			},
			want: ErrAPIKeyNotFound,
		},
		{
			name:  "identity not found",
			cause: notFound,
			exists: func(repo *mocks.MockAPIKeyRepository) {
				repo.EXPECT().IdentityExists(mock.Anything, identityID).Return(false, nil)
			},
			want: ErrIdentityNotFound,
		},
		{
			name:  "identity lookup fails",
			cause: notFound,
			exists: func(repo *mocks.MockAPIKeyRepository) {
				repo.EXPECT().IdentityExists(mock.Anything, identityID).Return(false, errDB)
			},
			want: errDB,
		},
		{name: "unexpected", cause: errDB, exists: func(*mocks.MockAPIKeyRepository) {}, want: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(repository.APIKeyRecord{}, tt.cause)
			tt.exists(repo)

			_, err := svc.Get(t.Context(), identityID, stored.ID)

			servicetest.AssertTranslated(t, err, tt.want, repository.ErrNotFound)
		})
	}
}

func TestAPIKeyService_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		recs    []repository.APIKeyRecord
		want    []APIKey
		hasMore bool
	}{
		{name: "last page", recs: []repository.APIKeyRecord{stored}, want: []APIKey{storedKey}},
		{
			name:    "more follow",
			recs:    []repository.APIKeyRecord{stored, stored},
			want:    []APIKey{storedKey},
			hasMore: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			repo.EXPECT().List(mock.Anything, identityID, 2, 2).Return(tt.recs, nil)

			got, err := svc.List(t.Context(), identityID, 2, 1)
			require.NoError(t, err)

			assert.Equal(t, pagination.Page[APIKey]{Items: tt.want, HasMore: tt.hasMore}, got)
		})
	}
}

func TestAPIKeyService_List_NoKeys(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().List(mock.Anything, identityID, 0, 51).Return(nil, nil)
	repo.EXPECT().IdentityExists(mock.Anything, identityID).Return(true, nil)

	got, err := svc.List(t.Context(), identityID, 0, 50)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

func TestAPIKeyService_List_IdentityNotFound(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().List(mock.Anything, identityID, 0, 51).Return(nil, nil)
	repo.EXPECT().IdentityExists(mock.Anything, identityID).Return(false, nil)

	_, err := svc.List(t.Context(), identityID, 0, 50)

	servicetest.AssertTranslated(t, err, ErrIdentityNotFound, repository.ErrIdentityNotFound)
}

func TestAPIKeyService_List_Errors(t *testing.T) {
	t.Parallel()

	t.Run("list fails", func(t *testing.T) {
		t.Parallel()
		svc, repo := newTestService(t)
		repo.EXPECT().List(mock.Anything, identityID, 0, 51).Return(nil, errDB)

		_, err := svc.List(t.Context(), identityID, 0, 50)

		assert.ErrorIs(t, err, errDB)
	})
	t.Run("identity lookup fails", func(t *testing.T) {
		t.Parallel()
		svc, repo := newTestService(t)
		repo.EXPECT().List(mock.Anything, identityID, 0, 51).Return(nil, nil)
		repo.EXPECT().IdentityExists(mock.Anything, identityID).Return(false, errDB)

		_, err := svc.List(t.Context(), identityID, 0, 50)

		assert.ErrorIs(t, err, errDB)
	})
}

func TestAPIKeyService_Delete(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().Delete(mock.Anything, identityID, stored.ID).Return(nil)

	assert.NoError(t, svc.Delete(t.Context(), identityID, stored.ID))
}

func TestAPIKeyService_Delete_Errors(t *testing.T) {
	t.Parallel()
	notFound := fmt.Errorf("delete api key: %w", repository.ErrNotFound)
	tests := []struct {
		name   string
		cause  error
		exists func(*mocks.MockAPIKeyRepository)
		want   error
	}{
		{
			name:  "api key not found",
			cause: notFound,
			exists: func(repo *mocks.MockAPIKeyRepository) {
				repo.EXPECT().IdentityExists(mock.Anything, identityID).Return(true, nil)
			},
			want: ErrAPIKeyNotFound,
		},
		{
			name:  "identity not found",
			cause: notFound,
			exists: func(repo *mocks.MockAPIKeyRepository) {
				repo.EXPECT().IdentityExists(mock.Anything, identityID).Return(false, nil)
			},
			want: ErrIdentityNotFound,
		},
		{name: "unexpected", cause: errDB, exists: func(*mocks.MockAPIKeyRepository) {}, want: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			repo.EXPECT().Delete(mock.Anything, identityID, stored.ID).Return(tt.cause)
			tt.exists(repo)

			err := svc.Delete(t.Context(), identityID, stored.ID)

			servicetest.AssertTranslated(t, err, tt.want, repository.ErrNotFound)
		})
	}
}

const presentedKey = "psk_PRESENTEDKEY"

func TestAPIKeyService_Authenticate(t *testing.T) {
	t.Parallel()
	recent := now.Add(-lastUsedInterval + time.Second)
	stale := now.Add(-lastUsedInterval)
	tests := []struct {
		name       string
		expiresAt  *time.Time
		lastUsedAt *time.Time
		wantUpdate bool
	}{
		{name: "first use", wantUpdate: true},
		{name: "used long ago", lastUsedAt: &stale, wantUpdate: true},
		{name: "used recently", lastUsedAt: &recent},
		{name: "not yet expired", expiresAt: new(now.Add(time.Second)), lastUsedAt: &recent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			rec := stored
			rec.ExpiresAt, rec.LastUsedAt = tt.expiresAt, tt.lastUsedAt
			repo.EXPECT().GetByHash(mock.Anything, svc.hash(presentedKey)).Return(rec, nil)
			want := toAPIKey(rec)
			if tt.wantUpdate {
				repo.EXPECT().UpdateLastUsed(mock.Anything, stored.ID, now).Return(nil)
				want.LastUsedAt = &now
			}

			got, err := svc.Authenticate(t.Context(), presentedKey)
			require.NoError(t, err)

			assert.Equal(t, want, got)
		})
	}
}

func TestAPIKeyService_Authenticate_MalformedKeyIsNotLookedUp(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"", "psk", "PSK_PRESENTEDKEY", "other_PRESENTEDKEY"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on any call.
			svc, _ := newTestService(t)

			_, err := svc.Authenticate(t.Context(), key)

			assert.ErrorIs(t, err, ErrMalformedKey)
		})
	}
}

func TestAPIKeyService_Authenticate_UnknownKey(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().GetByHash(mock.Anything, svc.hash(presentedKey)).
		Return(repository.APIKeyRecord{}, fmt.Errorf("get api key by hash: %w", repository.ErrNotFound))

	_, err := svc.Authenticate(t.Context(), presentedKey)

	servicetest.AssertTranslated(t, err, ErrUnknownKey, repository.ErrNotFound)
	assert.NotContains(t, err.Error(), presentedKey)
}

func TestAPIKeyService_Authenticate_ExpiredKey(t *testing.T) {
	t.Parallel()
	for name, expiry := range map[string]time.Time{"at the expiry": now, "after the expiry": now.Add(-time.Second)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			rec := stored
			rec.ExpiresAt = &expiry
			// An expired key's use isn't recorded: the mock fails the test on UpdateLastUsed.
			repo.EXPECT().GetByHash(mock.Anything, svc.hash(presentedKey)).Return(rec, nil)

			_, err := svc.Authenticate(t.Context(), presentedKey)

			require.ErrorIs(t, err, ErrExpiredKey)
			assert.Contains(t, err.Error(), stored.ID.String(), "the logs must tell which API key expired")
			assert.NotContains(t, err.Error(), presentedKey)
		})
	}
}

func TestAPIKeyService_Authenticate_Errors(t *testing.T) {
	t.Parallel()

	t.Run("lookup fails", func(t *testing.T) {
		t.Parallel()
		svc, repo := newTestService(t)
		repo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(repository.APIKeyRecord{}, errDB)

		_, err := svc.Authenticate(t.Context(), presentedKey)

		assert.ErrorIs(t, err, errDB)
	})
	t.Run("recording the use fails", func(t *testing.T) {
		t.Parallel()
		svc, repo := newTestService(t)
		rec := stored
		rec.LastUsedAt = nil
		repo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(rec, nil)
		repo.EXPECT().UpdateLastUsed(mock.Anything, stored.ID, now).Return(errDB)

		_, err := svc.Authenticate(t.Context(), presentedKey)

		assert.ErrorIs(t, err, errDB)
	})
}
