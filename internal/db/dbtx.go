package db

import (
	"context"
	"database/sql"
)

// DBTX is the common interface satisfied by both *sql.DB and *sql.Tx.
// Repository implementations depend on this interface instead of the
// concrete *sql.DB, enabling transactional composition.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Compile-time verification that *sql.DB and *sql.Tx satisfy DBTX.
var (
	_ DBTX = (*sql.DB)(nil)
	_ DBTX = (*sql.Tx)(nil)
)
