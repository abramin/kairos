package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper to set up all repos from a test DB
func setupRepos(t *testing.T) (
	repository.ProjectRepo,
	repository.PlanNodeRepo,
	repository.WorkItemRepo,
	repository.DependencyRepo,
	repository.SessionRepo,
	repository.UserProfileRepo,
	db.UnitOfWork,
) {
	database := testutil.NewTestDB(t)
	return repository.NewSQLiteProjectRepo(database),
		repository.NewSQLitePlanNodeRepo(database),
		repository.NewSQLiteWorkItemRepo(database),
		repository.NewSQLiteDependencyRepo(database),
		repository.NewSQLiteSessionRepo(database),
		repository.NewSQLiteUserProfileRepo(database),
		testutil.NewTestUoW(database)
}

func TestWhatNow_CriticalDeadline_OnlyCriticalRecommended(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	tomorrow := now.AddDate(0, 0, 1)
	farFuture := now.AddDate(0, 3, 0)

	// Project A: critical - due tomorrow with lots of work remaining, no sessions
	projA := testutil.NewTestProject("Critical Project", testutil.WithTargetDate(tomorrow))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Node A", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Critical Task",
		testutil.WithPlannedMin(300),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))

	// Project B: on track - due far in future, almost complete (remaining ~0)
	projB := testutil.NewTestProject("Safe Project", testutil.WithTargetDate(farFuture))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Node B", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Safe Task",
		testutil.WithPlannedMin(60),
		testutil.WithLoggedMin(30),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiB))

	// Log recent sessions for Project B so it has recentDailyMin > 0
	sessB := testutil.NewTestSession(wiB.ID, 30,
		testutil.WithStartedAt(now.Add(-24*time.Hour)),
	)
	require.NoError(t, sessions.Create(ctx, sessB))

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, domain.ModeCritical, resp.Mode, "should be critical mode")
	for _, rec := range resp.Recommendations {
		assert.Equal(t, projA.ID, rec.ProjectID, "all recommendations should be from critical project")
	}
}

func TestWhatNow_Balanced_IncludesSecondaryProject(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	farFuture := now.AddDate(0, 6, 0)

	// Project A: on track, well ahead of schedule
	projA := testutil.NewTestProject("Primary", testutil.WithTargetDate(farFuture))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Node A")
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Primary Task",
		testutil.WithPlannedMin(100),
		testutil.WithLoggedMin(80),
		testutil.WithSessionBounds(15, 45, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))

	// Log recent sessions so recentDailyMin > 0 for Project A
	sessA := testutil.NewTestSession(wiA.ID, 30,
		testutil.WithStartedAt(now.Add(-24*time.Hour)),
	)
	require.NoError(t, sessions.Create(ctx, sessA))

	// Project B: secondary, also on track
	projB := testutil.NewTestProject("Secondary", testutil.WithTargetDate(farFuture))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Node B")
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Secondary Task",
		testutil.WithPlannedMin(100),
		testutil.WithLoggedMin(20),
		testutil.WithSessionBounds(15, 45, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiB))

	// Log recent sessions so recentDailyMin > 0 for Project B
	sessB := testutil.NewTestSession(wiB.ID, 30,
		testutil.WithStartedAt(now.Add(-48*time.Hour)),
	)
	require.NoError(t, sessions.Create(ctx, sessB))

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(90)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, domain.ModeBalanced, resp.Mode, "should be balanced mode")
	projectIDs := make(map[string]bool)
	for _, rec := range resp.Recommendations {
		projectIDs[rec.ProjectID] = true
	}
	assert.True(t, len(projectIDs) >= 2, "balanced mode should include multiple projects")
}

func TestWhatNow_ArchivedItemsExcluded(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	proj := testutil.NewTestProject("Test Project")
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	// Active work item
	wiActive := testutil.NewTestWorkItem(node.ID, "Active Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiActive))

	// Archived work item
	wiArchived := testutil.NewTestWorkItem(node.ID, "Archived Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiArchived))
	require.NoError(t, workItems.Archive(ctx, wiArchived.ID))

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	for _, rec := range resp.Recommendations {
		assert.NotEqual(t, wiArchived.ID, rec.WorkItemID, "archived item should not appear")
	}
}

func TestWhatNow_DeterministicOutput(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Test", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	for i := 0; i < 5; i++ {
		wi := testutil.NewTestWorkItem(node.ID, "Task",
			testutil.WithPlannedMin(60),
			testutil.WithSessionBounds(15, 60, 30),
		)
		require.NoError(t, workItems.Create(ctx, wi))
	}

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(90)
	req.Now = &now

	resp1, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	resp2, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	// Both runs should produce same recommendations
	require.Equal(t, len(resp1.Recommendations), len(resp2.Recommendations))
	for i := range resp1.Recommendations {
		assert.Equal(t, resp1.Recommendations[i].WorkItemID, resp2.Recommendations[i].WorkItemID,
			"recommendation %d should be deterministic", i)
		assert.Equal(t, resp1.Recommendations[i].AllocatedMin, resp2.Recommendations[i].AllocatedMin)
	}
}

func TestWhatNow_BaselineFloor_PreventsSpuriousCritical(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	fiveDaysOut := now.AddDate(0, 0, 5)

	// Project with 70 min remaining, 5 days left = 14 min/day needed.
	// No recent sessions, so recentDailyMin = 0.
	// But baseline_daily_min = 30 (default), so effectiveDaily = 30.
	// remaining with buffer: 70 * 1.1 = 77, requiredDaily = 77/5 = 15.4
	// ratio = 15.4/30 = 0.51 < 1.0 => on_track (not critical).
	proj := testutil.NewTestProject("Easy Project", testutil.WithTargetDate(fiveDaysOut))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Easy Task",
		testutil.WithPlannedMin(270),
		testutil.WithLoggedMin(200),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))
	// No sessions logged — recentDailyMin = 0

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, domain.ModeBalanced, resp.Mode,
		"baseline floor should prevent trivially-achievable deadlines from being critical")
}

func TestWhatNow_BaselineZero_AllowsCritical(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	// Set baseline to 0 to disable the floor
	profile, err := profiles.Get(ctx)
	require.NoError(t, err)
	profile.BaselineDailyMin = 0
	require.NoError(t, profiles.Upsert(ctx, profile))

	now := time.Now().UTC()
	fiveDaysOut := now.AddDate(0, 0, 5)

	// Same scenario: 70 min remaining, 5 days. With baseline=0 and no sessions,
	// recentDailyMin=0 triggers the zero-activity critical path.
	proj := testutil.NewTestProject("Project", testutil.WithTargetDate(fiveDaysOut))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(270),
		testutil.WithLoggedMin(200),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, domain.ModeCritical, resp.Mode,
		"with baseline=0 and no sessions, zero-activity path should trigger critical")
}

func TestWhatNow_BackLoadedProject_NotFalseCritical(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0) // 2 months from now

	// Project with start date 5 months ago and target 2 months from now.
	// Simulates OU01: weekly readings (mostly done) + large future assessment.
	proj := testutil.NewTestProject("Course", testutil.WithTargetDate(target))
	proj.StartDate = now.AddDate(0, -5, 0)
	require.NoError(t, projects.Create(ctx, proj))

	weekNode := testutil.NewTestNode(proj.ID, "Readings", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodes.Create(ctx, weekNode))

	// 10 completed weekly items with past due dates
	for i := 0; i < 10; i++ {
		dueDate := now.AddDate(0, 0, -(10-i)*7)
		wi := testutil.NewTestWorkItem(weekNode.ID, "Week reading",
			testutil.WithPlannedMin(200),
			testutil.WithLoggedMin(200),
			testutil.WithWorkItemDueDate(dueDate),
			testutil.WithWorkItemStatus(domain.WorkItemDone),
		)
		require.NoError(t, workItems.Create(ctx, wi))
	}

	// 3 remaining weekly items with future due dates
	var futureItems []*domain.WorkItem
	for i := 0; i < 3; i++ {
		dueDate := now.AddDate(0, 0, (i+1)*7)
		wi := testutil.NewTestWorkItem(weekNode.ID, "Week reading",
			testutil.WithPlannedMin(200),
			testutil.WithSessionBounds(15, 60, 30),
			testutil.WithWorkItemDueDate(dueDate),
		)
		require.NoError(t, workItems.Create(ctx, wi))
		futureItems = append(futureItems, wi)
	}

	// Large assessment with future due date (back-loaded, correctly not started)
	assessNode := testutil.NewTestNode(proj.ID, "Assessment", testutil.WithNodeKind(domain.NodeAssessment))
	require.NoError(t, nodes.Create(ctx, assessNode))
	assessDue := now.AddDate(0, 1, 14)
	wiAssess := testutil.NewTestWorkItem(assessNode.ID, "Final Essay",
		testutil.WithPlannedMin(2000),
		testutil.WithSessionBounds(30, 120, 60),
		testutil.WithWorkItemDueDate(assessDue),
	)
	require.NoError(t, workItems.Create(ctx, wiAssess))

	// Log a recent session to avoid zero-activity path
	sess := testutil.NewTestSession(futureItems[0].ID, 30,
		testutil.WithStartedAt(now.Add(-24*time.Hour)),
	)
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	// Without due-date-aware on-pace: aggregate progress (43%) < timeline elapsed (71%)
	// => ratio > 1.5 + NOT on pace => CRITICAL.
	// With due-date-aware on-pace: all work due by now is done => on pace => capped at AT_RISK.
	assert.NotEqual(t, domain.ModeCritical, resp.Mode,
		"back-loaded project with all items on schedule should not be critical")
}

func TestWhatNow_DeadlineUpdate_ChangesRiskAndRanking(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	comfortableDeadline := now.AddDate(0, 6, 0) // 6 months away

	proj := testutil.NewTestProject("Test", testutil.WithTargetDate(comfortableDeadline))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(500),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	// Log recent sessions so project starts as balanced (not critical due to no activity)
	sess := testutil.NewTestSession(wi.ID, 60,
		testutil.WithStartedAt(now.Add(-24*time.Hour)),
	)
	require.NoError(t, sessions.Create(ctx, sess))

	// First request: should be balanced (comfortable deadline, has recent activity)
	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp1, err := svc.Recommend(ctx, req)
	require.NoError(t, err)
	mode1 := resp1.Mode

	// Update deadline to 2 days from now
	tightDeadline := now.AddDate(0, 0, 2)
	proj.TargetDate = &tightDeadline
	require.NoError(t, projects.Update(ctx, proj))

	resp2, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	// Mode should be different (was balanced, now critical or at_risk indicators)
	assert.NotEqual(t, mode1, resp2.Mode, "deadline change should affect mode")
}

// TestWhatNow_UserProfileWeightsAffectOrdering verifies that changing the
// UserProfile scoring weights actually changes recommendation ordering.
// This is the only personalization mechanism — zero coverage without this test.
func TestWhatNow_UserProfileWeightsAffectOrdering(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Both projects share the SAME deadline (90 days) so the canonical sort
	// falls through risk (both on-track) → due date (same) → score (varies by weights).
	deadline := now.AddDate(0, 3, 0)

	// Project A: more work remaining → higher deadline pressure score.
	projA := testutil.NewTestProject("Alpha", testutil.WithTargetDate(deadline))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Node A")
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Alpha Task",
		testutil.WithPlannedMin(500),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))

	// Project B: less work remaining → lower deadline pressure, but not worked recently → higher spacing.
	projB := testutil.NewTestProject("Beta", testutil.WithTargetDate(deadline))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Node B")
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Beta Task",
		testutil.WithPlannedMin(100),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiB))

	// Log recent sessions for both projects (avoid zero-activity critical path).
	// Project A worked 2 hours ago → low spacing bonus.
	// Project B worked 5 days ago → high spacing bonus.
	sessA := testutil.NewTestSession(wiA.ID, 30,
		testutil.WithStartedAt(now.Add(-2*time.Hour)),
	)
	require.NoError(t, sessions.Create(ctx, sessA))
	sessB := testutil.NewTestSession(wiB.ID, 30,
		testutil.WithStartedAt(now.Add(-5*24*time.Hour)),
	)
	require.NoError(t, sessions.Create(ctx, sessB))

	svc := NewWhatNowService(workItems, sessions, deps, profiles)

	// --- Weight set 1: Heavy deadline pressure, zero spacing/variation ---
	profile, err := profiles.Get(ctx)
	require.NoError(t, err)
	profile.WeightDeadlinePressure = 5.0
	profile.WeightBehindPace = 5.0
	profile.WeightSpacing = 0.0
	profile.WeightVariation = 0.0
	require.NoError(t, profiles.Upsert(ctx, profile))

	req := contract.NewWhatNowRequest(90)
	req.Now = &now

	resp1, err := svc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations)

	firstProjectID1 := resp1.Recommendations[0].ProjectID

	// --- Weight set 2: Zero deadline pressure, heavy spacing/variation ---
	profile.WeightDeadlinePressure = 0.0
	profile.WeightBehindPace = 0.0
	profile.WeightSpacing = 5.0
	profile.WeightVariation = 5.0
	require.NoError(t, profiles.Upsert(ctx, profile))

	resp2, err := svc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations)

	firstProjectID2 := resp2.Recommendations[0].ProjectID

	// The key assertion: changing weights should change which project ranks first.
	assert.NotEqual(t, firstProjectID1, firstProjectID2,
		"changing scoring weights should change recommendation ordering")
}
