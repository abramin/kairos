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

// TestE2E_WhatNow_AllBlockerStates verifies that all implemented blocker states
// are correctly detected and reported end-to-end. This ensures users get accurate
// error messages explaining why items can't be recommended.
//
// Covers 5 implemented blocker codes (out of 6 defined in contract):
// 1. BlockerNotBefore - not_before date not yet reached
// 2. BlockerDependency - dependency not completed
// 3. BlockerNotInCriticalScope - critical mode excludes non-critical items
// 4. BlockerSessionMinExceedsAvail - min_session_min > available time
// 5. BlockerWorkComplete - logged >= planned (work complete)
//
// Note: BlockerStatusDone is defined but not implemented - items with status='done'
// are filtered at the SQL level (ListSchedulable WHERE status IN ('todo','in_progress'))
// so they never generate blocker messages.
func TestE2E_WhatNow_AllBlockerStates(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("BlockerNotBefore - not_before constraint not satisfied", func(t *testing.T) {
		// Create project + node + work item with not_before in the future
		tomorrow := now.AddDate(0, 0, 1)
		proj := testutil.NewTestProject("NotBefore Project",
			testutil.WithTargetDate(now.AddDate(0, 1, 0)))
		require.NoError(t, projects.Create(ctx, proj))

		node := testutil.NewTestNode(proj.ID, "Module 1",
			testutil.WithNodeKind(domain.NodeModule))
		require.NoError(t, nodes.Create(ctx, node))

		item := testutil.NewTestWorkItem(node.ID, "Future Task",
			testutil.WithPlannedMin(60),
			testutil.WithSessionBounds(15, 60, 30),
			testutil.WithNotBefore(tomorrow))
		require.NoError(t, workItems.Create(ctx, item))

		// Request what-now
		svc := NewWhatNowService(workItems, sessions, deps, profiles)
		req := contract.NewWhatNowRequest(60)
		req.Now = &now

		resp, err := svc.Recommend(ctx, req)
		require.NoError(t, err)

		// Verify blocker present
		foundBlocker := false
		for _, blocker := range resp.Blockers {
			if blocker.EntityID == item.ID && blocker.Code == contract.BlockerNotBefore {
				foundBlocker = true
				assert.Contains(t, blocker.Message, "before",
					"Blocker message should mention not available before date")
				break
			}
		}
		assert.True(t, foundBlocker, "BlockerNotBefore not found for item with future not_before date")

		// Verify item not in recommendations
		for _, rec := range resp.Recommendations {
			assert.NotEqual(t, item.ID, rec.WorkItemID,
				"Blocked item should not appear in recommendations")
		}
	})

	t.Run("BlockerDependency - dependency not completed", func(t *testing.T) {
		// Create project + node + two work items with dependency
		proj := testutil.NewTestProject("Dependency Project",
			testutil.WithTargetDate(now.AddDate(0, 1, 0)))
		require.NoError(t, projects.Create(ctx, proj))

		node := testutil.NewTestNode(proj.ID, "Module 1",
			testutil.WithNodeKind(domain.NodeModule))
		require.NoError(t, nodes.Create(ctx, node))

		predecessor := testutil.NewTestWorkItem(node.ID, "Prerequisite",
			testutil.WithPlannedMin(60),
			testutil.WithSessionBounds(15, 60, 30))
		require.NoError(t, workItems.Create(ctx, predecessor))

		successor := testutil.NewTestWorkItem(node.ID, "Dependent Task",
			testutil.WithPlannedMin(60),
			testutil.WithSessionBounds(15, 60, 30))
		require.NoError(t, workItems.Create(ctx, successor))

		// Create dependency: successor depends on predecessor
		dep := &domain.Dependency{
			PredecessorWorkItemID: predecessor.ID,
			SuccessorWorkItemID:   successor.ID,
		}
		require.NoError(t, deps.Create(ctx, dep))

		// Request what-now (predecessor not done yet)
		svc := NewWhatNowService(workItems, sessions, deps, profiles)
		req := contract.NewWhatNowRequest(120)
		req.Now = &now

		resp, err := svc.Recommend(ctx, req)
		require.NoError(t, err)

		// Verify blocker present for successor
		foundBlocker := false
		for _, blocker := range resp.Blockers {
			if blocker.EntityID == successor.ID && blocker.Code == contract.BlockerDependency {
				foundBlocker = true
				assert.Contains(t, blocker.Message, "predecessors",
					"Blocker message should mention unfinished predecessors")
				break
			}
		}
		assert.True(t, foundBlocker, "BlockerDependency not found for item with incomplete dependency")

		// Verify predecessor CAN be recommended, but successor cannot
		predecessorRecommended := false
		for _, rec := range resp.Recommendations {
			if rec.WorkItemID == predecessor.ID {
				predecessorRecommended = true
			}
			assert.NotEqual(t, successor.ID, rec.WorkItemID,
				"Successor should not be recommended while dependency incomplete")
		}
		assert.True(t, predecessorRecommended,
			"Predecessor should be recommended (no blocker)")
	})

	t.Run("BlockerNotInCriticalScope - critical mode excludes non-critical items", func(t *testing.T) {
		// Create TWO projects:
		// 1. Critical project (due tomorrow, lots of work)
		// 2. Non-critical project (due in 3 months, little work)
		tomorrow := now.AddDate(0, 0, 1)
		criticalProj := testutil.NewTestProject("Critical Deadline",
			testutil.WithTargetDate(tomorrow))
		require.NoError(t, projects.Create(ctx, criticalProj))

		farFuture := now.AddDate(0, 3, 0)
		normalProj := testutil.NewTestProject("Relaxed Project",
			testutil.WithTargetDate(farFuture))
		require.NoError(t, projects.Create(ctx, normalProj))

		// Critical project: 240 min of work (will trigger critical mode)
		criticalNode := testutil.NewTestNode(criticalProj.ID, "Urgent Module",
			testutil.WithNodeKind(domain.NodeModule))
		require.NoError(t, nodes.Create(ctx, criticalNode))

		criticalItem := testutil.NewTestWorkItem(criticalNode.ID, "Urgent Task",
			testutil.WithPlannedMin(240),
			testutil.WithSessionBounds(15, 90, 45))
		require.NoError(t, workItems.Create(ctx, criticalItem))

		// Normal project: 60 min of work
		normalNode := testutil.NewTestNode(normalProj.ID, "Normal Module",
			testutil.WithNodeKind(domain.NodeModule))
		require.NoError(t, nodes.Create(ctx, normalNode))

		normalItem := testutil.NewTestWorkItem(normalNode.ID, "Normal Task",
			testutil.WithPlannedMin(60),
			testutil.WithSessionBounds(15, 60, 30))
		require.NoError(t, workItems.Create(ctx, normalItem))

		// Request what-now (should enter critical mode)
		svc := NewWhatNowService(workItems, sessions, deps, profiles)
		req := contract.NewWhatNowRequest(120)
		req.Now = &now

		resp, err := svc.Recommend(ctx, req)
		require.NoError(t, err)

		// Verify we're in critical mode
		assert.Equal(t, domain.ModeCritical, resp.Mode,
			"Should enter critical mode with tomorrow's deadline + heavy work")

		// Verify blocker present for normal project item
		foundBlocker := false
		for _, blocker := range resp.Blockers {
			if blocker.EntityID == normalItem.ID && blocker.Code == contract.BlockerNotInCriticalScope {
				foundBlocker = true
				assert.Contains(t, blocker.Message, "critical",
					"Blocker message should mention critical mode")
				break
			}
		}
		assert.True(t, foundBlocker,
			"BlockerNotInCriticalScope not found for non-critical item in critical mode")

		// Verify only critical project items recommended
		for _, rec := range resp.Recommendations {
			assert.Equal(t, criticalProj.ID, rec.ProjectID,
				"In critical mode, only critical project items should be recommended")
		}
	})

	t.Run("BlockerSessionMinExceedsAvail - min_session_min exceeds available time", func(t *testing.T) {
		// Create fresh repos to avoid interference from previous subtests
		freshProjects, freshNodes, freshWorkItems, freshDeps, freshSessions, freshProfiles, _ := setupRepos(t)
		freshCtx := context.Background()

		// Create project with normal deadline (far in future to avoid critical mode)
		farFuture := now.AddDate(0, 6, 0)
		proj := testutil.NewTestProject("Session Min Project",
			testutil.WithTargetDate(farFuture))
		require.NoError(t, freshProjects.Create(freshCtx, proj))

		node := testutil.NewTestNode(proj.ID, "Module 1",
			testutil.WithNodeKind(domain.NodeModule))
		require.NoError(t, freshNodes.Create(freshCtx, node))

		// Create ONLY this one item - min_session_min=60
		item := testutil.NewTestWorkItem(node.ID, "Long Session Task",
			testutil.WithPlannedMin(120),
			testutil.WithSessionBounds(60, 90, 75)) // min=60
		require.NoError(t, freshWorkItems.Create(freshCtx, item))

		// Request what-now with only 30 min available (less than min_session_min=60)
		svc := NewWhatNowService(freshWorkItems, freshSessions, freshDeps, freshProfiles)
		req := contract.NewWhatNowRequest(30)
		req.Now = &now

		resp, err := svc.Recommend(freshCtx, req)
		require.NoError(t, err)

		// Allocator will try to allocate but fail due to min_session_min > available time
		foundBlocker := false
		for _, blocker := range resp.Blockers {
			if blocker.EntityID == item.ID && blocker.Code == contract.BlockerSessionMinExceedsAvail {
				foundBlocker = true
				assert.Contains(t, blocker.Message, "time",
					"Blocker message should mention insufficient time")
				break
			}
		}
		assert.True(t, foundBlocker,
			"BlockerSessionMinExceedsAvail not found when available < min_session_min")

		// Verify item not in recommendations (couldn't allocate it)
		for _, rec := range resp.Recommendations {
			assert.NotEqual(t, item.ID, rec.WorkItemID,
				"Item requiring 60min session should not be recommended when only 30min available")
		}
	})

	t.Run("BlockerWorkComplete - logged >= planned (work complete)", func(t *testing.T) {
		// Create project + node + work item
		proj := testutil.NewTestProject("Complete Project",
			testutil.WithTargetDate(now.AddDate(0, 1, 0)))
		require.NoError(t, projects.Create(ctx, proj))

		node := testutil.NewTestNode(proj.ID, "Module 1",
			testutil.WithNodeKind(domain.NodeModule))
		require.NoError(t, nodes.Create(ctx, node))

		item := testutil.NewTestWorkItem(node.ID, "Full Task",
			testutil.WithPlannedMin(60),
			testutil.WithLoggedMin(60), // logged = planned (complete)
			testutil.WithSessionBounds(15, 60, 30))
		require.NoError(t, workItems.Create(ctx, item))

		// Request what-now
		svc := NewWhatNowService(workItems, sessions, deps, profiles)
		req := contract.NewWhatNowRequest(60)
		req.Now = &now

		resp, err := svc.Recommend(ctx, req)
		require.NoError(t, err)

		// Verify blocker present
		foundBlocker := false
		for _, blocker := range resp.Blockers {
			if blocker.EntityID == item.ID && blocker.Code == contract.BlockerWorkComplete {
				foundBlocker = true
				assert.Contains(t, blocker.Message, "logged",
					"Blocker message should mention fully logged")
				break
			}
		}
		assert.True(t, foundBlocker,
			"BlockerWorkComplete not found for item with logged >= planned")

		// Verify item not in recommendations
		for _, rec := range resp.Recommendations {
			assert.NotEqual(t, item.ID, rec.WorkItemID,
				"Complete item (logged >= planned) should not appear in recommendations")
		}
	})
}
