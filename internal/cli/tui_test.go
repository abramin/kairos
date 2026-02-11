package cli

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTUI_DashboardLoadsOnStartup(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())

	view := d.View()
	assert.NotEmpty(t, view)
	assert.NotContains(t, view, "Loading...")
}

func TestTUI_DashboardShowsProjects(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	view := d.View()
	assert.Contains(t, view, "CLI Test Project")
}

func TestTUI_QuitWithQ(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('q')

	assert.True(t, d.IsQuitting())
}

func TestTUI_QuitWithCtrlC(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressCtrlC()

	assert.True(t, d.IsQuitting())
}

func TestTUI_CommandBarFocusBlur(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	assert.False(t, d.CmdBarFocused())

	d.PressKey(':')
	assert.True(t, d.CmdBarFocused())

	d.PressEsc()
	assert.False(t, d.CmdBarFocused())
}

func TestTUI_NavigateDashboardToProjectsAndBack(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())

	d.PressKey('p')

	assert.Equal(t, ViewProjectList, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())
	assert.Equal(t, []ViewID{ViewDashboard, ViewProjectList}, d.ViewStackIDs())

	d.PressEsc()

	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())
}

func TestTUI_CommandUseSetsContext(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("TUI Context Test", testutil.WithShortID("TUI01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	d := NewTestDriver(t, app)

	d.Command("use TUI01")

	assert.Equal(t, proj.ID, d.State().ActiveProjectID)
	assert.Equal(t, "TUI01", d.State().ActiveShortID)
	assert.Equal(t, "TUI Context Test", d.State().ActiveProjectName)
	assert.NotEmpty(t, d.LastOutput())
}

func TestTUI_InspectPushesTaskList(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Inspect TUI", testutil.WithShortID("ITUI01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Read Chapter 1",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	d := NewTestDriver(t, app)

	d.Command("inspect ITUI01")

	assert.Equal(t, ViewTaskList, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)

	view := d.View()
	assert.Contains(t, view, "Read Chapter 1")
}

func TestTUI_DraftPushAndCancel(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('d')

	assert.Equal(t, ViewDraft, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())

	d.PressEsc()

	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())
}

func TestTUI_HelpChatPushAndExit(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.PressKey('h')

	assert.Equal(t, ViewHelpChat, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())

	d.PressEsc()

	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, 1, d.ViewStackLen())
}

func TestTUI_ContextPreservedAcrossNavigation(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Preserved", testutil.WithShortID("PRV01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Reading", testutil.WithPlannedMin(60))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	d := NewTestDriver(t, app)

	d.Command("use PRV01")
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)

	// Navigate to project list and back.
	d.PressKey('p')
	assert.Equal(t, ViewProjectList, d.ActiveViewID())
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)
	assert.Equal(t, "PRV01", d.State().ActiveShortID)

	d.PressEsc()
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
	assert.Equal(t, proj.ID, d.State().ActiveProjectID)
	assert.Equal(t, "PRV01", d.State().ActiveShortID)
}

func TestTUI_WindowResizePropagation(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	assert.Equal(t, 120, d.State().Width)
	assert.Equal(t, 40, d.State().Height)

	d.Send(tea.WindowSizeMsg{Width: 200, Height: 60})

	assert.Equal(t, 200, d.State().Width)
	assert.Equal(t, 60, d.State().Height)
}

func TestTUI_CommandBarExitQuits(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	d.Command("exit")

	assert.True(t, d.IsQuitting())
}

func TestTUI_RecommendationViewFromDashboard(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.PressKey('?')

	assert.Equal(t, ViewRecommendation, d.ActiveViewID())
	assert.Equal(t, 2, d.ViewStackLen())

	d.PressEsc()
	assert.Equal(t, ViewDashboard, d.ActiveViewID())
}

func TestTUI_DestructiveCommandPushesConfirmation(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Destructible", testutil.WithShortID("DEST01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	d := NewTestDriver(t, app)

	d.Command("project archive DEST01")

	assert.Equal(t, ViewForm, d.ActiveViewID())
	assert.GreaterOrEqual(t, d.ViewStackLen(), 2)

	// Project should NOT yet be archived.
	p, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Nil(t, p.ArchivedAt)

	// Cancel the confirmation.
	d.PressEsc()

	// Still not archived.
	p, err = app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Nil(t, p.ArchivedAt)
}

func TestTUI_StatusCommandProducesOutput(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	d := NewTestDriver(t, app)

	d.Command("status")

	assert.NotEmpty(t, d.LastOutput())
}

func TestTUI_QDoesNotQuitWhenCmdBarFocused(t *testing.T) {
	app := testApp(t)
	d := NewTestDriver(t, app)

	// Focus command bar.
	d.PressKey(':')
	assert.True(t, d.CmdBarFocused())

	// 'q' should go into the text input, not quit.
	d.PressKey('q')
	assert.False(t, d.IsQuitting())
	assert.True(t, d.CmdBarFocused())

	// Blur and then 'q' should quit.
	d.PressEsc()
	d.PressKey('q')
	assert.True(t, d.IsQuitting())
}
