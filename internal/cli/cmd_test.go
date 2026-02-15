package cli

import (
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
	uow := testutil.NewTestUoW(db)

	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	depRepo := repository.NewSQLiteDependencyRepo(db)
	sessRepo := repository.NewSQLiteSessionRepo(db)
	profRepo := repository.NewSQLiteUserProfileRepo(db)

	return &App{
		Projects:  service.NewProjectService(projRepo),
		Nodes:     service.NewNodeService(nodeRepo, uow),
		WorkItems: service.NewWorkItemService(wiRepo, nodeRepo, uow),
		Sessions:  service.NewSessionService(sessRepo, uow),
		WhatNow:   service.NewWhatNowService(wiRepo, sessRepo, depRepo, profRepo),
		Status:    service.NewStatusService(projRepo, wiRepo, sessRepo, profRepo),
		Replan:    service.NewReplanService(projRepo, wiRepo, sessRepo, profRepo, uow),
		// Templates and Import left nil — not tested here.
		// Intelligence services left nil — LLM disabled.
	}
}

// seedOpts configures seedProjectCore.
type seedOpts struct {
	shortID    string
	name       string
	plannedMin int
}

// seedProjectCore creates a project→node→work-item triple and returns all three IDs.
func seedProjectCore(t *testing.T, app *App, opts seedOpts) (projID, nodeID, wiID string) {
	t.Helper()
	ctx := context.Background()

	if opts.name == "" {
		opts.name = "CLI Test Project"
	}
	if opts.plannedMin == 0 {
		opts.plannedMin = 60
	}

	target := time.Now().UTC().AddDate(0, 3, 0)
	projOpts := []testutil.ProjectOption{testutil.WithTargetDate(target)}
	if opts.shortID != "" {
		projOpts = append(projOpts, testutil.WithShortID(opts.shortID))
	}
	proj := testutil.NewTestProject(opts.name, projOpts...)
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Reading",
		testutil.WithPlannedMin(opts.plannedMin),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, app.WorkItems.Create(ctx, wi))

	return proj.ID, node.ID, wi.ID
}

// seedProjectWithWork creates a project with a node and work item for CLI tests.
func seedProjectWithWork(t *testing.T, app *App) (string, string) {
	t.Helper()
	projID, _, wiID := seedProjectCore(t, app, seedOpts{})
	return projID, wiID
}

// --- helpers for full-service tests ---

// testAppFull wires an App with Templates and Import services.
func testAppFull(t *testing.T) *App {
	t.Helper()
	db := testutil.NewTestDB(t)
	uow := testutil.NewTestUoW(db)

	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	depRepo := repository.NewSQLiteDependencyRepo(db)
	sessRepo := repository.NewSQLiteSessionRepo(db)
	profRepo := repository.NewSQLiteUserProfileRepo(db)

	templateDir := findTemplatesDir(t)
	sessionSvc := service.NewSessionService(sessRepo, uow)
	templateSvc := service.NewTemplateService(templateDir, uow)
	importSvc := service.NewImportService(uow)

	return &App{
		Projects:      service.NewProjectService(projRepo),
		Nodes:         service.NewNodeService(nodeRepo, uow),
		WorkItems:     service.NewWorkItemService(wiRepo, nodeRepo, uow),
		Sessions:      sessionSvc,
		WhatNow:       service.NewWhatNowService(wiRepo, sessRepo, depRepo, profRepo),
		Status:        service.NewStatusService(projRepo, wiRepo, sessRepo, profRepo),
		Replan:        service.NewReplanService(projRepo, wiRepo, sessRepo, profRepo, uow),
		Templates:     templateSvc,
		Import:        importSvc,
		LogSession:    sessionSvc,
		InitProject:   templateSvc,
		ImportProject: importSvc,
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

// --- entity dispatch tests ---

func TestDispatchProject_Add(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "add", nil, map[string]string{
		"id": "TST01", "name": "Test Project", "domain": "education", "start": "2026-01-15",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "Created project")

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "TST01", projects[0].ShortID)
}

func TestDispatchProject_AddWithDue(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "add", nil, map[string]string{
		"id": "DUE01", "name": "Due Project", "domain": "fitness",
		"start": "2026-01-01", "due": "2026-06-01",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "Created project")

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.NotNil(t, projects[0].TargetDate)
}

func TestDispatchProject_List(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	seedProjectWithWork(t, app)

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "list", nil, map[string]string{})
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestDispatchProject_Inspect(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Inspect Me", testutil.WithShortID("INS01"))
	require.NoError(t, app.Projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, node))

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "inspect", []string{"INS01"}, map[string]string{})
	require.NoError(t, err)
	assert.Contains(t, result, "Module 1")
}

func TestDispatchProject_Update(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Original", testutil.WithShortID("UPD01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "update", []string{"UPD01"}, map[string]string{"name": "Renamed"})
	require.NoError(t, err)
	assert.Contains(t, result, "Updated")

	updated, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)
}

func TestDispatchProject_Archive(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Archivable", testutil.WithShortID("ARC01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "archive", []string{"ARC01"}, map[string]string{})
	require.NoError(t, err)
	assert.Contains(t, result, "Archived")

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, projects)
}

func TestDispatchProject_Remove(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	projID, _ := seedProjectWithWork(t, app)

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "remove", []string{projID}, map[string]string{"force": "true"})
	require.NoError(t, err)
	assert.Contains(t, result, "Removed")
}

func TestDispatchNode_Add(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Node Host", testutil.WithShortID("NOD01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	state := &SharedState{App: app, ActiveProjectID: proj.ID}
	cb := &commandBar{state: state}

	result, err := cb.dispatchNode(ctx, "add", nil, map[string]string{
		"project": proj.ID, "title": "Week 1", "kind": "week",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "Created node")

	nodes, err := app.Nodes.ListRoots(ctx, proj.ID)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "Week 1", nodes[0].Title)
}

func TestDispatchWork_Add(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Work Host", testutil.WithShortID("WRK01"))
	require.NoError(t, app.Projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, app.Nodes.Create(ctx, node))

	state := &SharedState{App: app, ActiveProjectID: proj.ID}
	cb := &commandBar{state: state}

	result, err := cb.dispatchWork(ctx, "add", nil, map[string]string{
		"node": node.ID, "title": "Read Chapter 1", "type": "reading", "planned-min": "45",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "Created")

	items, err := app.WorkItems.ListByNode(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Read Chapter 1", items[0].Title)
	assert.Equal(t, 45, items[0].PlannedMin)
}

func TestDispatchWork_Done(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchWork(ctx, "done", []string{wiID}, map[string]string{})
	require.NoError(t, err)
	assert.Contains(t, result, "done")

	wi, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, wi.Status)
}

func TestDispatchWork_Remove(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchWork(ctx, "remove", []string{wiID}, map[string]string{})
	require.NoError(t, err)
	assert.Contains(t, result, "Removed")

	_, err = app.WorkItems.GetByID(ctx, wiID)
	assert.Error(t, err)
}

func TestDispatchProject_Import(t *testing.T) {
	app := testAppFull(t)
	ctx := context.Background()

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

	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "import", []string{path}, map[string]string{})
	require.NoError(t, err)
	assert.Contains(t, result, "Imported")

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "IMP01", projects[0].ShortID)
}

// --- E2E round-trip tests using services directly ---

// seedCriticalAndOnTrack creates two projects: one critical and one on-track.
func seedCriticalAndOnTrack(t *testing.T, app *App) (criticalProjID, onTrackProjID string) {
	t.Helper()
	ctx := context.Background()

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

func TestWhatNow_RoundTrip(t *testing.T) {
	app := testApp(t)
	critProjID, _ := seedCriticalAndOnTrack(t, app)

	ctx := context.Background()
	resp, err := app.WhatNow.Recommend(ctx, contract.NewWhatNowRequest(60))
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Recommendations, "should have recommendations")
	assert.Equal(t, critProjID, resp.Recommendations[0].ProjectID,
		"critical project should be first recommendation")
	assert.LessOrEqual(t, resp.AllocatedMin, 60)
}

func TestStatus_RoundTrip(t *testing.T) {
	app := testApp(t)
	seedCriticalAndOnTrack(t, app)

	ctx := context.Background()
	resp, err := app.Status.GetStatus(ctx, contract.NewStatusRequest())
	require.NoError(t, err)

	assert.Len(t, resp.Projects, 2)

	riskMap := map[string]domain.RiskLevel{}
	for _, p := range resp.Projects {
		riskMap[p.ProjectName] = p.RiskLevel
	}
	assert.Equal(t, domain.RiskAtRisk, riskMap["Urgent Paper"])
	assert.Equal(t, domain.RiskOnTrack, riskMap["Leisurely Reading"])
}

// --- resolveProjectID integration tests ---

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

func TestResolveProjectID_IncludesArchivedProjects(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Archived", testutil.WithShortID("ARC99"))
	require.NoError(t, app.Projects.Create(ctx, proj))
	require.NoError(t, app.Projects.Archive(ctx, proj.ID))

	resolved, err := resolveProjectID(ctx, app, "ARC99")
	require.NoError(t, err)
	assert.Equal(t, proj.ID, resolved)
}

// --- resolveWorkItemID fallback tests ---

func TestResolveWorkItemID_FallbackToNodeSeq(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Fallback Test", testutil.WithShortID("FBK01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Homer – The Odyssey", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, app.Nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Read The Odyssey", testutil.WithPlannedMin(720))
	require.NoError(t, app.WorkItems.Create(ctx, wi))

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

	resolved, err := resolveWorkItemID(ctx, app, "2", proj.ID)
	require.NoError(t, err)
	assert.Equal(t, wi.ID, resolved)
}

// --- parseDurationArg ---

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

// --- parseShellFlags ---

func TestParseShellFlags(t *testing.T) {
	pos, flags := parseShellFlags([]string{"INS01", "--name", "Foo", "--all"})
	assert.Equal(t, []string{"INS01"}, pos)
	assert.Equal(t, "Foo", flags["name"])
	assert.Equal(t, "true", flags["all"])
}

func TestParseShellFlags_Empty(t *testing.T) {
	pos, flags := parseShellFlags(nil)
	assert.Empty(t, pos)
	assert.Empty(t, flags)
}
