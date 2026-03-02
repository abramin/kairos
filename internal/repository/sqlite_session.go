package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteSessionRepo implements SessionRepo using a SQLite database.
type SQLiteSessionRepo struct {
	db db.DBTX
}

// NewSQLiteSessionRepo creates a new SQLiteSessionRepo.
func NewSQLiteSessionRepo(conn db.DBTX) *SQLiteSessionRepo {
	return &SQLiteSessionRepo{db: conn}
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

func (r *SQLiteSessionRepo) ListRecentSummaryByType(ctx context.Context, days int) ([]domain.SessionSummaryByType, error) {
	query := `SELECT w.title, w.type, SUM(s.minutes) as total_minutes
		FROM work_session_logs s
		JOIN work_items w ON s.work_item_id = w.id
		WHERE s.started_at >= date('now', ? || ' days')
		GROUP BY w.id, w.title, w.type
		ORDER BY total_minutes DESC`
	rows, err := r.db.QueryContext(ctx, query, fmt.Sprintf("-%d", days))
	if err != nil {
		return nil, fmt.Errorf("listing recent session summary by type: %w", err)
	}
	defer rows.Close()

	var summaries []domain.SessionSummaryByType
	for rows.Next() {
		var s domain.SessionSummaryByType
		if err := rows.Scan(&s.WorkItemTitle, &s.WorkItemType, &s.TotalMinutes); err != nil {
			return nil, fmt.Errorf("scanning session summary row: %w", err)
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session summaries: %w", err)
	}
	return summaries, nil
}

func (r *SQLiteSessionRepo) ListSessionMinutesByWeek(ctx context.Context, from, to time.Time) ([]ProjectWeekMinutes, error) {
	// Group by project + calendar date in SQL; Go converts dates to ISO weeks.
	// SQLite lacks native ISO week support, so we avoid strftime('%W') which
	// doesn't match Go's time.ISOWeek().
	query := `SELECT p.name,
	                 date(s.started_at) AS session_date,
	                 SUM(s.minutes) AS total_min
	          FROM work_session_logs s
	          JOIN work_items w ON s.work_item_id = w.id
	          JOIN plan_nodes n ON w.node_id = n.id
	          JOIN projects p ON n.project_id = p.id
	          WHERE s.started_at >= ? AND s.started_at < ?
	          GROUP BY p.name, session_date
	          ORDER BY session_date DESC, total_min DESC`
	rows, err := r.db.QueryContext(ctx, query, from.Format(time.RFC3339), to.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("listing session minutes by week: %w", err)
	}
	defer rows.Close()

	// Aggregate per-date rows into per-ISO-week results.
	type key struct {
		project string
		isoWeek string
	}
	agg := make(map[key]int)
	for rows.Next() {
		var projectName, dateStr string
		var mins int
		if err := rows.Scan(&projectName, &dateStr, &mins); err != nil {
			return nil, fmt.Errorf("scanning session day row: %w", err)
		}
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("parsing session date %q: %w", dateStr, err)
		}
		y, w := d.ISOWeek()
		isoWk := fmt.Sprintf("%d-W%02d", y, w)
		agg[key{project: projectName, isoWeek: isoWk}] += mins
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session day rows: %w", err)
	}

	results := make([]ProjectWeekMinutes, 0, len(agg))
	for k, mins := range agg {
		results = append(results, ProjectWeekMinutes{
			ProjectName: k.project,
			ISOWeek:     k.isoWeek,
			TotalMin:    mins,
		})
	}
	return results, nil
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
