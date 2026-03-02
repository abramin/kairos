package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionRepo_ListSessionMinutesByWeek(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	sessRepo := NewSQLiteSessionRepo(db)

	// Create two projects.
	projA := testutil.NewTestProject("Alpha")
	require.NoError(t, projRepo.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "NodeA")
	require.NoError(t, nodeRepo.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "TaskA")
	require.NoError(t, wiRepo.Create(ctx, wiA))

	projB := testutil.NewTestProject("Beta")
	require.NoError(t, projRepo.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "NodeB")
	require.NoError(t, nodeRepo.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "TaskB")
	require.NoError(t, wiRepo.Create(ctx, wiB))

	// Log sessions: two for Alpha this week, one for Beta this week.
	now := time.Now().UTC()
	s1 := testutil.NewTestSession(wiA.ID, 30, testutil.WithStartedAt(now.Add(-1*time.Hour)))
	s2 := testutil.NewTestSession(wiA.ID, 45, testutil.WithStartedAt(now.Add(-2*time.Hour)))
	s3 := testutil.NewTestSession(wiB.ID, 20, testutil.WithStartedAt(now.Add(-3*time.Hour)))
	require.NoError(t, sessRepo.Create(ctx, s1))
	require.NoError(t, sessRepo.Create(ctx, s2))
	require.NoError(t, sessRepo.Create(ctx, s3))

	// Query for this week.
	from := now.AddDate(0, 0, -7)
	to := now.AddDate(0, 0, 1)

	results, err := sessRepo.ListSessionMinutesByWeek(ctx, from, to)
	require.NoError(t, err)

	// Should have two entries (Alpha + Beta) for the current week.
	require.Len(t, results, 2)

	// Build a lookup for easy assertions.
	byProject := map[string]int{}
	for _, r := range results {
		byProject[r.ProjectName] = r.TotalMin
	}
	assert.Equal(t, 75, byProject["Alpha"], "Alpha should have 30+45=75 minutes")
	assert.Equal(t, 20, byProject["Beta"], "Beta should have 20 minutes")
}

func TestSessionRepo_ListSessionMinutesByWeek_Empty(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	sessRepo := NewSQLiteSessionRepo(db)

	now := time.Now().UTC()
	results, err := sessRepo.ListSessionMinutesByWeek(ctx, now.AddDate(0, 0, -7), now)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSessionRepo_ListSessionMinutesByWeek_DateRange(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	sessRepo := NewSQLiteSessionRepo(db)

	proj := testutil.NewTestProject("RangeTest")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Task")
	require.NoError(t, wiRepo.Create(ctx, wi))

	now := time.Now().UTC()
	// Session inside range.
	inside := testutil.NewTestSession(wi.ID, 60, testutil.WithStartedAt(now.Add(-1*time.Hour)))
	// Session outside range (2 weeks ago).
	outside := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.AddDate(0, 0, -14)))
	require.NoError(t, sessRepo.Create(ctx, inside))
	require.NoError(t, sessRepo.Create(ctx, outside))

	// Query only last 7 days.
	from := now.AddDate(0, 0, -7)
	to := now.AddDate(0, 0, 1)

	results, err := sessRepo.ListSessionMinutesByWeek(ctx, from, to)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 60, results[0].TotalMin, "only the in-range session")
}
