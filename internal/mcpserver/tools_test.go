package mcpserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newToolsTestServer(t *testing.T) (*Server, *repository.SQLiteProjectRepo, *repository.SQLiteWorkItemRepo, context.Context) {
	t.Helper()
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	deps := Deps{Projects: projRepo, WorkItems: wiRepo}
	s := &Server{deps: deps}
	s.registerTools()
	return s, projRepo, wiRepo, context.Background()
}

func createProjectWithItem(t *testing.T,
	projRepo *repository.SQLiteProjectRepo,
	nodeRepo *repository.SQLitePlanNodeRepo,
	wiRepo *repository.SQLiteWorkItemRepo,
	ctx context.Context,
	projName, shortID, itemTitle string,
	dueDate *time.Time,
) (*domain.Project, *domain.WorkItem) {
	t.Helper()
	proj := testutil.NewTestProject(projName, testutil.WithShortID(shortID))
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, itemTitle,
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	if dueDate != nil {
		wi.DueDate = dueDate
	}
	require.NoError(t, wiRepo.Create(ctx, wi))
	return proj, wi
}

func TestHandleListProjects_Empty(t *testing.T) {
	s, _, _, ctx := newToolsTestServer(t)
	result, err := s.handleListProjects(ctx)
	require.NoError(t, err)
	assert.Contains(t, result, "No active projects")
}

func TestHandleListProjects_ReturnsList(t *testing.T) {
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	deps := Deps{Projects: projRepo, WorkItems: wiRepo}
	s := &Server{deps: deps}
	s.registerTools()
	ctx := context.Background()

	future := time.Now().UTC().AddDate(0, 1, 0)
	p1 := testutil.NewTestProject("Philosophy", testutil.WithShortID("PHI01"), testutil.WithTargetDate(future))
	require.NoError(t, projRepo.Create(ctx, p1))
	_ = nodeRepo

	result, err := s.handleListProjects(ctx)
	require.NoError(t, err)

	var projects []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &projects))
	require.Len(t, projects, 1)
	assert.Equal(t, "PHI01", projects[0]["short_id"])
	assert.Equal(t, "Philosophy", projects[0]["name"])
	assert.NotNil(t, projects[0]["target_date"])
}

func TestHandleListDueItems_NoItemsWithDueDates(t *testing.T) {
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	deps := Deps{Projects: projRepo, WorkItems: wiRepo}
	s := &Server{deps: deps}
	s.registerTools()
	ctx := context.Background()

	// Create an item with no due date
	proj := testutil.NewTestProject("NoDue")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Task", testutil.WithPlannedMin(30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	result, err := s.handleListDueItems(ctx, json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, result, "No items with due dates")
}

func TestHandleListDueItems_FiltersWithinCutoff(t *testing.T) {
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	deps := Deps{Projects: projRepo, WorkItems: wiRepo}
	s := &Server{deps: deps}
	s.registerTools()
	ctx := context.Background()

	proj := testutil.NewTestProject("DueProj")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	// Item due in 5 days — within default 14-day window
	soon := time.Now().UTC().AddDate(0, 0, 5)
	wiSoon := testutil.NewTestWorkItem(node.ID, "Soon task",
		testutil.WithPlannedMin(45),
		testutil.WithSessionBounds(15, 60, 30),
	)
	wiSoon.DueDate = &soon
	require.NoError(t, wiRepo.Create(ctx, wiSoon))

	// Item due in 30 days — outside default window
	later := time.Now().UTC().AddDate(0, 0, 30)
	wiLater := testutil.NewTestWorkItem(node.ID, "Later task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	wiLater.DueDate = &later
	require.NoError(t, wiRepo.Create(ctx, wiLater))

	result, err := s.handleListDueItems(ctx, json.RawMessage(`{"days_ahead":14}`))
	require.NoError(t, err)

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &items))
	require.Len(t, items, 1, "only the item due within 14 days should be returned")
	assert.Equal(t, "Soon task", items[0]["title"])
	assert.Equal(t, soon.Format("2006-01-02"), items[0]["due_date"])
}

func TestHandleListDueItems_DefaultDaysAhead(t *testing.T) {
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	deps := Deps{Projects: projRepo, WorkItems: wiRepo}
	s := &Server{deps: deps}
	s.registerTools()
	ctx := context.Background()

	proj := testutil.NewTestProject("Proj")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	wi := testutil.NewTestWorkItem(node.ID, "Tomorrow", testutil.WithPlannedMin(30))
	wi.DueDate = &tomorrow
	require.NoError(t, wiRepo.Create(ctx, wi))

	// Empty args — should default to 14 days
	result, err := s.handleListDueItems(ctx, json.RawMessage(`{}`))
	require.NoError(t, err)
	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &items))
	assert.Len(t, items, 1)
}

func TestHandleGetProjectItems_RequiresShortID(t *testing.T) {
	s, _, _, ctx := newToolsTestServer(t)
	_, err := s.handleGetProjectItems(ctx, json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project_short_id")
}

func TestHandleGetProjectItems_ProjectNotFound(t *testing.T) {
	s, _, _, ctx := newToolsTestServer(t)
	_, err := s.handleGetProjectItems(ctx, json.RawMessage(`{"project_short_id":"NOTEXIST"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHandleGetProjectItems_ReturnsProjectItems(t *testing.T) {
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	deps := Deps{Projects: projRepo, WorkItems: wiRepo}
	s := &Server{deps: deps}
	s.registerTools()
	ctx := context.Background()

	// Project A with 2 work items
	projA := testutil.NewTestProject("Math", testutil.WithShortID("MATH01"))
	require.NoError(t, projRepo.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Week 1")
	require.NoError(t, nodeRepo.Create(ctx, nodeA))
	wi1 := testutil.NewTestWorkItem(nodeA.ID, "Calculus", testutil.WithPlannedMin(60))
	wi2 := testutil.NewTestWorkItem(nodeA.ID, "Algebra", testutil.WithPlannedMin(45))
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))

	// Project B — should not appear in results
	projB := testutil.NewTestProject("Physics", testutil.WithShortID("PHY01"))
	require.NoError(t, projRepo.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Mechanics")
	require.NoError(t, nodeRepo.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Newton", testutil.WithPlannedMin(30))
	require.NoError(t, wiRepo.Create(ctx, wiB))

	result, err := s.handleGetProjectItems(ctx, json.RawMessage(`{"project_short_id":"MATH01"}`))
	require.NoError(t, err)

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &items))
	assert.Len(t, items, 2, "should return only items from the requested project")
	titles := []string{items[0]["title"].(string), items[1]["title"].(string)}
	assert.Contains(t, titles, "Calculus")
	assert.Contains(t, titles, "Algebra")
}

func TestHandleGetProjectItems_EmptyProject(t *testing.T) {
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	deps := Deps{Projects: projRepo, WorkItems: wiRepo}
	s := &Server{deps: deps}
	s.registerTools()
	ctx := context.Background()

	proj := testutil.NewTestProject("Empty", testutil.WithShortID("EMP01"))
	require.NoError(t, projRepo.Create(ctx, proj))

	result, err := s.handleGetProjectItems(ctx, json.RawMessage(`{"project_short_id":"EMP01"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "No active work items")
}

// Ensure uuid is used (imported for direct domain.WorkItem construction).
var _ = uuid.New
