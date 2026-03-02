package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteWorkoutLogRepo implements WorkoutLogRepo using SQLite.
type SQLiteWorkoutLogRepo struct {
	db db.DBTX
}

// NewSQLiteWorkoutLogRepo creates a new SQLiteWorkoutLogRepo.
func NewSQLiteWorkoutLogRepo(conn db.DBTX) *SQLiteWorkoutLogRepo {
	return &SQLiteWorkoutLogRepo{db: conn}
}

func (r *SQLiteWorkoutLogRepo) Create(ctx context.Context, log *domain.WorkoutLog) error {
	query := `INSERT INTO workout_logs (id, category, minutes, performed_at, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		log.ID,
		string(log.Category),
		log.Minutes,
		log.PerformedAt.Format(time.RFC3339),
		log.Notes,
		log.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting workout log: %w", err)
	}
	return nil
}

func (r *SQLiteWorkoutLogRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM workout_logs WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting workout log: %w", err)
	}
	return nil
}

func (r *SQLiteWorkoutLogRepo) ListByDateRange(ctx context.Context, from, to time.Time) ([]domain.WorkoutLog, error) {
	query := `SELECT id, category, minutes, performed_at, notes, created_at
		FROM workout_logs
		WHERE performed_at >= ? AND performed_at < ?
		ORDER BY performed_at DESC`
	rows, err := r.db.QueryContext(ctx, query,
		from.Format(time.RFC3339),
		to.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("listing workout logs by date range: %w", err)
	}
	defer rows.Close()
	return r.scanWorkouts(rows)
}

func (r *SQLiteWorkoutLogRepo) ListRecent(ctx context.Context, limit int) ([]domain.WorkoutLog, error) {
	query := `SELECT id, category, minutes, performed_at, notes, created_at
		FROM workout_logs
		ORDER BY performed_at DESC
		LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("listing recent workout logs: %w", err)
	}
	defer rows.Close()
	return r.scanWorkouts(rows)
}

func (r *SQLiteWorkoutLogRepo) scanWorkouts(rows *sql.Rows) ([]domain.WorkoutLog, error) {
	var logs []domain.WorkoutLog
	for rows.Next() {
		var w domain.WorkoutLog
		var category string
		var performedAtStr, createdAtStr string
		var notes sql.NullString

		if err := rows.Scan(&w.ID, &category, &w.Minutes,
			&performedAtStr, &notes, &createdAtStr); err != nil {
			return nil, fmt.Errorf("scanning workout log row: %w", err)
		}

		w.Category = domain.WorkoutCategory(category)

		var parseErr error
		w.PerformedAt, parseErr = time.Parse(time.RFC3339, performedAtStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing performed_at: %w", parseErr)
		}
		w.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing created_at: %w", parseErr)
		}
		if notes.Valid {
			w.Notes = &notes.String
		}

		logs = append(logs, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating workout logs: %w", err)
	}
	return logs, nil
}
