package databasetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Migrated(t *testing.T) {
	t.Parallel()
	db := New(t)

	var n int
	require.NoError(t, db.GetContext(t.Context(), &n, "SELECT count(*) FROM identities"))

	assert.Zero(t, n)
}

func TestNew_Isolated(t *testing.T) {
	t.Parallel()
	first, second := New(t), New(t)

	_, err := first.ExecContext(t.Context(), "CREATE TABLE only_in_first (id int)")
	require.NoError(t, err)

	var exists bool
	require.NoError(t, second.GetContext(t.Context(), &exists, "SELECT to_regclass('only_in_first') IS NOT NULL"))
	assert.False(t, exists)
}
