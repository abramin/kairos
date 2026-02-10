package cli

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitShellArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "single word",
			input: "status",
			want:  []string{"status"},
		},
		{
			name:  "double quoted phrase",
			input: `ask "can you update project OU10 to make weeks 1 to 17 as done"`,
			want:  []string{"ask", "can you update project OU10 to make weeks 1 to 17 as done"},
		},
		{
			name:  "single quoted phrase",
			input: "ask 'what should I work on?'",
			want:  []string{"ask", "what should I work on?"},
		},
		{
			name:  "flags with quoted value",
			input: `work update w1 --title "Deep work block"`,
			want:  []string{"work", "update", "w1", "--title", "Deep work block"},
		},
		{
			name:  "empty quoted arg",
			input: `ask ""`,
			want:  []string{"ask", ""},
		},
		{
			name:    "unterminated quote",
			input:   `ask "oops`,
			wantErr: true,
		},
		{
			name:    "unterminated escape",
			input:   `ask hi\`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := splitShellArgs(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShellModel_DispatchesToCobra(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand(`project add --id SHL01 --name "Shell Dispatch" --domain education --start 2026-01-15`)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "SHL01", projects[0].ShortID)
	assert.Equal(t, "Shell Dispatch", projects[0].Name)
}

func TestShellModel_UseSetsAndClearsActiveProject(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Shell Focus", testutil.WithShortID("USE01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)

	m.executeCommand("use use01")
	assert.Equal(t, proj.ID, m.activeProjectID)
	assert.Equal(t, "USE01", m.activeShortID)
	assert.Equal(t, "Shell Focus", m.activeProjectName)

	m.executeCommand("use")
	assert.Equal(t, "", m.activeProjectID)
	assert.Equal(t, "", m.activeShortID)
	assert.Equal(t, "", m.activeProjectName)
}

func TestShellModel_HelpChatModeLifecycle(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand("help chat")
	assert.Equal(t, modeHelpChat, m.mode)
	assert.Nil(t, m.helpConv)

	m.handleHelpChatInput("/quit")
	assert.Equal(t, modePrompt, m.mode)
	assert.Nil(t, m.helpConv)
}

func TestShellModel_HelpChatModeLifecycle_ExitWithoutSlash(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand("help chat")
	assert.Equal(t, modeHelpChat, m.mode)

	m.handleHelpChatInput("exit")
	assert.Equal(t, modePrompt, m.mode)
	assert.Nil(t, m.helpConv)
}

func TestShellModel_HelpChatModeDoesNotDispatchToCobra(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand("help chat")
	// In help chat mode, input goes to handleHelpChatInput, not executeCommand.
	m.handleHelpChatInput(`project add --id HC01 --name "Should Not Create" --domain education --start 2026-01-15`)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	assert.Len(t, projects, 0)
}

func TestShellModel_HelpChatOneShotDoesNotEnterMode(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand("help chat how do i list projects")
	assert.Equal(t, modePrompt, m.mode)
}

func TestShellModel_ExitSetsQuitting(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	assert.False(t, m.quitting)
	m.executeCommand("exit")
	assert.True(t, m.quitting)
}

func TestShellModel_QuitSetsQuitting(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	assert.False(t, m.quitting)
	m.executeCommand("quit")
	assert.True(t, m.quitting)
}

func TestShellModel_DraftEntersMode(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand("draft")
	assert.Equal(t, modeDraft, m.mode)
	assert.NotNil(t, m.draft)
}

func TestShellModel_DraftPhaseAdvances(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand("draft")
	assert.Equal(t, draftPhaseDescription, m.draft.phase)

	// Provide description.
	m.handleDraftInput("Physics A Level")
	assert.Equal(t, draftPhaseStartDate, m.draft.phase)

	// Empty Enter → use today.
	m.handleDraftInput("")
	assert.Equal(t, draftPhaseDeadline, m.draft.phase)
}

func TestShellModel_DraftCancel(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand("draft")
	assert.Equal(t, modeDraft, m.mode)

	m.handleDraftInput("/quit")
	assert.Equal(t, modePrompt, m.mode)
	assert.Nil(t, m.draft)
}

func TestPrepareShellCobraArgs_PassthroughForAsk(t *testing.T) {
	t.Parallel()

	got := prepareShellCobraArgs([]string{"ask", "mark OU10 done"}, "")
	assert.Equal(t, []string{"ask", "mark OU10 done"}, got)
}

func TestPrepareShellCobraArgs_DoesNotChangeNonAsk(t *testing.T) {
	t.Parallel()

	got := prepareShellCobraArgs([]string{"project", "list"}, "")
	assert.Equal(t, []string{"project", "list"}, got)
}

// --- Confirmation tests ---

func TestShellConfirm_ProjectArchive_YesExecutes(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Doomed", testutil.WithShortID("DOOM01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)

	// First call sets up the confirmation for archive.
	m.executeCommand("project archive DOOM01")
	require.NotNil(t, m.pendingConfirm)
	assert.Equal(t, modeConfirm, m.mode)

	// Simulate confirming with "y" — in bubbletea, the updateConfirm handler
	// would process this. We test the Cobra execution directly.
	m.execCobraCapture(m.pendingConfirm.args)
	m.pendingConfirm = nil
	m.mode = modePrompt

	// Project should be archived.
	p, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.NotNil(t, p.ArchivedAt)
}

func TestShellConfirm_ProjectRemove_NoAborts(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Safe", testutil.WithShortID("SAFE01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)

	m.executeCommand("project remove SAFE01")
	require.NotNil(t, m.pendingConfirm)

	// Deny: clear pending state.
	m.pendingConfirm = nil
	m.mode = modePrompt

	// Project should still exist.
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, projects, 1)
}

func TestShellConfirm_ForceFlagSkipsConfirmation(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Quick", testutil.WithShortID("QUIK01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)

	// With --force, should execute immediately without pending confirmation.
	m.executeCommand("project remove QUIK01 --force")
	assert.Nil(t, m.pendingConfirm)

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, projects, 0)
}

func TestShellConfirm_PromptPrefixShowsConfirm(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)
	m.mode = modeConfirm
	m.pendingConfirm = &pendingConfirmation{
		description: "project remove FOO",
		args:        []string{"project", "remove", "FOO"},
	}
	prefix := m.promptPrefix()
	assert.Contains(t, prefix, "confirm")
}

func TestShellConfirm_NonDestructivePassesThrough(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	// "project list" is not destructive — should not trigger confirmation.
	m.executeCommand("project list")
	assert.Nil(t, m.pendingConfirm)
}

// --- Transient context tests ---

func TestShellTransientContext_InspectSetsLastProject(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Inspectable", testutil.WithShortID("INSP01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)
	m.executeCommand("inspect INSP01")
	assert.Equal(t, proj.ID, m.lastInspectedProjectID)
}

func TestShellTransientContext_WhatNowSetsLastItem(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)

	m := newShellModel(app)
	m.executeCommand("what-now 60")

	// There should be at least one recommendation → last item set.
	assert.Equal(t, wiID, m.lastRecommendedItemID)
	assert.NotEmpty(t, m.lastRecommendedItemTitle)
}

// --- Error recovery tests ---

func TestShellErrorRecovery_StatePreservedAfterFailure(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Stable", testutil.WithShortID("STBL01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)

	// Set active project.
	m.executeCommand("use STBL01")
	assert.Equal(t, proj.ID, m.activeProjectID)

	// Run a command that errors (inspect nonexistent ID).
	m.executeCommand("inspect NONEXISTENT")

	// Active project should still be set.
	assert.Equal(t, proj.ID, m.activeProjectID)
	assert.Equal(t, "STBL01", m.activeShortID)
}

func TestShellErrorRecovery_InvalidCommandDoesNotCorruptState(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	m.executeCommand("definitely-not-a-command foo bar")
	assert.Equal(t, modePrompt, m.mode)
	assert.Nil(t, m.pendingConfirm)
}

// --- Replan passthrough test ---

func TestShellReplan_PassesThrough(t *testing.T) {
	app := testApp(t)
	m := newShellModel(app)

	// Should not panic — replan with no data just returns success or empty.
	m.executeCommand("replan")
	assert.Nil(t, m.pendingConfirm) // replan is not destructive
}

// TestShellModel_MultiStepJourney exercises a full REPL session:
// create project → use project → status → what-now → clear → exit.
func TestShellModel_MultiStepJourney(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	m := newShellModel(app)

	// Step 1: Create a project via shell executor
	m.executeCommand(`project add --id SHL01 --name "Shell Journey" --domain education --start 2026-01-15`)
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "SHL01", projects[0].ShortID)

	// Step 2: Add a node
	m.executeCommand(`node add --project ` + projects[0].ID + ` --title "Week 1" --kind week`)
	allNodes, err := app.Nodes.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	require.Len(t, allNodes, 1)

	// Step 3: Add a work item
	m.executeCommand(`work add --node ` + allNodes[0].ID + ` --title "Read Chapter 1" --type reading --planned-min 60`)
	allItems, err := app.WorkItems.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	require.Len(t, allItems, 1)

	// Step 4: Use project context
	m.executeCommand("use shl01")
	assert.Equal(t, projects[0].ID, m.activeProjectID)
	assert.Equal(t, "SHL01", m.activeShortID)
	assert.Equal(t, "Shell Journey", m.activeProjectName)

	// Step 5: Status command (should not error with active project context)
	output, _ := m.executeCommand("status")
	assert.NotEmpty(t, output)

	// Step 6: What-now with minutes
	output, _ = m.executeCommand("what-now 60")
	assert.NotEmpty(t, output)

	// Step 7: Clear project context (use with no arg clears active project)
	m.executeCommand("use")
	assert.Equal(t, "", m.activeProjectID)
	assert.Equal(t, "", m.activeShortID)

	// Step 8: Exit
	assert.False(t, m.quitting)
	m.executeCommand("exit")
	assert.True(t, m.quitting)
}

// =============================================================================
// Shell Integration Tests — scoped what-now and destructive confirmation
// =============================================================================

func TestShellUseThenScopedWhatNow(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	// Create two projects with work items.
	proj1 := testutil.NewTestProject("Project Alpha", testutil.WithShortID("ALP01"))
	require.NoError(t, app.Projects.Create(ctx, proj1))

	node1 := testutil.NewTestNode(proj1.ID, "Week 1")
	require.NoError(t, app.Nodes.Create(ctx, node1))

	wi1 := testutil.NewTestWorkItem(node1.ID, "Alpha Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wi1))

	proj2 := testutil.NewTestProject("Project Beta", testutil.WithShortID("BET01"))
	require.NoError(t, app.Projects.Create(ctx, proj2))

	node2 := testutil.NewTestNode(proj2.ID, "Week 1")
	require.NoError(t, app.Nodes.Create(ctx, node2))

	wi2 := testutil.NewTestWorkItem(node2.ID, "Beta Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wi2))

	m := newShellModel(app)

	// Use project Alpha.
	m.executeCommand("use ALP01")
	assert.Equal(t, proj1.ID, m.activeProjectID)
	assert.Equal(t, "ALP01", m.activeShortID)

	// Run what-now — should scope to Alpha and set last recommended item.
	m.executeCommand("what-now 60")
	assert.Equal(t, wi1.ID, m.lastRecommendedItemID,
		"scoped what-now should recommend Alpha's work item")
}

func TestShellDestructiveConfirm_WorkArchive(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	m := newShellModel(app)

	// Attempt to archive a work item — should trigger confirmation.
	m.executeCommand("work archive " + wiID)
	require.NotNil(t, m.pendingConfirm, "work archive should require confirmation")
	assert.Contains(t, m.pendingConfirm.description, "work archive")

	// Confirm by executing the pending args.
	m.execCobraCapture(m.pendingConfirm.args)
	m.pendingConfirm = nil
	m.mode = modePrompt

	// Verify the work item was archived.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.NotNil(t, wi.ArchivedAt, "work item should be archived after confirmation")
}

func TestShellDestructiveConfirm_SessionRemove_Denied(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	// Log a session first.
	sess := testutil.NewTestSession(wiID, 30)
	require.NoError(t, app.Sessions.LogSession(ctx, sess))

	m := newShellModel(app)

	// Attempt to remove the session — should trigger confirmation.
	m.executeCommand("session remove " + sess.ID)
	require.NotNil(t, m.pendingConfirm, "session remove should require confirmation")

	// Deny: clear pending state.
	m.pendingConfirm = nil
	m.mode = modePrompt

	// Session should still exist.
	sessions, err := app.Sessions.ListByWorkItem(ctx, wiID)
	require.NoError(t, err)
	assert.Len(t, sessions, 1, "session should still exist after denial")
}

func TestShellDestructiveConfirm_ProjectRemove_ForceBypasses(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Force Remove", testutil.WithShortID("FRC01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)

	// Remove with --force — should bypass confirmation.
	m.executeCommand("project remove " + proj.ID + " --force")
	assert.Nil(t, m.pendingConfirm, "--force should bypass confirmation")

	// Project should be deleted.
	projects, err := app.Projects.List(ctx, true)
	require.NoError(t, err)
	assert.Empty(t, projects, "project should be deleted after --force removal")
}

// =============================================================================
// Context command tests
// =============================================================================

func TestShellContext_ShowAndClear(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Context Test", testutil.WithShortID("CTX01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	m := newShellModel(app)

	// Set active project.
	m.executeCommand("use CTX01")
	assert.Equal(t, proj.ID, m.activeProjectID)

	// Show context.
	output := m.execContext(nil)
	assert.Contains(t, output, "Context Test")

	// Clear context.
	output = m.execContext([]string{"clear"})
	assert.Equal(t, "", m.activeProjectID)
	assert.Equal(t, "", m.activeItemID)
	assert.Equal(t, 0, m.lastDuration)
}

// =============================================================================
// New shortcut command tests
// =============================================================================

func TestShellModel_StartAndFinish(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	projID, wiID := seedProjectWithWork(t, app)

	m := newShellModel(app)

	// Set project context.
	m.setActiveProject(ctx, projID)

	// Start the item directly (bypasses wizard since we pass the ID).
	cmd := m.startExecute(wiID)
	assert.NotNil(t, cmd)
	assert.Equal(t, wiID, m.activeItemID)

	// Verify in-progress status.
	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, wi.Status)

	// Finish the item.
	cmd = m.finishExecute(wiID)
	assert.NotNil(t, cmd)

	// Active item should be cleared.
	assert.Equal(t, "", m.activeItemID)

	// Verify done status.
	wi, err = app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, wi.Status)
}

func TestShellModel_WithFlagsSkipsWizard(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Direct Work", testutil.WithShortID("DWK01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	m := newShellModel(app)

	// "work add" WITH flags should NOT enter wizard mode.
	m.executeCommand("work add --node " + node.ID + " --title Test --type task")
	assert.Equal(t, modePrompt, m.mode)

	items, err := app.WorkItems.ListByNode(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
}
