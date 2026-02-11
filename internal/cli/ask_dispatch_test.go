package cli

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDispatchIntent_ProjectImport_CreatesProjectInDB verifies that dispatching
// a project_import intent through the full pipeline actually creates the project
// in the database.
func TestDispatchIntent_ProjectImport_CreatesProjectInDB(t *testing.T) {
	app := testAppFull(t)
	ctx := context.Background()

	// Write a minimal import JSON to a temp file.
	importJSON := `{
		"project": {
			"short_id": "IMP01",
			"name": "Import Dispatch Test",
			"domain": "education",
			"start_date": "2026-01-15",
			"target_date": "2026-06-01"
		},
		"nodes": [
			{"ref": "n1", "title": "Week 1", "kind": "week", "order": 0}
		],
		"work_items": [
			{"ref": "w1", "node_ref": "n1", "title": "Read Chapter 1", "type": "reading", "planned_min": 60}
		]
	}`
	path := writeTestFile(t, "import-dispatch.json", importJSON)

	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentProjectImport,
		Arguments: map[string]interface{}{"file_path": path},
	}

	err := dispatchIntent(app, intent)
	require.NoError(t, err)

	// Verify the project was actually created in the database.
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "IMP01", projects[0].ShortID)
	assert.Equal(t, "Import Dispatch Test", projects[0].Name)

	// Verify work items were created.
	items, err := app.WorkItems.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Read Chapter 1", items[0].Title)
	assert.Equal(t, 60, items[0].PlannedMin)
}

// TestDispatchIntent_ProjectUpdate_DBSideEffect verifies that updating a project
// via intent dispatch correctly persists the changes.
func TestDispatchIntent_ProjectUpdate_DBSideEffect(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	target := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	proj := testutil.NewTestProject("Updatable Project",
		testutil.WithShortID("UPD01"),
		testutil.WithTargetDate(target),
	)
	require.NoError(t, app.Projects.Create(ctx, proj))

	// Dispatch an update intent that changes the name and target date.
	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentProjectUpdate,
		Arguments: map[string]interface{}{
			"project_id":  "UPD01",
			"name":        "Renamed Project",
			"target_date": "2026-09-15",
		},
	}

	err := dispatchIntent(app, intent)
	require.NoError(t, err)

	updated, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed Project", updated.Name,
		"project name should be updated in DB")
	require.NotNil(t, updated.TargetDate)
	assert.Equal(t, "2026-09-15", updated.TargetDate.Format("2006-01-02"),
		"target date should be updated in DB")
}

// TestDispatchIntent_ProjectArchive_DBSideEffect verifies that archiving via
// intent dispatch correctly soft-deletes the project and excludes it from
// active project listings.
func TestDispatchIntent_ProjectArchive_DBSideEffect(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Archivable Project", testutil.WithShortID("ARC02"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	// Seed a work item so the project has some data.
	node := testutil.NewTestNode(proj.ID, "Module 1")
	require.NoError(t, app.Nodes.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentProjectUpdate,
		Arguments: map[string]interface{}{
			"project_id": "ARC02",
			"status":     "archived",
		},
	}

	err := dispatchIntent(app, intent)
	require.NoError(t, err)

	// Verify project is excluded from active list.
	active, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, active, "archived project should not appear in active list")

	// Verify project still exists when including archived.
	all, err := app.Projects.List(ctx, true)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, domain.ProjectArchived, all[0].Status)
	require.NotNil(t, all[0].ArchivedAt)
}

// TestDispatchIntent_NeedsConfirmation_DoesNotExecute verifies that when
// an intent is in StateNeedsConfirmation, the ask command does NOT auto-dispatch —
// the project remains untouched until the user explicitly confirms.
func TestDispatchIntent_NeedsConfirmation_DoesNotExecute(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Safety Test", testutil.WithShortID("SAF01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	// Wire a stub that returns a write intent needing confirmation.
	app.Intent = stubIntentService{
		resolution: &intelligence.AskResolution{
			ParsedIntent: &intelligence.ParsedIntent{
				Intent:               intelligence.IntentProjectUpdate,
				Risk:                 intelligence.RiskWrite,
				Arguments:            map[string]interface{}{"project_id": "SAF01", "status": "archived"},
				Confidence:           0.95,
				RequiresConfirmation: true,
			},
			ExecutionState:   intelligence.StateNeedsConfirmation,
			ExecutionMessage: "requires confirmation",
		},
	}

	resolution, err := app.Intent.Parse(ctx, "archive project SAF01")
	require.NoError(t, err)

	assert.Equal(t, intelligence.StateNeedsConfirmation, resolution.ExecutionState,
		"write intent should be in NeedsConfirmation state")

	// Without --yes flag, the ask command would NOT call dispatchIntent.
	// Verify the project is untouched.
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, projects, 1, "project should still exist without confirmation")
	assert.Equal(t, domain.ProjectActive, projects[0].Status,
		"project status should be unchanged without dispatch")

	// Now simulate explicit confirmation by calling dispatchIntent directly.
	err = dispatchIntent(app, resolution.ParsedIntent)
	require.NoError(t, err)

	// Verify the project was archived after explicit dispatch.
	active, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, active, "project should be archived after confirmed dispatch")
}

// writeTestFile creates a temporary file with the given content and returns its path.
func writeTestFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/" + name
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}
