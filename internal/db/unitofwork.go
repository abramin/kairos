package db

import (
	"context"
	"database/sql"
	"fmt"
)

// UnitOfWork manages transactional boundaries. The callback receives a DBTX
// backed by a *sql.Tx; callers create tx-scoped repositories from it.
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx DBTX) error) error
}

// SQLiteUnitOfWork implements UnitOfWork using database/sql transactions.
type SQLiteUnitOfWork struct {
	db *sql.DB
}

// NewSQLiteUnitOfWork creates a UnitOfWork backed by the given *sql.DB.
func NewSQLiteUnitOfWork(db *sql.DB) *SQLiteUnitOfWork {
	return &SQLiteUnitOfWork{db: db}
}

func (u *SQLiteUnitOfWork) WithinTx(ctx context.Context, fn func(ctx context.Context, tx DBTX) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
