package servicetest

import (
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authz"
	authzmocks "pkgsilo/core/authz/mocks"
	"pkgsilo/test/tbtest"
)

var (
	errDomain = errors.New("domain")
	errRepo   = errors.New("repository")
)

func TestAssertTranslated(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		err         error
		want        error
		wantMessage string
	}{
		{name: "translated", err: fmt.Errorf("get: %w", errDomain), want: errDomain},
		{name: "unexpected error passes through", err: fmt.Errorf("get: %w", errRepo), want: errRepo},
		{
			name: "not translated", err: fmt.Errorf("get: %w", errRepo), want: errDomain,
			wantMessage: "domain",
		},
		{
			name: "cause leaks", err: fmt.Errorf("get: %w: %w", errDomain, errRepo), want: errDomain,
			wantMessage: "must not leak",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := tbtest.Run(t, func(tb *tbtest.Recorder) { AssertTranslated(tb, tt.err, tt.want, errRepo) })

			assert.Equal(t, tt.wantMessage != "", rec.Failed(), rec.Messages())
			assert.Contains(t, rec.Messages(), tt.wantMessage)
		})
	}
}

type store struct {
	mock.Mock
}

func (s *store) Save(name string) {
	s.Called(name)
}

func TestCapture(t *testing.T) {
	t.Parallel()
	s := &store{}
	var saved string
	s.On("Save", Capture(&saved)).Return()

	s.Save("ada")

	assert.Equal(t, "ada", saved)
}

func TestCapture_MatchesOnlyT(t *testing.T) {
	t.Parallel()
	var n int

	_, differences := mock.Arguments{Capture(&n)}.Diff([]any{"ada"})

	assert.Equal(t, 1, differences)
}

func TestExpectReadableFields(t *testing.T) {
	t.Parallel()
	fields := authz.NewReadFieldActions("identity", "username", "displayName")
	resource := authz.Resource{Type: authz.IdentityType, ID: uuid.NewV4()}
	decider := authzmocks.NewMockDecider(t)

	ExpectReadableFields(decider, fields, resource, "displayName")

	readable, err := fields.Readable(decider, resource)
	require.NoError(t, err)
	assert.Equal(t, authz.Field[string]{}, authz.Reveal(readable, "username", "ada"))
	assert.Equal(t, authz.Visible("Ada"), authz.Reveal(readable, "displayName", "Ada"))
}
