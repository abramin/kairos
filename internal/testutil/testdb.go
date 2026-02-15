package testutil

import (
	"database/sql"
	"testing"

	"github.com/alexanderramin/kairos/internal/db"
)

// NewTestDB creates an in-memory SQLite database with all migrations applied.
// The database is closed when the test completes.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	return database
}

// NewTestUoW creates a UnitOfWork backed by the given test database.
func NewTestUoW(database *sql.DB) db.UnitOfWork {
	return db.NewSQLiteUnitOfWork(database)
}
