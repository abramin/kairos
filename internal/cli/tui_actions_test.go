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

// =============================================================================
// A. Action Menu — view_action_menu.go
// =============================================================================

func TestTUI_ActionMenu_RecommendationEnterPushesMenu(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	// Open recommendation view.
	d.PressKey('?')
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())

	// Press enter on first recommendation → action menu.
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())
	assert.Equal(t, 3, d.ViewStackLen())

	view := d.View()
	assert.Contains(t, view, "ACTIONS")
	assert.Contains(t, view, "Start Timer")
	assert.Contains(t, view, "Log Past Session")
	assert.Contains(t, view, "Mark Done")
	assert.Contains(t, view, "Edit Details")
	assert.Contains(t, view, "Adjust Logged Time")
}

func TestTUI_ActionMenu_EscPops(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.PressKey('?')
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	d.PressEsc()
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())
}

func TestTUI_ActionMenu_CursorNavigation(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.PressKey('?')
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// Cursor starts at 0 (Start Timer). Move down.
	view := d.View()
	assert.Contains(t, view, "▸") // cursor indicator

	d.PressKey('j') // down
	d.PressKey('j') // down again
	d.PressKey('k') // up

	// Should not crash and should stay on action menu.
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())
}

func TestTUI_ActionMenu_ShortcutKeys(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.PressKey('?')
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// 'd' shortcut for Mark Done should execute and pop back.
	d.PressKey('d')

	// Should have triggered wizardCompleteMsg → back to recommendation.
	assert.NotEqual(t, ViewActionMenu, d.ActiveViewID())

	// Verify work item was marked done.
	wi, err := app.WorkItems.GetByID(context.Background(), wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, wi.Status)
}

func TestTUI_ActionMenu_StartSetsInProgress(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.PressKey('?')
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// 's' shortcut for Start Timer.
	d.PressKey('s')

	assert.NotEqual(t, ViewActionMenu, d.ActiveViewID())

	// Verify work item is in progress.
	wi, err := app.WorkItems.GetByID(context.Background(), wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, wi.Status)
}

func TestTUI_ActionMenu_LogPushesLogForm(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.PressKey('?')
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// 'l' shortcut for Log Past Session → replaces with log form (wizard).
	d.PressKey('l')

	// Should transition to a form view (wizard).
	assert.Equal(t, ViewForm, d.ActiveViewID())
}

func TestTUI_ActionMenu_EditOpensForm(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.PressKey('?')
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// 'e' shortcut for Edit Details → opens edit form (wizard).
	d.PressKey('e')

	assert.Equal(t, ViewForm, d.ActiveViewID())
}

func TestTUI_ActionMenu_EditEscReturnsToMenu(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.PressKey('?')
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	// 'e' → edit form (pushed on top of action menu).
	d.PressKey('e')
	assert.Equal(t, ViewForm, d.ActiveViewID())

	// ESC cancels the edit form → should return to action menu, not skip past it.
	d.PressEsc()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())
}

// =============================================================================
// B. Task List → Action Menu
// =============================================================================

func TestTUI_TaskList_EnterPushesActionMenu(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Task List Action", testutil.WithShortID("TLA01"),
		testutil.WithTargetDate(time.Now().UTC().AddDate(0, 3, 0)))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task List Item",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	d := NewTestDriver(t, app)

	// Inspect project → task list.
	d.Command("inspect TLA01")
	assert.Equal(t, ViewTaskList, d.ActiveViewID())

	// Navigate to the work item row (skip node row).
	d.PressKey('j') // cursor down to work item

	// Press enter → action menu.
	d.PressEnter()
	assert.Equal(t, ViewActionMenu, d.ActiveViewID())

	view := d.View()
	assert.Contains(t, view, "ACTIONS")
	assert.Contains(t, view, "Task List Item")
}

// =============================================================================
// C. Work Actions — work_actions.go
// =============================================================================

func TestExecLogSession_CreatesSession(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	_, wiID := seedProjectWithWork(t, app)

	state := &SharedState{App: app}
	msg, err := execLogSession(ctx, app, state, LogSessionInput{
		ItemID: wiID, Title: "Reading", Minutes: 45,
	})

	require.NoError(t, err)
	assert.Contains(t, msg, "45m")
	assert.Contains(t, msg, "Reading")
	assert.Equal(t, wiID, state.ActiveItemID)
	assert.Equal(t, 45, state.LastDuration)

	// Verify session was persisted.
	sessions, err := app.Sessions.ListByWorkItem(ctx, wiID)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, 45, sessions[0].Minutes)
}

func TestExecLogSession_WithUnits(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	target := time.Now().UTC().AddDate(0, 3, 0)
	proj := testutil.NewTestProject("Units Test", testutil.WithTargetDate(target))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Read Chapters",
		testutil.WithPlannedMin(120),
		testutil.WithUnits("chapters", 10, 0),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	state := &SharedState{App: app}
	msg, err := execLogSession(ctx, app, state, LogSessionInput{
		ItemID: wi.ID, Title: "Read Chapters", Minutes: 30, UnitsDelta: 3, Note: "good progress",
	})

	require.NoError(t, err)
	assert.Contains(t, msg, "+3 units")
}

func TestExecStartItem_SetsInProgress(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	_, wiID := seedProjectWithWork(t, app)

	state := &SharedState{App: app}
	msg, err := execStartItem(ctx, app, state, wiID, "Reading", 1)

	require.NoError(t, err)
	assert.Contains(t, msg, "Started")
	assert.Contains(t, msg, "Reading")
	assert.Equal(t, wiID, state.ActiveItemID)

	// Verify DB state.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, wi.Status)
}

func TestExecMarkDone_SetsCompleted(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	_, wiID := seedProjectWithWork(t, app)

	state := &SharedState{App: app, ActiveItemID: wiID}
	msg, err := execMarkDone(ctx, app, state, wiID, "Reading")

	require.NoError(t, err)
	assert.Contains(t, msg, "Done")
	assert.Contains(t, msg, "Reading")
	assert.Empty(t, state.ActiveItemID, "should clear item context when marking active item done")

	// Verify DB state.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, wi.Status)
}

func TestExecMarkDone_DoesNotClearOtherItem(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	_, wiID := seedProjectWithWork(t, app)

	// State points to a different item.
	state := &SharedState{App: app, ActiveItemID: "other-item-id"}
	_, err := execMarkDone(ctx, app, state, wiID, "Reading")

	require.NoError(t, err)
	assert.Equal(t, "other-item-id", state.ActiveItemID,
		"should not clear context for a different item")
}

// =============================================================================
// D. Action Menu Construction — view_action_menu.go + view_log_form.go
// =============================================================================

func TestActionMenu_AdjustLoggedTime(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Adjust Test", testutil.WithShortID("ADJ01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Reading", testutil.WithPlannedMin(300))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	// Set logged time to 270 to simulate a logging error.
	wi.LoggedMin = 270
	require.NoError(t, app.WorkItems.Update(ctx, wi))

	t.Run("action menu includes adjust logged time", func(t *testing.T) {
		state := &SharedState{App: app}
		menu := newActionMenuView(state, wi.ID, wi.Title, wi.Seq)
		require.Len(t, menu.actions, 6)
		assert.Equal(t, "Adjust Logged Time", menu.actions[2].label)
		assert.Equal(t, "a", menu.actions[2].key)
	})

	t.Run("adjust updates logged min on work item", func(t *testing.T) {
		state := &SharedState{App: app}
		view := newAdjustLoggedView(state, wi.ID, wi.Title)
		require.NotNil(t, view)

		// Directly update the work item to simulate form submission.
		updated, err := app.WorkItems.GetByID(ctx, wi.ID)
		require.NoError(t, err)
		assert.Equal(t, 270, updated.LoggedMin)

		updated.LoggedMin = 150
		require.NoError(t, app.WorkItems.Update(ctx, updated))

		final, err := app.WorkItems.GetByID(ctx, wi.ID)
		require.NoError(t, err)
		assert.Equal(t, 150, final.LoggedMin)
	})
}
