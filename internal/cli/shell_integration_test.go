package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShellIntegration_ImportUseInspectWhatNow exercises a full REPL session:
// import a project via Cobra fallthrough → use <shortID> → inspect → what-now 60.
func TestShellIntegration_ImportUseInspectWhatNow(t *testing.T) {
	app := testAppFull(t)
	ctx := context.Background()

	// Write import JSON to a temp file.
	importJSON := `{
		"project": {
			"short_id": "INT01",
			"name": "Integration Shell Test",
			"domain": "education",
			"start_date": "2026-01-15",
			"target_date": "2026-06-01"
		},
		"nodes": [
			{"ref": "n1", "title": "Week 1", "kind": "week", "order": 0},
			{"ref": "n2", "title": "Week 2", "kind": "week", "order": 1}
		],
		"work_items": [
			{"ref": "w1", "node_ref": "n1", "title": "Read Chapter 1", "type": "reading", "planned_min": 60},
			{"ref": "w2", "node_ref": "n2", "title": "Read Chapter 2", "type": "reading", "planned_min": 45}
		]
	}`
	dir := t.TempDir()
	path := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(path, []byte(importJSON), 0644))

	m := newShellModel(app)

	// Step 1: Import via Cobra fallthrough.
	m.executeCommand("project import " + path)

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1, "import should create one project")
	assert.Equal(t, "INT01", projects[0].ShortID)

	// Step 2: Use the imported project.
	m.executeCommand("use INT01")
	assert.Equal(t, projects[0].ID, m.activeProjectID)
	assert.Equal(t, "INT01", m.activeShortID)
	assert.Equal(t, "Integration Shell Test", m.activeProjectName)

	// Step 3: Inspect (should show project details, not error).
	output, _ := m.executeCommand("inspect")
	assert.NotEmpty(t, output)

	// Step 4: What-now should recommend imported items.
	output, _ = m.executeCommand("what-now 60")
	assert.NotEmpty(t, output)
	assert.NotEmpty(t, m.lastRecommendedItemID, "what-now should set last recommended item")

	// Context should be preserved throughout.
	assert.Equal(t, projects[0].ID, m.activeProjectID)
}

// TestShellIntegration_UseContextScopesScheduling verifies that setting an active
// project context causes what-now to scope recommendations to that project only.
func TestShellIntegration_UseContextScopesScheduling(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	// Create two projects with work items.
	projA := testutil.NewTestProject("Scoped Alpha", testutil.WithShortID("SCA01"))
	require.NoError(t, app.Projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Week 1")
	require.NoError(t, app.Nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Alpha Reading",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wiA))

	projB := testutil.NewTestProject("Scoped Beta", testutil.WithShortID("SCB01"))
	require.NoError(t, app.Projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Week 1")
	require.NoError(t, app.Nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Beta Reading",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wiB))

	m := newShellModel(app)

	// Scope to Alpha only.
	m.executeCommand("use SCA01")

	// What-now should only recommend Alpha's item.
	m.executeCommand("what-now 60")
	assert.Equal(t, wiA.ID, m.lastRecommendedItemID,
		"scoped what-now should only recommend the active project's item")
}

// TestShellIntegration_SessionLogViaShell verifies that logging a session through
// the shell's Cobra fallthrough correctly updates work item state.
func TestShellIntegration_SessionLogViaShell(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	m := newShellModel(app)

	// Log a session via the shell.
	m.executeCommand("session log --work-item " + wiID + " --minutes 30")

	// Verify work item was updated.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, 30, wi.LoggedMin, "logged_min should reflect session logged via shell")
	assert.Equal(t, domain.WorkItemInProgress, wi.Status, "should auto-transition to in_progress")
}

// TestShellIntegration_DestructiveConfirmationFlow exercises the full destructive
// command lifecycle: trigger → pending confirmation → confirm → verify effect.
func TestShellIntegration_DestructiveConfirmationFlow(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Archive Me", testutil.WithShortID("ARC01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Archivable Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	m := newShellModel(app)

	// Step 1: Attempt project archive — should trigger confirmation.
	m.executeCommand("project archive " + proj.ID)
	require.NotNil(t, m.pendingConfirm, "should enter confirmation mode")
	assert.Equal(t, modeConfirm, m.mode)

	// Step 2: Confirm the action.
	m.execCobraCapture(m.pendingConfirm.args)
	m.pendingConfirm = nil
	m.mode = modePrompt

	// Step 3: Verify project is archived (excluded from active list).
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, projects, "project should be archived after confirmation")

	// Verify it still exists when including archived.
	allProjects, err := app.Projects.List(ctx, true)
	require.NoError(t, err)
	assert.Len(t, allProjects, 1, "archived project should still exist")
	assert.Equal(t, domain.ProjectArchived, allProjects[0].Status)
}

// TestShellIntegration_ClearResetsContext verifies that the `use` command
// (with no args) and `context clear` both reset active context.
func TestShellIntegration_ClearResetsContext(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Clearable", testutil.WithShortID("CLR01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)

	// Set context.
	m.executeCommand("use CLR01")
	assert.Equal(t, proj.ID, m.activeProjectID)
	assert.Equal(t, "CLR01", m.activeShortID)

	// Clear via `use` with no args.
	m.executeCommand("use")
	assert.Equal(t, "", m.activeProjectID)
	assert.Equal(t, "", m.activeShortID)
	assert.Equal(t, "", m.activeProjectName)

	// Re-set and clear via context command.
	m.executeCommand("use CLR01")
	assert.Equal(t, proj.ID, m.activeProjectID)

	m.execContext([]string{"clear"})
	assert.Equal(t, "", m.activeProjectID)
	assert.Equal(t, "", m.activeItemID)
}
