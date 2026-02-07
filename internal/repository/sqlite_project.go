package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteProjectRepo implements ProjectRepo using a SQLite database.
type SQLiteProjectRepo struct {
	db *sql.DB
}

// NewSQLiteProjectRepo creates a new SQLiteProjectRepo.
func NewSQLiteProjectRepo(db *sql.DB) *SQLiteProjectRepo {
	return &SQLiteProjectRepo{db: db}
}

const dateLayout = "2006-01-02"

func (r *SQLiteProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	query := `INSERT INTO projects (id, short_id, name, domain, start_date, target_date, status, archived_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		p.ID,
		p.ShortID,
		p.Name,
		p.Domain,
		p.StartDate.Format(dateLayout),
		nullableTimeToString(p.TargetDate, dateLayout),
		string(p.Status),
		nullableTimeToString(p.ArchivedAt, time.RFC3339),
		p.CreatedAt.Format(time.RFC3339),
		p.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting project: %w", err)
	}
	return nil
}

func (r *SQLiteProjectRepo) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	query := `SELECT id, short_id, name, domain, start_date, target_date, status, archived_at, created_at, updated_at
		FROM projects WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanProject(row)
}

func (r *SQLiteProjectRepo) GetByShortID(ctx context.Context, shortID string) (*domain.Project, error) {
	query := `SELECT id, short_id, name, domain, start_date, target_date, status, archived_at, created_at, updated_at
		FROM projects WHERE UPPER(short_id) = UPPER(?)`
	row := r.db.QueryRowContext(ctx, query, shortID)
	return r.scanProject(row)
}

func (r *SQLiteProjectRepo) List(ctx context.Context, includeArchived bool) ([]*domain.Project, error) {
	var query string
	if includeArchived {
		query = `SELECT id, short_id, name, domain, start_date, target_date, status, archived_at, created_at, updated_at
			FROM projects ORDER BY created_at`
	} else {
		query = `SELECT id, short_id, name, domain, start_date, target_date, status, archived_at, created_at, updated_at
			FROM projects WHERE archived_at IS NULL ORDER BY created_at`
	}
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p, err := r.scanProjectFromRows(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating projects: %w", err)
	}
	return projects, nil
}

func (r *SQLiteProjectRepo) Update(ctx context.Context, p *domain.Project) error {
	query := `UPDATE projects SET short_id = ?, name = ?, domain = ?, start_date = ?, target_date = ?, status = ?, updated_at = ?
		WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		p.ShortID,
		p.Name,
		p.Domain,
		p.StartDate.Format(dateLayout),
		nullableTimeToString(p.TargetDate, dateLayout),
		string(p.Status),
		p.UpdatedAt.Format(time.RFC3339),
		p.ID,
	)
	if err != nil {
		return fmt.Errorf("updating project: %w", err)
	}
	return nil
}

func (r *SQLiteProjectRepo) Archive(ctx context.Context, id string) error {
	now := nowUTC()
	query := `UPDATE projects SET status = 'archived', archived_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, now, now, id)
	if err != nil {
		return fmt.Errorf("archiving project: %w", err)
	}
	return nil
}

func (r *SQLiteProjectRepo) Unarchive(ctx context.Context, id string) error {
	now := nowUTC()
	query := `UPDATE projects SET status = 'active', archived_at = NULL, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("unarchiving project: %w", err)
	}
	return nil
}

func (r *SQLiteProjectRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM projects WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}
	return nil
}

// scanProject scans a single project row from a *sql.Row.
func (r *SQLiteProjectRepo) scanProject(row *sql.Row) (*domain.Project, error) {
	var p domain.Project
	var startDateStr, createdAtStr, updatedAtStr, statusStr string
	var targetDateStr, archivedAtStr sql.NullString

	err := row.Scan(
		&p.ID, &p.ShortID, &p.Name, &p.Domain,
		&startDateStr, &targetDateStr,
		&statusStr, &archivedAtStr,
		&createdAtStr, &updatedAtStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("scanning project: %w", err)
	}

	p.Status = domain.ProjectStatus(statusStr)

	var parseErr error
	p.StartDate, parseErr = time.Parse(dateLayout, startDateStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing start_date: %w", parseErr)
	}
	p.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing created_at: %w", parseErr)
	}
	p.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
	}

	p.TargetDate = parseNullableTime(targetDateStr, dateLayout)
	p.ArchivedAt = parseNullableTime(archivedAtStr, time.RFC3339)

	return &p, nil
}

// scanProjectFromRows scans a single project row from *sql.Rows.
func (r *SQLiteProjectRepo) scanProjectFromRows(rows *sql.Rows) (*domain.Project, error) {
	var p domain.Project
	var startDateStr, createdAtStr, updatedAtStr, statusStr string
	var targetDateStr, archivedAtStr sql.NullString

	err := rows.Scan(
		&p.ID, &p.ShortID, &p.Name, &p.Domain,
		&startDateStr, &targetDateStr,
		&statusStr, &archivedAtStr,
		&createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning project row: %w", err)
	}

	p.Status = domain.ProjectStatus(statusStr)

	var parseErr error
	p.StartDate, parseErr = time.Parse(dateLayout, startDateStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing start_date: %w", parseErr)
	}
	p.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing created_at: %w", parseErr)
	}
	p.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
	}

	p.TargetDate = parseNullableTime(targetDateStr, dateLayout)
	p.ArchivedAt = parseNullableTime(archivedAtStr, time.RFC3339)

	return &p, nil
}
