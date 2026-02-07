package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
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
) {
	db := testutil.NewTestDB(t)
	return repository.NewSQLiteProjectRepo(db),
		repository.NewSQLitePlanNodeRepo(db),
		repository.NewSQLiteWorkItemRepo(db),
		repository.NewSQLiteDependencyRepo(db),
		repository.NewSQLiteSessionRepo(db),
		repository.NewSQLiteUserProfileRepo(db)
}

func TestWhatNow_CriticalDeadline_OnlyCriticalRecommended(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
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

	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
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
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
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

	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
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
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
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

	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	for _, rec := range resp.Recommendations {
		assert.NotEqual(t, wiArchived.ID, rec.WorkItemID, "archived item should not appear")
	}
}

func TestWhatNow_DeterministicOutput(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
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

	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
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

func TestWhatNow_DeadlineUpdate_ChangesRiskAndRanking(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
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
	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
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
