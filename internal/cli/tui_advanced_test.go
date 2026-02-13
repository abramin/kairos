package cli

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/testutil"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// draftType types text into the active draft/help-chat view and presses Enter.
func draftType(d *TestDriver, input string) {
	d.T.Helper()
	if input != "" {
		d.Type(input)
	}
	d.PressEnter()
}

// seedProjectWithShortID creates a project+node+work-item with a specific ShortID.
func seedProjectWithShortID(t *testing.T, app *App, shortID string) string {
	t.Helper()
	projID, _, _ := seedProjectCore(t, app, seedOpts{shortID: shortID, name: shortID + " Project"})
	return projID
}

// walkToGroupPhase opens draft and advances through metadata to the group-count phase.
func walkToGroupPhase(d *TestDriver) {
	d.T.Helper()
	d.PressKey('d')                  // push draft
	draftType(d, "Test Project 101") // description
	draftType(d, "")                 // start date (default today)
	draftType(d, "2026-12-01")       // deadline
}

// walkToWorkItemPhase advances through metadata + 1 group to the work-item title phase.
func walkToWorkItemPhase(d *TestDriver) {
	d.T.Helper()
	walkToGroupPhase(d)
	draftType(d, "")        // group count = 1 (default)
	draftType(d, "Chapter") // label
	draftType(d, "3")       // count
	draftType(d, "module")  // kind
	draftType(d, "7")       // days per node
}

// walkToSpecialPhase advances through metadata + group + 1 work item to special-node title phase.
func walkToSpecialPhase(d *TestDriver) {
	d.T.Helper()
	walkToWorkItemPhase(d)
	draftType(d, "Reading") // work item title
	draftType(d, "reading") // type
	draftType(d, "60")      // minutes
	draftType(d, "")        // done with work items
}

// walkToReview advances through all phases to the wizard review prompt.
func walkToReview(d *TestDriver) {
	d.T.Helper()
	walkToSpecialPhase(d)
	draftType(d, "") // skip special nodes -> review
}

// =============================================================================
// A. Draft Wizard Phase Transitions
// =============================================================================

func TestTUI_DraftWizard_MetadataPhases(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('d')
	assert.Equal(t, ViewDraft, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())

	view := d.View()
	assert.Contains(t, view, "Describe your project")

	draftType(d, "Quantum Mechanics 101")
	view = d.View()
	assert.Contains(t, view, "Quantum Mechanics 101")
	assert.Contains(t, view, "When do you want to start")

	draftType(d, "") // default start date
	view = d.View()
	today := time.Now().Format("2006-01-02")
	assert.Contains(t, view, today)
	assert.Contains(t, view, "When is the deadline")

	draftType(d, "2026-08-01")
	view = d.View()
	assert.Contains(t, view, "2026-08-01")
	assert.Contains(t, view, "How many groups")
}

func TestTUI_DraftWizard_DescriptionRequired(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('d')
	assert.Equal(t, ViewDraft, d.ActiveViewID())

	// Empty description should show error and stay on same phase.
	draftType(d, "")
	view := d.View()
	assert.Contains(t, view, "description is required")
	assert.Equal(t, ViewDraft, d.ActiveViewID()) // still on draft

	// Valid description advances.
	draftType(d, "Valid Description")
	view = d.View()
	assert.Contains(t, view, "When do you want to start")
}

func TestTUI_DraftWizard_GroupAndWorkItemPhases(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	walkToGroupPhase(d)

	// Group count (default 1).
	draftType(d, "")
	view := d.View()
	assert.Contains(t, view, "label")

	// Label.
	draftType(d, "Chapter")
	view = d.View()
	assert.Contains(t, view, "How many")

	// Count.
	draftType(d, "3")
	view = d.View()
	assert.Contains(t, view, "kind")

	// Kind.
	draftType(d, "module")
	view = d.View()
	assert.Contains(t, view, "Days per node")

	// Days.
	draftType(d, "7")
	view = d.View()
	assert.Contains(t, view, "Chapter x3")
	assert.Contains(t, view, "module")
	assert.Contains(t, view, "Work Items")
	assert.Contains(t, view, "Title")

	// Work item: Reading.
	draftType(d, "Reading")
	view = d.View()
	assert.Contains(t, view, "Type")

	draftType(d, "reading")
	view = d.View()
	assert.Contains(t, view, "Estimated minutes")

	draftType(d, "60")
	view = d.View()
	assert.Contains(t, view, "Reading (reading, 60m)")
	assert.Contains(t, view, "Title")

	// Done with work items.
	draftType(d, "")
	view = d.View()
	assert.Contains(t, view, "Special Nodes")
}

func TestTUI_DraftWizard_SkipSpecialNodesShowsReview(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	walkToSpecialPhase(d)

	// Skip special nodes.
	draftType(d, "")
	view := d.View()
	assert.Contains(t, view, "[a]ccept")
	assert.Equal(t, ViewDraft, d.ActiveViewID())
}

func TestTUI_DraftWizard_ReviewCancelPopsView(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	walkToReview(d)
	view := d.View()
	assert.Contains(t, view, "[a]ccept")

	// Cancel.
	draftType(d, "c")
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())
}

func TestTUI_DraftWizard_ReviewAcceptImportsProject(t *testing.T) {
	app := testAppFull(t)
	d := NewTestDriver(t, app)

	// Walk through the full wizard.
	d.PressKey('d')
	draftType(d, "Physics Lab")     // description
	draftType(d, "2026-03-01")      // start date
	draftType(d, "2026-09-01")      // deadline
	draftType(d, "")                // group count = 1
	draftType(d, "Week")            // label
	draftType(d, "2")               // count
	draftType(d, "week")            // kind
	draftType(d, "7")               // days
	draftType(d, "Problems")        // work item title
	draftType(d, "practice")        // type
	draftType(d, "45")              // minutes
	draftType(d, "")                // done with work items
	draftType(d, "")                // skip special nodes

	view := d.View()
	assert.Contains(t, view, "[a]ccept")

	// Accept.
	draftType(d, "a")
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())

	output := d.LastOutput()
	assert.Contains(t, output, "created successfully")

	// Verify project exists in DB.
	ctx := context.Background()
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "Physics Lab", projects[0].Name)
	// generateShortID("Physics Lab") -> "PHYS01"
	assert.Equal(t, "PHYS01", projects[0].ShortID)

	// Verify nodes and work items were created.
	nodes, err := app.Nodes.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(nodes))

	items, err := app.WorkItems.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(items)) // 1 work item template x 2 nodes
	for _, wi := range items {
		assert.Equal(t, 45, wi.PlannedMin)
	}
}

func TestTUI_DraftWizard_SpecialNodePhases(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	walkToSpecialPhase(d)

	// Special node: Final Exam.
	draftType(d, "Final Exam")
	view := d.View()
	assert.Contains(t, view, "Kind")

	draftType(d, "assessment")
	view = d.View()
	assert.Contains(t, view, "Due date")

	draftType(d, "2026-08-25")
	view = d.View()
	assert.Contains(t, view, "Work item title")

	// Special node work item.
	draftType(d, "Exam Prep")
	view = d.View()
	assert.Contains(t, view, "Type")

	draftType(d, "review")
	view = d.View()
	assert.Contains(t, view, "Estimated minutes")

	draftType(d, "120")
	view = d.View()
	assert.Contains(t, view, "Work item title") // loops back for more WIs

	// Done with special WI -> finishes this special node.
	draftType(d, "")
	view = d.View()
	assert.Contains(t, view, "Final Exam")
	assert.Contains(t, view, "assessment")

	// Done with special nodes -> review.
	draftType(d, "")
	view = d.View()
	assert.Contains(t, view, "[a]ccept")
}

func TestTUI_DraftWizard_MultipleGroups(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	walkToGroupPhase(d)

	draftType(d, "2") // 2 groups
	view := d.View()
	assert.Contains(t, view, "Group 1")

	// Group 1.
	draftType(d, "Module")   // label
	draftType(d, "3")        // count
	draftType(d, "module")   // kind
	draftType(d, "7")        // days
	view = d.View()
	assert.Contains(t, view, "Module x3")
	assert.Contains(t, view, "Group 2")

	// Group 2.
	draftType(d, "Assessment")  // label
	draftType(d, "1")           // count
	draftType(d, "assessment")  // kind
	draftType(d, "")            // days (default)
	view = d.View()
	assert.Contains(t, view, "Assessment x1")
	assert.Contains(t, view, "Work Items") // advanced to work item phase
}

func TestTUI_DraftWizard_WorkItemDefaultMinutes(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	walkToWorkItemPhase(d)

	draftType(d, "Quick Task") // work item title
	draftType(d, "task")       // type
	draftType(d, "")           // empty minutes -> should default to 30

	view := d.View()
	assert.Contains(t, view, "Quick Task (task, 30m)")
}

func TestTUI_DraftWizard_EscCancelsAtAnyPhase(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('d')
	assert.Equal(t, ViewDraft, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())

	// Type something partial but don't submit.
	d.Type("partial input")

	// Esc cancels the draft.
	d.PressEsc()
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())
}

func TestTUI_DraftWizard_SlashQuitCancels(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('d')
	draftType(d, "Valid Description") // advance past description phase

	// /quit cancels the draft.
	draftType(d, "/quit")
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())
}

// =============================================================================
// B. View Stack Push/Pop Correctness
// =============================================================================

func TestTUI_ViewStack_MultiplePushes(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)
	d := NewTestDriver(t, app)

	assert.Equal(t, []ViewID{ViewDashboard}, d.ViewStackIDs())

	// Push project list.
	d.PressKey('p')
	assert.Equal(t, []ViewID{ViewDashboard, ViewProjectList}, d.ViewStackIDs())

	// Push draft from project list via command bar.
	d.Command("draft")
	assert.Equal(t, []ViewID{ViewDashboard, ViewProjectList, ViewDraft}, d.ViewStackIDs())
	assert.Equal(t, 3, d.ViewStackLen())

	// Pop draft (Esc cancels draft via wizardCompleteMsg).
	d.PressEsc()
	assert.Equal(t, []ViewID{ViewDashboard, ViewProjectList}, d.ViewStackIDs())

	// wizardCompleteMsg focused the command bar, so first Esc blurs it.
	if d.CmdBarFocused() {
		d.PressEsc()
	}

	// Pop project list.
	d.PressEsc()
	assert.Equal(t, []ViewID{ViewDashboard}, d.ViewStackIDs())
}

func TestTUI_ViewStack_ReplaceDoesNotGrowStack(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('p')
	assert.Equal(t, 2, d.ViewStackLen())
	assert.Equal(t, ViewProjectList, d.ActiveViewID())

	// Replace top view with a draft view.
	d.Send(replaceViewMsg{view: newDraftView(d.State(), "")})
	assert.Equal(t, 2, d.ViewStackLen()) // did NOT grow
	assert.Equal(t, ViewDraft, d.ActiveViewID())
	assert.Equal(t, []ViewID{ViewDashboard, ViewDraft}, d.ViewStackIDs())
}

func TestTUI_ViewStack_PopFromSingleViewIsNoop(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	assert.Equal(t, 1, d.ViewStackLen())
	assert.Equal(t, ViewDashboard, d.ActiveViewID())

	// Esc on single-view stack is a no-op.
	d.PressEsc()
	assert.Equal(t, 1, d.ViewStackLen())
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
}

func TestTUI_ViewStack_WizardCompletePopsAndFocusesBar(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('d')
	assert.Equal(t, ViewDraft, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())
	assert.False(t, d.CmdBarFocused())

	// Esc in draft triggers wizardCompleteMsg.
	d.PressEsc()
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())
	assert.True(t, d.CmdBarFocused()) // wizardCompleteMsg focuses bar
	assert.Contains(t, d.LastOutput(), "cancelled")
}

// =============================================================================
// C. Focus Routing
// =============================================================================

func TestTUI_FocusRouting_CmdBarEatsAllKeys(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey(':')
	assert.True(t, d.CmdBarFocused())

	// All character keys go to text input, not global handlers.
	d.PressKey('q')
	assert.False(t, d.IsQuitting(), "q should go to text input, not quit")
	assert.True(t, d.CmdBarFocused())

	d.PressKey(':')
	assert.False(t, d.IsQuitting())
	assert.True(t, d.CmdBarFocused())

	d.PressKey('d')
	assert.False(t, d.IsQuitting())
	assert.True(t, d.CmdBarFocused())

	// Blur and now 'q' should quit.
	d.PressEsc()
	assert.False(t, d.CmdBarFocused())
	d.PressKey('q')
	assert.True(t, d.IsQuitting())
}

func TestTUI_FocusRouting_DraftCapturesQKey(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('d')
	assert.Equal(t, ViewDraft, d.ActiveViewID())

	// 'q' goes to draft textinput, not quit.
	d.PressKey('q')
	assert.False(t, d.IsQuitting())

	// ':' goes to draft textinput, not command bar.
	d.PressKey(':')
	assert.False(t, d.CmdBarFocused())
	assert.Equal(t, ViewDraft, d.ActiveViewID())
}

func TestTUI_FocusRouting_HelpChatCapturesColonKey(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('h')
	assert.Equal(t, ViewHelpChat, d.ActiveViewID())

	// ':' goes to help chat input, not command bar.
	d.PressKey(':')
	assert.False(t, d.CmdBarFocused())

	// 'q' goes to help chat input, not quit.
	d.PressKey('q')
	assert.False(t, d.IsQuitting())
}

func TestTUI_FocusRouting_ProjectListNonCapturing(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)
	d := NewTestDriver(t, app)

	d.PressKey('p')
	assert.Equal(t, ViewProjectList, d.ActiveViewID())

	// ':' focuses command bar (global handler for non-capturing views).
	d.PressKey(':')
	assert.True(t, d.CmdBarFocused())

	// Blur.
	d.PressEsc()
	assert.False(t, d.CmdBarFocused())

	// 'q' quits (global handler for non-capturing views).
	d.PressKey('q')
	assert.True(t, d.IsQuitting())
}

func TestTUI_FocusRouting_EscBehaviorByContext(t *testing.T) {
	t.Run("esc blurs command bar", func(t *testing.T) {
		app := testApp(t)
		d := NewTestDriver(t, app)

		d.PressKey(':')
		assert.True(t, d.CmdBarFocused())

		d.PressEsc()
		assert.False(t, d.CmdBarFocused())
		assert.Equal(t, 1, d.ViewStackLen()) // didn't pop
	})

	t.Run("esc pops non-capturing view", func(t *testing.T) {
		app := testApp(t)
		d := NewTestDriver(t, app)

		d.PressKey('p')
		assert.Equal(t, 2, d.ViewStackLen())

		d.PressEsc()
		assert.Equal(t, 1, d.ViewStackLen())
		assert.Equal(t, ViewDashboard, d.ActiveViewID())
	})

	t.Run("esc cancels capturing view via wizardComplete", func(t *testing.T) {
		app := testApp(t)
		d := NewTestDriver(t, app)

		d.PressKey('d')
		assert.Equal(t, ViewDraft, d.ActiveViewID())
		assert.Equal(t, 2, d.ViewStackLen())

		d.PressEsc()
		assert.Equal(t, 1, d.ViewStackLen())
		assert.Equal(t, ViewDashboard, d.ActiveViewID())
		assert.True(t, d.CmdBarFocused()) // wizardCompleteMsg focuses bar
	})
}

// =============================================================================
// D. Async Data Loading
// =============================================================================

func TestTUI_AsyncLoad_RecommendationLoads(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)
	d := NewTestDriver(t, app)

	d.PressKey('?')
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())

	view := d.View()
	// Data should have loaded synchronously (in-memory DB completes within 10ms timeout).
	assert.NotContains(t, view, "Computing recommendations")

	// Should show mode badge and recommendation data.
	hasBadge := assert.Condition(t, func() bool {
		return containsAny(view, "BALANCED", "CRITICAL")
	})
	if !hasBadge {
		t.Logf("View output: %s", view)
	}
}

func TestTUI_AsyncLoad_KeyDuringRecommendation(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)
	d := NewTestDriver(t, app)

	d.PressKey('?')
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())

	// Navigation keys should not crash.
	d.PressKey('j') // down
	d.PressKey('k') // up
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())

	// Refresh should reload data.
	d.PressKey('r')
	view := d.View()
	assert.NotContains(t, view, "Computing recommendations")
	assert.Equal(t, ViewRecommendation, d.ActiveViewID())
}

func TestTUI_AsyncLoad_DashboardRefresh(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)
	d := NewTestDriver(t, app)

	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	view := d.View()
	assert.NotContains(t, view, "Loading...")
	assert.Contains(t, view, "CLI Test Project")

	// 'r' refreshes the dashboard.
	d.PressKey('r')
	view = d.View()
	assert.NotContains(t, view, "Loading...")
	assert.Contains(t, view, "CLI Test Project")
}

// =============================================================================
// E. Context Pollution
// =============================================================================

func TestTUI_Context_PreservedAfterDraftCancel(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Preserved Context", testutil.WithShortID("PRJ01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	d := NewTestDriver(t, app)
	d.Command("use PRJ01")
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)
	assert.Equal(t, "PRJ01", d.State().ActiveShortID)

	// Open draft and type partial input.
	d.PressKey('d')
	assert.Equal(t, ViewDraft, d.ActiveViewID())
	d.Type("partial description")

	// Cancel.
	d.PressEsc()
	assert.Equal(t, ViewDashboard, d.ActiveViewID())

	// Context should be unchanged.
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)
	assert.Equal(t, "PRJ01", d.State().ActiveShortID)
	assert.Equal(t, "Preserved Context", d.State().ActiveProjectName)
}

func TestTUI_Context_PreservedAfterSuccessfulDraft(t *testing.T) {
	app := testAppFull(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Pre-existing", testutil.WithShortID("AAA01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	d := NewTestDriver(t, app)
	d.Command("use AAA01")
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)

	// Walk through the entire draft wizard and accept.
	d.PressKey('d')
	draftType(d, "New Draft Project") // description
	draftType(d, "")                  // start date
	draftType(d, "2026-12-01")        // deadline
	draftType(d, "")                  // group count = 1
	draftType(d, "Module")            // label
	draftType(d, "1")                 // count
	draftType(d, "")                  // kind (default module)
	draftType(d, "")                  // days (default)
	draftType(d, "Task")              // work item title
	draftType(d, "task")              // type
	draftType(d, "30")                // minutes
	draftType(d, "")                  // done with work items
	draftType(d, "")                  // skip special nodes
	draftType(d, "a")                 // accept

	assert.Equal(t, ViewDashboard, d.ActiveViewID())

	// Verify new project was created.
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(projects)) // original + draft

	// Context should still point to the original project.
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)
	assert.Equal(t, "AAA01", d.State().ActiveShortID)
	assert.Equal(t, "Pre-existing", d.State().ActiveProjectName)
}

func TestTUI_Context_PreservedAfterHelpChat(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Help Context", testutil.WithShortID("CTX01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	d := NewTestDriver(t, app)
	d.Command("use CTX01")
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)

	// Open help chat and ask a question.
	d.PressKey('h')
	assert.Equal(t, ViewHelpChat, d.ActiveViewID())

	draftType(d, "how do I log a session")
	view := d.View()
	assert.Contains(t, view, "how do I log a session") // transcript shows question

	// Exit help chat.
	d.PressEsc()
	assert.Equal(t, ViewDashboard, d.ActiveViewID())

	// Context should be unchanged.
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)
	assert.Equal(t, "CTX01", d.State().ActiveShortID)
	assert.Equal(t, "Help Context", d.State().ActiveProjectName)
}

func TestTUI_Context_ClearUseNoArgs(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Clearable", testutil.WithShortID("CLR01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	d := NewTestDriver(t, app)
	d.Command("use CLR01")
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)
	assert.Equal(t, "CLR01", d.State().ActiveShortID)
	assert.Equal(t, "Clearable", d.State().ActiveProjectName)

	// Clear context.
	d.Command("use")
	assert.Equal(t, "", d.State().ActiveProjectID)
	assert.Equal(t, "", d.State().ActiveShortID)
	assert.Equal(t, "", d.State().ActiveProjectName)
}

// ── internal helpers ─────────────────────────────────────────────────────────

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// Ensure the test uses the tea import.
var _ = tea.KeyMsg{}
