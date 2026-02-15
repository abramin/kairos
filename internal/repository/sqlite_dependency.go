package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteDependencyRepo implements DependencyRepo using a SQLite database.
type SQLiteDependencyRepo struct {
	db db.DBTX
}

// NewSQLiteDependencyRepo creates a new SQLiteDependencyRepo.
func NewSQLiteDependencyRepo(conn db.DBTX) *SQLiteDependencyRepo {
	return &SQLiteDependencyRepo{db: conn}
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

func (r *SQLiteDependencyRepo) ListBlockedWorkItemIDs(ctx context.Context, candidateIDs []string) (map[string]bool, error) {
	if len(candidateIDs) == 0 {
		return make(map[string]bool), nil
	}

	placeholders := make([]string, len(candidateIDs))
	args := make([]any, len(candidateIDs))
	for i, id := range candidateIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `SELECT DISTINCT d.successor_work_item_id
		FROM dependencies d
		JOIN work_items w ON d.predecessor_work_item_id = w.id
		WHERE d.successor_work_item_id IN (` + strings.Join(placeholders, ",") + `)
		  AND w.status NOT IN ('done', 'skipped', 'archived')`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing blocked work item IDs: %w", err)
	}
	defer rows.Close()

	blocked := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning blocked work item ID: %w", err)
		}
		blocked[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating blocked work item IDs: %w", err)
	}
	return blocked, nil
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
