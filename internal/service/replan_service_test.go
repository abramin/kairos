package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplan_SmoothReEstimation_UpdatesDB(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Study", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node))

	// Work item with units tracking: planned 100 min for 10 chapters,
	// but logged 60 min for only 3 chapters → pace implies ~200 min total
	// Smooth: round(0.7*100 + 0.3*200) = round(70+60) = 130
	wi := testutil.NewTestWorkItem(node.ID, "Read Chapters",
		testutil.WithPlannedMin(100),
		testutil.WithLoggedMin(60),
		testutil.WithUnits("chapters", 10, 3),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	// Log a recent session so project has activity
	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewReplanService(projects, workItems, sessions, profiles)
	req := contract.NewReplanRequest(domain.TriggerManual)
	req.Now = &now

	resp, err := svc.Replan(ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.Deltas, 1)
	assert.Equal(t, 1, resp.Deltas[0].ChangedItemsCount, "should have re-estimated 1 work item")

	// Verify the DB was actually updated
	updated, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.NotEqual(t, 100, updated.PlannedMin, "planned min should have been updated by smoothing")
	assert.Equal(t, 130, updated.PlannedMin, "should be round(0.7*100 + 0.3*200)")
}

func TestReplan_Converges_WithRepeatedRuns(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Study", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node))

	// Pace implies 200 min total (60 logged / 3 done * 10 total)
	// Re-estimation smooths: 0.7*current + 0.3*200
	// Eventually converges toward implied total
	wi := testutil.NewTestWorkItem(node.ID, "Read",
		testutil.WithPlannedMin(100),
		testutil.WithLoggedMin(60),
		testutil.WithUnits("chapters", 10, 3),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewReplanService(projects, workItems, sessions, profiles)
	req := contract.NewReplanRequest(domain.TriggerManual)
	req.Now = &now

	// Run replan multiple times — should eventually converge (changes become 0)
	var lastPlanned int
	for i := 0; i < 20; i++ {
		_, err := svc.Replan(ctx, req)
		require.NoError(t, err)

		updated, err := workItems.GetByID(ctx, wi.ID)
		require.NoError(t, err)

		if updated.PlannedMin == lastPlanned {
			// Converged — no further changes
			return
		}
		lastPlanned = updated.PlannedMin
	}

	// After 20 iterations, should have converged to the implied total (200 min)
	final, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 200, final.PlannedMin, "should converge to implied total")
}

func TestReplan_NoActiveProjects_ReturnsError(t *testing.T) {
	projects, _, _, _, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Create and immediately archive a project
	proj := testutil.NewTestProject("Abandoned")
	require.NoError(t, projects.Create(ctx, proj))
	require.NoError(t, projects.Archive(ctx, proj.ID))

	svc := NewReplanService(projects, nil, sessions, profiles)
	req := contract.NewReplanRequest(domain.TriggerManual)
	req.Now = &now

	_, err := svc.Replan(ctx, req)
	require.Error(t, err)

	var replanErr *contract.ReplanError
	require.ErrorAs(t, err, &replanErr)
	assert.Equal(t, contract.ReplanErrNoActiveProjects, replanErr.Code)
}

func TestReplan_RiskDeltaCalculated(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 1, 0)

	proj := testutil.NewTestProject("Project", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(200),
		testutil.WithLoggedMin(50),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewReplanService(projects, workItems, sessions, profiles)
	req := contract.NewReplanRequest(domain.TriggerManual)
	req.Now = &now

	resp, err := svc.Replan(ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.Deltas, 1)
	delta := resp.Deltas[0]
	assert.Equal(t, proj.ID, delta.ProjectID)
	assert.Equal(t, proj.Name, delta.ProjectName)
	// Risk levels should be populated (specific level depends on parameters)
	assert.NotEmpty(t, string(delta.RiskBefore))
	assert.NotEmpty(t, string(delta.RiskAfter))
}

func TestReplan_Idempotency_UnchangedInputProducesZeroChanges(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	// Create project with work items that have NO unit tracking
	// (so no re-estimation occurs, and planned min stays constant)
	proj := testutil.NewTestProject("Stable Project", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node))

	// Work item without unit tracking: planned = 100, logged = 50
	// No units defined => no re-estimation => planned stays 100
	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(100),
		testutil.WithLoggedMin(50),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	// Add a session so project has recent activity
	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewReplanService(projects, workItems, sessions, profiles)
	req := contract.NewReplanRequest(domain.TriggerManual)
	req.Now = &now

	// First replan
	resp1, err := svc.Replan(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp1.Deltas, 1)

	// Capture planned min and risk levels after first replan
	wi1, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	plannedAfterFirstReplan := wi1.PlannedMin
	riskAfterFirstReplan := resp1.Deltas[0].RiskAfter

	// Second replan with identical input (same timestamp, no DB changes between calls)
	resp2, err := svc.Replan(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp2.Deltas, 1)

	// Verify second replan produces zero changes
	assert.Equal(t, 0, resp2.Deltas[0].ChangedItemsCount,
		"second replan with unchanged input should report zero changed items")

	// Verify planned min unchanged after second replan
	wi2, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, plannedAfterFirstReplan, wi2.PlannedMin,
		"planned min should not change between replans")

	// Verify risk level unchanged
	assert.Equal(t, riskAfterFirstReplan, resp2.Deltas[0].RiskAfter,
		"risk level should remain consistent")
}

func TestReplan_Idempotency_MultipleCallsConvergeThenStabilize(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Study", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node))

	// Work item with unit tracking that converges after a few iterations
	wi := testutil.NewTestWorkItem(node.ID, "Read",
		testutil.WithPlannedMin(100),
		testutil.WithLoggedMin(60),
		testutil.WithUnits("chapters", 10, 3),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewReplanService(projects, workItems, sessions, profiles)
	req := contract.NewReplanRequest(domain.TriggerManual)
	req.Now = &now

	// Run replan until convergence (changes = 0)
	var convergedAfter int
	for i := 0; i < 30; i++ {
		resp, err := svc.Replan(ctx, req)
		require.NoError(t, err)
		require.Len(t, resp.Deltas, 1)

		if resp.Deltas[0].ChangedItemsCount == 0 {
			convergedAfter = i
			break
		}
	}

	require.Greater(t, convergedAfter, 0, "should converge within 30 iterations")

	// After convergence, subsequent replans should always report zero changes
	for i := 0; i < 5; i++ {
		resp, err := svc.Replan(ctx, req)
		require.NoError(t, err)
		require.Len(t, resp.Deltas, 1)
		assert.Equal(t, 0, resp.Deltas[0].ChangedItemsCount,
			"after convergence, all subsequent replans should report zero changes (iteration %d)", i)
	}
}
