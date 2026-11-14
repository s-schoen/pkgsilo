package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authz"
	authzmocks "pkgsilo/core/authz/mocks"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/policy/repository"
	"pkgsilo/feature/policy/repository/mocks"
	"pkgsilo/test/servicetest"
)

const (
	readSource = `permit (principal, action == Action::"identity:read", resource);`
	// statement is the smallest statement, which never matches.
	statement = `permit(principal,action,resource)when{false};`
)

var (
	now   = time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	actor = uuid.MustParse("6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b")
	input = PolicyInput{Name: "read-identities", Source: readSource}
	patch = PolicyPatch{Name: new("read-identities"), Source: new(readSource)}

	// stored is a record as the repository returns it, with a distinct value in every field.
	stored = repository.PolicyRecord{
		ID:        uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		Name:      "everything",
		Source:    `permit (principal, action, resource);`,
		CreatedBy: uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		UpdatedBy: uuid.MustParse("2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e"),
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Minute),
	}
	// storedPolicy is stored, translated to the domain model, with every field visible.
	storedPolicy = Policy{
		ID:        stored.ID,
		Name:      authz.Visible("everything"),
		Source:    authz.Visible(stored.Source),
		CreatedBy: authz.Visible(stored.CreatedBy),
		UpdatedBy: authz.Visible(stored.UpdatedBy),
		CreatedAt: authz.Visible(stored.CreatedAt),
		UpdatedAt: authz.Visible(stored.UpdatedAt),
	}

	// storedResource is stored as a resource of decisions.
	storedResource = authz.Resource{Type: "Policy", ID: stored.ID}

	maintainers = uuid.MustParse("3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f")
	reviewers   = uuid.MustParse("4d5e6f7a-8b9c-4d0e-9f1a-2b3c4d5e6f7a")
	ada         = uuid.MustParse("5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b")
	grace       = uuid.MustParse("6f7a8b9c-0d1e-4f2a-9b3c-4d5e6f7a8b9c")

	errDB = errors.New("connection refused")
)

// testSchema is the schema of policies and of identities, as far as these tests need them.
var testSchema = authz.Schema(
	authz.Declaration{
		Type:    authz.IdentityType,
		Actions: []authz.Action{authz.NewAction("identity:read", authz.ActionGroupRead)},
	},
	Actions().Declaration(),
)

// newTestService returns a service that may do everything, on mock repositories.
func newTestService(t *testing.T) (*policyService, *mocks.MockPolicyRepository, *mocks.MockPolicyWriter) {
	t.Helper()

	return newTestServiceWith(t, authz.AllowAll())
}

// newTestServiceWith returns a service that asks authorizer, on a mock repository whose writes run with a mock
// writer.
func newTestServiceWith(
	t *testing.T, authorizer authz.Authorizer,
) (*policyService, *mocks.MockPolicyRepository, *mocks.MockPolicyWriter) {
	t.Helper()

	repo := mocks.NewMockPolicyRepository(t)
	writer := mocks.NewMockPolicyWriter(t)

	return newPolicyService(repo, authorizer, testSchema, func() time.Time { return now }), repo, writer
}

// expectWrite makes the repository run a write with writer, as a transaction would, and return its error.
func expectWrite(repo *mocks.MockPolicyRepository, writer *mocks.MockPolicyWriter) {
	repo.EXPECT().Write(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context, repository.PolicyWriter) error) error {
			return fn(ctx, writer)
		},
	)
}

// newDeciding returns an authorizer that answers actor's requests with a mock decider.
func newDeciding(t *testing.T) (*authzmocks.MockAuthorizer, *authzmocks.MockDecider) {
	t.Helper()

	authorizer := authzmocks.NewMockAuthorizer(t)
	decider := authzmocks.NewMockDecider(t)
	authorizer.EXPECT().For(mock.Anything, actor).Return(decider, nil)

	return authorizer, decider
}

// expectReadable makes the repository return stored, and the decider allow reading it.
func expectReadable(repo *mocks.MockPolicyRepository, decider *authzmocks.MockDecider) {
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	decider.EXPECT().Can(actions.Read(), storedResource).Return(true, nil)
}

// expectAllFieldsReadable makes the decider allow reading every field of the resource, once each.
func expectAllFieldsReadable(decider *authzmocks.MockDecider, resource authz.Resource) {
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), resource, actions.ReadFields().Fields()...)
}

// isReadField reports whether the action is one of reading a field of a policy.
func isReadField(action authz.Action) bool {
	return slices.ContainsFunc(actions.ReadFields().Actions(), func(a authz.Action) bool {
		return assert.ObjectsAreEqual(a, action)
	})
}

// newRecords returns stored records with the given IDs' last bytes, in order.
func newRecords(lastBytes ...byte) []repository.PolicyRecord {
	recs := make([]repository.PolicyRecord, len(lastBytes))
	for i, lastByte := range lastBytes {
		recs[i] = stored
		recs[i].ID = uuid.UUID{15: lastByte}
	}

	return recs
}

// idsOf returns the IDs' last bytes of the policies, in order.
func idsOf(policies []Policy) []byte {
	ids := make([]byte, len(policies))
	for i, policy := range policies {
		ids[i] = policy.ID[15]
	}

	return ids
}

// assertInvalid asserts that err is an *InvalidPolicyError whose reason contains reason.
func assertInvalid(t *testing.T, err error, reason string) {
	t.Helper()

	var invalid *InvalidPolicyError
	require.ErrorAs(t, err, &invalid)
	assert.Contains(t, invalid.Reason, reason)
}

func TestNewPolicyService_InvalidSchema(t *testing.T) {
	t.Parallel()
	for name, schema := range map[string]string{
		"unparsable": "not a schema",
		"unresolved": "entity Identity in [Missing];",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				NewPolicyService(mocks.NewMockPolicyRepository(t), authz.AllowAll(), schema)
			})
		})
	}
}

// TestPolicyService_Schema checks that the service returns the schema that it was given, without asking the
// authorizer, whose mock fails any call.
func TestPolicyService_Schema(t *testing.T) {
	t.Parallel()
	svc := NewPolicyService(mocks.NewMockPolicyRepository(t), authzmocks.NewMockAuthorizer(t), testSchema)

	assert.Equal(t, testSchema, svc.Schema())
}

func TestPolicyService_Create(t *testing.T) {
	t.Parallel()
	svc, repo, writer := newTestService(t)
	expectWrite(repo, writer)
	writer.EXPECT().ListAll(mock.Anything).Return(nil, nil)
	var inserted repository.PolicyRecord
	writer.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)

	got, err := svc.Create(t.Context(), actor, input)
	require.NoError(t, err)

	assert.Equal(t, byte(4), inserted.ID[6]>>4, "ID must be a version 4 UUID")
	assert.Equal(t, repository.PolicyRecord{
		ID:        inserted.ID,
		Name:      "read-identities",
		Source:    readSource,
		CreatedBy: actor,
		UpdatedBy: actor,
		CreatedAt: now,
		UpdatedAt: now,
	}, inserted)
	// The policy as stored is returned, not the one sent to the repository.
	assert.Equal(t, storedPolicy, got)
}

// TestPolicyService_Create_ChecksEveryField checks that creating needs the action of each field, on the policy
// to be created, and that a denial lists every denied field, before the source is checked, and creates
// nothing.
func TestPolicyService_Create_ChecksEveryField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		allowed    []string
		wantDenied []string
	}{
		{name: "all allowed", allowed: []string{"name", "source"}},
		{name: "source denied", allowed: []string{"name"}, wantDenied: []string{"source"}},
		{name: "all denied", wantDenied: []string{"name", "source"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo, writer := newTestServiceWith(t, authorizer)
			var checked []authz.Resource
			decider.EXPECT().Can(mock.Anything, mock.Anything).RunAndReturn(
				func(action authz.Action, resource authz.Resource) (bool, error) {
					checked = append(checked, resource)
					for _, field := range []string{"name", "source"} {
						if assert.ObjectsAreEqual(authz.NewAction("policy:create:"+field, "policy:create"), action) {
							return slices.Contains(tt.allowed, field), nil
						}
					}

					// Reading the new policy.
					return true, nil
				},
			)
			var inserted repository.PolicyRecord
			if tt.wantDenied == nil {
				expectWrite(repo, writer)
				writer.EXPECT().ListAll(mock.Anything).Return(nil, nil)
				writer.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)
			}

			source := readSource
			if tt.wantDenied != nil {
				// Invalid, so that checking it before the fields would fail differently.
				source = "not cedar"
			}

			_, err := svc.Create(t.Context(), actor, PolicyInput{Name: "x", Source: source})

			if tt.wantDenied == nil {
				require.NoError(t, err)
				want := authz.Resource{Type: "Policy", ID: inserted.ID}
				assert.Equal(t, slices.Repeat([]authz.Resource{want}, len(checked)), checked,
					"the checks must be on the policy to be created")
				return
			}
			// The mocks fail the test if the repository is called.
			var denied *authz.FieldsDeniedError
			require.ErrorAs(t, err, &denied)
			assert.Equal(t, tt.wantDenied, denied.Fields)
		})
	}
}

// TestPolicyService_Create_ReadableFields checks that the new policy is returned with the fields that the
// actor may read, and with only its ID if it may not read the policy.
func TestPolicyService_Create_ReadableFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		canRead  bool
		readable []string
		want     Policy
	}{
		{name: "all", canRead: true, readable: actions.ReadFields().Fields(), want: storedPolicy},
		{
			name: "some", canRead: true, readable: []string{"name"},
			want: Policy{ID: stored.ID, Name: authz.Visible("everything")},
		},
		{name: "unreadable policy", canRead: false, want: Policy{ID: stored.ID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo, writer := newTestServiceWith(t, authorizer)
			// The service generates the ID, so the checks match any resource. That they're on the new
			// policy is checked by TestPolicyService_Create_ChecksEveryField.
			for _, action := range actions.CreateFields().Actions() {
				decider.EXPECT().Can(action, mock.Anything).Return(true, nil).Once()
			}
			decider.EXPECT().Can(actions.Read(), mock.Anything).Return(tt.canRead, nil).Once()
			if tt.canRead {
				for i, action := range actions.ReadFields().Actions() {
					decider.EXPECT().Can(action, mock.Anything).
						Return(slices.Contains(tt.readable, actions.ReadFields().Fields()[i]), nil).Once()
				}
			}
			expectWrite(repo, writer)
			writer.EXPECT().ListAll(mock.Anything).Return(nil, nil)
			writer.EXPECT().Insert(mock.Anything, mock.Anything).Return(stored, nil)

			got, err := svc.Create(t.Context(), actor, input)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPolicyService_Declaration checks that policies can use every action on policies, which the schema
// would reject otherwise.
func TestPolicyService_Declaration(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestService(t)

	for _, action := range []string{
		"policy:read", "policy:delete", "policy:read:name", "policy:read:source",
		"policy:create:name", "policy:create:source", "policy:update:name", "policy:update:source",
	} {
		_, err := svc.checker.check(`permit (principal, action == Action::"` + action + `", resource is Policy);`)
		require.NoError(t, err, action)
	}
	for _, group := range []string{"read", "policy:readFields", "policy:create", "policy:update"} {
		_, err := svc.checker.check(`permit (principal, action in Action::"` + group + `", resource is Policy);`)
		require.NoError(t, err, group)
	}
}

// TestPolicyService_Create_InvalidSource checks the mistakes that the checks report, without writing anything.
// The end-to-end tests cover the schema's reasons.
func TestPolicyService_Create_InvalidSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, source, reason string
	}{
		{name: "parse error", source: `permit (`, reason: "the source can't be parsed"},
		{name: "largest", source: readSource + strings.Repeat(" ", maxSourceBytes-len(readSource))},
		{
			name: "oversized", source: readSource + strings.Repeat(" ", maxSourceBytes-len(readSource)+1),
			reason: "the source is larger than 16384 bytes",
		},
		{name: "no statements", source: "// Nothing yet.\n", reason: "the source holds no statements"},
		{name: "schema", source: `permit (principal, action == Action::"policy:raed", resource);`, reason: "policy:raed"},
		{
			name:   "has on an undeclared attribute",
			source: readSource + `forbid (principal, action, resource) when { principal has secrte && principal.secrte };`,
			reason: `statement 2 tests for the attribute "secrte" with has, which no entity type declares`,
		},
		{
			name:   "group ID that isn't a UUID",
			source: `permit (principal in Group::"admins", action, resource);`,
			reason: `the group "admins" does not exist`,
		},
		{
			name:   "group ID in uppercase",
			source: `forbid (principal in Group::"3C4D5E6F-7A8B-4C9D-8E0F-1A2B3C4D5E6F", action, resource);`,
			reason: `the group ID "3C4D5E6F-7A8B-4C9D-8E0F-1A2B3C4D5E6F" never matches, ` +
				`since IDs are written in lowercase with hyphens, as "3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f"`,
		},
		{
			name:   "group ID in braces",
			source: `forbid (principal in Group::"{3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f}", action, resource);`,
			reason: `the group ID "{3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f}" never matches`,
		},
		{
			name:   "identity ID that isn't a UUID",
			source: `forbid (principal == Identity::"ada", action, resource);`,
			reason: `the identity "ada" does not exist`,
		},
		{
			name:   "identity ID without hyphens",
			source: `forbid (principal, action, resource) when { principal == Identity::"5e6f7a8b9c0d4e1f8a2b3c4d5e6f7a8b" };`,
			reason: `the identity ID "5e6f7a8b9c0d4e1f8a2b3c4d5e6f7a8b" never matches`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo, writer := newTestService(t)
			if tt.reason == "" {
				expectWrite(repo, writer)
				writer.EXPECT().ListAll(mock.Anything).Return(nil, nil)
				writer.EXPECT().Insert(mock.Anything, mock.Anything).Return(stored, nil)
			}

			_, err := svc.Create(t.Context(), actor, PolicyInput{Name: "x", Source: tt.source})

			if tt.reason == "" {
				require.NoError(t, err)
				return
			}
			// The mocks fail the test if the repository is called.
			assertInvalid(t, err, tt.reason)
		})
	}
}

// TestPolicyService_Create_References checks that every group and identity that the source references, in its
// scope or its conditions, must exist, and that each kind is looked up in one query.
func TestPolicyService_Create_References(t *testing.T) {
	t.Parallel()
	source := `permit (principal in Group::"` + maintainers.String() + `", action, resource)
		when { principal in [Group::"` + reviewers.String() + `", Group::"` + maintainers.String() + `"] };
		forbid (principal == Identity::"` + ada.String() + `", action, resource)
		when { resource == Identity::"` + grace.String() + `" };`
	tests := []struct {
		name                        string
		existingGroups, existingIDs []uuid.UUID
		reason                      string
	}{
		{
			name:           "all exist",
			existingGroups: []uuid.UUID{reviewers, maintainers},
			existingIDs:    []uuid.UUID{grace, ada},
		},
		{
			name:           "one group missing",
			existingGroups: []uuid.UUID{maintainers},
			reason:         `the group "` + reviewers.String() + `" does not exist`,
		},
		{name: "no group exists", reason: `the group "` + maintainers.String() + `" does not exist`},
		{
			name:           "one identity missing",
			existingGroups: []uuid.UUID{reviewers, maintainers},
			existingIDs:    []uuid.UUID{ada},
			reason:         `the identity "` + grace.String() + `" does not exist`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo, writer := newTestService(t)
			repo.EXPECT().ListExistingGroups(mock.Anything, []uuid.UUID{maintainers, reviewers}).
				Return(tt.existingGroups, nil).Once()
			repo.EXPECT().ListExistingIdentities(mock.Anything, []uuid.UUID{ada, grace}).
				Return(tt.existingIDs, nil).Maybe()
			if tt.reason == "" {
				expectWrite(repo, writer)
				writer.EXPECT().ListAll(mock.Anything).Return(nil, nil)
				writer.EXPECT().Insert(mock.Anything, mock.Anything).Return(stored, nil)
			}

			_, err := svc.Create(t.Context(), actor, PolicyInput{Name: "x", Source: source})

			if tt.reason == "" {
				require.NoError(t, err)
				return
			}
			assertInvalid(t, err, tt.reason)
		})
	}
}

// TestPolicyService_StatementLimit checks that the statements of all policies are counted in the write, the
// policy that's replaced only once.
func TestPolicyService_StatementLimit(t *testing.T) {
	t.Parallel()
	others := newRecords(1, 2)
	others[0].Source = strings.Repeat(statement, 400)
	others[1].Source = strings.Repeat(statement, 300)
	replaced := stored
	replaced.Source = strings.Repeat(statement, 299)
	const tooMany = "the policies would hold 1001 statements, more than the 1000"

	type call func(svc *policyService, writer *mocks.MockPolicyWriter, statements int) error
	create := func(svc *policyService, writer *mocks.MockPolicyWriter, statements int) error {
		writer.EXPECT().Insert(mock.Anything, mock.Anything).Return(stored, nil).Maybe()
		_, err := svc.Create(t.Context(), actor, PolicyInput{Name: "x", Source: strings.Repeat(statement, statements)})
		return err
	}
	replace := func(svc *policyService, writer *mocks.MockPolicyWriter, statements int) error {
		writer.EXPECT().Update(mock.Anything, mock.Anything).Return(stored, nil).Maybe()
		in := PolicyInput{Name: "x", Source: strings.Repeat(statement, statements)}
		_, err := svc.Replace(t.Context(), actor, stored.ID, in)
		return err
	}

	patchCall := func(svc *policyService, writer *mocks.MockPolicyWriter, statements int) error {
		writer.EXPECT().Get(mock.Anything, stored.ID).Return(replaced, nil)
		writer.EXPECT().Patch(mock.Anything, mock.Anything).Return(stored, nil).Maybe()
		_, err := svc.Patch(t.Context(), actor, stored.ID, PolicyPatch{Source: new(strings.Repeat(statement, statements))})
		return err
	}

	tests := []struct {
		name       string
		call       call
		statements int
		wantReason string
	}{
		// 400 + 300 + 299 of the policy to replace, which creating keeps.
		{name: "create at the limit", call: create, statements: 1},
		{name: "create over the limit", call: create, statements: 2, wantReason: tooMany},
		// 400 + 300, without the 299 of the policy to replace.
		{name: "replace at the limit", call: replace, statements: 300},
		{name: "replace over the limit", call: replace, statements: 301, wantReason: tooMany},
		{name: "patch at the limit", call: patchCall, statements: 300},
		{name: "patch over the limit", call: patchCall, statements: 301, wantReason: tooMany},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo, writer := newTestService(t)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).Maybe()
			expectWrite(repo, writer)
			writer.EXPECT().ListAll(mock.Anything).Return(append(others, replaced), nil)

			err := tt.call(svc, writer, tt.statements)

			if tt.wantReason == "" {
				require.NoError(t, err)
				return
			}
			assertInvalid(t, err, tt.wantReason)
			writer.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
			writer.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
			writer.AssertNotCalled(t, "Patch", mock.Anything, mock.Anything)
		})
	}
}

// TestPolicyService_StatementLimit_UnparsableStoredPolicy checks that a stored policy that can't be parsed,
// which the checks keep from being saved, fails the write rather than being counted as empty.
func TestPolicyService_StatementLimit_UnparsableStoredPolicy(t *testing.T) {
	t.Parallel()
	svc, repo, writer := newTestService(t)
	expectWrite(repo, writer)
	broken := stored
	broken.Source = "not cedar"
	writer.EXPECT().ListAll(mock.Anything).Return([]repository.PolicyRecord{broken}, nil)

	_, err := svc.Create(t.Context(), actor, input)

	require.ErrorContains(t, err, "count statements of policy "+stored.ID.String())
	var invalid *InvalidPolicyError
	assert.NotErrorAs(t, err, &invalid, "a broken stored policy is not the new policy's mistake")
}

func TestPolicyService_Get(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	expectAllFieldsReadable(decider, storedResource)

	got, err := svc.Get(t.Context(), actor, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, storedPolicy, got)
}

// TestPolicyService_Get_HiddenFields checks that only the fields that the actor may read are visible, and
// the ID always.
func TestPolicyService_Get_HiddenFields(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), storedResource, "name", "updatedAt")

	got, err := svc.Get(t.Context(), actor, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, Policy{
		ID:        stored.ID,
		Name:      authz.Visible("everything"),
		UpdatedAt: authz.Visible(stored.UpdatedAt),
	}, got)
}

func TestPolicyService_Get_Hidden(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, _ := newTestServiceWith(t, authorizer)
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	decider.EXPECT().Can(actions.Read(), storedResource).Return(false, nil)

	got, err := svc.Get(t.Context(), actor, stored.ID)

	require.ErrorIs(t, err, ErrPolicyNotFound, "a hidden policy must look like a missing one")
	assert.Equal(t, Policy{}, got)
}

// TestPolicyService_List_Pages cuts the pages out of what the actor may read, with hidden rows between them, so
// that only the last page can be short and HasMore counts readable rows only.
func TestPolicyService_List_Pages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		offset, limit int
		wantIDs       []byte
		wantHasMore   bool
	}{
		{name: "first page", offset: 0, limit: 2, wantIDs: []byte{2, 4}, wantHasMore: true},
		{name: "full last page, with hidden rows after it", offset: 2, limit: 2, wantIDs: []byte{6, 8}},
		{name: "short last page", offset: 3, limit: 2, wantIDs: []byte{8}},
		{name: "past the end", offset: 4, limit: 2, wantIDs: []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo, _ := newTestServiceWith(t, authorizer)
			repo.EXPECT().ListAll(mock.Anything).Return(newRecords(1, 2, 3, 4, 5, 6, 7, 8, 9), nil)
			decider.EXPECT().Can(actions.Read(), mock.Anything).RunAndReturn(
				func(_ authz.Action, resource authz.Resource) (bool, error) {
					assert.Equal(t, "Policy", resource.Type)
					return resource.ID[15]%2 == 0, nil
				},
			).Times(9)
			fieldChecks := map[byte]int{}
			decider.EXPECT().Can(mock.MatchedBy(isReadField), mock.Anything).RunAndReturn(
				func(_ authz.Action, resource authz.Resource) (bool, error) {
					fieldChecks[resource.ID[15]]++
					return true, nil
				},
			).Maybe()

			got, err := svc.List(t.Context(), actor, tt.offset, tt.limit)
			require.NoError(t, err)

			assert.Equal(t, tt.wantIDs, idsOf(got.Items))
			assert.Equal(t, tt.wantHasMore, got.HasMore)
			wantFieldChecks := map[byte]int{}
			for _, id := range tt.wantIDs {
				wantFieldChecks[id] = len(actions.ReadFields().Fields())
			}
			assert.Equal(t, wantFieldChecks, fieldChecks, "the fields of the rows on the page must be checked, and only those")
		})
	}
}

func TestPolicyService_List_TranslatesRecords(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	repo.EXPECT().ListAll(mock.Anything).Return([]repository.PolicyRecord{stored}, nil)

	got, err := svc.List(t.Context(), actor, 0, 10)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Policy]{Items: []Policy{storedPolicy}}, got)
}

func TestPolicyService_List_ChecksContextBetweenRows(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, _ := newTestServiceWith(t, authorizer)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	repo.EXPECT().ListAll(mock.Anything).Return(newRecords(1, 2, 3), nil)
	decider.EXPECT().Can(actions.Read(), mock.Anything).RunAndReturn(
		func(authz.Action, authz.Resource) (bool, error) {
			cancel()
			return true, nil
		},
	).Once()

	_, err := svc.List(ctx, actor, 0, 5)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPolicyService_Replace(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, writer := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	for _, field := range []string{"name", "source"} {
		decider.EXPECT().Can(authz.NewAction("policy:update:"+field, "policy:update"), storedResource).
			Return(true, nil).Once()
	}
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), storedResource, "name")
	expectWrite(repo, writer)
	writer.EXPECT().ListAll(mock.Anything).Return([]repository.PolicyRecord{stored}, nil)
	writer.EXPECT().Update(mock.Anything, repository.PolicyRecord{
		ID:        stored.ID,
		Name:      "read-identities",
		Source:    readSource,
		UpdatedBy: actor,
		UpdatedAt: now,
	}).Return(stored, nil)

	got, err := svc.Replace(t.Context(), actor, stored.ID, input)
	require.NoError(t, err)

	// The response to a write is masked like a read.
	assert.Equal(t, Policy{ID: stored.ID, Name: authz.Visible("everything")}, got)
}

// TestPolicyService_Replace_FieldsDenied checks that replacing checks every field, even one whose value doesn't
// change, and that a denial lists every denied field, before the source is checked, and writes nothing.
func TestPolicyService_Replace_FieldsDenied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mocks fail the test if the repository writes anything.
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(authz.NewAction("policy:update:name", "policy:update"), storedResource).Return(true, nil)
	decider.EXPECT().Can(authz.NewAction("policy:update:source", "policy:update"), storedResource).Return(false, nil)
	unchanged := PolicyInput{Name: "renamed", Source: stored.Source}

	_, err := svc.Replace(t.Context(), actor, stored.ID, unchanged)

	var denied *authz.FieldsDeniedError
	require.ErrorAs(t, err, &denied)
	assert.Equal(t, []string{"source"}, denied.Fields)
}

func TestPolicyService_Patch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		patch PolicyPatch
		// fields are the fields that the patch sets, each of which must be checked.
		fields []string
	}{
		{name: "name", patch: PolicyPatch{Name: new("read-identities")}, fields: []string{"name"}},
		{name: "source", patch: PolicyPatch{Source: new(readSource)}, fields: []string{"source"}},
		{name: "both", patch: patch, fields: []string{"name", "source"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo, writer := newTestServiceWith(t, authorizer)
			expectReadable(repo, decider)
			// The mock fails the test if a field that the patch doesn't set is checked.
			for _, field := range tt.fields {
				decider.EXPECT().Can(authz.NewAction("policy:update:"+field, "policy:update"), storedResource).
					Return(true, nil).Once()
			}
			servicetest.ExpectReadableFields(decider, actions.ReadFields(), storedResource, "name")
			expectWrite(repo, writer)
			writer.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
			writer.EXPECT().ListAll(mock.Anything).Return([]repository.PolicyRecord{stored}, nil)
			writer.EXPECT().Patch(mock.Anything, repository.PolicyPatch{
				ID:        stored.ID,
				Name:      tt.patch.Name,
				Source:    tt.patch.Source,
				UpdatedBy: actor,
				UpdatedAt: now,
			}).Return(stored, nil)

			got, err := svc.Patch(t.Context(), actor, stored.ID, tt.patch)
			require.NoError(t, err)

			// The response to a write is masked like a read.
			assert.Equal(t, Policy{ID: stored.ID, Name: authz.Visible("everything")}, got)
		})
	}
}

// TestPolicyService_Patch_Empty checks that a patch that sets no field returns the policy as it is, with no
// write, which would advance the revision, and no write decision.
func TestPolicyService_Patch_Empty(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mocks fail the test on a write decision or a write.
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	expectAllFieldsReadable(decider, storedResource)

	got, err := svc.Patch(t.Context(), actor, stored.ID, PolicyPatch{})
	require.NoError(t, err)

	assert.Equal(t, storedPolicy, got)
}

// TestPolicyService_Patch_FieldsDenied checks that a denial lists every denied field of the patch, before the
// source is checked, and writes nothing.
func TestPolicyService_Patch_FieldsDenied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mocks fail the test if the repository writes anything.
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(authz.NewAction("policy:update:name", "policy:update"), storedResource).Return(true, nil)
	decider.EXPECT().Can(authz.NewAction("policy:update:source", "policy:update"), storedResource).Return(false, nil)

	_, err := svc.Patch(t.Context(), actor, stored.ID, PolicyPatch{Name: new("x"), Source: new("not cedar")})

	var denied *authz.FieldsDeniedError
	require.ErrorAs(t, err, &denied)
	assert.Equal(t, []string{"source"}, denied.Fields)
}

// TestPolicyService_Patch_ChecksMergedSource checks that the source that results from a patch is checked like
// a saved one, even if the patch keeps it, and that it's the source read in the write, which no other policy
// write can change until the commit. The references are looked up in the write too, so that it needs no
// second connection: the mocks fail the test if the repository looks them up.
func TestPolicyService_Patch_ChecksMergedSource(t *testing.T) {
	t.Parallel()
	missingGroup := `permit (principal in Group::"` + maintainers.String() + `", action, resource);`
	missingIdentity := `forbid (principal == Identity::"` + ada.String() + `", action, resource);`
	tests := []struct {
		name string
		// current is the source that the write reads.
		current string
		patch   PolicyPatch
		reason  string
	}{
		{
			name: "new source", current: stored.Source, patch: PolicyPatch{Source: new("permit (")},
			reason: "the source can't be parsed",
		},
		{name: "kept source", current: "not cedar", patch: PolicyPatch{Name: new("x")}, reason: "the source can't be parsed"},
		{
			name: "kept source with a deleted group", current: missingGroup, patch: PolicyPatch{Name: new("x")},
			reason: `the group "` + maintainers.String() + `" does not exist`,
		},
		{
			name: "kept source with a deleted identity", current: missingIdentity, patch: PolicyPatch{Name: new("x")},
			reason: `the identity "` + ada.String() + `" does not exist`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The mocks fail the test if the writer patches anything.
			svc, repo, writer := newTestService(t)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
			writer.EXPECT().ListExistingGroups(mock.Anything, mock.Anything).Return(nil, nil).Maybe()
			writer.EXPECT().ListExistingIdentities(mock.Anything, mock.Anything).Return(nil, nil).Maybe()
			expectWrite(repo, writer)
			current := stored
			current.Source = tt.current
			writer.EXPECT().Get(mock.Anything, stored.ID).Return(current, nil)

			_, err := svc.Patch(t.Context(), actor, stored.ID, tt.patch)

			assertInvalid(t, err, tt.reason)
		})
	}
}

func TestPolicyService_Delete(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, writer := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(actions.Delete(), storedResource).Return(true, nil)
	expectWrite(repo, writer)
	writer.EXPECT().Delete(mock.Anything, stored.ID).Return(nil)

	assert.NoError(t, svc.Delete(t.Context(), actor, stored.ID))
}

func TestPolicyService_Delete_Denied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mocks fail the test if the repository writes anything.
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(actions.Delete(), storedResource).Return(false, nil)

	err := svc.Delete(t.Context(), actor, stored.ID)

	require.ErrorIs(t, err, authz.ErrDenied)
	assert.NotErrorIs(t, err, ErrPolicyNotFound)
}

// TestPolicyService_Writes_Hidden checks that a write to a policy that the actor may not read is reported like
// a missing policy, before any write or the source is checked: either would otherwise reveal that it
// exists.
func TestPolicyService_Writes_Hidden(t *testing.T) {
	t.Parallel()
	calls := map[string]func(*policyService) error{
		"replace": func(svc *policyService) error {
			_, err := svc.Replace(t.Context(), actor, stored.ID, PolicyInput{Name: "x", Source: "not cedar"})
			return err
		},
		"patch": func(svc *policyService) error {
			_, err := svc.Patch(t.Context(), actor, stored.ID, PolicyPatch{Source: new("not cedar")})
			return err
		},
		"delete": func(svc *policyService) error {
			return svc.Delete(t.Context(), actor, stored.ID)
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mocks fail the test on any other decision or on a write.
			svc, repo, _ := newTestServiceWith(t, authorizer)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
			decider.EXPECT().Can(actions.Read(), storedResource).Return(false, nil)

			err := call(svc)

			require.ErrorIs(t, err, ErrPolicyNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
	}
}

type (
	mockRepo   = mocks.MockPolicyRepository
	mockWriter = mocks.MockPolicyWriter
)

// repoCall makes a service call whose repository fails with repoErr.
type repoCall func(ctx context.Context, svc *policyService, repo *mockRepo, w *mockWriter, repoErr error) error

// repoCalls returns the service calls of TestPolicyService_TranslatesRepositoryErrors, by name.
func repoCalls() map[string]repoCall {
	// groupSource and identitySource reference a group and an identity, for the calls that fail to look them
	// up.
	groupSource := `permit (principal in Group::"` + maintainers.String() + `", action, resource);`
	identitySource := `permit (principal == Identity::"` + ada.String() + `", action, resource);`
	readable := func(repo *mocks.MockPolicyRepository) {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	}

	return map[string]repoCall{
		"create": func(ctx context.Context, svc *policyService, repo *mockRepo, w *mockWriter, repoErr error) error {
			expectWrite(repo, w)
			w.EXPECT().ListAll(mock.Anything).Return(nil, nil)
			w.EXPECT().Insert(mock.Anything, mock.Anything).Return(repository.PolicyRecord{}, repoErr)
			_, err := svc.Create(ctx, actor, input)
			return err
		},
		"create groups": func(ctx context.Context, svc *policyService, repo *mockRepo, _ *mockWriter, repoErr error) error {
			repo.EXPECT().ListExistingGroups(mock.Anything, mock.Anything).Return(nil, repoErr)
			_, err := svc.Create(ctx, actor, PolicyInput{Name: "x", Source: groupSource})
			return err
		},
		"create identities": func(
			ctx context.Context, svc *policyService, repo *mockRepo, _ *mockWriter, repoErr error,
		) error {
			repo.EXPECT().ListExistingIdentities(mock.Anything, mock.Anything).Return(nil, repoErr)
			_, err := svc.Create(ctx, actor, PolicyInput{Name: "x", Source: identitySource})
			return err
		},
		"create count": func(ctx context.Context, svc *policyService, repo *mockRepo, w *mockWriter, repoErr error) error {
			expectWrite(repo, w)
			w.EXPECT().ListAll(mock.Anything).Return(nil, repoErr)
			_, err := svc.Create(ctx, actor, input)
			return err
		},
		"create write": func(ctx context.Context, svc *policyService, repo *mockRepo, _ *mockWriter, repoErr error) error {
			repo.EXPECT().Write(mock.Anything, mock.Anything).Return(repoErr)
			_, err := svc.Create(ctx, actor, input)
			return err
		},
		"get": func(ctx context.Context, svc *policyService, repo *mockRepo, _ *mockWriter, repoErr error) error {
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.PolicyRecord{}, repoErr)
			_, err := svc.Get(ctx, actor, stored.ID)
			return err
		},
		"list": func(ctx context.Context, svc *policyService, repo *mockRepo, _ *mockWriter, repoErr error) error {
			repo.EXPECT().ListAll(mock.Anything).Return(nil, repoErr)
			_, err := svc.List(ctx, actor, 0, 5)
			return err
		},
		"replace get": func(ctx context.Context, svc *policyService, repo *mockRepo, _ *mockWriter, repoErr error) error {
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.PolicyRecord{}, repoErr)
			_, err := svc.Replace(ctx, actor, stored.ID, input)
			return err
		},
		"replace": func(ctx context.Context, svc *policyService, repo *mockRepo, w *mockWriter, repoErr error) error {
			readable(repo)
			expectWrite(repo, w)
			w.EXPECT().ListAll(mock.Anything).Return(nil, nil)
			w.EXPECT().Update(mock.Anything, mock.Anything).Return(repository.PolicyRecord{}, repoErr)
			_, err := svc.Replace(ctx, actor, stored.ID, input)
			return err
		},
		"patch get": func(ctx context.Context, svc *policyService, repo *mockRepo, _ *mockWriter, repoErr error) error {
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.PolicyRecord{}, repoErr)
			_, err := svc.Patch(ctx, actor, stored.ID, patch)
			return err
		},
		"patch load": func(ctx context.Context, svc *policyService, repo *mockRepo, w *mockWriter, repoErr error) error {
			readable(repo)
			expectWrite(repo, w)
			w.EXPECT().Get(mock.Anything, stored.ID).Return(repository.PolicyRecord{}, repoErr)
			_, err := svc.Patch(ctx, actor, stored.ID, patch)
			return err
		},
		"patch groups": func(ctx context.Context, svc *policyService, repo *mockRepo, w *mockWriter, repoErr error) error {
			readable(repo)
			expectWrite(repo, w)
			w.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
			w.EXPECT().ListExistingGroups(mock.Anything, mock.Anything).Return(nil, repoErr)
			_, err := svc.Patch(ctx, actor, stored.ID, PolicyPatch{Source: &groupSource})
			return err
		},
		"patch": func(ctx context.Context, svc *policyService, repo *mockRepo, w *mockWriter, repoErr error) error {
			readable(repo)
			expectWrite(repo, w)
			w.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
			w.EXPECT().ListAll(mock.Anything).Return(nil, nil)
			w.EXPECT().Patch(mock.Anything, mock.Anything).Return(repository.PolicyRecord{}, repoErr)
			_, err := svc.Patch(ctx, actor, stored.ID, patch)
			return err
		},
		"delete get": func(ctx context.Context, svc *policyService, repo *mockRepo, _ *mockWriter, repoErr error) error {
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.PolicyRecord{}, repoErr)
			return svc.Delete(ctx, actor, stored.ID)
		},
		"delete": func(ctx context.Context, svc *policyService, repo *mockRepo, w *mockWriter, repoErr error) error {
			readable(repo)
			expectWrite(repo, w)
			w.EXPECT().Delete(mock.Anything, stored.ID).Return(repoErr)
			return svc.Delete(ctx, actor, stored.ID)
		},
	}
}

func TestPolicyService_TranslatesRepositoryErrors(t *testing.T) {
	t.Parallel()
	calls := repoCalls()
	tests := []struct {
		call    string
		repoErr error
		wantErr error
	}{
		{call: "create", repoErr: repository.ErrDuplicateName, wantErr: ErrPolicyNameTaken},
		{call: "create", repoErr: errDB, wantErr: errDB},
		{call: "create groups", repoErr: errDB, wantErr: errDB},
		{call: "create identities", repoErr: errDB, wantErr: errDB},
		{call: "create count", repoErr: errDB, wantErr: errDB},
		{call: "create write", repoErr: errDB, wantErr: errDB},
		{call: "get", repoErr: repository.ErrNotFound, wantErr: ErrPolicyNotFound},
		{call: "get", repoErr: errDB, wantErr: errDB},
		{call: "list", repoErr: errDB, wantErr: errDB},
		{call: "replace get", repoErr: repository.ErrNotFound, wantErr: ErrPolicyNotFound},
		{call: "replace get", repoErr: errDB, wantErr: errDB},
		{call: "replace", repoErr: repository.ErrNotFound, wantErr: ErrPolicyNotFound},
		{call: "replace", repoErr: repository.ErrDuplicateName, wantErr: ErrPolicyNameTaken},
		{call: "replace", repoErr: errDB, wantErr: errDB},
		{call: "patch get", repoErr: repository.ErrNotFound, wantErr: ErrPolicyNotFound},
		{call: "patch get", repoErr: errDB, wantErr: errDB},
		// The policy was deleted after it was read.
		{call: "patch load", repoErr: repository.ErrNotFound, wantErr: ErrPolicyNotFound},
		{call: "patch load", repoErr: errDB, wantErr: errDB},
		{call: "patch groups", repoErr: errDB, wantErr: errDB},
		{call: "patch", repoErr: repository.ErrNotFound, wantErr: ErrPolicyNotFound},
		{call: "patch", repoErr: repository.ErrDuplicateName, wantErr: ErrPolicyNameTaken},
		{call: "patch", repoErr: errDB, wantErr: errDB},
		{call: "delete get", repoErr: repository.ErrNotFound, wantErr: ErrPolicyNotFound},
		{call: "delete get", repoErr: errDB, wantErr: errDB},
		{call: "delete", repoErr: repository.ErrNotFound, wantErr: ErrPolicyNotFound},
		{call: "delete", repoErr: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.call+" "+tt.repoErr.Error(), func(t *testing.T) {
			t.Parallel()
			svc, repo, writer := newTestService(t)
			// The repository wraps its errors with context.
			err := calls[tt.call](t.Context(), svc, repo, writer, fmt.Errorf("query: %w", tt.repoErr))

			servicetest.AssertTranslated(t, err, tt.wantErr, tt.repoErr)
		})
	}
}

// TestPolicyService_AuthorizationFails checks that failing to decide is an error and never an answer: not
// access, and not a missing policy, which would hide a broken policy from the logs.
func TestPolicyService_AuthorizationFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")

	calls := map[string]func(*policyService) error{
		"create": func(svc *policyService) error {
			_, err := svc.Create(t.Context(), actor, input)
			return err
		},
		"get": func(svc *policyService) error {
			_, err := svc.Get(t.Context(), actor, stored.ID)
			return err
		},
		"list": func(svc *policyService) error {
			_, err := svc.List(t.Context(), actor, 0, 5)
			return err
		},
		"replace": func(svc *policyService) error {
			_, err := svc.Replace(t.Context(), actor, stored.ID, input)
			return err
		},
		"patch": func(svc *policyService) error {
			_, err := svc.Patch(t.Context(), actor, stored.ID, patch)
			return err
		},
		"delete": func(svc *policyService) error {
			return svc.Delete(t.Context(), actor, stored.ID)
		},
	}
	for name, call := range calls {
		t.Run(name+" no decider", func(t *testing.T) {
			t.Parallel()
			authorizer := authzmocks.NewMockAuthorizer(t)
			authorizer.EXPECT().For(mock.Anything, actor).Return(nil, errDecision)
			// The mocks fail the test if the repository is called.
			svc, _, _ := newTestServiceWith(t, authorizer)

			err := call(svc)

			require.ErrorIs(t, err, errDecision)
			require.NotErrorIs(t, err, ErrPolicyNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
		t.Run(name+" no decision", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			decider.EXPECT().Can(mock.Anything, mock.Anything).Return(false, errDecision)
			// The mocks fail the test if the repository writes anything.
			svc, repo, _ := newTestServiceWith(t, authorizer)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).Maybe()
			repo.EXPECT().ListAll(mock.Anything).Return([]repository.PolicyRecord{stored}, nil).Maybe()

			err := call(svc)

			require.ErrorIs(t, err, errDecision)
			require.NotErrorIs(t, err, ErrPolicyNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
		// The field read decisions, after every other decision allowed the request. They come before any
		// write, so that a failed one can't fail a request whose write succeeded.
		if name != "delete" {
			t.Run(name+" no field decision", func(t *testing.T) {
				t.Parallel()
				authorizer, decider := newDeciding(t)
				decider.EXPECT().Can(mock.MatchedBy(isReadField), mock.Anything).Return(false, errDecision)
				decider.EXPECT().Can(mock.Anything, mock.Anything).Return(true, nil).Maybe()
				// The mocks fail the test if the repository writes anything.
				svc, repo, _ := newTestServiceWith(t, authorizer)
				repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).Maybe()
				repo.EXPECT().ListAll(mock.Anything).Return([]repository.PolicyRecord{stored}, nil).Maybe()

				err := call(svc)

				require.ErrorIs(t, err, errDecision)
				assert.NotErrorIs(t, err, authz.ErrDenied)
			})
		}
		// The write decisions, after the actor was allowed to read the policy.
		if name == "replace" || name == "patch" || name == "delete" {
			t.Run(name+" no write decision", func(t *testing.T) {
				t.Parallel()
				authorizer, decider := newDeciding(t)
				svc, repo, _ := newTestServiceWith(t, authorizer)
				expectReadable(repo, decider)
				decider.EXPECT().Can(mock.Anything, storedResource).Return(false, errDecision)

				err := call(svc)

				require.ErrorIs(t, err, errDecision)
				assert.NotErrorIs(t, err, authz.ErrDenied)
			})
		}
	}
}
