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

// TestE2E_ArchiveProject_FullWorkflow verifies end-to-end project archival workflow:
// 1. Project-level archived_at timestamp is set
// 2. All child work items are excluded from scheduling (via JOIN filters)
// 3. Dependencies and session logs are preserved
// 4. Unarchive restores full functionality
//
// Note: Kairos uses behavioral archival (SQL filters), not DB-level cascade.
// Work items don't get archived_at set - they're excluded via `p.archived_at IS NOT NULL` JOIN.
func TestE2E_ArchiveProject_FullWorkflow(t *testing.T) {
	projects, nodes, workItems, deps, sessions, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	// Create project with nodes, work items, and session logs
	proj := testutil.NewTestProject("Cascade Test",
		testutil.WithTargetDate(target),
		testutil.WithShortID("CASC01"))
	require.NoError(t, projects.Create(ctx, proj))

	node1 := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	node2 := testutil.NewTestNode(proj.ID, "Week 2", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodes.Create(ctx, node1))
	require.NoError(t, nodes.Create(ctx, node2))

	wi1 := testutil.NewTestWorkItem(node1.ID, "Task A",
		testutil.WithPlannedMin(60), testutil.WithSessionBounds(15, 60, 30))
	wi2 := testutil.NewTestWorkItem(node1.ID, "Task B",
		testutil.WithPlannedMin(45), testutil.WithSessionBounds(15, 60, 30))
	wi3 := testutil.NewTestWorkItem(node2.ID, "Task C",
		testutil.WithPlannedMin(90), testutil.WithSessionBounds(15, 90, 45))
	require.NoError(t, workItems.Create(ctx, wi1))
	require.NoError(t, workItems.Create(ctx, wi2))
	require.NoError(t, workItems.Create(ctx, wi3))

	// Create session logs
	sessionSvc := NewSessionService(sessions, workItems)
	session1 := &domain.WorkSessionLog{
		WorkItemID: wi1.ID,
		StartedAt:  now.Add(-2 * time.Hour),
		Minutes:    30,
	}
	session2 := &domain.WorkSessionLog{
		WorkItemID: wi2.ID,
		StartedAt:  now.Add(-1 * time.Hour),
		Minutes:    25,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, session1))
	require.NoError(t, sessionSvc.LogSession(ctx, session2))

	// Create dependency (wi3 depends on wi1)
	dep := &domain.Dependency{
		PredecessorWorkItemID: wi1.ID,
		SuccessorWorkItemID:   wi3.ID,
	}
	require.NoError(t, deps.Create(ctx, dep))

	// === BEFORE ARCHIVE: Verify schedulability ===
	beforeSchedulable, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	beforeCount := 0
	for _, c := range beforeSchedulable {
		if c.ProjectID == proj.ID {
			beforeCount++
		}
	}
	assert.Equal(t, 3, beforeCount, "all 3 work items should be schedulable before archive")

	beforeProj, err := projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Nil(t, beforeProj.ArchivedAt, "project should not be archived initially")
	assert.Equal(t, domain.ProjectActive, beforeProj.Status)

	// === ARCHIVE PROJECT ===
	require.NoError(t, projects.Archive(ctx, proj.ID))

	// === AFTER ARCHIVE: Verify project-level state ===
	afterProj, err := projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.NotNil(t, afterProj.ArchivedAt, "project should have archived_at set")
	assert.WithinDuration(t, now, *afterProj.ArchivedAt, 5*time.Second,
		"archived_at should be close to current time")
	assert.Equal(t, domain.ProjectArchived, afterProj.Status)

	// === BEHAVIORAL EXCLUSION: Work items excluded from scheduling via JOIN filters ===
	// Note: Work items themselves don't have archived_at set - exclusion happens via:
	//   WHERE p.archived_at IS NULL (in ListSchedulable JOIN)
	schedulable, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	for _, candidate := range schedulable {
		assert.NotEqual(t, proj.ID, candidate.ProjectID,
			"archived project's items must not appear in schedulable candidates (filtered via JOIN)")
	}

	// Work items still retrievable via ListByProject (no filter on p.archived_at)
	allItems, err := workItems.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Len(t, allItems, 3, "work items should still be retrievable via direct query")

	// === Session logs and dependencies preserved (immutable history) ===
	allSessions1, err := sessions.ListByWorkItem(ctx, wi1.ID)
	require.NoError(t, err)
	assert.Len(t, allSessions1, 1, "session logs should be preserved")

	dependents, err := deps.ListPredecessors(ctx, wi3.ID)
	require.NoError(t, err)
	assert.Len(t, dependents, 1, "dependency should still exist")

	// === UNARCHIVE: Verify restoration ===
	require.NoError(t, projects.Unarchive(ctx, proj.ID))

	unarchivedProj, err := projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Nil(t, unarchivedProj.ArchivedAt, "project archived_at should be NULL after unarchive")
	assert.Equal(t, domain.ProjectActive, unarchivedProj.Status)

	// Items should be schedulable again
	schedulableAfter, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	foundItems := 0
	for _, candidate := range schedulableAfter {
		if candidate.ProjectID == proj.ID {
			foundItems++
		}
	}
	assert.Equal(t, 3, foundItems, "all 3 work items should be schedulable after unarchive")
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
