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

// TestDependencyBlocked_ChainABC verifies that in a chain A→B→C:
// - Only A is schedulable initially
// - B becomes schedulable after A is done
// - C remains blocked until B is done
// Tests through the service layer (not just repo).
func TestDependencyBlocked_ChainABC(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	// Create project with three chained work items.
	proj := testutil.NewTestProject("Dependency Chain", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	wiA := testutil.NewTestWorkItem(node.ID, "Foundation",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	wiB := testutil.NewTestWorkItem(node.ID, "Intermediate",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	wiC := testutil.NewTestWorkItem(node.ID, "Advanced",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))
	require.NoError(t, workItems.Create(ctx, wiB))
	require.NoError(t, workItems.Create(ctx, wiC))

	// Create dependency chain: A→B→C
	require.NoError(t, deps.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: wiA.ID,
		SuccessorWorkItemID:   wiB.ID,
	}))
	require.NoError(t, deps.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: wiB.ID,
		SuccessorWorkItemID:   wiC.ID,
	}))

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(120)
	req.Now = &now
	req.ProjectScope = []string{proj.ID}

	// === Phase 1: Only A should be recommended ===
	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	recommendedTitles := extractTitles(resp1.Recommendations)
	assert.Contains(t, recommendedTitles, "Foundation", "A should be recommended (no predecessors)")
	assert.NotContains(t, recommendedTitles, "Intermediate", "B should be blocked (A not done)")
	assert.NotContains(t, recommendedTitles, "Advanced", "C should be blocked (B not done)")

	// Verify B and C appear as dependency-blocked.
	depBlockedCount := 0
	for _, b := range resp1.Blockers {
		if b.Code == contract.BlockerDependency {
			depBlockedCount++
		}
	}
	assert.Equal(t, 2, depBlockedCount, "B and C should both be dependency-blocked")

	// === Phase 2: Complete A, B should become available ===
	wiA.Status = domain.WorkItemDone
	wiA.LoggedMin = 60
	require.NoError(t, workItems.Update(ctx, wiA))

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	recommendedTitles2 := extractTitles(resp2.Recommendations)
	assert.Contains(t, recommendedTitles2, "Intermediate", "B should be recommended after A is done")
	assert.NotContains(t, recommendedTitles2, "Advanced", "C should still be blocked (B not done)")

	// Only C should be dependency-blocked now.
	depBlockedCount2 := 0
	for _, b := range resp2.Blockers {
		if b.Code == contract.BlockerDependency {
			depBlockedCount2++
		}
	}
	assert.Equal(t, 1, depBlockedCount2, "only C should be dependency-blocked")

	// === Phase 3: Complete B, C should become available ===
	wiB.Status = domain.WorkItemDone
	wiB.LoggedMin = 60
	require.NoError(t, workItems.Update(ctx, wiB))

	resp3, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	recommendedTitles3 := extractTitles(resp3.Recommendations)
	assert.Contains(t, recommendedTitles3, "Advanced", "C should be recommended after B is done")

	// No dependency blockers should remain.
	for _, b := range resp3.Blockers {
		assert.NotEqual(t, contract.BlockerDependency, b.Code,
			"no dependency blockers should remain after all predecessors are done")
	}
}

// TestDependencyBlocked_SkippedPredecessorUnblocks verifies that marking a predecessor
// as "skipped" also unblocks the successor (not just "done").
func TestDependencyBlocked_SkippedPredecessorUnblocks(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	proj := testutil.NewTestProject("Skip Unblock", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module")
	require.NoError(t, nodes.Create(ctx, node))

	wiA := testutil.NewTestWorkItem(node.ID, "Prerequisite",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	wiB := testutil.NewTestWorkItem(node.ID, "Dependent",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))
	require.NoError(t, workItems.Create(ctx, wiB))

	require.NoError(t, deps.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: wiA.ID,
		SuccessorWorkItemID:   wiB.ID,
	}))

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(120)
	req.Now = &now
	req.ProjectScope = []string{proj.ID}

	// Initially B is blocked.
	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	titles1 := extractTitles(resp1.Recommendations)
	assert.NotContains(t, titles1, "Dependent", "B should be blocked initially")

	// Skip A (not done, but skipped).
	wiA.Status = domain.WorkItemSkipped
	require.NoError(t, workItems.Update(ctx, wiA))

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	titles2 := extractTitles(resp2.Recommendations)
	assert.Contains(t, titles2, "Dependent", "B should be unblocked when predecessor is skipped")
}

// TestDependencyBlocked_DiamondDependency verifies a diamond dependency pattern:
// A → B, A → C, B → D, C → D. D requires both B and C to complete.
func TestDependencyBlocked_DiamondDependency(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 6, 0)

	proj := testutil.NewTestProject("Diamond Deps", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module")
	require.NoError(t, nodes.Create(ctx, node))

	makeWI := func(title string) *domain.WorkItem {
		wi := testutil.NewTestWorkItem(node.ID, title,
			testutil.WithPlannedMin(60),
			testutil.WithSessionBounds(15, 60, 30),
		)
		require.NoError(t, workItems.Create(ctx, wi))
		return wi
	}

	wiA := makeWI("A: Foundation")
	wiB := makeWI("B: Track 1")
	wiC := makeWI("C: Track 2")
	wiD := makeWI("D: Synthesis")

	// Diamond: A→B, A→C, B→D, C→D
	for _, dep := range []struct{ pred, succ string }{
		{wiA.ID, wiB.ID},
		{wiA.ID, wiC.ID},
		{wiB.ID, wiD.ID},
		{wiC.ID, wiD.ID},
	} {
		require.NoError(t, deps.Create(ctx, &domain.Dependency{
			PredecessorWorkItemID: dep.pred,
			SuccessorWorkItemID:   dep.succ,
		}))
	}

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(120)
	req.Now = &now
	req.ProjectScope = []string{proj.ID}

	// Phase 1: Only A is available.
	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	titles1 := extractTitles(resp1.Recommendations)
	assert.Contains(t, titles1, "A: Foundation")
	assert.NotContains(t, titles1, "D: Synthesis")

	// Complete A: B and C should both become available.
	wiA.Status = domain.WorkItemDone
	wiA.LoggedMin = 60
	require.NoError(t, workItems.Update(ctx, wiA))

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	titles2 := extractTitles(resp2.Recommendations)
	assert.Contains(t, titles2, "B: Track 1", "B should be available after A is done")
	assert.Contains(t, titles2, "C: Track 2", "C should be available after A is done")
	assert.NotContains(t, titles2, "D: Synthesis", "D requires both B and C")

	// Complete B only: D should still be blocked (C is unfinished).
	wiB.Status = domain.WorkItemDone
	wiB.LoggedMin = 60
	require.NoError(t, workItems.Update(ctx, wiB))

	resp3, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	titles3 := extractTitles(resp3.Recommendations)
	assert.NotContains(t, titles3, "D: Synthesis", "D should still be blocked (C not done)")

	// Complete C: D should now be available.
	wiC.Status = domain.WorkItemDone
	wiC.LoggedMin = 60
	require.NoError(t, workItems.Update(ctx, wiC))

	resp4, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	titles4 := extractTitles(resp4.Recommendations)
	assert.Contains(t, titles4, "D: Synthesis", "D should be available after both B and C are done")
}

func extractTitles(recs []contract.WorkSlice) []string {
	titles := make([]string, len(recs))
	for i, r := range recs {
		titles[i] = r.Title
	}
	return titles
}
