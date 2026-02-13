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

// TestWhatNow_AllItemsBlocked_SessionMinExceedsAvail verifies that when all
// work items require more session time than available, the response contains
// blockers and zero recommendations — not a panic or opaque error.
func TestWhatNow_AllItemsBlocked_SessionMinExceedsAvail(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	proj := testutil.NewTestProject("Blocked Project", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	// Work item requires minimum 45 minutes per session.
	wi := testutil.NewTestWorkItem(node.ID, "Long Task",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(45, 120, 60), // min=45
	)
	require.NoError(t, workItems.Create(ctx, wi))

	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)

	// Only 20 minutes available — less than min_session_min of 45.
	req := contract.NewWhatNowRequest(20)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.Empty(t, resp.Recommendations,
		"should produce zero recommendations when all items need more time than available")
	assert.NotEmpty(t, resp.Blockers,
		"should report blockers explaining why items couldn't be scheduled")
	assert.Equal(t, 20, resp.UnallocatedMin,
		"all available time should be unallocated")
}

// TestWhatNow_AllItemsDone_NoCandidates verifies that when every work item
// is completed, the service returns ErrNoCandidates.
func TestWhatNow_AllItemsDone_NoCandidates(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	proj := testutil.NewTestProject("Done Project", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Completed Task",
		testutil.WithPlannedMin(60),
		testutil.WithWorkItemStatus(domain.WorkItemDone),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	_, err := svc.Recommend(ctx, req)
	require.Error(t, err)

	var wnErr *contract.WhatNowError
	require.ErrorAs(t, err, &wnErr)
	assert.Equal(t, contract.ErrNoCandidates, wnErr.Code)
}

// TestWhatNow_MixedBlockedAndSchedulable verifies partial blocking: some items
// blocked by session bounds, others schedulable. The response should contain
// both recommendations and blockers.
func TestWhatNow_MixedBlockedAndSchedulable(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	proj := testutil.NewTestProject("Mixed Project", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	// This item needs at least 60 min — will be blocked with 30 available.
	wiBlocked := testutil.NewTestWorkItem(node.ID, "Long Session Task",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(60, 120, 90),
	)
	require.NoError(t, workItems.Create(ctx, wiBlocked))

	// This item fits in 30 min.
	wiFits := testutil.NewTestWorkItem(node.ID, "Quick Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 30, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiFits))

	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(30)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Recommendations,
		"should schedule the item that fits")
	assert.LessOrEqual(t, resp.AllocatedMin, 30)

	// The high-min-session item should NOT appear in recommendations.
	for _, rec := range resp.Recommendations {
		assert.NotEqual(t, wiBlocked.ID, rec.WorkItemID,
			"item requiring 60-min session should not be scheduled with 30 min available")
	}

	// The quick item should be the one recommended.
	found := false
	for _, rec := range resp.Recommendations {
		if rec.WorkItemID == wiFits.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "the quick task that fits should be recommended")
}

// TestWhatNow_DependencyBlocked verifies that items with unfinished
// predecessors are blocked and reported correctly.
func TestWhatNow_DependencyBlocked(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	proj := testutil.NewTestProject("Dep Project", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	wiPredecessor := testutil.NewTestWorkItem(node.ID, "Prerequisite",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiPredecessor))

	wiSuccessor := testutil.NewTestWorkItem(node.ID, "Dependent Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiSuccessor))

	// Create dependency: successor depends on predecessor.
	dep := &domain.Dependency{
		PredecessorWorkItemID: wiPredecessor.ID,
		SuccessorWorkItemID:   wiSuccessor.ID,
	}
	require.NoError(t, deps.Create(ctx, dep))

	svc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	// The successor should NOT appear in recommendations (predecessor not done).
	for _, rec := range resp.Recommendations {
		assert.NotEqual(t, wiSuccessor.ID, rec.WorkItemID,
			"item with unfinished predecessor should not be recommended")
	}

	// The predecessor SHOULD be recommended.
	found := false
	for _, rec := range resp.Recommendations {
		if rec.WorkItemID == wiPredecessor.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "prerequisite item should be recommended")
}
