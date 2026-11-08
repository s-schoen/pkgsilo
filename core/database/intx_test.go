package database_test

import (
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/database"
	"pkgsilo/test/databasetest"
)

// newScratchTable creates a table for the test's rows on a new database and returns the database.
func newScratchTable(t *testing.T) *sqlx.DB {
	t.Helper()

	db := databasetest.New(t)
	_, err := db.ExecContext(t.Context(), `CREATE TABLE scratch (n int NOT NULL)`)
	require.NoError(t, err)

	return db
}

// insertRow returns a function for InTx that inserts a row and then returns err.
func insertRow(t *testing.T, err error) func(database.Querier) error {
	t.Helper()

	return func(tx database.Querier) error {
		_, execErr := tx.ExecContext(t.Context(), `INSERT INTO scratch (n) VALUES (1)`)
		require.NoError(t, execErr)

		return err
	}
}

// countRows returns the number of rows in the scratch table.
func countRows(t *testing.T, q database.Querier) int {
	t.Helper()

	var count int
	require.NoError(t, q.GetContext(t.Context(), &count, `SELECT count(*) FROM scratch`))

	return count
}

func TestInTx_DBCommits(t *testing.T) {
	t.Parallel()
	db := newScratchTable(t)

	require.NoError(t, database.InTx(t.Context(), db, insertRow(t, nil)))

	assert.Equal(t, 1, countRows(t, db))
}

func TestInTx_DBRollsBack(t *testing.T) {
	t.Parallel()
	db := newScratchTable(t)
	errFailed := errors.New("failed")

	err := database.InTx(t.Context(), db, insertRow(t, errFailed))

	require.ErrorIs(t, err, errFailed)
	assert.Equal(t, 0, countRows(t, db))
}

// TestInTx_TxTakesPart checks that fn runs in the caller's transaction: what it wrote is visible in it,
// and goes with it when the caller rolls back, even though fn succeeded.
func TestInTx_TxTakesPart(t *testing.T) {
	t.Parallel()
	db := newScratchTable(t)
	tx, err := db.BeginTxx(t.Context(), nil)
	require.NoError(t, err)

	require.NoError(t, database.InTx(t.Context(), tx, insertRow(t, nil)))
	assert.Equal(t, 1, countRows(t, tx), "the caller's transaction must see the row")
	assert.Equal(t, 0, countRows(t, db), "nothing may commit before the caller does")

	require.NoError(t, tx.Rollback())
	assert.Equal(t, 0, countRows(t, db))
}

// TestInTx_TxErrorLeavesTransactionOpen checks that fn's error is returned without ending the caller's
// transaction, which the caller still owns.
func TestInTx_TxErrorLeavesTransactionOpen(t *testing.T) {
	t.Parallel()
	db := newScratchTable(t)
	tx, err := db.BeginTxx(t.Context(), nil)
	require.NoError(t, err)
	errFailed := errors.New("failed")

	err = database.InTx(t.Context(), tx, insertRow(t, errFailed))

	require.ErrorIs(t, err, errFailed)
	require.NoError(t, tx.Commit(), "the caller's transaction must still be open")
	assert.Equal(t, 1, countRows(t, db))
}

// wrappedQuerier is a Querier that wraps another, such as an instrumented connection pool, and neither is a
// transaction nor begins one.
type wrappedQuerier struct {
	database.Querier
}

// TestInTx_OtherQuerierFails checks that a Querier that neither is a transaction nor begins one is an error,
// instead of running fn without a transaction.
func TestInTx_OtherQuerierFails(t *testing.T) {
	t.Parallel()
	ran := false

	err := database.InTx(t.Context(), wrappedQuerier{}, func(database.Querier) error {
		ran = true

		return nil
	})

	require.Error(t, err)
	assert.False(t, ran, "fn must not run without a transaction")
}
