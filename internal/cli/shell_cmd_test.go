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
