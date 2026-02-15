package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteUserProfileRepo implements UserProfileRepo using a SQLite database.
type SQLiteUserProfileRepo struct {
	db db.DBTX
}

// NewSQLiteUserProfileRepo creates a new SQLiteUserProfileRepo.
func NewSQLiteUserProfileRepo(conn db.DBTX) *SQLiteUserProfileRepo {
	return &SQLiteUserProfileRepo{db: conn}
}

func (r *SQLiteUserProfileRepo) Get(ctx context.Context) (*domain.UserProfile, error) {
	query := `SELECT id, buffer_pct, weight_deadline_pressure, weight_behind_pace,
		weight_spacing, weight_variation, default_max_slices, baseline_daily_min
		FROM user_profile WHERE id = 'default'`
	row := r.db.QueryRowContext(ctx, query)

	var p domain.UserProfile
	err := row.Scan(
		&p.ID,
		&p.BufferPct,
		&p.WeightDeadlinePressure,
		&p.WeightBehindPace,
		&p.WeightSpacing,
		&p.WeightVariation,
		&p.DefaultMaxSlices,
		&p.BaselineDailyMin,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user profile: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("scanning user profile: %w", err)
	}
	return &p, nil
}

func (r *SQLiteUserProfileRepo) Upsert(ctx context.Context, p *domain.UserProfile) error {
	query := `INSERT OR REPLACE INTO user_profile (id, buffer_pct, weight_deadline_pressure,
		weight_behind_pace, weight_spacing, weight_variation, default_max_slices, baseline_daily_min)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		p.ID,
		p.BufferPct,
		p.WeightDeadlinePressure,
		p.WeightBehindPace,
		p.WeightSpacing,
		p.WeightVariation,
		p.DefaultMaxSlices,
		p.BaselineDailyMin,
	)
	if err != nil {
		return fmt.Errorf("upserting user profile: %w", err)
	}
	return nil
}
