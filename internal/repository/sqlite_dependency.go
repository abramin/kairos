package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteDependencyRepo implements DependencyRepo using a SQLite database.
type SQLiteDependencyRepo struct {
	db *sql.DB
}

// NewSQLiteDependencyRepo creates a new SQLiteDependencyRepo.
func NewSQLiteDependencyRepo(db *sql.DB) *SQLiteDependencyRepo {
	return &SQLiteDependencyRepo{db: db}
}

func (r *SQLiteDependencyRepo) Create(ctx context.Context, d *domain.Dependency) error {
	query := `INSERT INTO dependencies (predecessor_work_item_id, successor_work_item_id) VALUES (?, ?)`
	_, err := r.db.ExecContext(ctx, query, d.PredecessorWorkItemID, d.SuccessorWorkItemID)
	if err != nil {
		return fmt.Errorf("inserting dependency: %w", err)
	}
	return nil
}

func (r *SQLiteDependencyRepo) Delete(ctx context.Context, predecessorID, successorID string) error {
	query := `DELETE FROM dependencies WHERE predecessor_work_item_id = ? AND successor_work_item_id = ?`
	_, err := r.db.ExecContext(ctx, query, predecessorID, successorID)
	if err != nil {
		return fmt.Errorf("deleting dependency: %w", err)
	}
	return nil
}

func (r *SQLiteDependencyRepo) ListPredecessors(ctx context.Context, workItemID string) ([]domain.Dependency, error) {
	query := `SELECT predecessor_work_item_id, successor_work_item_id
		FROM dependencies WHERE successor_work_item_id = ?`
	rows, err := r.db.QueryContext(ctx, query, workItemID)
	if err != nil {
		return nil, fmt.Errorf("listing predecessors: %w", err)
	}
	defer rows.Close()
	return r.scanDependencies(rows)
}

func (r *SQLiteDependencyRepo) ListSuccessors(ctx context.Context, workItemID string) ([]domain.Dependency, error) {
	query := `SELECT predecessor_work_item_id, successor_work_item_id
		FROM dependencies WHERE predecessor_work_item_id = ?`
	rows, err := r.db.QueryContext(ctx, query, workItemID)
	if err != nil {
		return nil, fmt.Errorf("listing successors: %w", err)
	}
	defer rows.Close()
	return r.scanDependencies(rows)
}

func (r *SQLiteDependencyRepo) HasUnfinishedPredecessors(ctx context.Context, workItemID string) (bool, error) {
	query := `SELECT COUNT(*) FROM dependencies d
		JOIN work_items w ON d.predecessor_work_item_id = w.id
		WHERE d.successor_work_item_id = ?
		  AND w.status NOT IN ('done', 'skipped', 'archived')`
	var count int
	err := r.db.QueryRowContext(ctx, query, workItemID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking unfinished predecessors: %w", err)
	}
	return count > 0, nil
}

// scanDependencies scans multiple dependency rows from *sql.Rows.
func (r *SQLiteDependencyRepo) scanDependencies(rows *sql.Rows) ([]domain.Dependency, error) {
	var deps []domain.Dependency
	for rows.Next() {
		var d domain.Dependency
		if err := rows.Scan(&d.PredecessorWorkItemID, &d.SuccessorWorkItemID); err != nil {
			return nil, fmt.Errorf("scanning dependency: %w", err)
		}
		deps = append(deps, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating dependencies: %w", err)
	}
	return deps, nil
}
