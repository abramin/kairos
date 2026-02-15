package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"

	"github.com/alexanderramin/kairos/internal/db"
)

// FailOnNthExecUoW is a test UoW that injects an error on the Nth ExecContext
// call within a transaction. This enables rollback integration tests by
// simulating failures at precise points in multi-write operations.
//
// ExecContext calls are counted starting at 1. QueryContext and QueryRowContext
// are not counted (reads pass through normally).
type FailOnNthExecUoW struct {
	DB     *sql.DB
	FailOn int32
	Err    error
}

func (u *FailOnNthExecUoW) WithinTx(ctx context.Context, fn func(ctx context.Context, tx db.DBTX) error) error {
	tx, err := u.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	wrapped := &failOnNthExec{DBTX: tx, failOn: u.FailOn, err: u.Err}
	if fnErr := fn(ctx, wrapped); fnErr != nil {
		_ = tx.Rollback()
		return fnErr
	}
	return tx.Commit()
}

type failOnNthExec struct {
	db.DBTX
	count  atomic.Int32
	failOn int32
	err    error
}

func (f *failOnNthExec) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	n := f.count.Add(1)
	if n == f.failOn {
		return nil, f.err
	}
	return f.DBTX.ExecContext(ctx, query, args...)
}
