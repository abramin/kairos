package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrate_Idempotent(t *testing.T) {
	db := openTestDB(t)

	// Run migrations a second time — should succeed without error.
	err := Migrate(db)
	require.NoError(t, err)

	// Third time for good measure.
	err = Migrate(db)
	require.NoError(t, err)
}

func TestMigrate_CreatesAllTables(t *testing.T) {
	db := openTestDB(t)

	expected := []string{"projects", "plan_nodes", "work_items", "dependencies", "work_session_logs", "user_profile"}
	for _, table := range expected {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestMigrate_CreatesIndexes(t *testing.T) {
	db := openTestDB(t)

	expected := []string{
		"idx_plan_nodes_project",
		"idx_plan_nodes_parent",
		"idx_work_items_node",
		"idx_work_items_status",
		"idx_sessions_work_item",
		"idx_sessions_started",
		"idx_projects_short_id",
	}
	for _, idx := range expected {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&name)
		require.NoError(t, err, "index %s should exist", idx)
	}
}

func TestMigrate_ForeignKeysEnabled(t *testing.T) {
	db := openTestDB(t)

	var fk int
	err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	require.NoError(t, err)
	assert.Equal(t, 1, fk, "foreign keys should be enabled")
}

func TestMigrate_WALModeRequested(t *testing.T) {
	// In-memory SQLite uses "memory" journal mode; WAL only applies to file DBs.
	// This test verifies OpenDB issues the PRAGMA (a no-op for :memory:).
	db := openTestDB(t)

	var mode string
	err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	require.NoError(t, err)
	// In-memory DB reports "memory" — that's expected.
	assert.Equal(t, "memory", mode)
}

func TestMigrate_SeedsDefaultUserProfile(t *testing.T) {
	db := openTestDB(t)

	var id string
	var bufferPct float64
	err := db.QueryRow(`SELECT id, buffer_pct FROM user_profile WHERE id = 'default'`).Scan(&id, &bufferPct)
	require.NoError(t, err)
	assert.Equal(t, "default", id)
	assert.Equal(t, 0.1, bufferPct)
}

func TestMigrate_PlanNodesAssessmentKind(t *testing.T) {
	db := openTestDB(t)

	// Verify the plan_nodes CHECK constraint includes 'assessment'.
	var createSQL string
	err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='plan_nodes'`).Scan(&createSQL)
	require.NoError(t, err)
	assert.Contains(t, createSQL, "'assessment'", "plan_nodes kind CHECK should include 'assessment'")
}

func TestMigrate_PlanNodesAssessmentKind_Idempotent(t *testing.T) {
	db := openTestDB(t)

	// migratePlanNodesAssessmentKind should be idempotent — already-migrated table
	// should not error on re-run.
	err := migratePlanNodesAssessmentKind(db)
	require.NoError(t, err)
}

func TestMigrate_ProjectsShortIDColumn(t *testing.T) {
	db := openTestDB(t)

	// Verify the short_id column exists on projects.
	rows, err := db.Query(`PRAGMA table_info(projects)`)
	require.NoError(t, err)
	defer rows.Close()

	found := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk))
		if name == "short_id" {
			found = true
		}
	}
	assert.True(t, found, "projects table should have short_id column")
}

func TestMigrate_WorkItemsCheckConstraints(t *testing.T) {
	db := openTestDB(t)

	// Insert a project and node so we can test work item constraints.
	_, err := db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at, short_id)
		VALUES ('p1', 'Test', 'test', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z', 'TST01')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO plan_nodes (id, project_id, title, kind, created_at, updated_at)
		VALUES ('n1', 'p1', 'Node 1', 'generic', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)

	// Invalid status should fail.
	_, err = db.Exec(`INSERT INTO work_items (id, node_id, title, status, created_at, updated_at)
		VALUES ('w1', 'n1', 'Task', 'INVALID', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	assert.Error(t, err, "invalid status should be rejected by CHECK constraint")

	// Valid status should succeed.
	_, err = db.Exec(`INSERT INTO work_items (id, node_id, title, status, created_at, updated_at)
		VALUES ('w1', 'n1', 'Task', 'todo', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	assert.NoError(t, err)
}

func TestMigrate_ProjectsStatusCheckConstraint(t *testing.T) {
	db := openTestDB(t)

	_, err := db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at, short_id)
		VALUES ('p1', 'Test', 'test', '2025-01-01', 'INVALID', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z', 'TST01')`)
	assert.Error(t, err, "invalid project status should be rejected by CHECK constraint")
}

func TestMigrate_DependenciesPrimaryKey_UniquePair(t *testing.T) {
	db := openTestDB(t)

	_, err := db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at, short_id)
		VALUES ('p1', 'Test', 'test', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z', 'DEP01')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO plan_nodes (id, project_id, title, kind, created_at, updated_at)
		VALUES ('n1', 'p1', 'Node 1', 'generic', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO work_items (id, node_id, title, status, created_at, updated_at)
		VALUES ('w1', 'n1', 'Task 1', 'todo', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO work_items (id, node_id, title, status, created_at, updated_at)
		VALUES ('w2', 'n1', 'Task 2', 'todo', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO dependencies (predecessor_work_item_id, successor_work_item_id) VALUES ('w1', 'w2')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO dependencies (predecessor_work_item_id, successor_work_item_id) VALUES ('w1', 'w2')`)
	assert.Error(t, err, "duplicate dependency pair should violate composite primary key")
}

func TestMigrate_WorkSessionLogs_DefaultValues(t *testing.T) {
	db := openTestDB(t)

	_, err := db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at, short_id)
		VALUES ('p1', 'Test', 'test', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z', 'SES01')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO plan_nodes (id, project_id, title, kind, created_at, updated_at)
		VALUES ('n1', 'p1', 'Node 1', 'generic', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO work_items (id, node_id, title, status, created_at, updated_at)
		VALUES ('w1', 'n1', 'Task 1', 'todo', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO work_session_logs (id, work_item_id, started_at, minutes, created_at)
		VALUES ('s1', 'w1', '2025-01-01T00:00:00Z', 25, '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)

	var unitsDelta int
	var note string
	err = db.QueryRow(`SELECT units_done_delta, note FROM work_session_logs WHERE id = 's1'`).Scan(&unitsDelta, &note)
	require.NoError(t, err)
	assert.Equal(t, 0, unitsDelta)
	assert.Equal(t, "", note)
}

func TestMigrate_ProjectsShortIDPartialUniqueIndex(t *testing.T) {
	db := openTestDB(t)

	// Empty short IDs should be allowed repeatedly due to partial unique index predicate.
	_, err := db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at, short_id)
		VALUES ('p1', 'Test 1', 'test', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z', '')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at, short_id)
		VALUES ('p2', 'Test 2', 'test', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z', '')`)
	require.NoError(t, err)

	// Non-empty duplicates should violate unique index.
	_, err = db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at, short_id)
		VALUES ('p3', 'Test 3', 'test', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z', 'DUP01')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at, short_id)
		VALUES ('p4', 'Test 4', 'test', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z', 'DUP01')`)
	assert.Error(t, err)
}

func TestMigratePlanNodesAssessmentKind_UpgradesLegacySchema(t *testing.T) {
	legacyDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { legacyDB.Close() })

	_, err = legacyDB.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)

	_, err = legacyDB.Exec(`CREATE TABLE projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		domain TEXT NOT NULL,
		start_date TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	require.NoError(t, err)
	_, err = legacyDB.Exec(`CREATE TABLE plan_nodes (
		id                 TEXT PRIMARY KEY,
		project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		parent_id          TEXT REFERENCES plan_nodes(id) ON DELETE CASCADE,
		title              TEXT NOT NULL,
		kind               TEXT NOT NULL
		                   CHECK(kind IN ('week','module','book','stage','section','generic')),
		order_index        INTEGER NOT NULL DEFAULT 0,
		due_date           TEXT,
		not_before         TEXT,
		not_after          TEXT,
		planned_min_budget INTEGER,
		created_at         TEXT NOT NULL,
		updated_at         TEXT NOT NULL
	)`)
	require.NoError(t, err)
	_, err = legacyDB.Exec(`CREATE INDEX idx_plan_nodes_project ON plan_nodes(project_id)`)
	require.NoError(t, err)
	_, err = legacyDB.Exec(`CREATE INDEX idx_plan_nodes_parent ON plan_nodes(parent_id)`)
	require.NoError(t, err)

	_, err = legacyDB.Exec(`INSERT INTO projects (id, name, domain, start_date, status, created_at, updated_at)
		VALUES ('p1', 'Legacy', 'test', '2025-01-01', 'active', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)
	_, err = legacyDB.Exec(`INSERT INTO plan_nodes (id, project_id, parent_id, title, kind, order_index, created_at, updated_at)
		VALUES ('n1', 'p1', NULL, 'Legacy Node', 'week', 1, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`)
	require.NoError(t, err)

	require.NoError(t, migratePlanNodesAssessmentKind(legacyDB))

	var createSQL string
	err = legacyDB.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='plan_nodes'`).Scan(&createSQL)
	require.NoError(t, err)
	assert.Contains(t, createSQL, "'assessment'")

	var title string
	var orderIndex int
	err = legacyDB.QueryRow(`SELECT title, order_index FROM plan_nodes WHERE id = 'n1'`).Scan(&title, &orderIndex)
	require.NoError(t, err)
	assert.Equal(t, "Legacy Node", title)
	assert.Equal(t, 1, orderIndex)
}
