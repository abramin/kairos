package repository

import (
	"context"
	"database/sql"

	"github.com/alexanderramin/kairos/internal/db"
)

type SQLiteWorkItemRefRepo struct {
	db db.DBTX
}

func NewSQLiteWorkItemRefRepo(dbConn db.DBTX) *SQLiteWorkItemRefRepo {
	return &SQLiteWorkItemRefRepo{db: dbConn}
}

// Set stores or updates a work item ref mapping (UPSERT).
func (r *SQLiteWorkItemRefRepo) Set(ctx context.Context, workItemID, projectID, ref string) error {
	query := `INSERT INTO workitem_refs (work_item_id, project_id, ref)
		VALUES (?, ?, ?)
		ON CONFLICT(work_item_id) DO UPDATE SET ref = excluded.ref, project_id = excluded.project_id`
	_, err := r.db.ExecContext(ctx, query, workItemID, projectID, ref)
	return err
}

// GetByProjectAndRef looks up a work item by project and ref. Returns empty string if not found.
func (r *SQLiteWorkItemRefRepo) GetByProjectAndRef(ctx context.Context, projectID, ref string) (string, error) {
	var workItemID string
	query := `SELECT work_item_id FROM workitem_refs WHERE project_id = ? AND ref = ?`
	err := r.db.QueryRowContext(ctx, query, projectID, ref).Scan(&workItemID)
	if err == sql.ErrNoRows {
		return "", nil // not found is not an error
	}
	return workItemID, err
}

// DeleteByWorkItemID removes a work item ref mapping.
func (r *SQLiteWorkItemRefRepo) DeleteByWorkItemID(ctx context.Context, workItemID string) error {
	query := `DELETE FROM workitem_refs WHERE work_item_id = ?`
	_, err := r.db.ExecContext(ctx, query, workItemID)
	return err
}
