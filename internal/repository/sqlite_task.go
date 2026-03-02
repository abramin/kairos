package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
)

// SQLiteTaskRepo implements TaskRepo using SQLite.
type SQLiteTaskRepo struct {
	db db.DBTX
}

// NewSQLiteTaskRepo creates a new SQLiteTaskRepo.
func NewSQLiteTaskRepo(conn db.DBTX) *SQLiteTaskRepo {
	return &SQLiteTaskRepo{db: conn}
}

func (r *SQLiteTaskRepo) Create(ctx context.Context, t *domain.Task) error {
	query := `INSERT INTO tasks (id, title, description, order_index, created_at, updated_at)
		VALUES (?, ?, ?, (SELECT COALESCE(MAX(order_index), 0) + 1 FROM tasks WHERE archived_at IS NULL), ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		t.ID,
		t.Title,
		t.Description,
		t.CreatedAt.Format(time.RFC3339),
		t.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting task: %w", err)
	}
	return nil
}

func (r *SQLiteTaskRepo) GetByID(ctx context.Context, id string) (*domain.Task, error) {
	query := `SELECT id, title, description, order_index, archived_at, created_at, updated_at
		FROM tasks WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	t, err := r.scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("getting task by id: %w", err)
	}
	return t, nil
}

func (r *SQLiteTaskRepo) ListActive(ctx context.Context) ([]*domain.Task, error) {
	query := `SELECT id, title, description, order_index, archived_at, created_at, updated_at
		FROM tasks
		WHERE archived_at IS NULL
		ORDER BY order_index ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing active tasks: %w", err)
	}
	defer rows.Close()
	return r.scanTasks(rows)
}

func (r *SQLiteTaskRepo) Update(ctx context.Context, t *domain.Task) error {
	query := `UPDATE tasks SET title = ?, description = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		t.Title,
		t.Description,
		t.UpdatedAt.Format(time.RFC3339),
		t.ID,
	)
	if err != nil {
		return fmt.Errorf("updating task: %w", err)
	}
	return nil
}

func (r *SQLiteTaskRepo) Archive(ctx context.Context, id string, now time.Time) error {
	query := `UPDATE tasks SET archived_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("archiving task: %w", err)
	}
	return nil
}

func (r *SQLiteTaskRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tasks WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting task: %w", err)
	}
	return nil
}

// SwapOrder atomically swaps the order_index of two tasks.
// SQLite serializes writes, so two sequential UPDATEs are safe in the single-user CLI context.
func (r *SQLiteTaskRepo) SwapOrder(ctx context.Context, idA, idB string) error {
	// Read both current order_index values first.
	var orderA, orderB int
	if err := r.db.QueryRowContext(ctx, `SELECT order_index FROM tasks WHERE id = ?`, idA).Scan(&orderA); err != nil {
		return fmt.Errorf("reading order_index for task %s: %w", idA, err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT order_index FROM tasks WHERE id = ?`, idB).Scan(&orderB); err != nil {
		return fmt.Errorf("reading order_index for task %s: %w", idB, err)
	}

	if _, err := r.db.ExecContext(ctx, `UPDATE tasks SET order_index = ? WHERE id = ?`, orderB, idA); err != nil {
		return fmt.Errorf("swapping order for task %s: %w", idA, err)
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE tasks SET order_index = ? WHERE id = ?`, orderA, idB); err != nil {
		return fmt.Errorf("swapping order for task %s: %w", idB, err)
	}
	return nil
}

func (r *SQLiteTaskRepo) scanTask(row *sql.Row) (*domain.Task, error) {
	var t domain.Task
	var archivedAtStr sql.NullString
	var createdAtStr, updatedAtStr string

	if err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.OrderIndex,
		&archivedAtStr, &createdAtStr, &updatedAtStr,
	); err != nil {
		return nil, err
	}

	var parseErr error
	t.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing created_at: %w", parseErr)
	}
	t.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
	}
	if archivedAtStr.Valid {
		parsed, err := time.Parse(time.RFC3339, archivedAtStr.String)
		if err != nil {
			return nil, fmt.Errorf("parsing archived_at: %w", err)
		}
		t.ArchivedAt = &parsed
	}
	return &t, nil
}

func (r *SQLiteTaskRepo) scanTasks(rows *sql.Rows) ([]*domain.Task, error) {
	var tasks []*domain.Task
	for rows.Next() {
		var t domain.Task
		var archivedAtStr sql.NullString
		var createdAtStr, updatedAtStr string

		if err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.OrderIndex,
			&archivedAtStr, &createdAtStr, &updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("scanning task row: %w", err)
		}

		var parseErr error
		t.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing created_at: %w", parseErr)
		}
		t.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
		}
		if archivedAtStr.Valid {
			parsed, err := time.Parse(time.RFC3339, archivedAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("parsing archived_at: %w", err)
			}
			t.ArchivedAt = &parsed
		}
		tasks = append(tasks, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating task rows: %w", err)
	}
	return tasks, nil
}
