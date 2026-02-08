package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Migrate runs all schema migrations.
func Migrate(db *sql.DB) error {
	for i, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			// Tolerate "duplicate column name" errors from ALTER TABLE
			// since the migration system re-runs all statements.
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	if err := migratePlanNodesAssessmentKind(db); err != nil {
		return fmt.Errorf("migrating plan_nodes kind constraint: %w", err)
	}
	return nil
}

func migratePlanNodesAssessmentKind(db *sql.DB) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring db connection: %w", err)
	}
	defer conn.Close()

	var createSQL string
	if err := conn.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'plan_nodes'`).Scan(&createSQL); err != nil {
		return fmt.Errorf("loading plan_nodes schema: %w", err)
	}
	if strings.Contains(strings.ToLower(createSQL), "'assessment'") {
		return nil
	}

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disabling foreign keys: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting migration transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS plan_nodes_new`); err != nil {
		return fmt.Errorf("dropping stale plan_nodes_new: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE plan_nodes_new (
		id                 TEXT PRIMARY KEY,
		project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		parent_id          TEXT REFERENCES plan_nodes_new(id) ON DELETE CASCADE,
		title              TEXT NOT NULL,
		kind               TEXT NOT NULL
		                   CHECK(kind IN ('week','module','book','stage','section','assessment','generic')),
		order_index        INTEGER NOT NULL DEFAULT 0,
		due_date           TEXT,
		not_before         TEXT,
		not_after          TEXT,
		planned_min_budget INTEGER,
		created_at         TEXT NOT NULL,
		updated_at         TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("creating plan_nodes_new: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO plan_nodes_new (
		id, project_id, parent_id, title, kind, order_index,
		due_date, not_before, not_after, planned_min_budget, created_at, updated_at
	) SELECT
		id, project_id, parent_id, title, kind, order_index,
		due_date, not_before, not_after, planned_min_budget, created_at, updated_at
	FROM plan_nodes`); err != nil {
		return fmt.Errorf("copying plan_nodes data: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE plan_nodes`); err != nil {
		return fmt.Errorf("dropping old plan_nodes: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE plan_nodes_new RENAME TO plan_nodes`); err != nil {
		return fmt.Errorf("renaming plan_nodes_new: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_plan_nodes_project ON plan_nodes(project_id)`); err != nil {
		return fmt.Errorf("recreating idx_plan_nodes_project: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_plan_nodes_parent ON plan_nodes(parent_id)`); err != nil {
		return fmt.Errorf("recreating idx_plan_nodes_parent: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing plan_nodes migration: %w", err)
	}
	committed = true

	return nil
}

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS projects (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		domain      TEXT NOT NULL DEFAULT '',
		start_date  TEXT NOT NULL,
		target_date TEXT,
		status      TEXT NOT NULL DEFAULT 'active'
		            CHECK(status IN ('active','paused','done','archived')),
		archived_at TEXT,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS plan_nodes (
		id                 TEXT PRIMARY KEY,
		project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		parent_id          TEXT REFERENCES plan_nodes(id) ON DELETE CASCADE,
		title              TEXT NOT NULL,
		kind               TEXT NOT NULL
		                   CHECK(kind IN ('week','module','book','stage','section','assessment','generic')),
		order_index        INTEGER NOT NULL DEFAULT 0,
		due_date           TEXT,
		not_before         TEXT,
		not_after          TEXT,
		planned_min_budget INTEGER,
		created_at         TEXT NOT NULL,
		updated_at         TEXT NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS idx_plan_nodes_project ON plan_nodes(project_id)`,
	`CREATE INDEX IF NOT EXISTS idx_plan_nodes_parent ON plan_nodes(parent_id)`,

	`CREATE TABLE IF NOT EXISTS work_items (
		id                   TEXT PRIMARY KEY,
		node_id              TEXT NOT NULL REFERENCES plan_nodes(id) ON DELETE CASCADE,
		title                TEXT NOT NULL,
		type                 TEXT NOT NULL DEFAULT '',
		status               TEXT NOT NULL DEFAULT 'todo'
		                     CHECK(status IN ('todo','in_progress','done','skipped','archived')),
		archived_at          TEXT,
		duration_mode        TEXT NOT NULL DEFAULT 'estimate'
		                     CHECK(duration_mode IN ('fixed','estimate','derived')),
		planned_min          INTEGER NOT NULL DEFAULT 0,
		logged_min           INTEGER NOT NULL DEFAULT 0,
		duration_source      TEXT NOT NULL DEFAULT 'manual'
		                     CHECK(duration_source IN ('manual','template','rollup')),
		estimate_confidence  REAL NOT NULL DEFAULT 0.5,
		min_session_min      INTEGER NOT NULL DEFAULT 15,
		max_session_min      INTEGER NOT NULL DEFAULT 60,
		default_session_min  INTEGER NOT NULL DEFAULT 30,
		splittable           INTEGER NOT NULL DEFAULT 1,
		units_kind           TEXT NOT NULL DEFAULT '',
		units_total          INTEGER NOT NULL DEFAULT 0,
		units_done           INTEGER NOT NULL DEFAULT 0,
		due_date             TEXT,
		not_before           TEXT,
		created_at           TEXT NOT NULL,
		updated_at           TEXT NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS idx_work_items_node ON work_items(node_id)`,
	`CREATE INDEX IF NOT EXISTS idx_work_items_status ON work_items(status)`,

	`CREATE TABLE IF NOT EXISTS dependencies (
		predecessor_work_item_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
		successor_work_item_id   TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
		PRIMARY KEY (predecessor_work_item_id, successor_work_item_id)
	)`,

	`CREATE TABLE IF NOT EXISTS work_session_logs (
		id               TEXT PRIMARY KEY,
		work_item_id     TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
		started_at       TEXT NOT NULL,
		minutes          INTEGER NOT NULL,
		units_done_delta INTEGER NOT NULL DEFAULT 0,
		note             TEXT NOT NULL DEFAULT '',
		created_at       TEXT NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS idx_sessions_work_item ON work_session_logs(work_item_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_started ON work_session_logs(started_at)`,

	`CREATE TABLE IF NOT EXISTS user_profile (
		id                       TEXT PRIMARY KEY DEFAULT 'default',
		buffer_pct               REAL NOT NULL DEFAULT 0.1,
		weight_deadline_pressure REAL NOT NULL DEFAULT 1.0,
		weight_behind_pace       REAL NOT NULL DEFAULT 0.8,
		weight_spacing           REAL NOT NULL DEFAULT 0.5,
		weight_variation         REAL NOT NULL DEFAULT 0.3,
		default_max_slices       INTEGER NOT NULL DEFAULT 3
	)`,

	// Seed default user profile
	`INSERT OR IGNORE INTO user_profile (id) VALUES ('default')`,

	// Add short_id column to projects
	`ALTER TABLE projects ADD COLUMN short_id TEXT NOT NULL DEFAULT ''`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_short_id ON projects(short_id) WHERE short_id != ''`,
}
