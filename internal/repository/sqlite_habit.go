package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteHabitRepo implements HabitRepo using SQLite.
type SQLiteHabitRepo struct {
	db db.DBTX
}

// NewSQLiteHabitRepo creates a new SQLiteHabitRepo.
func NewSQLiteHabitRepo(conn db.DBTX) *SQLiteHabitRepo {
	return &SQLiteHabitRepo{db: conn}
}

func (r *SQLiteHabitRepo) Create(ctx context.Context, h *domain.Habit) error {
	query := `INSERT INTO habits (id, title, cadence_days, target_min, min_session_min, max_session_min, archived_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var archivedAt *string
	if h.ArchivedAt != nil {
		s := h.ArchivedAt.Format(time.RFC3339)
		archivedAt = &s
	}
	_, err := r.db.ExecContext(ctx, query,
		h.ID,
		h.Title,
		h.CadenceDays,
		h.TargetMin,
		h.MinSessionMin,
		h.MaxSessionMin,
		archivedAt,
		h.CreatedAt.Format(time.RFC3339),
		h.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting habit: %w", err)
	}
	return nil
}

func (r *SQLiteHabitRepo) ListActive(ctx context.Context) ([]*domain.Habit, error) {
	query := `SELECT id, title, cadence_days, target_min, min_session_min, max_session_min, archived_at, created_at, updated_at
		FROM habits
		WHERE archived_at IS NULL
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing active habits: %w", err)
	}
	defer rows.Close()
	return r.scanHabits(rows)
}

func (r *SQLiteHabitRepo) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	query := `SELECT id, title, cadence_days, target_min, min_session_min, max_session_min, archived_at, created_at, updated_at
		FROM habits WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	h, err := r.scanHabit(row)
	if err != nil {
		return nil, fmt.Errorf("getting habit by id: %w", err)
	}
	return h, nil
}

func (r *SQLiteHabitRepo) Archive(ctx context.Context, id string, now time.Time) error {
	query := `UPDATE habits SET archived_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("archiving habit: %w", err)
	}
	return nil
}

func (r *SQLiteHabitRepo) LogSession(ctx context.Context, log *domain.HabitLog) error {
	query := `INSERT INTO habit_logs (id, habit_id, performed_at, minutes, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		log.ID,
		log.HabitID,
		log.PerformedAt.Format(time.RFC3339),
		log.Minutes,
		log.Note,
		log.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting habit log: %w", err)
	}
	return nil
}

func (r *SQLiteHabitRepo) LastLog(ctx context.Context, habitID string) (*domain.HabitLog, error) {
	query := `SELECT id, habit_id, performed_at, minutes, note, created_at
		FROM habit_logs
		WHERE habit_id = ?
		ORDER BY performed_at DESC
		LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, habitID)
	var l domain.HabitLog
	var performedAtStr, createdAtStr string
	err := row.Scan(&l.ID, &l.HabitID, &performedAtStr, &l.Minutes, &l.Note, &createdAtStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning habit log: %w", err)
	}
	l.PerformedAt, err = time.Parse(time.RFC3339, performedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing performed_at: %w", err)
	}
	l.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	return &l, nil
}

func (r *SQLiteHabitRepo) ListLogs(ctx context.Context, habitID string, limit int) ([]domain.HabitLog, error) {
	query := `SELECT id, habit_id, performed_at, minutes, note, created_at
		FROM habit_logs
		WHERE habit_id = ?
		ORDER BY performed_at DESC
		LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, habitID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing habit logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.HabitLog
	for rows.Next() {
		var l domain.HabitLog
		var performedAtStr, createdAtStr string
		if err := rows.Scan(&l.ID, &l.HabitID, &performedAtStr, &l.Minutes, &l.Note, &createdAtStr); err != nil {
			return nil, fmt.Errorf("scanning habit log row: %w", err)
		}
		l.PerformedAt, err = time.Parse(time.RFC3339, performedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parsing performed_at: %w", err)
		}
		l.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating habit logs: %w", err)
	}
	return logs, nil
}

func (r *SQLiteHabitRepo) scanHabit(row *sql.Row) (*domain.Habit, error) {
	var h domain.Habit
	var archivedAtStr sql.NullString
	var createdAtStr, updatedAtStr string
	err := row.Scan(
		&h.ID, &h.Title, &h.CadenceDays, &h.TargetMin,
		&h.MinSessionMin, &h.MaxSessionMin,
		&archivedAtStr, &createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, err
	}
	if archivedAtStr.Valid {
		t, parseErr := time.Parse(time.RFC3339, archivedAtStr.String)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing archived_at: %w", parseErr)
		}
		h.ArchivedAt = &t
	}
	h.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	h.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", err)
	}
	return &h, nil
}

func (r *SQLiteHabitRepo) scanHabits(rows *sql.Rows) ([]*domain.Habit, error) {
	var habits []*domain.Habit
	for rows.Next() {
		var h domain.Habit
		var archivedAtStr sql.NullString
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(
			&h.ID, &h.Title, &h.CadenceDays, &h.TargetMin,
			&h.MinSessionMin, &h.MaxSessionMin,
			&archivedAtStr, &createdAtStr, &updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("scanning habit row: %w", err)
		}
		if archivedAtStr.Valid {
			t, err := time.Parse(time.RFC3339, archivedAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("parsing archived_at: %w", err)
			}
			h.ArchivedAt = &t
		}
		var err error
		h.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}
		h.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", err)
		}
		habits = append(habits, &h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating habits: %w", err)
	}
	return habits, nil
}
