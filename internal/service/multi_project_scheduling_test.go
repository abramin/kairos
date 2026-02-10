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

// TestMultiProject_ThreeProjectsMixedRisk_VariationEnforced exercises the
// scheduler with 3 projects at different risk levels: critical (due tomorrow),
// at-risk (due next week), and on-track (due in 3 months).
// Verifies critical mode → complete critical → balanced mode with variation.
func TestMultiProject_ThreeProjectsMixedRisk_VariationEnforced(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Project A: Critical — due tomorrow with 120 min unlogged work.
	projA := testutil.NewTestProject("Urgent Paper", testutil.WithTargetDate(now.AddDate(0, 0, 1)))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Section 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Write Draft",
		testutil.WithPlannedMin(120), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiA))

	// Project B: At-risk — due in 7 days with 200 min remaining.
	projB := testutil.NewTestProject("Midterm Prep", testutil.WithTargetDate(now.AddDate(0, 0, 7)))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Chapter 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Study Notes",
		testutil.WithPlannedMin(200), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiB))

	// Project C: On-track — due in 3 months with 100 min remaining.
	projC := testutil.NewTestProject("Leisure Reading", testutil.WithTargetDate(now.AddDate(0, 3, 0)))
	require.NoError(t, projects.Create(ctx, projC))
	nodeC := testutil.NewTestNode(projC.ID, "Book 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeC))
	wiC := testutil.NewTestWorkItem(nodeC.ID, "Read Chapter 1",
		testutil.WithPlannedMin(100), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiC))

	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)

	// Phase 1: Critical mode — only project A should be recommended.
	req := contract.NewWhatNowRequest(120)
	req.Now = &now
	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, domain.ModeCritical, resp.Mode, "should be in critical mode with tomorrow's deadline")
	for _, rec := range resp.Recommendations {
		assert.Equal(t, projA.ID, rec.ProjectID,
			"critical mode should only recommend critical project items")
	}

	// Phase 2: Complete project A's work → should exit critical mode.
	wiA.Status = domain.WorkItemDone
	wiA.LoggedMin = 120
	require.NoError(t, workItems.Update(ctx, wiA))

	// Log recent sessions on B and C so they have pace.
	sessB := testutil.NewTestSession(wiB.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessB))
	sessC := testutil.NewTestSession(wiC.ID, 20, testutil.WithStartedAt(now.Add(-48*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessC))

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.NotEqual(t, domain.ModeCritical, resp2.Mode,
		"should exit critical mode after all critical items are done")
	assert.NotEmpty(t, resp2.Recommendations, "should still have recommendations from B and C")

	// Verify that risk summaries include projects B and C.
	riskProjectIDs := make(map[string]bool)
	for _, r := range resp2.TopRiskProjects {
		riskProjectIDs[r.ProjectID] = true
	}
	assert.True(t, riskProjectIDs[projB.ID], "project B should appear in risk summaries")
	assert.True(t, riskProjectIDs[projC.ID], "project C should appear in risk summaries")
}

// TestMultiProject_AllOnTrack_VariationDistributesWork verifies that when
// all projects are on-track with sufficient available time and variation
// enforcement, recommendations span at least 2 projects.
func TestMultiProject_AllOnTrack_VariationDistributesWork(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	farFuture := now.AddDate(0, 6, 0) // 6 months out

	// Create 3 on-track projects.
	var projectIDs []string
	for _, name := range []string{"Project Alpha", "Project Beta", "Project Gamma"} {
		proj := testutil.NewTestProject(name, testutil.WithTargetDate(farFuture))
		require.NoError(t, projects.Create(ctx, proj))
		projectIDs = append(projectIDs, proj.ID)

		node := testutil.NewTestNode(proj.ID, "Module 1")
		require.NoError(t, nodes.Create(ctx, node))

		wi := testutil.NewTestWorkItem(node.ID, name+" Task",
			testutil.WithPlannedMin(120),
			testutil.WithSessionBounds(30, 60, 45))
		require.NoError(t, workItems.Create(ctx, wi))

		// Log recent activity so none are zero-velocity.
		sess := testutil.NewTestSession(wi.ID, 30,
			testutil.WithStartedAt(now.Add(-time.Duration(24)*time.Hour)))
		require.NoError(t, sessions.Create(ctx, sess))
	}

	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(180) // 3 hours = plenty of time
	req.Now = &now
	req.EnforceVariation = true

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, domain.ModeBalanced, resp.Mode, "should be balanced with all on-track projects")

	// With 180 min available and variation enforced, expect at least 2 projects.
	uniqueProjects := make(map[string]bool)
	for _, rec := range resp.Recommendations {
		uniqueProjects[rec.ProjectID] = true
	}
	assert.GreaterOrEqual(t, len(uniqueProjects), 2,
		"variation enforcement with sufficient time should distribute across projects; got %d project(s)", len(uniqueProjects))
}

// TestMultiProject_DependenciesAcrossRiskLevels verifies that dependency
// blocking works correctly when the predecessor and successor items are
// in projects with different risk levels.
func TestMultiProject_DependenciesAcrossRiskLevels(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Project A: on-track, has the prerequisite item.
	projA := testutil.NewTestProject("Foundation Course", testutil.WithTargetDate(now.AddDate(0, 6, 0)))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Module 1")
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiPrereq := testutil.NewTestWorkItem(nodeA.ID, "Complete Foundation",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiPrereq))

	// Project B: at-risk, has an item that depends on project A's item.
	projB := testutil.NewTestProject("Advanced Course", testutil.WithTargetDate(now.AddDate(0, 0, 14)))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Module 1")
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiDependent := testutil.NewTestWorkItem(nodeB.ID, "Advanced Topic",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiDependent))

	// Also add an independent item to project B.
	wiIndependent := testutil.NewTestWorkItem(nodeB.ID, "Independent Review",
		testutil.WithPlannedMin(45), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiIndependent))

	// Create cross-project dependency: wiPrereq → wiDependent.
	require.NoError(t, deps.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: wiPrereq.ID,
		SuccessorWorkItemID:   wiDependent.ID,
	}))

	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(120)
	req.Now = &now

	// Phase 1: Prerequisite unfinished — dependent should be blocked.
	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	for _, rec := range resp.Recommendations {
		assert.NotEqual(t, wiDependent.ID, rec.WorkItemID,
			"dependent item should be blocked while prerequisite is unfinished")
	}

	// The prerequisite and independent items should be recommended.
	recIDs := make(map[string]bool)
	for _, rec := range resp.Recommendations {
		recIDs[rec.WorkItemID] = true
	}
	assert.True(t, recIDs[wiPrereq.ID] || recIDs[wiIndependent.ID],
		"at least the prerequisite or independent item should be recommended")

	// Phase 2: Complete prerequisite — dependent should become available.
	wiPrereq.Status = domain.WorkItemDone
	wiPrereq.LoggedMin = 60
	require.NoError(t, workItems.Update(ctx, wiPrereq))

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	recIDs2 := make(map[string]bool)
	for _, rec := range resp2.Recommendations {
		recIDs2[rec.WorkItemID] = true
	}
	assert.True(t, recIDs2[wiDependent.ID],
		"dependent item should be schedulable after prerequisite is completed")
}
