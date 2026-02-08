package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testApp wires a full App backed by an in-memory DB for CLI integration tests.
func testApp(t *testing.T) *App {
	t.Helper()
	db := testutil.NewTestDB(t)

	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	depRepo := repository.NewSQLiteDependencyRepo(db)
	sessRepo := repository.NewSQLiteSessionRepo(db)
	profRepo := repository.NewSQLiteUserProfileRepo(db)

	return &App{
		Projects:  service.NewProjectService(projRepo),
		Nodes:     service.NewNodeService(nodeRepo),
		WorkItems: service.NewWorkItemService(wiRepo),
		Sessions:  service.NewSessionService(sessRepo, wiRepo),
		WhatNow:   service.NewWhatNowService(wiRepo, sessRepo, projRepo, depRepo, profRepo),
		Status:    service.NewStatusService(projRepo, wiRepo, sessRepo, profRepo),
		Replan:    service.NewReplanService(projRepo, wiRepo, sessRepo, profRepo),
		// Templates and Import left nil — not tested here.
		// Intelligence services left nil — LLM disabled.
	}
}

// seedProjectWithWork creates a project with a node and work item for CLI tests.
func seedProjectWithWork(t *testing.T, app *App) (string, string) {
	t.Helper()
	ctx := context.Background()

	target := time.Now().UTC().AddDate(0, 3, 0)
	proj := testutil.NewTestProject("CLI Test Project", testutil.WithTargetDate(target))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Reading",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	return proj.ID, wi.ID
}

// executeCmd runs a cobra command and captures stdout/stderr.
func executeCmd(t *testing.T, app *App, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd(app)
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// --- Root command shortcut ---

func TestRootCmd_MinutesShortcut(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "45")
	require.NoError(t, err)
}

func TestRootCmd_InvalidMinutes(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid minutes")
}

func TestRootCmd_NegativeMinutes(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "-5")
	// Cobra treats -5 as a flag; should error.
	assert.Error(t, err)
}

func TestRootCmd_NoArgs_ShowsHelp(t *testing.T) {
	app := testApp(t)

	output, err := executeCmd(t, app)
	require.NoError(t, err)
	assert.Contains(t, output, "kairos")
}

// --- what-now command ---

func TestWhatNowCmd_RequiresMinutesFlag(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "what-now")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minutes")
}

func TestWhatNowCmd_WithData(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "what-now", "--minutes", "60")
	require.NoError(t, err)
}

func TestWhatNowCmd_EmptyDB(t *testing.T) {
	app := testApp(t)

	// No projects — should handle gracefully (return error or empty output).
	_, err := executeCmd(t, app, "what-now", "--minutes", "60")
	// The WhatNow service returns ErrNoCandidates when the DB is empty.
	// This is propagated as an error through the CLI.
	assert.Error(t, err)
}

func TestWhatNowCmd_DryRun(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "what-now", "--minutes", "60", "--dry-run")
	require.NoError(t, err)
}

func TestWhatNowCmd_MaxSlices(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "what-now", "--minutes", "60", "--max-slices", "1")
	require.NoError(t, err)
}

// --- status command ---

func TestStatusCmd_EmptyDB(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "status")
	// Empty DB should succeed (0 projects to show).
	require.NoError(t, err)
}

func TestStatusCmd_WithData(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "status")
	require.NoError(t, err)
}

func TestStatusCmd_WithRecalc(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "status", "--recalc")
	require.NoError(t, err)
}

// --- session commands ---

func TestSessionLogCmd_RequiresFlags(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "session", "log")
	assert.Error(t, err)
}

func TestSessionLogCmd_Success(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "session", "log", "--work-item", wiID, "--minutes", "30")
	require.NoError(t, err)
}

func TestSessionListCmd_EmptyDB(t *testing.T) {
	app := testApp(t)

	// session list outputs via fmt.Print (not cmd.OutOrStdout), so we just
	// verify it runs without error.
	_, err := executeCmd(t, app, "session", "list")
	require.NoError(t, err)
}

func TestSessionListCmd_WithData(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)

	// Log a session first.
	_, err := executeCmd(t, app, "session", "log", "--work-item", wiID, "--minutes", "30")
	require.NoError(t, err)

	_, err = executeCmd(t, app, "session", "list")
	require.NoError(t, err)
}

// --- replan command ---

func TestReplanCmd_WithData(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "replan")
	require.NoError(t, err)
}

// --- ask command (LLM disabled) ---

func TestAskCmd_LLMDisabled(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "ask", "what should I work on?")
	assert.Error(t, err, "ask should error when LLM is disabled")
}

// --- explain/review commands use deterministic fallback when LLM is nil ---

func TestExplainNowCmd_DeterministicFallback(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	// explain now uses deterministic fallback when LLM is nil — should not error.
	_, err := executeCmd(t, app, "explain", "now", "--minutes", "60")
	require.NoError(t, err)
}

func TestReviewWeeklyCmd_DeterministicFallback(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	// review weekly uses deterministic fallback when LLM is nil — should not error.
	_, err := executeCmd(t, app, "review", "weekly")
	require.NoError(t, err)
}
