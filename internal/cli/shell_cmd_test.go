package cli

import (
	"context"
	"testing"

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

func TestShellExecutor_DispatchesToCobra(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	sess.executor(`project add --id SHL01 --name "Shell Dispatch" --domain education --start 2026-01-15`)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "SHL01", projects[0].ShortID)
	assert.Equal(t, "Shell Dispatch", projects[0].Name)
}

func TestShellExecutor_UseSetsAndClearsActiveProject(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Shell Focus", testutil.WithShortID("USE01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	sess.executor("use use01")
	assert.Equal(t, proj.ID, sess.activeProjectID)
	assert.Equal(t, "USE01", sess.activeShortID)
	assert.Equal(t, "Shell Focus", sess.activeProjectName)

	sess.executor("use")
	assert.Equal(t, "", sess.activeProjectID)
	assert.Equal(t, "", sess.activeShortID)
	assert.Equal(t, "", sess.activeProjectName)
}

func TestShellExecutor_HelpChatModeLifecycle(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	sess.executor("help chat")
	assert.True(t, sess.helpChatMode)
	assert.Nil(t, sess.helpConv)

	sess.executor("/quit")
	assert.False(t, sess.helpChatMode)
	assert.Nil(t, sess.helpConv)
}

func TestShellExecutor_HelpChatModeLifecycle_ExitWithoutSlash(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	sess.executor("help chat")
	assert.True(t, sess.helpChatMode)

	sess.executor("exit")
	assert.False(t, sess.helpChatMode)
	assert.Nil(t, sess.helpConv)
}

func TestShellExecutor_HelpChatModeDoesNotDispatchToCobra(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	sess.executor("help chat")
	sess.executor(`project add --id HC01 --name "Should Not Create" --domain education --start 2026-01-15`)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	assert.Len(t, projects, 0)
}

func TestShellExecutor_HelpChatOneShotDoesNotEnterMode(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	sess.executor("help chat how do i list projects")
	assert.False(t, sess.helpChatMode)
}

func TestShellExecutor_ExitSetsWantExit(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	assert.False(t, sess.wantExit)
	sess.executor("exit")
	assert.True(t, sess.wantExit)
}

func TestShellExecutor_QuitSetsWantExit(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	assert.False(t, sess.wantExit)
	sess.executor("quit")
	assert.True(t, sess.wantExit)
}

func TestShellExecutor_EmptyInputInDraftModeAdvancesPhase(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:        app,
		cache:      newShellProjectCache(),
		draftMode:  true,
		draftPhase: draftPhaseStartDate,
	}

	// Empty Enter in start-date phase should advance to deadline phase.
	sess.executor("")
	assert.Equal(t, draftPhaseDeadline, sess.draftPhase)
}

func TestShellExecutor_EmptyInputInNormalModeIsNoop(t *testing.T) {
	app := testApp(t)
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	// Should not panic or change any state.
	sess.executor("")
	assert.False(t, sess.draftMode)
	assert.False(t, sess.helpChatMode)
}

func TestPrepareShellCobraArgs_AutoAddsYesForAsk(t *testing.T) {
	t.Parallel()

	got := prepareShellCobraArgs([]string{"ask", "mark OU10 done"})
	assert.Equal(t, []string{"ask", "mark OU10 done", "--yes"}, got)
}

func TestPrepareShellCobraArgs_DoesNotDuplicateYes(t *testing.T) {
	t.Parallel()

	got := prepareShellCobraArgs([]string{"ask", "mark OU10 done", "--yes"})
	assert.Equal(t, []string{"ask", "mark OU10 done", "--yes"}, got)
}

func TestPrepareShellCobraArgs_DoesNotChangeNonAsk(t *testing.T) {
	t.Parallel()

	got := prepareShellCobraArgs([]string{"project", "list"})
	assert.Equal(t, []string{"project", "list"}, got)
}

// TestShellSession_MultiStepJourney exercises a full REPL session:
// create project → use project → status → what-now → clear → exit.
func TestShellSession_MultiStepJourney(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	// Step 1: Create a project via shell executor
	sess.executor(`project add --id SHL01 --name "Shell Journey" --domain education --start 2026-01-15`)
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "SHL01", projects[0].ShortID)

	// Step 2: Add a node
	sess.executor(`node add --project ` + projects[0].ID + ` --title "Week 1" --kind week`)
	allNodes, err := app.Nodes.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	require.Len(t, allNodes, 1)

	// Step 3: Add a work item
	sess.executor(`work add --node ` + allNodes[0].ID + ` --title "Read Chapter 1" --type reading --planned-min 60`)
	allItems, err := app.WorkItems.ListByProject(ctx, projects[0].ID)
	require.NoError(t, err)
	require.Len(t, allItems, 1)

	// Step 4: Use project context
	sess.executor("use shl01")
	assert.Equal(t, projects[0].ID, sess.activeProjectID)
	assert.Equal(t, "SHL01", sess.activeShortID)
	assert.Equal(t, "Shell Journey", sess.activeProjectName)

	// Step 5: Status command (should not error with active project context)
	sess.executor("status")

	// Step 6: What-now with minutes
	sess.executor("what-now 60")

	// Step 7: Clear project context (use with no arg clears active project)
	sess.executor("use")
	assert.Equal(t, "", sess.activeProjectID)
	assert.Equal(t, "", sess.activeShortID)

	// Step 8: Exit
	assert.False(t, sess.wantExit)
	sess.executor("exit")
	assert.True(t, sess.wantExit)
}
