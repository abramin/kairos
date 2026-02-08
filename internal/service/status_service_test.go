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

func TestStatus_CriticalProjectDetected(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	tomorrow := now.AddDate(0, 0, 1)

	// Project with tight deadline and lots of remaining work, no recent sessions
	proj := testutil.NewTestProject("Urgent Essay", testutil.WithTargetDate(tomorrow))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Chapter 1")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Write Chapter",
		testutil.WithPlannedMin(500),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	svc := NewStatusService(projects, workItems, sessions, profiles)
	req := contract.NewStatusRequest()
	req.Now = &now

	resp, err := svc.GetStatus(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, domain.ModeCritical, resp.Summary.GlobalModeIfNow, "should detect critical mode")
	assert.Equal(t, 1, resp.Summary.CountsCritical, "should have 1 critical project")
	require.Len(t, resp.Projects, 1)
	assert.Equal(t, domain.RiskCritical, resp.Projects[0].RiskLevel)
	assert.False(t, resp.Projects[0].SafeForSecondaryWork, "critical project should not be safe for secondary work")
}

func TestStatus_AllOnTrack_SafeForSecondary(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	farFuture := now.AddDate(0, 6, 0)

	// Project well ahead of schedule
	proj := testutil.NewTestProject("Relaxed Project", testutil.WithTargetDate(farFuture))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Easy Task",
		testutil.WithPlannedMin(60),
		testutil.WithLoggedMin(50),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	// Log recent sessions to show pace
	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewStatusService(projects, workItems, sessions, profiles)
	req := contract.NewStatusRequest()
	req.Now = &now

	resp, err := svc.GetStatus(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, domain.ModeBalanced, resp.Summary.GlobalModeIfNow, "should be balanced mode")
	assert.Equal(t, 1, resp.Summary.CountsOnTrack)
	assert.Equal(t, 0, resp.Summary.CountsCritical)
	require.Len(t, resp.Projects, 1)
	assert.True(t, resp.Projects[0].SafeForSecondaryWork, "on-track project should be safe for secondary work")
}

func TestStatus_ArchivedProjectExcluded(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Active project
	active := testutil.NewTestProject("Active")
	require.NoError(t, projects.Create(ctx, active))
	nodeA := testutil.NewTestNode(active.ID, "Node A")
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Task A", testutil.WithPlannedMin(60))
	require.NoError(t, workItems.Create(ctx, wiA))

	// Archived project
	archived := testutil.NewTestProject("Archived")
	require.NoError(t, projects.Create(ctx, archived))
	require.NoError(t, projects.Archive(ctx, archived.ID))

	svc := NewStatusService(projects, workItems, sessions, profiles)
	req := contract.NewStatusRequest()
	req.Now = &now
	req.IncludeArchived = false

	resp, err := svc.GetStatus(ctx, req)
	require.NoError(t, err)

	for _, view := range resp.Projects {
		assert.NotEqual(t, archived.ID, view.ProjectID, "archived project should not appear")
	}
}

func TestStatus_ProgressPctCanExceed100(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	farFuture := now.AddDate(0, 6, 0)

	proj := testutil.NewTestProject("Overworked", testutil.WithTargetDate(farFuture))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	// Logged more than planned
	wi := testutil.NewTestWorkItem(node.ID, "Over-logged Task",
		testutil.WithPlannedMin(60),
		testutil.WithLoggedMin(120),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	// Recent session
	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewStatusService(projects, workItems, sessions, profiles)
	req := contract.NewStatusRequest()
	req.Now = &now

	resp, err := svc.GetStatus(ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.Projects, 1)
	assert.Greater(t, resp.Projects[0].ProgressTimePct, 100.0, "progress should exceed 100%% when logged > planned")
}

func TestStatus_SortingOrder_CriticalBeforeOnTrack(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	tomorrow := now.AddDate(0, 0, 1)
	farFuture := now.AddDate(0, 6, 0)

	// On-track project (created first, name sorts first alphabetically)
	safe := testutil.NewTestProject("AAA Safe", testutil.WithTargetDate(farFuture))
	require.NoError(t, projects.Create(ctx, safe))
	nodeSafe := testutil.NewTestNode(safe.ID, "Node Safe")
	require.NoError(t, nodes.Create(ctx, nodeSafe))
	wiSafe := testutil.NewTestWorkItem(nodeSafe.ID, "Safe Task",
		testutil.WithPlannedMin(60),
		testutil.WithLoggedMin(50),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiSafe))
	sessSafe := testutil.NewTestSession(wiSafe.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessSafe))

	// Critical project (created second but should sort first due to risk)
	critical := testutil.NewTestProject("ZZZ Critical", testutil.WithTargetDate(tomorrow))
	require.NoError(t, projects.Create(ctx, critical))
	nodeCrit := testutil.NewTestNode(critical.ID, "Node Crit")
	require.NoError(t, nodes.Create(ctx, nodeCrit))
	wiCrit := testutil.NewTestWorkItem(nodeCrit.ID, "Urgent Task",
		testutil.WithPlannedMin(500),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiCrit))

	svc := NewStatusService(projects, workItems, sessions, profiles)
	req := contract.NewStatusRequest()
	req.Now = &now

	resp, err := svc.GetStatus(ctx, req)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(resp.Projects), 2)
	assert.Equal(t, critical.ID, resp.Projects[0].ProjectID, "critical project should sort before on-track")
}
