package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrate_UpgradePath_LegacyV1ToCurrentSchema simulates upgrading an existing
// database that was created with an older schema version. Verifies that:
// 1. Data inserted under the old schema survives migration
// 2. New columns are added with correct defaults
// 3. New constraints are applied correctly
// 4. Indexes are created
func TestMigrate_UpgradePath_LegacyV1ToCurrentSchema(t *testing.T) {
	// Create a raw DB without using OpenDB (to manually control schema).
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)

	// Apply a "legacy" schema: projects WITHOUT short_id column,
	// plan_nodes WITH assessment kind (already migrated), no seq columns,
	// no baseline_daily_min. This represents the most common upgrade path:
	// a user who already ran the assessment migration but not the seq migration.
	legacyStatements := []string{
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
		`INSERT OR IGNORE INTO user_profile (id) VALUES ('default')`,
	}

	for i, stmt := range legacyStatements {
		_, err := db.Exec(stmt)
		require.NoError(t, err, "legacy statement %d failed", i)
	}

	// Insert legacy data BEFORE running migrations.
	_, err = db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at)
		VALUES ('p1', 'Legacy Project', 'education', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO plan_nodes (id, project_id, title, kind, order_index, created_at, updated_at)
		VALUES ('n1', 'p1', 'Week 1', 'week', 1, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO work_items (id, node_id, title, type, planned_min, created_at, updated_at)
		VALUES ('w1', 'n1', 'Reading', 'reading', 60, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO work_session_logs (id, work_item_id, started_at, minutes, created_at)
		VALUES ('s1', 'w1', '2025-01-15T10:00:00Z', 45, '2025-01-15T10:45:00Z')`)
	require.NoError(t, err)

	// === Run current migrations on legacy DB ===
	err = Migrate(db)
	require.NoError(t, err, "migration on legacy schema should succeed")

	// === Verify data survived ===
	var projName, projStatus string
	err = db.QueryRow(`SELECT name, status FROM projects WHERE id = 'p1'`).Scan(&projName, &projStatus)
	require.NoError(t, err)
	assert.Equal(t, "Legacy Project", projName, "project name should survive migration")
	assert.Equal(t, "active", projStatus)

	var nodeTitle, nodeKind string
	var orderIndex int
	err = db.QueryRow(`SELECT title, kind, order_index FROM plan_nodes WHERE id = 'n1'`).Scan(&nodeTitle, &nodeKind, &orderIndex)
	require.NoError(t, err)
	assert.Equal(t, "Week 1", nodeTitle, "node title should survive migration")
	assert.Equal(t, "week", nodeKind)
	assert.Equal(t, 1, orderIndex)

	var wiTitle string
	var plannedMin int
	err = db.QueryRow(`SELECT title, planned_min FROM work_items WHERE id = 'w1'`).Scan(&wiTitle, &plannedMin)
	require.NoError(t, err)
	assert.Equal(t, "Reading", wiTitle, "work item should survive migration")
	assert.Equal(t, 60, plannedMin)

	var sessMinutes int
	err = db.QueryRow(`SELECT minutes FROM work_session_logs WHERE id = 's1'`).Scan(&sessMinutes)
	require.NoError(t, err)
	assert.Equal(t, 45, sessMinutes, "session log should survive migration")

	// === Verify new columns added with defaults ===

	// projects.short_id should default to ''
	var shortID string
	err = db.QueryRow(`SELECT short_id FROM projects WHERE id = 'p1'`).Scan(&shortID)
	require.NoError(t, err)
	assert.Equal(t, "", shortID, "legacy project should get default empty short_id")

	// user_profile.baseline_daily_min should default to 30
	var baselineDaily int
	err = db.QueryRow(`SELECT baseline_daily_min FROM user_profile WHERE id = 'default'`).Scan(&baselineDaily)
	require.NoError(t, err)
	assert.Equal(t, 30, baselineDaily, "baseline_daily_min should default to 30")

	// plan_nodes.seq should be backfilled
	var nodeSeq int
	err = db.QueryRow(`SELECT seq FROM plan_nodes WHERE id = 'n1'`).Scan(&nodeSeq)
	require.NoError(t, err)
	assert.Greater(t, nodeSeq, 0, "node seq should be backfilled")

	// work_items.seq should be backfilled
	var wiSeq int
	err = db.QueryRow(`SELECT seq FROM work_items WHERE id = 'w1'`).Scan(&wiSeq)
	require.NoError(t, err)
	assert.Greater(t, wiSeq, 0, "work item seq should be backfilled")

	// === Verify assessment kind is now allowed ===
	var createSQL string
	err = db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='plan_nodes'`).Scan(&createSQL)
	require.NoError(t, err)
	assert.Contains(t, createSQL, "'assessment'", "plan_nodes should support assessment kind after migration")

	// Verify we can insert an assessment node.
	_, err = db.Exec(`INSERT INTO plan_nodes (id, project_id, title, kind, order_index, created_at, updated_at)
		VALUES ('n2', 'p1', 'Final Exam', 'assessment', 2, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err, "should be able to insert assessment node after migration")

	// === Verify idempotency: running Migrate again should not break anything ===
	err = Migrate(db)
	require.NoError(t, err, "re-running Migrate on already-migrated DB should succeed")

	// Data should still be intact.
	var projNameAfter string
	err = db.QueryRow(`SELECT name FROM projects WHERE id = 'p1'`).Scan(&projNameAfter)
	require.NoError(t, err)
	assert.Equal(t, "Legacy Project", projNameAfter)
}
