package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteSessionRepo implements SessionRepo using a SQLite database.
type SQLiteSessionRepo struct {
	db *sql.DB
}

// NewSQLiteSessionRepo creates a new SQLiteSessionRepo.
func NewSQLiteSessionRepo(db *sql.DB) *SQLiteSessionRepo {
	return &SQLiteSessionRepo{db: db}
}

func (r *SQLiteSessionRepo) Create(ctx context.Context, s *domain.WorkSessionLog) error {
	query := `INSERT INTO work_session_logs (id, work_item_id, started_at, minutes, units_done_delta, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		s.ID,
		s.WorkItemID,
		s.StartedAt.Format(time.RFC3339),
		s.Minutes,
		s.UnitsDoneDelta,
		s.Note,
		s.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting work session log: %w", err)
	}
	return nil
}

func (r *SQLiteSessionRepo) GetByID(ctx context.Context, id string) (*domain.WorkSessionLog, error) {
	query := `SELECT id, work_item_id, started_at, minutes, units_done_delta, note, created_at
		FROM work_session_logs WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanSession(row)
}

func (r *SQLiteSessionRepo) ListByWorkItem(ctx context.Context, workItemID string) ([]*domain.WorkSessionLog, error) {
	query := `SELECT id, work_item_id, started_at, minutes, units_done_delta, note, created_at
		FROM work_session_logs WHERE work_item_id = ? ORDER BY started_at`
	rows, err := r.db.QueryContext(ctx, query, workItemID)
	if err != nil {
		return nil, fmt.Errorf("listing sessions by work item: %w", err)
	}
	defer rows.Close()
	return r.scanSessions(rows)
}

func (r *SQLiteSessionRepo) ListRecent(ctx context.Context, days int) ([]*domain.WorkSessionLog, error) {
	query := `SELECT id, work_item_id, started_at, minutes, units_done_delta, note, created_at
		FROM work_session_logs
		WHERE started_at >= date('now', ? || ' days')
		ORDER BY started_at DESC`
	rows, err := r.db.QueryContext(ctx, query, fmt.Sprintf("-%d", days))
	if err != nil {
		return nil, fmt.Errorf("listing recent sessions: %w", err)
	}
	defer rows.Close()
	return r.scanSessions(rows)
}

func (r *SQLiteSessionRepo) ListRecentByProject(ctx context.Context, projectID string, days int) ([]*domain.WorkSessionLog, error) {
	query := `SELECT s.id, s.work_item_id, s.started_at, s.minutes, s.units_done_delta, s.note, s.created_at
		FROM work_session_logs s
		JOIN work_items w ON s.work_item_id = w.id
		JOIN plan_nodes n ON w.node_id = n.id
		WHERE n.project_id = ?
		  AND s.started_at >= date('now', ? || ' days')
		ORDER BY s.started_at DESC`
	rows, err := r.db.QueryContext(ctx, query, projectID, fmt.Sprintf("-%d", days))
	if err != nil {
		return nil, fmt.Errorf("listing recent sessions by project: %w", err)
	}
	defer rows.Close()
	return r.scanSessions(rows)
}

func (r *SQLiteSessionRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM work_session_logs WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting work session log: %w", err)
	}
	return nil
}

// scanSession scans a single session from a *sql.Row.
func (r *SQLiteSessionRepo) scanSession(row *sql.Row) (*domain.WorkSessionLog, error) {
	var s domain.WorkSessionLog
	var startedAtStr, createdAtStr string

	err := row.Scan(
		&s.ID, &s.WorkItemID, &startedAtStr, &s.Minutes, &s.UnitsDoneDelta, &s.Note, &createdAtStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("work session log: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("scanning work session log: %w", err)
	}

	return r.populateSession(&s, startedAtStr, createdAtStr)
}

// scanSessions scans multiple sessions from *sql.Rows.
func (r *SQLiteSessionRepo) scanSessions(rows *sql.Rows) ([]*domain.WorkSessionLog, error) {
	var sessions []*domain.WorkSessionLog
	for rows.Next() {
		var s domain.WorkSessionLog
		var startedAtStr, createdAtStr string

		err := rows.Scan(
			&s.ID, &s.WorkItemID, &startedAtStr, &s.Minutes, &s.UnitsDoneDelta, &s.Note, &createdAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning session row: %w", err)
		}

		session, parseErr := r.populateSession(&s, startedAtStr, createdAtStr)
		if parseErr != nil {
			return nil, parseErr
		}

		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating sessions: %w", err)
	}
	return sessions, nil
}

// populateSession fills in parsed fields on a WorkSessionLog after scanning raw strings.
func (r *SQLiteSessionRepo) populateSession(s *domain.WorkSessionLog, startedAtStr, createdAtStr string) (*domain.WorkSessionLog, error) {
	var parseErr error
	s.StartedAt, parseErr = time.Parse(time.RFC3339, startedAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing started_at: %w", parseErr)
	}
	s.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing created_at: %w", parseErr)
	}

	return s, nil
}
