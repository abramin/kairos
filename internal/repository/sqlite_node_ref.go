package repository

import (
	"context"
	"database/sql"

	"github.com/alexanderramin/kairos/internal/db"
)

type SQLiteNodeRefRepo struct {
	db db.DBTX
}

func NewSQLiteNodeRefRepo(dbConn db.DBTX) *SQLiteNodeRefRepo {
	return &SQLiteNodeRefRepo{db: dbConn}
}

// Set stores or updates a node ref mapping (UPSERT).
func (r *SQLiteNodeRefRepo) Set(ctx context.Context, nodeID, projectID, ref string) error {
	query := `INSERT INTO node_refs (node_id, project_id, ref)
		VALUES (?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET ref = excluded.ref, project_id = excluded.project_id`
	_, err := r.db.ExecContext(ctx, query, nodeID, projectID, ref)
	return err
}

// GetByProjectAndRef looks up a node by project and ref. Returns empty string if not found.
func (r *SQLiteNodeRefRepo) GetByProjectAndRef(ctx context.Context, projectID, ref string) (string, error) {
	var nodeID string
	query := `SELECT node_id FROM node_refs WHERE project_id = ? AND ref = ?`
	err := r.db.QueryRowContext(ctx, query, projectID, ref).Scan(&nodeID)
	if err == sql.ErrNoRows {
		return "", nil // not found is not an error
	}
	return nodeID, err
}

// DeleteByNodeID removes a node ref mapping.
func (r *SQLiteNodeRefRepo) DeleteByNodeID(ctx context.Context, nodeID string) error {
	query := `DELETE FROM node_refs WHERE node_id = ?`
	_, err := r.db.ExecContext(ctx, query, nodeID)
	return err
}
