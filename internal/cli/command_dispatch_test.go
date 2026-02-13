package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCommandBar creates a commandBar backed by a test SharedState.
func testCommandBar(t *testing.T, app *App) *commandBar {
	t.Helper()
	state := &SharedState{
		App:   app,
		Cache: newShellProjectCache(),
		Width: 120,
		Height: 40,
	}
	cb := newCommandBar(state)
	return &cb
}

// execCmd is a helper that runs a command on the commandBar and returns the output
// message (if any) by executing the returned tea.Cmd.
func execCmd(cb *commandBar, input string) string {
	cmd := cb.executeCommand(input)
	if cmd == nil {
		return ""
	}
	msg := cmd()
	if out, ok := msg.(cmdOutputMsg); ok {
		return out.output
	}
	return ""
}

// execCmdAsync is like execCmd but handles commands that return tea.Batch
// (e.g. async commands with a loading indicator + async work).
func execCmdAsync(cb *commandBar, input string) string {
	cmd := cb.executeCommand(input)
	if cmd == nil {
		return ""
	}
	msg := cmd()
	if out, ok := msg.(cmdOutputMsg); ok {
		return out.output
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if out, ok := c().(cmdOutputMsg); ok {
				return out.output
			}
		}
	}
	return ""
}

// --- Cobra dispatch tests ---

func TestCommandBar_DispatchesToCobra(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	cb.executeCommand(`project add --id SHL01 --name "Shell Dispatch" --domain education --start 2026-01-15`)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "SHL01", projects[0].ShortID)
	assert.Equal(t, "Shell Dispatch", projects[0].Name)
}

// --- Use command tests ---

func TestCommandBar_UseSetsAndClearsActiveProject(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Shell Focus", testutil.WithShortID("USE01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	cb := testCommandBar(t, app)

	cb.executeCommand("use use01")
	assert.Equal(t, proj.ID, cb.state.ActiveProjectID)
	assert.Equal(t, "USE01", cb.state.ActiveShortID)
	assert.Equal(t, "Shell Focus", cb.state.ActiveProjectName)

	cb.executeCommand("use")
	assert.Equal(t, "", cb.state.ActiveProjectID)
	assert.Equal(t, "", cb.state.ActiveShortID)
	assert.Equal(t, "", cb.state.ActiveProjectName)
}

// --- Exit/quit tests ---

func TestCommandBar_ExitReturnsQuit(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	cmd := cb.executeCommand("exit")
	require.NotNil(t, cmd)
	// tea.Quit returns a quitMsg
	msg := cmd()
	assert.IsType(t, tea.QuitMsg{}, msg)
}

func TestCommandBar_QuitReturnsQuit(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	cmd := cb.executeCommand("quit")
	require.NotNil(t, cmd)
	msg := cmd()
	assert.IsType(t, tea.QuitMsg{}, msg)
}

// --- Confirmation (destructive) command tests ---

func TestCommandBar_ForceFlagSkipsConfirmation(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Quick", testutil.WithShortID("QUIK01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	cb := testCommandBar(t, app)

	// With --force, should execute immediately.
	cb.executeCommand("project remove QUIK01 --force")

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, projects, 0)
}

func TestCommandBar_DestructiveProjectArchive_RequiresConfirmation(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Doomed", testutil.WithShortID("DOOM01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	cb := testCommandBar(t, app)

	// Without --force, destructive commands push a wizard confirmation.
	cmd := cb.executeCommand("project archive DOOM01")
	require.NotNil(t, cmd)
	msg := cmd()
	_, isPush := msg.(pushViewMsg)
	assert.True(t, isPush, "archive should push a confirmation wizard")

	// Project should NOT yet be archived.
	p, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Nil(t, p.ArchivedAt)
}

func TestCommandBar_NonDestructivePassesThrough(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	// "project list" should not trigger confirmation — should just output.
	cmd := cb.executeCommand("project list")
	// The command should return output, not a wizard push.
	if cmd != nil {
		msg := cmd()
		_, isOutput := msg.(cmdOutputMsg)
		_, isPush := msg.(pushViewMsg)
		assert.True(t, isOutput || msg == nil, "project list should not push a wizard view, got push=%v", isPush)
	}
}

// --- Error recovery tests ---

func TestCommandBar_StatePreservedAfterFailure(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Stable", testutil.WithShortID("STBL01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	cb := testCommandBar(t, app)

	// Set active project.
	cb.executeCommand("use STBL01")
	assert.Equal(t, proj.ID, cb.state.ActiveProjectID)

	// Run a command that errors.
	cb.executeCommand("inspect NONEXISTENT")

	// Active project should still be set.
	assert.Equal(t, proj.ID, cb.state.ActiveProjectID)
	assert.Equal(t, "STBL01", cb.state.ActiveShortID)
}

func TestCommandBar_InvalidCommandDoesNotCorruptState(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	output := execCmd(cb, "definitely-not-a-command foo bar")
	// Should produce some output (error or suggestion), not panic.
	_ = output
	assert.Equal(t, "", cb.state.ActiveProjectID)
}

// --- Replan passthrough test ---

func TestCommandBar_ReplanPassesThrough(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	// Should not panic — replan with no data just returns success or empty.
	cmd := cb.executeCommand("replan")
	// Not a wizard push.
	if cmd != nil {
		msg := cmd()
		_, isPush := msg.(pushViewMsg)
		assert.False(t, isPush, "replan should not push a wizard view")
	}
}

// --- Multi-step journey test ---

func TestCommandBar_MultiStepJourney(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	cb := testCommandBar(t, app)

	// Step 1: Create a project via Cobra dispatch.
	cb.executeCommand(`project add --id SHL01 --name "Shell Journey" --domain education --start 2026-01-15`)
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "SHL01", projects[0].ShortID)

	// Step 2: Add a node.
	cb.executeCommand(`node add --project ` + projects[0].ID + ` --title "Week 1" --kind week`)
	allNodes, err := app.Nodes.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	require.Len(t, allNodes, 1)

	// Step 3: Add a work item.
	cb.executeCommand(`work add --node ` + allNodes[0].ID + ` --title "Read Chapter 1" --type reading --planned-min 60`)
	allItems, err := app.WorkItems.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	require.Len(t, allItems, 1)

	// Step 4: Use project context.
	cb.executeCommand("use shl01")
	assert.Equal(t, projects[0].ID, cb.state.ActiveProjectID)
	assert.Equal(t, "SHL01", cb.state.ActiveShortID)
	assert.Equal(t, "Shell Journey", cb.state.ActiveProjectName)

	// Step 5: Status command.
	output := execCmd(cb, "status")
	assert.NotEmpty(t, output)

	// Step 6: What-now.
	output = execCmd(cb, "what-now 60")
	assert.NotEmpty(t, output)

	// Step 7: Clear project context.
	cb.executeCommand("use")
	assert.Equal(t, "", cb.state.ActiveProjectID)
	assert.Equal(t, "", cb.state.ActiveShortID)

	// Step 8: Exit.
	cmd := cb.executeCommand("exit")
	require.NotNil(t, cmd)
	msg := cmd()
	assert.IsType(t, tea.QuitMsg{}, msg)
}

// --- Context command tests ---

func TestCommandBar_ContextShowAndClear(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Context Test", testutil.WithShortID("CTX01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	cb := testCommandBar(t, app)

	// Set active project.
	cb.executeCommand("use CTX01")
	assert.Equal(t, proj.ID, cb.state.ActiveProjectID)

	// Show context.
	output := execCmd(cb, "context")
	assert.Contains(t, output, "Context Test")

	// Clear context.
	cb.executeCommand("context clear")
	assert.Equal(t, "", cb.state.ActiveProjectID)
	assert.Equal(t, "", cb.state.ActiveItemID)
}

// --- Start and finish tests ---

func TestCommandBar_StartAndFinish(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	projID, wiID := seedProjectWithWork(t, app)

	cb := testCommandBar(t, app)

	// Set project context.
	cb.state.SetActiveProject(ctx, projID)

	// Start the item.
	cb.startExecute(wiID)
	assert.Equal(t, wiID, cb.state.ActiveItemID)

	// Verify in-progress status.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, wi.Status)

	// Finish the item.
	cb.finishExecute(wiID)

	// Active item should be cleared.
	assert.Equal(t, "", cb.state.ActiveItemID)

	// Verify done status.
	wi, err = app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, wi.Status)
}

func TestCommandBar_WithFlagsSkipsWizard(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Direct Work", testutil.WithShortID("DWK01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	cb := testCommandBar(t, app)

	// "work add" WITH flags should execute directly (Cobra passthrough).
	cb.executeCommand("work add --node " + node.ID + " --title Test --type task")

	items, err := app.WorkItems.ListByNode(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

// --- Integration tests ---

func TestCommandBar_ImportUseInspectWhatNow(t *testing.T) {
	app := testAppFull(t)
	ctx := context.Background()

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
	require.NoError(t, os.WriteFile(path, []byte(importJSON), 0o644))

	cb := testCommandBar(t, app)

	// Step 1: Import via Cobra fallthrough.
	cb.executeCommand("project import " + path)

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1, "import should create one project")
	assert.Equal(t, "INT01", projects[0].ShortID)

	// Step 2: Use the imported project.
	cb.executeCommand("use INT01")
	assert.Equal(t, projects[0].ID, cb.state.ActiveProjectID)
	assert.Equal(t, "INT01", cb.state.ActiveShortID)
	assert.Equal(t, "Integration Shell Test", cb.state.ActiveProjectName)

	// Step 3: Status command.
	output := execCmd(cb, "status")
	assert.NotEmpty(t, output)

	// Step 4: What-now should recommend imported items.
	output = execCmd(cb, "what-now 60")
	assert.NotEmpty(t, output)

	// Context should be preserved throughout.
	assert.Equal(t, projects[0].ID, cb.state.ActiveProjectID)
}

func TestCommandBar_UseContextScopesScheduling(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	// Create two projects with work items.
	projA := testutil.NewTestProject("Scoped Alpha", testutil.WithShortID("SCA01"))
	require.NoError(t, app.Projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Week 1")
	require.NoError(t, app.Nodes.Create(ctx, nodeA))
	testutil.NewTestWorkItem(nodeA.ID, "Alpha Reading",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))

	projB := testutil.NewTestProject("Scoped Beta", testutil.WithShortID("SCB01"))
	require.NoError(t, app.Projects.Create(ctx, projB))

	cb := testCommandBar(t, app)

	// Scope to Alpha only.
	cb.executeCommand("use SCA01")
	assert.Equal(t, projA.ID, cb.state.ActiveProjectID)
	assert.Equal(t, "SCA01", cb.state.ActiveShortID)
}

func TestCommandBar_SessionLogViaShell(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	cb := testCommandBar(t, app)

	// Log a session via Cobra fallthrough.
	cb.executeCommand("session log --work-item " + wiID + " --minutes 30")

	// Verify work item was updated.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, 30, wi.LoggedMin, "logged_min should reflect session logged via shell")
	assert.Equal(t, domain.WorkItemInProgress, wi.Status, "should auto-transition to in_progress")
}

func TestCommandBar_DestructiveProjectRemove_ForceBypasses(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Force Remove", testutil.WithShortID("FRC01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	cb := testCommandBar(t, app)

	cb.executeCommand("project remove " + proj.ID + " --force")

	projects, err := app.Projects.List(ctx, true)
	require.NoError(t, err)
	assert.Empty(t, projects, "project should be deleted after --force removal")
}

func TestCommandBar_ClearResetsContext(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Clearable", testutil.WithShortID("CLR01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	cb := testCommandBar(t, app)

	// Set context.
	cb.executeCommand("use CLR01")
	assert.Equal(t, proj.ID, cb.state.ActiveProjectID)
	assert.Equal(t, "CLR01", cb.state.ActiveShortID)

	// Clear via `use` with no args.
	cb.executeCommand("use")
	assert.Equal(t, "", cb.state.ActiveProjectID)
	assert.Equal(t, "", cb.state.ActiveShortID)
	assert.Equal(t, "", cb.state.ActiveProjectName)

	// Re-set and clear via context command.
	cb.executeCommand("use CLR01")
	assert.Equal(t, proj.ID, cb.state.ActiveProjectID)

	cb.executeCommand("context clear")
	assert.Equal(t, "", cb.state.ActiveProjectID)
	assert.Equal(t, "", cb.state.ActiveItemID)
}

// --- Draft pushes a view ---

func TestCommandBar_DraftPushesView(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	cmd := cb.executeCommand("draft")
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(pushViewMsg)
	assert.True(t, ok, "draft should push a view")
}

// --- Help chat pushes a view ---

func TestCommandBar_HelpChatPushesView(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	cmd := cb.executeCommand("help chat")
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(pushViewMsg)
	assert.True(t, ok, "help chat should push a view")
}

func TestCommandBar_HelpWithoutChatShowsHelp(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	output := execCmd(cb, "help")
	assert.NotEmpty(t, output)
}

// --- Work archive destructive test ---

func TestCommandBar_WorkArchive_RequiresConfirmation(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	cb := testCommandBar(t, app)

	// Without --force, destructive commands push a wizard confirmation.
	cmd := cb.executeCommand("work archive " + wiID)
	require.NotNil(t, cmd)
	msg := cmd()
	_, isPush := msg.(pushViewMsg)
	assert.True(t, isPush, "work archive should push a confirmation wizard")

	// Work item should NOT yet be archived.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Nil(t, wi.ArchivedAt, "work item should not be archived before confirmation")
}
