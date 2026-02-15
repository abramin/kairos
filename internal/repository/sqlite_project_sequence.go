package repository

import (
	"context"
	"fmt"

	"github.com/alexanderramin/kairos/internal/db"
)

// SQLiteProjectSequenceRepo allocates project-scoped sequence values
// atomically using the project_sequences table.
type SQLiteProjectSequenceRepo struct {
	db db.DBTX
}

// NewSQLiteProjectSequenceRepo creates a new SQLiteProjectSequenceRepo.
func NewSQLiteProjectSequenceRepo(conn db.DBTX) *SQLiteProjectSequenceRepo {
	return &SQLiteProjectSequenceRepo{db: conn}
}

// NextProjectSeq returns the next available sequential ID for a project.
// Allocation is atomic and safe under concurrent writes.
func (r *SQLiteProjectSequenceRepo) NextProjectSeq(ctx context.Context, projectID string) (int, error) {
	seedQuery := `INSERT OR IGNORE INTO project_sequences (project_id, next_seq)
		SELECT ?, COALESCE(MAX(seq_val), 0) + 1
		FROM (
			SELECT seq AS seq_val FROM plan_nodes WHERE project_id = ? AND seq > 0
			UNION ALL
			SELECT w.seq AS seq_val FROM work_items w
			JOIN plan_nodes n ON w.node_id = n.id
			WHERE n.project_id = ? AND w.seq > 0
		)`
	if _, err := r.db.ExecContext(ctx, seedQuery, projectID, projectID, projectID); err != nil {
		return 0, fmt.Errorf("seeding project sequence for %s: %w", projectID, err)
	}

	var next int
	allocQuery := `UPDATE project_sequences
		SET next_seq = next_seq + 1
		WHERE project_id = ?
		RETURNING next_seq - 1`
	if err := r.db.QueryRowContext(ctx, allocQuery, projectID).Scan(&next); err != nil {
		return 0, fmt.Errorf("allocating next seq for project %s: %w", projectID, err)
	}

	return next, nil
}
