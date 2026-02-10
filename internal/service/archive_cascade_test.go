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

// TestArchiveProject_ExcludesFromScheduling verifies that archiving a project
// causes all its work items to be excluded from ListSchedulable() and
// WhatNow recommendations.
func TestArchiveProject_ExcludesFromScheduling(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	// Create a project with work items.
	proj := testutil.NewTestProject("Archivable", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodes.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Task A",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	wi2 := testutil.NewTestWorkItem(node.ID, "Task B",
		testutil.WithPlannedMin(45), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wi1))
	require.NoError(t, workItems.Create(ctx, wi2))

	// Verify items are schedulable before archiving.
	candidates, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	assert.Len(t, candidates, 2, "both items should be schedulable before archive")

	// Archive the project.
	require.NoError(t, projects.Archive(ctx, proj.ID))

	// Items should no longer be schedulable.
	candidates, err = workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	for _, c := range candidates {
		assert.NotEqual(t, proj.ID, c.ProjectID,
			"archived project's items should not appear in schedulable candidates")
	}

	// WhatNow should not recommend archived project items.
	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(120)
	req.Now = &now

	// Create a second active project so WhatNow has something to return.
	proj2 := testutil.NewTestProject("Active", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj2))
	node2 := testutil.NewTestNode(proj2.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node2))
	wi3 := testutil.NewTestWorkItem(node2.ID, "Active Task",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wi3))

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	for _, rec := range resp.Recommendations {
		assert.NotEqual(t, proj.ID, rec.ProjectID,
			"archived project should not appear in recommendations")
	}
}

// TestArchiveProject_ExcludesFromStatus verifies that StatusService omits
// archived projects from the status summary.
func TestArchiveProject_ExcludesFromStatus(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	// Create two projects.
	proj1 := testutil.NewTestProject("Visible", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj1))
	node1 := testutil.NewTestNode(proj1.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node1))
	wi1 := testutil.NewTestWorkItem(node1.ID, "Task",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wi1))

	proj2 := testutil.NewTestProject("ToArchive", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj2))
	node2 := testutil.NewTestNode(proj2.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node2))
	wi2 := testutil.NewTestWorkItem(node2.ID, "Task",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wi2))

	statusSvc := NewStatusService(projects, workItems, sessions, profiles)

	// Both projects should appear before archiving.
	req := contract.NewStatusRequest()
	req.Now = &now
	resp, err := statusSvc.GetStatus(ctx, req)
	require.NoError(t, err)
	assert.Len(t, resp.Projects, 2)

	// Archive one project.
	require.NoError(t, projects.Archive(ctx, proj2.ID))

	resp2, err := statusSvc.GetStatus(ctx, req)
	require.NoError(t, err)
	assert.Len(t, resp2.Projects, 1, "archived project should be excluded from status")
	assert.Equal(t, "Visible", resp2.Projects[0].ProjectName)
}

// TestArchiveProject_UnarchiveRestoresScheduling verifies that unarchiving
// a project causes its items to reappear in scheduling.
func TestArchiveProject_UnarchiveRestoresScheduling(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	proj := testutil.NewTestProject("Restorable", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Revived Task",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wi))

	// Archive.
	require.NoError(t, projects.Archive(ctx, proj.ID))

	candidates, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, candidates, "no candidates while archived")

	// Unarchive.
	require.NoError(t, projects.Unarchive(ctx, proj.ID))

	candidates, err = workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	assert.Len(t, candidates, 1, "item should reappear after unarchive")

	// Verify WhatNow works.
	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now
	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations)
	assert.Equal(t, "Revived Task", resp.Recommendations[0].Title)
}

// TestArchiveWorkItem_ExcludesFromSchedulingOnly verifies that archiving a
// single work item excludes only that item, while sibling items remain schedulable.
func TestArchiveWorkItem_ExcludesFromSchedulingOnly(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)

	proj := testutil.NewTestProject("Partial Archive", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node))

	wiActive := testutil.NewTestWorkItem(node.ID, "Surviving Task",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	wiArchived := testutil.NewTestWorkItem(node.ID, "Archived Task",
		testutil.WithPlannedMin(45), testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiActive))
	require.NoError(t, workItems.Create(ctx, wiArchived))

	// Archive one work item.
	require.NoError(t, workItems.Archive(ctx, wiArchived.ID))

	// Only the active item should be schedulable.
	candidates, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	assert.Len(t, candidates, 1)
	assert.Equal(t, wiActive.ID, candidates[0].WorkItem.ID)

	// WhatNow should only recommend the surviving item.
	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now
	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	for _, rec := range resp.Recommendations {
		assert.NotEqual(t, wiArchived.ID, rec.WorkItemID,
			"archived work item should not be recommended")
	}
}
