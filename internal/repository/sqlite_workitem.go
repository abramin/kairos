package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

// workItemColumns is the canonical SELECT column list for work_items.
const workItemColumns = `id, node_id, title, type, status, archived_at,
		duration_mode, planned_min, logged_min, duration_source, estimate_confidence,
		min_session_min, max_session_min, default_session_min, splittable,
		units_kind, units_total, units_done, due_date, not_before, seq, created_at, updated_at,
		description, completed_at`

// workItemColumnsAliased is the same column list prefixed with "w." for join queries.
const workItemColumnsAliased = `w.id, w.node_id, w.title, w.type, w.status, w.archived_at,
		w.duration_mode, w.planned_min, w.logged_min, w.duration_source, w.estimate_confidence,
		w.min_session_min, w.max_session_min, w.default_session_min, w.splittable,
		w.units_kind, w.units_total, w.units_done, w.due_date, w.not_before, w.seq,
		w.created_at, w.updated_at,
		w.description, w.completed_at`

// SQLiteWorkItemRepo implements WorkItemRepo using a SQLite database.
type SQLiteWorkItemRepo struct {
	db *sql.DB
}

// NewSQLiteWorkItemRepo creates a new SQLiteWorkItemRepo.
func NewSQLiteWorkItemRepo(db *sql.DB) *SQLiteWorkItemRepo {
	return &SQLiteWorkItemRepo{db: db}
}

func (r *SQLiteWorkItemRepo) Create(ctx context.Context, w *domain.WorkItem) error {
	query := `INSERT INTO work_items (id, node_id, title, type, status, archived_at,
		duration_mode, planned_min, logged_min, duration_source, estimate_confidence,
		min_session_min, max_session_min, default_session_min, splittable,
		units_kind, units_total, units_done, due_date, not_before, seq, created_at, updated_at,
		description, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		w.ID,
		w.NodeID,
		w.Title,
		w.Type,
		string(w.Status),
		nullableTimeToString(w.ArchivedAt, time.RFC3339),
		string(w.DurationMode),
		w.PlannedMin,
		w.LoggedMin,
		string(w.DurationSource),
		w.EstimateConfidence,
		w.MinSessionMin,
		w.MaxSessionMin,
		w.DefaultSessionMin,
		boolToInt(w.Splittable),
		w.UnitsKind,
		w.UnitsTotal,
		w.UnitsDone,
		nullableTimeToString(w.DueDate, dateLayout),
		nullableTimeToString(w.NotBefore, dateLayout),
		w.Seq,
		w.CreatedAt.Format(time.RFC3339),
		w.UpdatedAt.Format(time.RFC3339),
		w.Description,
		nullableTimeToString(w.CompletedAt, time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting work item: %w", err)
	}
	return nil
}

func (r *SQLiteWorkItemRepo) GetByID(ctx context.Context, id string) (*domain.WorkItem, error) {
	query := `SELECT ` + workItemColumns + ` FROM work_items WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanWorkItem(row)
}

func (r *SQLiteWorkItemRepo) GetBySeq(ctx context.Context, projectID string, seq int) (*domain.WorkItem, error) {
	query := `SELECT ` + workItemColumnsAliased + `
		FROM work_items w
		JOIN plan_nodes n ON w.node_id = n.id
		WHERE n.project_id = ? AND w.seq = ?`
	row := r.db.QueryRowContext(ctx, query, projectID, seq)
	return r.scanWorkItem(row)
}

func (r *SQLiteWorkItemRepo) ListByNode(ctx context.Context, nodeID string) ([]*domain.WorkItem, error) {
	query := `SELECT ` + workItemColumns + ` FROM work_items WHERE node_id = ? ORDER BY created_at`
	rows, err := r.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("listing work items by node: %w", err)
	}
	defer rows.Close()
	return r.scanWorkItems(rows)
}

func (r *SQLiteWorkItemRepo) ListByProject(ctx context.Context, projectID string) ([]*domain.WorkItem, error) {
	query := `SELECT ` + workItemColumnsAliased + `
		FROM work_items w
		JOIN plan_nodes n ON w.node_id = n.id
		WHERE n.project_id = ?
		ORDER BY w.created_at`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing work items by project: %w", err)
	}
	defer rows.Close()
	return r.scanWorkItems(rows)
}

func (r *SQLiteWorkItemRepo) ListSchedulable(ctx context.Context, includeArchived bool) ([]SchedulableCandidate, error) {
	schedulableJoinedColumns := workItemColumnsAliased + `,
			n.project_id, p.name AS project_name, p.domain AS project_domain,
			n.title AS node_title, n.due_date AS node_due_date, p.target_date, p.start_date`

	var query string
	if includeArchived {
		query = `SELECT ` + schedulableJoinedColumns + `
			FROM work_items w
			JOIN plan_nodes n ON w.node_id = n.id
			JOIN projects p ON n.project_id = p.id
			WHERE w.status IN ('todo', 'in_progress')
			  AND p.status = 'active'
			ORDER BY w.id`
	} else {
		query = `SELECT ` + schedulableJoinedColumns + `
			FROM work_items w
			JOIN plan_nodes n ON w.node_id = n.id
			JOIN projects p ON n.project_id = p.id
			WHERE w.status IN ('todo', 'in_progress')
			  AND (w.archived_at IS NULL)
			  AND p.status = 'active'
			  AND (p.archived_at IS NULL)
			ORDER BY w.id`
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing schedulable work items: %w", err)
	}
	defer rows.Close()

	var candidates []SchedulableCandidate
	for rows.Next() {
		var w domain.WorkItem
		var statusStr, durationModeStr, durationSourceStr string
		var archivedAtStr, dueDateStr, notBeforeStr sql.NullString
		var splittableInt int
		var createdAtStr, updatedAtStr string
		var completedAtStr sql.NullString

		// Extra joined fields
		var projectID, projectName, projectDomain, nodeTitle string
		var nodeDueDateStr, targetDateStr, startDateStr sql.NullString

		err := rows.Scan(
			&w.ID, &w.NodeID, &w.Title, &w.Type, &statusStr, &archivedAtStr,
			&durationModeStr, &w.PlannedMin, &w.LoggedMin, &durationSourceStr, &w.EstimateConfidence,
			&w.MinSessionMin, &w.MaxSessionMin, &w.DefaultSessionMin, &splittableInt,
			&w.UnitsKind, &w.UnitsTotal, &w.UnitsDone, &dueDateStr, &notBeforeStr,
			&w.Seq, &createdAtStr, &updatedAtStr,
			&w.Description, &completedAtStr,
			&projectID, &projectName, &projectDomain,
			&nodeTitle, &nodeDueDateStr, &targetDateStr, &startDateStr,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning schedulable candidate: %w", err)
		}

		w.Status = domain.WorkItemStatus(statusStr)
		w.DurationMode = domain.DurationMode(durationModeStr)
		w.DurationSource = domain.DurationSource(durationSourceStr)
		w.Splittable = intToBool(splittableInt)
		w.ArchivedAt = parseNullableTime(archivedAtStr, time.RFC3339)
		w.DueDate = parseNullableTime(dueDateStr, dateLayout)
		w.NotBefore = parseNullableTime(notBeforeStr, dateLayout)
		w.CompletedAt = parseNullableTime(completedAtStr, time.RFC3339)

		var parseErr error
		w.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing created_at: %w", parseErr)
		}
		w.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
		}

		candidate := SchedulableCandidate{
			WorkItem:          w,
			ProjectID:         projectID,
			ProjectName:       projectName,
			ProjectDomain:     projectDomain,
			NodeTitle:         nodeTitle,
			NodeDueDate:       parseNullableTime(nodeDueDateStr, dateLayout),
			ProjectTargetDate: parseNullableTime(targetDateStr, dateLayout),
			ProjectStartDate:  parseNullableTime(startDateStr, dateLayout),
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating schedulable candidates: %w", err)
	}
	return candidates, nil
}

func (r *SQLiteWorkItemRepo) ListCompletedSummaryByProject(ctx context.Context) ([]CompletedWorkSummary, error) {
	query := `SELECT n.project_id,
			COALESCE(SUM(CASE WHEN w.status IN ('done','skipped') THEN w.planned_min ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN w.status IN ('done','skipped') THEN w.logged_min ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN w.status IN ('done','skipped') THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM work_items w
		JOIN plan_nodes n ON w.node_id = n.id
		JOIN projects p ON n.project_id = p.id
		WHERE w.status != 'archived'
		  AND (w.archived_at IS NULL)
		  AND p.status = 'active'
		  AND (p.archived_at IS NULL)
		GROUP BY n.project_id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing completed work summary: %w", err)
	}
	defer rows.Close()

	var summaries []CompletedWorkSummary
	for rows.Next() {
		var s CompletedWorkSummary
		if err := rows.Scan(&s.ProjectID, &s.PlannedMin, &s.LoggedMin, &s.DoneItemCount, &s.TotalItemCount); err != nil {
			return nil, fmt.Errorf("scanning completed work summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating completed work summaries: %w", err)
	}
	return summaries, nil
}

func (r *SQLiteWorkItemRepo) Update(ctx context.Context, w *domain.WorkItem) error {
	query := `UPDATE work_items SET node_id = ?, title = ?, type = ?, status = ?, archived_at = ?,
		duration_mode = ?, planned_min = ?, logged_min = ?, duration_source = ?, estimate_confidence = ?,
		min_session_min = ?, max_session_min = ?, default_session_min = ?, splittable = ?,
		units_kind = ?, units_total = ?, units_done = ?, due_date = ?, not_before = ?,
		seq = ?, updated_at = ?, description = ?, completed_at = ?
		WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		w.NodeID,
		w.Title,
		w.Type,
		string(w.Status),
		nullableTimeToString(w.ArchivedAt, time.RFC3339),
		string(w.DurationMode),
		w.PlannedMin,
		w.LoggedMin,
		string(w.DurationSource),
		w.EstimateConfidence,
		w.MinSessionMin,
		w.MaxSessionMin,
		w.DefaultSessionMin,
		boolToInt(w.Splittable),
		w.UnitsKind,
		w.UnitsTotal,
		w.UnitsDone,
		nullableTimeToString(w.DueDate, dateLayout),
		nullableTimeToString(w.NotBefore, dateLayout),
		w.Seq,
		w.UpdatedAt.Format(time.RFC3339),
		w.Description,
		nullableTimeToString(w.CompletedAt, time.RFC3339),
		w.ID,
	)
	if err != nil {
		return fmt.Errorf("updating work item: %w", err)
	}
	return nil
}

func (r *SQLiteWorkItemRepo) Archive(ctx context.Context, id string) error {
	now := nowUTC()
	query := `UPDATE work_items SET status = 'archived', archived_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, now, now, id)
	if err != nil {
		return fmt.Errorf("archiving work item: %w", err)
	}
	return nil
}

func (r *SQLiteWorkItemRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM work_items WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting work item: %w", err)
	}
	return nil
}

// scanWorkItem scans a single work item from a *sql.Row.
func (r *SQLiteWorkItemRepo) scanWorkItem(row *sql.Row) (*domain.WorkItem, error) {
	var w domain.WorkItem
	var statusStr, durationModeStr, durationSourceStr string
	var archivedAtStr, dueDateStr, notBeforeStr sql.NullString
	var splittableInt int
	var createdAtStr, updatedAtStr string
	var completedAtStr sql.NullString

	err := row.Scan(
		&w.ID, &w.NodeID, &w.Title, &w.Type, &statusStr, &archivedAtStr,
		&durationModeStr, &w.PlannedMin, &w.LoggedMin, &durationSourceStr, &w.EstimateConfidence,
		&w.MinSessionMin, &w.MaxSessionMin, &w.DefaultSessionMin, &splittableInt,
		&w.UnitsKind, &w.UnitsTotal, &w.UnitsDone, &dueDateStr, &notBeforeStr,
		&w.Seq, &createdAtStr, &updatedAtStr,
		&w.Description, &completedAtStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("work item: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("scanning work item: %w", err)
	}

	return r.populateWorkItem(&w, statusStr, durationModeStr, durationSourceStr,
		archivedAtStr, dueDateStr, notBeforeStr, completedAtStr, splittableInt, createdAtStr, updatedAtStr)
}

// scanWorkItems scans multiple work items from *sql.Rows.
func (r *SQLiteWorkItemRepo) scanWorkItems(rows *sql.Rows) ([]*domain.WorkItem, error) {
	var items []*domain.WorkItem
	for rows.Next() {
		var w domain.WorkItem
		var statusStr, durationModeStr, durationSourceStr string
		var archivedAtStr, dueDateStr, notBeforeStr sql.NullString
		var splittableInt int
		var createdAtStr, updatedAtStr string
		var completedAtStr sql.NullString

		err := rows.Scan(
			&w.ID, &w.NodeID, &w.Title, &w.Type, &statusStr, &archivedAtStr,
			&durationModeStr, &w.PlannedMin, &w.LoggedMin, &durationSourceStr, &w.EstimateConfidence,
			&w.MinSessionMin, &w.MaxSessionMin, &w.DefaultSessionMin, &splittableInt,
			&w.UnitsKind, &w.UnitsTotal, &w.UnitsDone, &dueDateStr, &notBeforeStr,
			&w.Seq, &createdAtStr, &updatedAtStr,
			&w.Description, &completedAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning work item row: %w", err)
		}

		item, err := r.populateWorkItem(&w, statusStr, durationModeStr, durationSourceStr,
			archivedAtStr, dueDateStr, notBeforeStr, completedAtStr, splittableInt, createdAtStr, updatedAtStr)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating work items: %w", err)
	}
	return items, nil
}

// populateWorkItem fills in parsed fields on a WorkItem after scanning raw values.
func (r *SQLiteWorkItemRepo) populateWorkItem(
	w *domain.WorkItem,
	statusStr, durationModeStr, durationSourceStr string,
	archivedAtStr, dueDateStr, notBeforeStr, completedAtStr sql.NullString,
	splittableInt int,
	createdAtStr, updatedAtStr string,
) (*domain.WorkItem, error) {
	w.Status = domain.WorkItemStatus(statusStr)
	w.DurationMode = domain.DurationMode(durationModeStr)
	w.DurationSource = domain.DurationSource(durationSourceStr)
	w.Splittable = intToBool(splittableInt)

	w.ArchivedAt = parseNullableTime(archivedAtStr, time.RFC3339)
	w.DueDate = parseNullableTime(dueDateStr, dateLayout)
	w.NotBefore = parseNullableTime(notBeforeStr, dateLayout)
	w.CompletedAt = parseNullableTime(completedAtStr, time.RFC3339)

	var parseErr error
	w.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing created_at: %w", parseErr)
	}
	w.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
	}

	return w, nil
}
