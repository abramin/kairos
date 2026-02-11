package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
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
		WorkItems: service.NewWorkItemService(wiRepo, nodeRepo),
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

// --- Root command ---

func TestRootCmd_NoArgs_NonInteractive(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interactive terminal")
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

// --- helpers for full-service tests ---

// testAppFull wires an App with Templates and Import services.
func testAppFull(t *testing.T) *App {
	t.Helper()
	db := testutil.NewTestDB(t)

	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	depRepo := repository.NewSQLiteDependencyRepo(db)
	sessRepo := repository.NewSQLiteSessionRepo(db)
	profRepo := repository.NewSQLiteUserProfileRepo(db)

	templateDir := findTemplatesDir(t)

	return &App{
		Projects:  service.NewProjectService(projRepo),
		Nodes:     service.NewNodeService(nodeRepo),
		WorkItems: service.NewWorkItemService(wiRepo, nodeRepo),
		Sessions:  service.NewSessionService(sessRepo, wiRepo),
		WhatNow:   service.NewWhatNowService(wiRepo, sessRepo, projRepo, depRepo, profRepo),
		Status:    service.NewStatusService(projRepo, wiRepo, sessRepo, profRepo),
		Replan:    service.NewReplanService(projRepo, wiRepo, sessRepo, profRepo),
		Templates: service.NewTemplateService(templateDir, projRepo, nodeRepo, wiRepo, depRepo),
		Import:    service.NewImportService(projRepo, nodeRepo, wiRepo, depRepo),
	}
}

func findTemplatesDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		candidate := filepath.Join(dir, "templates")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find templates directory")
		}
		dir = parent
	}
}

// --- project commands ---

func TestProjectAddCmd_Success(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "project", "add",
		"--id", "TST01",
		"--name", "Test Project",
		"--domain", "education",
		"--start", "2026-01-15",
	)
	require.NoError(t, err)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "TST01", projects[0].ShortID)
	assert.Equal(t, "Test Project", projects[0].Name)
}

func TestProjectAddCmd_WithDueDate(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "project", "add",
		"--id", "DUE01",
		"--name", "Due Project",
		"--domain", "fitness",
		"--start", "2026-01-01",
		"--due", "2026-06-01",
	)
	require.NoError(t, err)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.NotNil(t, projects[0].TargetDate)
}

func TestProjectAddCmd_MissingRequiredFlags(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "project", "add", "--name", "No ID")
	assert.Error(t, err)
}

func TestProjectListCmd_Empty(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "project", "list")
	require.NoError(t, err)
}

func TestProjectListCmd_WithData(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "project", "list")
	require.NoError(t, err)
}

func TestProjectInspectCmd_ByShortID(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Inspect Me", testutil.WithShortID("INS01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, node))

	_, err := executeCmd(t, app, "project", "inspect", "INS01")
	require.NoError(t, err)
}

func TestProjectInspectCmd_NotFound(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "project", "inspect", "NOPE99")
	assert.Error(t, err)
}

func TestProjectUpdateCmd_Name(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Original", testutil.WithShortID("UPD01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	_, err := executeCmd(t, app, "project", "update", "UPD01", "--name", "Renamed")
	require.NoError(t, err)

	updated, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)
}

func TestProjectUpdateCmd_Status(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Pausable", testutil.WithShortID("PAU01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	_, err := executeCmd(t, app, "project", "update", "PAU01", "--status", "paused")
	require.NoError(t, err)

	updated, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ProjectPaused, updated.Status)
}

func TestProjectArchiveCmd_Success(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Archivable", testutil.WithShortID("ARC01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	_, err := executeCmd(t, app, "project", "archive", "ARC01")
	require.NoError(t, err)

	// Archived projects should not appear in default list.
	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, projects)

	// But should appear with --all.
	all, err := app.Projects.List(ctx, true)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestProjectUnarchiveCmd_Success(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Restorable", testutil.WithShortID("RES01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	_, err := executeCmd(t, app, "project", "archive", "RES01")
	require.NoError(t, err)

	_, err = executeCmd(t, app, "project", "unarchive", "RES01")
	require.NoError(t, err)

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, projects, 1)
}

func TestProjectRemoveCmd_ForceDelete(t *testing.T) {
	app := testApp(t)
	projID, _ := seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "project", "remove", projID, "--force")
	require.NoError(t, err)

	projects, err := app.Projects.List(context.Background(), true)
	require.NoError(t, err)
	assert.Empty(t, projects)
}

func TestProjectRemoveCmd_NotFound(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "project", "remove", "nonexistent-id")
	assert.Error(t, err)
}

// --- node commands ---

func TestNodeAddCmd_Success(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Node Host", testutil.WithShortID("NOD01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	_, err := executeCmd(t, app, "node", "add",
		"--project", proj.ID,
		"--title", "Week 1",
		"--kind", "week",
	)
	require.NoError(t, err)

	nodes, err := app.Nodes.ListRoots(ctx, proj.ID)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "Week 1", nodes[0].Title)
	assert.Equal(t, domain.NodeWeek, nodes[0].Kind)
}

func TestNodeAddCmd_MissingRequiredFlags(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "node", "add", "--title", "No Project")
	assert.Error(t, err)
}

func TestNodeInspectCmd_Success(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Inspect Node", testutil.WithShortID("NIN01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module A", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, node))

	_, err := executeCmd(t, app, "node", "inspect", node.ID)
	require.NoError(t, err)
}

func TestNodeUpdateCmd_Title(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Update Node", testutil.WithShortID("NUP01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Old Title", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	_, err := executeCmd(t, app, "node", "update", node.ID, "--title", "New Title")
	require.NoError(t, err)

	updated, err := app.Nodes.GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Title", updated.Title)
}

func TestNodeRemoveCmd_Success(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Remove Node", testutil.WithShortID("NRM01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Deletable", testutil.WithNodeKind(domain.NodeGeneric))
	require.NoError(t, app.Nodes.Create(ctx, node))

	_, err := executeCmd(t, app, "node", "remove", node.ID)
	require.NoError(t, err)

	_, err = app.Nodes.GetByID(ctx, node.ID)
	assert.Error(t, err)
}

// --- work item commands ---

func TestWorkAddCmd_Success(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Work Host", testutil.WithShortID("WRK01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	_, err := executeCmd(t, app, "work", "add",
		"--node", node.ID,
		"--title", "Read Chapter 1",
		"--type", "reading",
		"--planned-min", "45",
	)
	require.NoError(t, err)

	items, err := app.WorkItems.ListByNode(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Read Chapter 1", items[0].Title)
	assert.Equal(t, 45, items[0].PlannedMin)
}

func TestWorkAddCmd_MissingRequiredFlags(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "work", "add", "--title", "No Node")
	assert.Error(t, err)
}

func TestWorkInspectCmd_Success(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "work", "inspect", wiID)
	require.NoError(t, err)
}

func TestWorkDoneCmd_Success(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "work", "done", wiID)
	require.NoError(t, err)

	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, wi.Status)
}

func TestWorkDoneCmd_NotFound(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "work", "done", "nonexistent-id")
	assert.Error(t, err)
}

func TestWorkArchiveCmd_Success(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "work", "archive", wiID)
	require.NoError(t, err)
}

func TestWorkUpdateCmd_PlannedMin(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "work", "update", wiID, "--planned-min", "120")
	require.NoError(t, err)

	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, 120, wi.PlannedMin)
}

func TestWorkRemoveCmd_Success(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	_, err := executeCmd(t, app, "work", "remove", wiID)
	require.NoError(t, err)

	_, err = app.WorkItems.GetByID(ctx, wiID)
	assert.Error(t, err)
}

// --- project import command ---

func TestProjectImportCmd_Success(t *testing.T) {
	app := testAppFull(t)

	importJSON := `{
		"project": {
			"short_id": "IMP01",
			"name": "Imported Project",
			"domain": "education",
			"start_date": "2026-01-15"
		},
		"nodes": [
			{"ref": "n1", "title": "Chapter 1", "kind": "module", "order": 0}
		],
		"work_items": [
			{"ref": "w1", "node_ref": "n1", "title": "Read Ch1", "type": "reading", "planned_min": 45}
		]
	}`

	dir := t.TempDir()
	path := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(path, []byte(importJSON), 0644))

	_, err := executeCmd(t, app, "project", "import", path)
	require.NoError(t, err)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "IMP01", projects[0].ShortID)
}

func TestProjectImportCmd_InvalidJSON(t *testing.T) {
	app := testAppFull(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	_, err := executeCmd(t, app, "project", "import", path)
	assert.Error(t, err)
}

func TestProjectImportCmd_FileNotFound(t *testing.T) {
	app := testAppFull(t)

	_, err := executeCmd(t, app, "project", "import", "/nonexistent/path.json")
	assert.Error(t, err)
}

// --- project init command ---

func TestProjectInitCmd_Success(t *testing.T) {
	app := testAppFull(t)

	_, err := executeCmd(t, app, "project", "init",
		"--id", "INI01",
		"--template", "course_weekly_generic",
		"--name", "Init Test",
		"--start", "2026-02-10",
		"--var", "weeks=2",
		"--var", "assignment_count=1",
	)
	require.NoError(t, err)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "INI01", projects[0].ShortID)
	assert.Equal(t, "Init Test", projects[0].Name)
}

func TestProjectInitCmd_MissingRequiredFlags(t *testing.T) {
	app := testAppFull(t)

	_, err := executeCmd(t, app, "project", "init", "--name", "No Template")
	assert.Error(t, err)
}

// --- template commands ---

func TestTemplateListCmd_Success(t *testing.T) {
	app := testAppFull(t)

	_, err := executeCmd(t, app, "template", "list")
	require.NoError(t, err)
}

func TestTemplateShowCmd_Success(t *testing.T) {
	app := testAppFull(t)

	_, err := executeCmd(t, app, "template", "show", "course_weekly_generic")
	require.NoError(t, err)
}

func TestTemplateShowCmd_NotFound(t *testing.T) {
	app := testAppFull(t)

	_, err := executeCmd(t, app, "template", "show", "nonexistent_template")
	assert.Error(t, err)
}

func TestTemplateDraftCmd_LLMDisabled(t *testing.T) {
	app := testAppFull(t)

	_, err := executeCmd(t, app, "template", "draft", "a study plan for calculus")
	assert.Error(t, err)
}

// =============================================================================
// E2E CLI Round-Trip Tests
// =============================================================================

// seedCriticalAndOnTrack creates two projects: one critical (due soon, no work logged)
// and one on-track (distant deadline). Returns project IDs.
func seedCriticalAndOnTrack(t *testing.T, app *App) (criticalProjID, onTrackProjID string) {
	t.Helper()
	ctx := context.Background()

	// Critical project: due in 3 days, 120 min planned, 0 logged.
	critDeadline := time.Now().UTC().AddDate(0, 0, 3)
	critProj := testutil.NewTestProject("Urgent Paper",
		testutil.WithShortID("URG01"),
		testutil.WithTargetDate(critDeadline),
	)
	require.NoError(t, app.Projects.Create(ctx, critProj))

	critNode := testutil.NewTestNode(critProj.ID, "Section 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, critNode))

	critWI := testutil.NewTestWorkItem(critNode.ID, "Write Introduction",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, app.WorkItems.Create(ctx, critWI))

	// On-track project: due in 6 months, 60 min planned.
	otDeadline := time.Now().UTC().AddDate(0, 6, 0)
	otProj := testutil.NewTestProject("Leisurely Reading",
		testutil.WithShortID("LEI01"),
		testutil.WithTargetDate(otDeadline),
	)
	require.NoError(t, app.Projects.Create(ctx, otProj))

	otNode := testutil.NewTestNode(otProj.ID, "Chapter 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, otNode))

	otWI := testutil.NewTestWorkItem(otNode.ID, "Read Chapter 1",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, app.WorkItems.Create(ctx, otWI))

	return critProj.ID, otProj.ID
}

func TestWhatNowCLI_RoundTrip(t *testing.T) {
	app := testApp(t)
	critProjID, _ := seedCriticalAndOnTrack(t, app)

	// CLI round-trip: Cobra parses flags → service → DB → formatter.
	_, err := executeCmd(t, app, "what-now", "--minutes", "60")
	require.NoError(t, err, "what-now CLI should succeed with critical+on-track data")

	// Verify the service produces the expected response for the same input.
	ctx := context.Background()
	resp, err := app.WhatNow.Recommend(ctx, contract.NewWhatNowRequest(60))
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Recommendations, "should have recommendations")
	// The critical project's item should be recommended first.
	assert.Equal(t, critProjID, resp.Recommendations[0].ProjectID,
		"critical project should be first recommendation")
	assert.Equal(t, "Write Introduction", resp.Recommendations[0].Title)
	assert.LessOrEqual(t, resp.AllocatedMin, 60,
		"allocated_min must not exceed requested 60")
}

func TestWhatNowCLI_RoundTrip_CriticalMode(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	// Past-due project guarantees RiskCritical → ModeCritical.
	pastDue := time.Now().UTC().AddDate(0, 0, -1)
	critProj := testutil.NewTestProject("Overdue Paper",
		testutil.WithShortID("OVD01"),
		testutil.WithTargetDate(pastDue),
	)
	require.NoError(t, app.Projects.Create(ctx, critProj))

	critNode := testutil.NewTestNode(critProj.ID, "Section 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, critNode))

	critWI := testutil.NewTestWorkItem(critNode.ID, "Write Introduction",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, app.WorkItems.Create(ctx, critWI))

	// CLI round-trip: 30 min available with an overdue project.
	_, err := executeCmd(t, app, "what-now", "--minutes", "30")
	require.NoError(t, err)

	// Verify critical mode behavior via service.
	resp, err := app.WhatNow.Recommend(ctx, contract.NewWhatNowRequest(30))
	require.NoError(t, err)

	assert.Equal(t, domain.ModeCritical, resp.Mode,
		"should be in critical mode with a past-due project")
	for _, rec := range resp.Recommendations {
		assert.Equal(t, critProj.ID, rec.ProjectID,
			"in critical mode, only critical project items should be recommended")
	}
}

func TestStatusCLI_RoundTrip(t *testing.T) {
	app := testApp(t)
	seedCriticalAndOnTrack(t, app)

	// CLI round-trip: status command should succeed.
	_, err := executeCmd(t, app, "status")
	require.NoError(t, err)

	// Verify the service returns both projects with correct risk levels.
	ctx := context.Background()
	resp, err := app.Status.GetStatus(ctx, contract.NewStatusRequest())
	require.NoError(t, err)

	assert.Len(t, resp.Projects, 2, "should show both projects")

	riskMap := map[string]domain.RiskLevel{}
	for _, p := range resp.Projects {
		riskMap[p.ProjectName] = p.RiskLevel
	}
	assert.Equal(t, domain.RiskAtRisk, riskMap["Urgent Paper"],
		"project due in 3 days with no work should be at risk")
	assert.Equal(t, domain.RiskOnTrack, riskMap["Leisurely Reading"],
		"project due in 6 months should be on track")
}

func TestStatusCLI_RoundTrip_ScopedToProject(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Scoped Status", testutil.WithShortID("SCP01"),
		testutil.WithTargetDate(time.Now().UTC().AddDate(0, 3, 0)))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Study",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	// CLI round-trip with --project flag.
	_, err := executeCmd(t, app, "status", "--project", proj.ID)
	require.NoError(t, err)

	// Verify scoping via service.
	req := contract.NewStatusRequest()
	req.ProjectScope = []string{proj.ID}
	resp, err := app.Status.GetStatus(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Projects, 1)
	assert.Equal(t, "Scoped Status", resp.Projects[0].ProjectName)
}

func TestReplanCLI_RoundTrip(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	target := time.Now().UTC().AddDate(0, 3, 0)
	proj := testutil.NewTestProject("Replan Project",
		testutil.WithShortID("RPL01"),
		testutil.WithTargetDate(target),
	)
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Reading",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
		testutil.WithUnits("pages", 20, 0),
	)
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	// Log a session so replan has data to work with.
	// 5 of 20 pages done in 30 min → implied pace: 30*20/5 = 120 min total.
	_, err := executeCmd(t, app, "session", "log", "--work-item", wi.ID, "--minutes", "30", "--units-done", "5")
	require.NoError(t, err)

	// Session logging already applies SmoothReEstimate: 0.7*60 + 0.3*120 = 78.
	wiAfterLog, err := app.WorkItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	plannedAfterLog := wiAfterLog.PlannedMin
	assert.Greater(t, plannedAfterLog, 60,
		"session log should have smoothed planned min upward")

	_, err = executeCmd(t, app, "replan")
	require.NoError(t, err)

	// Replan applies a second smoothing pass: 0.7*78 + 0.3*120 ≈ 91.
	wiAfterReplan, err := app.WorkItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Greater(t, wiAfterReplan.PlannedMin, plannedAfterLog,
		"replan should further adjust planned minutes via smoothing")
}

// =============================================================================
// resolveProjectID Integration Tests
// =============================================================================

func TestResolveProjectID_ExactShortID(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Resolvable", testutil.WithShortID("RESO01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	resolved, err := resolveProjectID(ctx, app, "RESO01")
	require.NoError(t, err)
	assert.Equal(t, proj.ID, resolved)
}

func TestResolveProjectID_CaseInsensitive(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Case Test", testutil.WithShortID("CASE01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	resolved, err := resolveProjectID(ctx, app, "case01")
	require.NoError(t, err)
	assert.Equal(t, proj.ID, resolved)

	resolved2, err := resolveProjectID(ctx, app, "Case01")
	require.NoError(t, err)
	assert.Equal(t, proj.ID, resolved2)
}

func TestResolveProjectID_ExactUUID(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("UUID Test", testutil.WithShortID("UUID01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	resolved, err := resolveProjectID(ctx, app, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, proj.ID, resolved)
}

func TestResolveProjectID_UUIDPrefix(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Prefix Test", testutil.WithShortID("PFX01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	// Use the first 8 chars of the UUID as prefix.
	prefix := proj.ID[:8]
	resolved, err := resolveProjectID(ctx, app, prefix)
	require.NoError(t, err)
	assert.Equal(t, proj.ID, resolved)
}

func TestResolveProjectID_NotFound(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	_, err := resolveProjectID(ctx, app, "NOPE99")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveProjectID_EmptyInput(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	_, err := resolveProjectID(ctx, app, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

// =============================================================================
// resolveWorkItemID Fallback Tests
// =============================================================================

func TestResolveWorkItemID_FallbackToNodeSeq(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Fallback Test", testutil.WithShortID("FBK01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Homer – The Odyssey", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Read The Odyssey", testutil.WithPlannedMin(720))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	// Node gets seq=1, work item gets seq=2.
	// Requesting work item by node's seq (1) should fall back to the single work item.
	resolved, err := resolveWorkItemID(ctx, app, "1", proj.ID)
	require.NoError(t, err)
	assert.Equal(t, wi.ID, resolved)
}

func TestResolveWorkItemID_NoFallbackMultipleWorkItems(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Multi WI", testutil.WithShortID("MWI01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Reading", testutil.WithPlannedMin(60))
	require.NoError(t, app.WorkItems.Create(ctx, wi1))
	wi2 := testutil.NewTestWorkItem(node.ID, "Exercises", testutil.WithPlannedMin(30))
	require.NoError(t, app.WorkItems.Create(ctx, wi2))

	// Node gets seq=1 with 2 work items → fallback should NOT resolve.
	_, err := resolveWorkItemID(ctx, app, "1", proj.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveWorkItemID_DirectSeqStillWorks(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Direct Seq", testutil.WithShortID("DIR01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Chapter 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Read Chapter", testutil.WithPlannedMin(60))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	// Direct work item seq (2) should resolve without needing fallback.
	resolved, err := resolveWorkItemID(ctx, app, "2", proj.ID)
	require.NoError(t, err)
	assert.Equal(t, wi.ID, resolved)
}

func TestResolveProjectID_IncludesArchivedProjects(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Archived", testutil.WithShortID("ARC99"))
	require.NoError(t, app.Projects.Create(ctx, proj))
	require.NoError(t, app.Projects.Archive(ctx, proj.ID))

	// resolveProjectID lists with includeArchived=true, so archived should resolve.
	resolved, err := resolveProjectID(ctx, app, "ARC99")
	require.NoError(t, err)
	assert.Equal(t, proj.ID, resolved)
}

// =============================================================================
// Project Import → What-Now E2E
// =============================================================================

func TestProjectImportThenWhatNow_E2E(t *testing.T) {
	app := testAppFull(t)

	importJSON := `{
		"project": {
			"short_id": "IMP99",
			"name": "Imported for WhatNow",
			"domain": "education",
			"start_date": "2026-01-15",
			"target_date": "2026-06-01"
		},
		"nodes": [
			{"ref": "n1", "title": "Week 1", "kind": "week", "order": 0},
			{"ref": "n2", "title": "Week 2", "kind": "week", "order": 1}
		],
		"work_items": [
			{"ref": "w1", "node_ref": "n1", "title": "Reading Ch1", "type": "reading", "planned_min": 45},
			{"ref": "w2", "node_ref": "n1", "title": "Practice Ch1", "type": "practice", "planned_min": 30},
			{"ref": "w3", "node_ref": "n2", "title": "Reading Ch2", "type": "reading", "planned_min": 45}
		]
	}`

	dir := t.TempDir()
	path := filepath.Join(dir, "import_e2e.json")
	require.NoError(t, os.WriteFile(path, []byte(importJSON), 0644))

	// Step 1: Import the project.
	_, err := executeCmd(t, app, "project", "import", path)
	require.NoError(t, err)

	// Step 2: Verify project exists.
	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "IMP99", projects[0].ShortID)

	// Step 3: Run what-now and verify imported items appear as candidates.
	_, err = executeCmd(t, app, "what-now", "--minutes", "90")
	require.NoError(t, err)

	// Verify via service that imported items are schedulable.
	resp, err := app.WhatNow.Recommend(context.Background(), contract.NewWhatNowRequest(90))
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Recommendations, "imported items should be schedulable")

	foundImported := false
	for _, rec := range resp.Recommendations {
		if rec.Title == "Reading Ch1" || rec.Title == "Practice Ch1" || rec.Title == "Reading Ch2" {
			foundImported = true
			break
		}
	}
	assert.True(t, foundImported, "at least one imported work item should appear in recommendations")
}

// =============================================================================
// Review Weekly & Explain Now E2E
// =============================================================================

func TestReviewWeeklyCLI_RoundTrip(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	// review weekly uses fmt.Print → output goes to real stdout, not Cobra buffer.
	// We verify the command completes without error (deterministic fallback path).
	_, err := executeCmd(t, app, "review", "weekly")
	require.NoError(t, err)
}

func TestExplainNowCLI_RoundTrip(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	// explain now uses fmt.Print → output goes to real stdout, not Cobra buffer.
	// We verify the command completes without error (deterministic fallback path).
	_, err := executeCmd(t, app, "explain", "now", "--minutes", "60")
	require.NoError(t, err)
}

// =============================================================================
// parseDurationArg
// =============================================================================

func TestParseDurationArg(t *testing.T) {
	tests := []struct {
		input   string
		wantMin int
		wantOK  bool
	}{
		{"120", 120, true},
		{"60", 60, true},
		{"1", 1, true},
		{"2h", 120, true},
		{"30m", 30, true},
		{"1h30m", 90, true},
		{"1h0m", 60, true},
		{"0h30m", 30, true},
		{"0", 0, false},
		{"-1", 0, false},
		{"", 0, false},
		{"abc", 0, false},
		{"h", 0, false},
		{"m", 0, false},
		{"2x", 0, false},
		{"Review", 0, false},
		{"#3", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseDurationArg(tt.input)
			assert.Equal(t, tt.wantOK, ok, "ok mismatch for %q", tt.input)
			if ok {
				assert.Equal(t, tt.wantMin, got, "minutes mismatch for %q", tt.input)
			}
		})
	}
}
