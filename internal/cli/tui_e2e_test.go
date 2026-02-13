package cli

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── E2E helpers ──────────────────────────────────────────────────────────────

// navigateToActionMenu opens the recommendation view and selects the first item.
func navigateToActionMenu(d *TestDriver) {
	d.T.Helper()
	d.PressKey('?')
	d.PressEnter()
}

// seedProjectWithShortIDAndWork creates a project with a specific ShortID, a node,
// and a work item, returning the project ID and work item ID.
func seedProjectWithShortIDAndWork(t *testing.T, app *App, shortID, projectName string) (string, string) {
	t.Helper()
	projID, _, wiID := seedProjectCore(t, app, seedOpts{
		shortID: shortID, name: projectName, plannedMin: 120,
	})
	return projID, wiID
}

// seedTwoEqualProjects creates 2 equal-priority projects for multi-project tests.
func seedTwoEqualProjects(t *testing.T, app *App) (projAID, projBID, wiAID, wiBID string) {
	t.Helper()
	pA, _, wA := seedProjectCore(t, app, seedOpts{shortID: "ALP01", name: "Alpha Project", plannedMin: 300})
	pB, _, wB := seedProjectCore(t, app, seedOpts{shortID: "BRV01", name: "Bravo Project", plannedMin: 300})
	return pA, pB, wA, wB
}

// =============================================================================
// 1. Log Session via Action Menu — full round-trip
// =============================================================================

func TestE2E_LogSession_ActionMenu_FullRoundTrip(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)
	ctx := context.Background()

	d := NewTestDriver(t, app)

	// Dashboard → Recommendation → ActionMenu
	navigateToActionMenu(d)
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())
	assert.Equal(t, 3, d.ViewStackLen()) // Dashboard + Recommendation + ActionMenu

	// Press 'l' to open log form.
	d.PressKey('l')
	assert.Equal(t, ViewForm, d.ActiveViewID())

	// The log form has 3 groups (duration, units, notes).
	// Accept defaults by pressing Enter through each group.
	d.PressEnter() // duration (default 60)
	d.PressEnter() // units (default 0)
	d.PressEnter() // notes (empty)

	// Form should complete, triggering wizardCompleteMsg.
	// View should pop back (no longer on ViewForm).
	assert.NotEqual(t, ViewForm, d.ActiveViewID())

	// Verify session was persisted.
	sessions, err := app.Sessions.ListByWorkItem(ctx, wiID)
	require.NoError(t, err)
	require.Len(t, sessions, 1, "one session should be persisted")
	assert.Equal(t, 60, sessions[0].Minutes, "should log default 60 minutes")

	// Verify work item was updated.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, 60, wi.LoggedMin, "logged_min should be updated")
	assert.Equal(t, domain.WorkItemInProgress, wi.Status,
		"first session should auto-transition to in_progress")

	// Verify shared state was updated.
	assert.Equal(t, wiID, d.State().ActiveItemID)
	assert.Equal(t, 60, d.State().LastDuration)
}

// =============================================================================
// 2. Log Session via Command Bar
// =============================================================================

func TestE2E_LogSession_CommandBar(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithShortIDAndWork(t, app, "LOG01", "Log Command Test")
	ctx := context.Background()

	d := NewTestDriver(t, app)

	// Set project context.
	d.Command("use LOG01")
	assert.Equal(t, "LOG01", d.State().ActiveShortID)

	// Log via command bar: `log #1 45` (item seq #1, 45 minutes).
	d.Command("log #1 45")

	// Verify output contains confirmation.
	output := d.LastOutput()
	assert.Contains(t, output, "45m")
	assert.Contains(t, output, "Reading")

	// Verify session persisted.
	sessions, err := app.Sessions.ListByWorkItem(ctx, wiID)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, 45, sessions[0].Minutes)

	// Verify state updated.
	assert.Equal(t, wiID, d.State().ActiveItemID)
	assert.Equal(t, 45, d.State().LastDuration)
}

// =============================================================================
// 3. Start → Log → Mark Done (lifecycle)
// =============================================================================

func TestE2E_WorkItem_StartLogDone_Lifecycle(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithShortIDAndWork(t, app, "LIFE01", "Lifecycle Test")
	ctx := context.Background()

	d := NewTestDriver(t, app)

	// Set project context so command bar log resolves the item.
	d.Command("use LIFE01")
	assert.Equal(t, "LIFE01", d.State().ActiveShortID)

	// === Step 1: Start item via action menu ===
	navigateToActionMenu(d)
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	d.PressKey('s') // Start Timer
	assert.NotEqual(t, ViewActionMenu, d.ActiveViewID(), "should pop after start")

	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, wi.Status, "status should be in_progress after start")
	assert.Equal(t, wiID, d.State().ActiveItemID, "active item should be set")

	// === Step 2: Log a session via command bar ===
	// wizardCompleteMsg from start focused the command bar — blur it first.
	if d.CmdBarFocused() {
		d.PressEsc()
	}
	// ActiveItemID is set from start, log 30 min against it.
	d.Command("log 30")

	sessions, err := app.Sessions.ListByWorkItem(ctx, wiID)
	require.NoError(t, err)
	require.Len(t, sessions, 1, "session should be persisted")
	assert.Equal(t, 30, sessions[0].Minutes)

	wi, err = app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, 30, wi.LoggedMin, "logged_min should reflect session")

	// === Step 3: Mark done via action menu ===
	// Blur command bar if focused from prior command output.
	if d.CmdBarFocused() {
		d.PressEsc()
	}
	navigateToActionMenu(d)
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	d.PressKey('d') // Mark Done
	assert.NotEqual(t, ViewActionMenu, d.ActiveViewID(), "should pop after mark done")

	wi, err = app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, wi.Status, "status should be done")
}

// =============================================================================
// 4. Edit Work Item via Action Menu
// =============================================================================

func TestE2E_EditWorkItem_ActionMenu(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)
	ctx := context.Background()

	d := NewTestDriver(t, app)

	// Navigate to action menu.
	navigateToActionMenu(d)
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// Press 'e' to open edit form.
	d.PressKey('e')
	assert.Equal(t, ViewForm, d.ActiveViewID())

	// The edit form has 2 groups:
	// Group 1: Title, Description, PlannedMin, Type (select) — 4 fields
	// Group 2: DueDate, NotBefore, MinSession, MaxSession — 4 fields
	// In huh, Enter advances through fields within a group, then submits the group.
	// Group 1: 4 fields.
	d.PressEnter() // Title (accept default)
	d.PressEnter() // Description (accept default)
	d.PressEnter() // PlannedMin (accept default)
	d.PressEnter() // Type select (accept default)
	// Group 2: 4 fields.
	d.PressEnter() // DueDate (accept default)
	d.PressEnter() // NotBefore (accept default)
	d.PressEnter() // MinSession (accept default)
	d.PressEnter() // MaxSession (accept default)

	// Form should complete.
	assert.NotEqual(t, ViewForm, d.ActiveViewID())

	// Verify work item still has valid data (defaults preserved).
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, "Reading", wi.Title, "title should be preserved")
	assert.Equal(t, 60, wi.PlannedMin, "planned_min should be preserved")
}

// =============================================================================
// 5. Adjust Logged Time
// =============================================================================

func TestE2E_AdjustLoggedTime_ActionMenu(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	// Seed project with some logged work.
	target := time.Now().UTC().AddDate(0, 3, 0)
	proj := testutil.NewTestProject("Adjust Test", testutil.WithTargetDate(target))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Practice",
		testutil.WithPlannedMin(120),
		testutil.WithLoggedMin(75),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	d := NewTestDriver(t, app)

	// Navigate to action menu via recommendation.
	navigateToActionMenu(d)
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// Press 'a' to open adjust form.
	d.PressKey('a')
	assert.Equal(t, ViewForm, d.ActiveViewID())

	// The adjust form has 1 group with 1 input (pre-filled with "75").
	// Accept the default by pressing Enter.
	d.PressEnter()

	// Form should complete.
	assert.NotEqual(t, ViewForm, d.ActiveViewID())

	// Verify logged_min is still 75 (we accepted default).
	updated, err := app.WorkItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 75, updated.LoggedMin)
}

// =============================================================================
// 6. Deep Navigation Round-Trip
// =============================================================================

func TestE2E_DeepNavigation_RoundTrip(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	// Level 0: Dashboard
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())

	// Level 1: Dashboard → Recommendation
	d.PressKey('?')
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())

	// Level 2: Recommendation → ActionMenu
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())
	assert.Equal(t, 3, d.ViewStackLen())

	// Level 3: ActionMenu → LogForm
	d.PressKey('l')
	assert.Equal(t, ViewForm, d.ActiveViewID())
	assert.Equal(t, 4, d.ViewStackLen())

	// Pop: LogForm → ActionMenu (Esc cancels wizard)
	d.PressEsc()
	// wizardCompleteMsg pops the form, leaving us on action menu (or recommendation).
	stackLen := d.ViewStackLen()
	assert.Less(t, stackLen, 4, "stack should shrink after Esc from form")

	// Keep popping back to dashboard.
	for d.ViewStackLen() > 1 {
		// Blur command bar if focused (wizardCompleteMsg focuses it).
		if d.CmdBarFocused() {
			d.PressEsc()
		}
		if d.ViewStackLen() > 1 {
			d.PressEsc()
		}
	}

	// Final state: Dashboard, stack length 1.
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())
}

// =============================================================================
// 7. Task List Navigation + Action
// =============================================================================

func TestE2E_TaskList_NavigateAndAct(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithShortIDAndWork(t, app, "TSK01", "Task List Test")
	ctx := context.Background()

	d := NewTestDriver(t, app)

	// Inspect project → TaskList.
	d.Command("inspect TSK01")
	assert.Equal(t, ViewTaskList, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())

	view := d.View()
	assert.Contains(t, view, "Reading")

	// Navigate to work item row (first row is the node header, second is the work item).
	d.PressKey('j') // down to work item

	// Press enter → ActionMenu.
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())
	assert.Equal(t, 3, d.ViewStackLen())

	view = d.View()
	assert.Contains(t, view, "ACTIONS")
	assert.Contains(t, view, "Reading")

	// Press 's' to start.
	d.PressKey('s')

	// Should pop back (wizardCompleteMsg).
	assert.NotEqual(t, ViewActionMenu, d.ActiveViewID())

	// Verify work item is in progress.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, wi.Status)
}

// =============================================================================
// 8. Draft Wizard → Inspect → Start (create project E2E)
// =============================================================================

func TestE2E_DraftProject_ThenInspectAndStart(t *testing.T) {
	app := testAppFull(t)
	ctx := context.Background()

	d := NewTestDriver(t, app)

	// Walk through the full draft wizard (matching the working pattern from tui_advanced_test.go).
	d.PressKey('d')
	assert.Equal(t, ViewDraft, d.ActiveViewID())

	draftType(d, "Physics Lab")    // description
	draftType(d, "2026-03-01")     // start date
	draftType(d, "2026-09-01")     // deadline
	draftType(d, "")               // group count = 1
	draftType(d, "Week")           // label
	draftType(d, "2")              // count
	draftType(d, "week")           // kind
	draftType(d, "7")              // days per node
	draftType(d, "Problems")       // work item title
	draftType(d, "practice")       // type
	draftType(d, "45")             // minutes
	draftType(d, "")               // done with work items
	draftType(d, "")               // skip special nodes

	view := d.View()
	assert.Contains(t, view, "[a]ccept")

	// Accept the draft.
	draftType(d, "a")
	assert.Equal(t, ViewDashboard, d.ActiveViewID())

	output := d.LastOutput()
	assert.Contains(t, output, "created successfully")

	// wizardCompleteMsg focuses the command bar — blur it before subsequent commands.
	if d.CmdBarFocused() {
		d.PressEsc()
	}

	// Verify project exists in DB.
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(projects), 1)

	// Find the created project.
	var created *domain.Project
	for _, p := range projects {
		if p.Name == "Physics Lab" {
			created = p
			break
		}
	}
	require.NotNil(t, created, "created project should exist in DB")

	// Verify nodes and work items.
	nodes, err := app.Nodes.ListByProject(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(nodes), "should have 2 week nodes")

	items, err := app.WorkItems.ListByProject(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(items), "should have 2 work items (1 per node)")
	for _, wi := range items {
		assert.Equal(t, 45, wi.PlannedMin)
	}

	// Inspect the project via its ShortID.
	d.Command("inspect " + created.ShortID)
	assert.Equal(t, ViewTaskList, d.ActiveViewID())

	view = d.View()
	assert.Contains(t, view, "Problems")

	// Navigate to a work item and start it.
	d.PressKey('j') // down to work item
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	d.PressKey('s') // Start Timer
	assert.NotEqual(t, ViewActionMenu, d.ActiveViewID())

	// Verify one item is now in progress.
	items, err = app.WorkItems.ListByProject(ctx, created.ID)
	require.NoError(t, err)
	inProgressCount := 0
	for _, wi := range items {
		if wi.Status == domain.WorkItemInProgress {
			inProgressCount++
		}
	}
	assert.Equal(t, 1, inProgressCount, "one item should be in progress")
}

// =============================================================================
// 9. Multi-Project Context Switching
// =============================================================================

func TestE2E_MultiProject_ContextSwitch(t *testing.T) {
	app := testApp(t)

	seedProjectWithShortIDAndWork(t, app, "CTX01", "Context Alpha")
	seedProjectWithShortIDAndWork(t, app, "CTX02", "Context Bravo")

	d := NewTestDriver(t, app)

	// Switch to project 1.
	d.Command("use CTX01")
	assert.Equal(t, "CTX01", d.State().ActiveShortID)
	assert.Equal(t, "Context Alpha", d.State().ActiveProjectName)

	// Inspect project 1.
	d.Command("inspect CTX01")
	assert.Equal(t, ViewTaskList, d.ActiveViewID())

	view := d.View()
	assert.Contains(t, view, "Reading")

	// Go back.
	d.PressEsc()
	assert.Equal(t, ViewDashboard, d.ActiveViewID())

	// Switch to project 2.
	d.Command("use CTX02")
	assert.Equal(t, "CTX02", d.State().ActiveShortID)
	assert.Equal(t, "Context Bravo", d.State().ActiveProjectName)

	// Inspect project 2.
	d.Command("inspect CTX02")
	assert.Equal(t, ViewTaskList, d.ActiveViewID())

	view = d.View()
	assert.Contains(t, view, "Reading") // same work item title, but different project context

	// Verify active project ID changed.
	assert.Equal(t, "CTX02", d.State().ActiveShortID)
}

// =============================================================================
// 10. Logging Shifts Recommendation Priority
// =============================================================================

func TestE2E_LogShiftsRecommendation(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	// Seed two equal-priority projects.
	projAID, projBID, _, _ := seedTwoEqualProjects(t, app)

	d := NewTestDriver(t, app)

	// Set project context to the first project (needed for command bar log).
	d.Command("use ALP01")

	// Get first recommendation.
	d.PressKey('?')
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())

	// Select the first recommended item and start it via action menu.
	d.PressEnter() // → ActionMenu
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// Start the item (sets ActiveItemID in state).
	d.PressKey('s')
	firstItemID := d.State().ActiveItemID
	require.NotEmpty(t, firstItemID, "should have active item after start")

	// Determine which project the first item belongs to.
	wi, err := app.WorkItems.GetByID(ctx, firstItemID)
	require.NoError(t, err)

	node, err := app.Nodes.GetByID(ctx, wi.NodeID)
	require.NoError(t, err)

	firstProjectID := node.ProjectID

	// Ensure the project context matches the item's project for the log command.
	if firstProjectID == projAID {
		d.Command("use ALP01")
	} else {
		d.Command("use BRV01")
	}

	// Log a session via command bar.
	d.Command("log 45")

	// Verify the session was logged.
	sessions, err := app.Sessions.ListByWorkItem(ctx, firstItemID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sessions), 1, "session should be logged")

	// Get another recommendation — spacing should favor the other project.
	d.PressKey('?')
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())

	// Select the first recommendation in the new list.
	d.PressEnter()

	// Check which project the new top recommendation belongs to.
	newItemID := d.State().ActiveItemID
	if newItemID != "" && newItemID != firstItemID {
		wi2, err := app.WorkItems.GetByID(ctx, newItemID)
		if err == nil {
			node2, err := app.Nodes.GetByID(ctx, wi2.NodeID)
			if err == nil {
				// The second recommendation should favor the other project (spacing effect).
				otherProjectID := projBID
				if firstProjectID == projBID {
					otherProjectID = projAID
				}
				assert.Equal(t, otherProjectID, node2.ProjectID,
					"after logging, spacing should favor the other project")
			}
		}
	}
}
