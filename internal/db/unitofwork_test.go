package db_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *db.SQLiteUnitOfWork {
	t.Helper()
	database, err := db.OpenDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })

	// Create a simple test table outside the migration set.
	_, err = database.Exec(`CREATE TABLE IF NOT EXISTS uow_test (id TEXT PRIMARY KEY, val TEXT)`)
	require.NoError(t, err)

	return db.NewSQLiteUnitOfWork(database)
}

// readVal is a helper that reads val for a given id using the underlying DB.
func readVal(uow *db.SQLiteUnitOfWork, id string) (string, bool) {
	// We need a raw *sql.DB to read outside transactions.
	// Re-use WithinTx with a read-only operation for simplicity.
	var val string
	var found bool
	_ = uow.WithinTx(context.Background(), func(ctx context.Context, tx db.DBTX) error {
		row := tx.QueryRowContext(ctx, `SELECT val FROM uow_test WHERE id = ?`, id)
		if err := row.Scan(&val); err != nil {
			return nil // not found
		}
		found = true
		return nil
	})
	return val, found
}

func TestWithinTx_CommitOnSuccess(t *testing.T) {
	uow := openTestDB(t)

	err := uow.WithinTx(context.Background(), func(ctx context.Context, tx db.DBTX) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO uow_test (id, val) VALUES (?, ?)`, "k1", "v1")
		return err
	})
	require.NoError(t, err)

	val, found := readVal(uow, "k1")
	assert.True(t, found, "row should exist after commit")
	assert.Equal(t, "v1", val)
}

func TestWithinTx_RollbackOnError(t *testing.T) {
	uow := openTestDB(t)

	err := uow.WithinTx(context.Background(), func(ctx context.Context, tx db.DBTX) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO uow_test (id, val) VALUES (?, ?)`, "k2", "v2")
		if err != nil {
			return err
		}
		return fmt.Errorf("deliberate failure")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deliberate failure")

	_, found := readVal(uow, "k2")
	assert.False(t, found, "row should not exist after rollback")
}

func TestWithinTx_RollbackOnPanic(t *testing.T) {
	uow := openTestDB(t)

	assert.Panics(t, func() {
		_ = uow.WithinTx(context.Background(), func(ctx context.Context, tx db.DBTX) error {
			_, _ = tx.ExecContext(ctx, `INSERT INTO uow_test (id, val) VALUES (?, ?)`, "k3", "v3")
			panic("boom")
		})
	})

	_, found := readVal(uow, "k3")
	assert.False(t, found, "row should not exist after panic rollback")
}
