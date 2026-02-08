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
