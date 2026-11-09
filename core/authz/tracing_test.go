package authz

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"pkgsilo/test/databasetest"
	"pkgsilo/test/telemetrytest"
)

// loadSpans returns the ended spans named pkgsilo.authz.load, in the order in which they ended.
func loadSpans(spans []sdktrace.ReadOnlySpan) []sdktrace.ReadOnlySpan {
	var found []sdktrace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() == "pkgsilo.authz.load" {
			found = append(found, span)
		}
	}

	return found
}

// reloaded returns the value of pkgsilo.authz.policies.reloaded on span, failing the test if it's not set.
func reloaded(t *testing.T, span sdktrace.ReadOnlySpan) bool {
	t.Helper()

	attrs := attribute.NewSet(span.Attributes()...)
	value, ok := attrs.Value("pkgsilo.authz.policies.reloaded")
	require.True(t, ok, "pkgsilo.authz.policies.reloaded is not set")

	return value.AsBool()
}

// queryNames returns the names of the spans whose parent is span.
func queryNames(spans []sdktrace.ReadOnlySpan, parent sdktrace.ReadOnlySpan) []string {
	var names []string
	for _, span := range spans {
		if span.Parent().SpanID() == parent.SpanContext().SpanID() {
			names = append(names, span.Name())
		}
	}

	return names
}

// TestEngine_For_LoadSpan checks that For runs its queries in the span pkgsilo.authz.load, which tells whether
// it parsed the policies: the first For does, and a second one on the same revision doesn't.
func TestEngine_For_LoadSpan(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithProviders(t, rec.MeterProvider(), rec.TracerProvider())
	ada := insertIdentity(t, db)
	engine := New(db, rec.MeterProvider(), rec.TracerProvider())

	for range 2 {
		_, err := engine.For(t.Context(), ada)
		require.NoError(t, err)
	}

	spans := rec.Spans()
	loads := loadSpans(spans)
	require.Len(t, loads, 2)
	for _, span := range loads {
		assert.Equal(t, "pkgsilo/core/authz", span.InstrumentationScope().Name)
		assert.Equal(t, codes.Unset, span.Status().Code)
	}
	assert.True(t, reloaded(t, loads[0]), "the first For parses the policies")
	assert.False(t, reloaded(t, loads[1]), "a For on the same revision reuses them")
	// The revision, the policies and the groups, then the revision and the groups.
	assert.Equal(t, []string{"SELECT", "SELECT", "SELECT"}, queryNames(spans, loads[0]))
	assert.Equal(t, []string{"SELECT", "SELECT"}, queryNames(spans, loads[1]))
}

// TestEngine_For_LoadSpanFails checks that the span of a For that fails has status Error, whether a query or
// the parse of the policies fails.
func TestEngine_For_LoadSpanFails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		sabotage func(t *testing.T, db *sqlx.DB)
	}{
		{name: "revision", sabotage: func(t *testing.T, db *sqlx.DB) {
			t.Helper()
			_, err := db.ExecContext(t.Context(), `DROP TABLE policy_revision`)
			require.NoError(t, err)
		}},
		{name: "groups", sabotage: func(t *testing.T, db *sqlx.DB) {
			t.Helper()
			_, err := db.ExecContext(t.Context(), `DROP TABLE group_members`)
			require.NoError(t, err)
		}},
		{name: "parse", sabotage: func(t *testing.T, db *sqlx.DB) {
			t.Helper()
			addPolicy(t, db, `permit (`)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := telemetrytest.New(t)
			db := databasetest.New(t)
			ada := insertIdentity(t, db)
			tt.sabotage(t, db)

			_, err := New(db, rec.MeterProvider(), rec.TracerProvider()).For(t.Context(), ada)
			require.Error(t, err)

			loads := loadSpans(rec.Spans())
			require.Len(t, loads, 1)
			assert.Equal(t, codes.Error, loads[0].Status().Code)
			assert.Equal(t, "load policies and groups", loads[0].Status().Description)
		})
	}
}
