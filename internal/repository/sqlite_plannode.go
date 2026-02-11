package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

// planNodeColumns is the canonical SELECT column list for plan_nodes.
const planNodeColumns = `id, project_id, parent_id, title, kind, order_index,
		due_date, not_before, not_after, planned_min_budget, seq, created_at, updated_at,
		is_default`

// SQLitePlanNodeRepo implements PlanNodeRepo using a SQLite database.
type SQLitePlanNodeRepo struct {
	db *sql.DB
}

// NewSQLitePlanNodeRepo creates a new SQLitePlanNodeRepo.
func NewSQLitePlanNodeRepo(db *sql.DB) *SQLitePlanNodeRepo {
	return &SQLitePlanNodeRepo{db: db}
}

func (r *SQLitePlanNodeRepo) Create(ctx context.Context, n *domain.PlanNode) error {
	query := `INSERT INTO plan_nodes (id, project_id, parent_id, title, kind, order_index,
		due_date, not_before, not_after, planned_min_budget, seq, created_at, updated_at,
		is_default)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		n.ID,
		n.ProjectID,
		n.ParentID, // *string: nil becomes SQL NULL
		n.Title,
		string(n.Kind),
		n.OrderIndex,
		nullableTimeToString(n.DueDate, dateLayout),
		nullableTimeToString(n.NotBefore, dateLayout),
		nullableTimeToString(n.NotAfter, dateLayout),
		nullableIntToValue(n.PlannedMinBudget),
		n.Seq,
		n.CreatedAt.Format(time.RFC3339),
		n.UpdatedAt.Format(time.RFC3339),
		boolToInt(n.IsDefault),
	)
	if err != nil {
		return fmt.Errorf("inserting plan node: %w", err)
	}
	return nil
}

func (r *SQLitePlanNodeRepo) GetByID(ctx context.Context, id string) (*domain.PlanNode, error) {
	query := `SELECT ` + planNodeColumns + ` FROM plan_nodes WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanNode(row)
}

func (r *SQLitePlanNodeRepo) GetBySeq(ctx context.Context, projectID string, seq int) (*domain.PlanNode, error) {
	query := `SELECT ` + planNodeColumns + ` FROM plan_nodes WHERE project_id = ? AND seq = ?`
	row := r.db.QueryRowContext(ctx, query, projectID, seq)
	return r.scanNode(row)
}

// NextProjectSeq returns the next available sequential ID for a project,
// computed as MAX(seq) + 1 across both plan_nodes and work_items.
func (r *SQLitePlanNodeRepo) NextProjectSeq(ctx context.Context, projectID string) (int, error) {
	query := `SELECT COALESCE(MAX(seq_val), 0) + 1 FROM (
		SELECT seq AS seq_val FROM plan_nodes WHERE project_id = ? AND seq > 0
		UNION ALL
		SELECT w.seq AS seq_val FROM work_items w
		JOIN plan_nodes n ON w.node_id = n.id
		WHERE n.project_id = ? AND w.seq > 0
	)`
	var next int
	err := r.db.QueryRowContext(ctx, query, projectID, projectID).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("computing next seq for project %s: %w", projectID, err)
	}
	return next, nil
}

func (r *SQLitePlanNodeRepo) ListByProject(ctx context.Context, projectID string) ([]*domain.PlanNode, error) {
	query := `SELECT ` + planNodeColumns + ` FROM plan_nodes WHERE project_id = ? ORDER BY order_index`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing plan nodes by project: %w", err)
	}
	defer rows.Close()
	return r.scanNodes(rows)
}

func (r *SQLitePlanNodeRepo) ListChildren(ctx context.Context, parentID string) ([]*domain.PlanNode, error) {
	query := `SELECT ` + planNodeColumns + ` FROM plan_nodes WHERE parent_id = ? ORDER BY order_index`
	rows, err := r.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("listing child plan nodes: %w", err)
	}
	defer rows.Close()
	return r.scanNodes(rows)
}

func (r *SQLitePlanNodeRepo) ListRoots(ctx context.Context, projectID string) ([]*domain.PlanNode, error) {
	query := `SELECT ` + planNodeColumns + ` FROM plan_nodes WHERE project_id = ? AND parent_id IS NULL ORDER BY order_index`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing root plan nodes: %w", err)
	}
	defer rows.Close()
	return r.scanNodes(rows)
}

func (r *SQLitePlanNodeRepo) Update(ctx context.Context, n *domain.PlanNode) error {
	query := `UPDATE plan_nodes SET project_id = ?, parent_id = ?, title = ?, kind = ?,
		order_index = ?, due_date = ?, not_before = ?, not_after = ?, planned_min_budget = ?,
		seq = ?, updated_at = ?, is_default = ?
		WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		n.ProjectID,
		n.ParentID,
		n.Title,
		string(n.Kind),
		n.OrderIndex,
		nullableTimeToString(n.DueDate, dateLayout),
		nullableTimeToString(n.NotBefore, dateLayout),
		nullableTimeToString(n.NotAfter, dateLayout),
		nullableIntToValue(n.PlannedMinBudget),
		n.Seq,
		n.UpdatedAt.Format(time.RFC3339),
		boolToInt(n.IsDefault),
		n.ID,
	)
	if err != nil {
		return fmt.Errorf("updating plan node: %w", err)
	}
	return nil
}

func (r *SQLitePlanNodeRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM plan_nodes WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting plan node: %w", err)
	}
	return nil
}

// scanNode scans a single plan node from a *sql.Row.
func (r *SQLitePlanNodeRepo) scanNode(row *sql.Row) (*domain.PlanNode, error) {
	var n domain.PlanNode
	var kindStr, createdAtStr, updatedAtStr string
	var parentID sql.NullString
	var dueDateStr, notBeforeStr, notAfterStr sql.NullString
	var plannedMinBudget sql.NullInt64
	var isDefaultInt int

	err := row.Scan(
		&n.ID, &n.ProjectID, &parentID, &n.Title, &kindStr, &n.OrderIndex,
		&dueDateStr, &notBeforeStr, &notAfterStr, &plannedMinBudget,
		&n.Seq, &createdAtStr, &updatedAtStr,
		&isDefaultInt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("plan node: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("scanning plan node: %w", err)
	}

	n.IsDefault = intToBool(isDefaultInt)
	return r.populateNode(&n, kindStr, createdAtStr, updatedAtStr, parentID,
		dueDateStr, notBeforeStr, notAfterStr, plannedMinBudget)
}

// scanNodes scans multiple plan nodes from *sql.Rows.
func (r *SQLitePlanNodeRepo) scanNodes(rows *sql.Rows) ([]*domain.PlanNode, error) {
	var nodes []*domain.PlanNode
	for rows.Next() {
		var n domain.PlanNode
		var kindStr, createdAtStr, updatedAtStr string
		var parentID sql.NullString
		var dueDateStr, notBeforeStr, notAfterStr sql.NullString
		var plannedMinBudget sql.NullInt64
		var isDefaultInt int

		err := rows.Scan(
			&n.ID, &n.ProjectID, &parentID, &n.Title, &kindStr, &n.OrderIndex,
			&dueDateStr, &notBeforeStr, &notAfterStr, &plannedMinBudget,
			&n.Seq, &createdAtStr, &updatedAtStr,
			&isDefaultInt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning plan node row: %w", err)
		}

		n.IsDefault = intToBool(isDefaultInt)
		node, err := r.populateNode(&n, kindStr, createdAtStr, updatedAtStr, parentID,
			dueDateStr, notBeforeStr, notAfterStr, plannedMinBudget)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating plan nodes: %w", err)
	}
	return nodes, nil
}

// populateNode fills in parsed fields on a PlanNode after scanning raw strings.
func (r *SQLitePlanNodeRepo) populateNode(
	n *domain.PlanNode,
	kindStr, createdAtStr, updatedAtStr string,
	parentID sql.NullString,
	dueDateStr, notBeforeStr, notAfterStr sql.NullString,
	plannedMinBudget sql.NullInt64,
) (*domain.PlanNode, error) {
	n.Kind = domain.NodeKind(kindStr)

	if parentID.Valid {
		n.ParentID = &parentID.String
	}

	var parseErr error
	n.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing created_at: %w", parseErr)
	}
	n.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAtStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
	}

	n.DueDate = parseNullableTime(dueDateStr, dateLayout)
	n.NotBefore = parseNullableTime(notBeforeStr, dateLayout)
	n.NotAfter = parseNullableTime(notAfterStr, dateLayout)

	if plannedMinBudget.Valid {
		v := int(plannedMinBudget.Int64)
		n.PlannedMinBudget = &v
	}

	return n, nil
}
