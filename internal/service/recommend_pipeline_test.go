package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetermineMode_Critical(t *testing.T) {
	agg := ProjectAggregates{
		Risks: map[string]scheduler.RiskResult{
			"p1": {Level: domain.RiskOnTrack},
			"p2": {Level: domain.RiskCritical},
		},
	}
	assert.Equal(t, domain.ModeCritical, DetermineMode(agg))
}

func TestDetermineMode_Balanced(t *testing.T) {
	agg := ProjectAggregates{
		Risks: map[string]scheduler.RiskResult{
			"p1": {Level: domain.RiskOnTrack},
			"p2": {Level: domain.RiskAtRisk},
		},
	}
	assert.Equal(t, domain.ModeBalanced, DetermineMode(agg))
}

func TestDetermineMode_EmptyRisks(t *testing.T) {
	agg := ProjectAggregates{
		Risks: map[string]scheduler.RiskResult{},
	}
	assert.Equal(t, domain.ModeBalanced, DetermineMode(agg))
}

func TestBlockResolver_BatchDependencyCheck(t *testing.T) {
	projects, nodes, workItems, deps, _, _, _ := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("BlockTest")
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Predecessor",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	wi2 := testutil.NewTestWorkItem(node.ID, "Blocked",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	wi3 := testutil.NewTestWorkItem(node.ID, "Free",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi1))
	require.NoError(t, workItems.Create(ctx, wi2))
	require.NoError(t, workItems.Create(ctx, wi3))

	require.NoError(t, deps.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: wi1.ID,
		SuccessorWorkItemID:  wi2.ID,
	}))

	resolver := &BlockResolver{deps: deps}
	candidates := []repository.SchedulableCandidate{
		{WorkItem: *wi1, ProjectID: proj.ID, ProjectName: proj.Name},
		{WorkItem: *wi2, ProjectID: proj.ID, ProjectName: proj.Name},
		{WorkItem: *wi3, ProjectID: proj.ID, ProjectName: proj.Name},
	}

	now := time.Now().UTC()
	unblocked, blockers, err := resolver.Resolve(ctx, candidates, now)
	require.NoError(t, err)

	assert.Len(t, unblocked, 2, "wi1 and wi3 should pass through")
	assert.Len(t, blockers, 1, "wi2 should be blocked")
	assert.Equal(t, app.BlockerDependency, blockers[0].Code)
	assert.Equal(t, wi2.ID, blockers[0].EntityID)
}

func TestBlockResolver_NotBeforeConstraint(t *testing.T) {
	_, _, _, deps, _, _, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	future := now.AddDate(0, 0, 7)

	resolver := &BlockResolver{deps: deps}
	candidates := []repository.SchedulableCandidate{
		{
			WorkItem: domain.WorkItem{
				ID:        "wi-future",
				Title:     "Future Task",
				NotBefore: &future,
				PlannedMin: 60,
			},
			ProjectID:   "proj-1",
			ProjectName: "Test",
		},
	}

	unblocked, blockers, err := resolver.Resolve(ctx, candidates, now)
	require.NoError(t, err)
	assert.Empty(t, unblocked)
	assert.Len(t, blockers, 1)
	assert.Equal(t, app.BlockerNotBefore, blockers[0].Code)
}

func TestBlockResolver_WorkCompleteConstraint(t *testing.T) {
	_, _, _, deps, _, _, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	resolver := &BlockResolver{deps: deps}
	candidates := []repository.SchedulableCandidate{
		{
			WorkItem: domain.WorkItem{
				ID:         "wi-done",
				Title:      "Done Task",
				PlannedMin: 60,
				LoggedMin:  60,
			},
			ProjectID:   "proj-1",
			ProjectName: "Test",
		},
	}

	unblocked, blockers, err := resolver.Resolve(ctx, candidates, now)
	require.NoError(t, err)
	assert.Empty(t, unblocked)
	assert.Len(t, blockers, 1)
	assert.Equal(t, app.BlockerWorkComplete, blockers[0].Code)
}

func TestBlockResolver_MixedConstraints(t *testing.T) {
	projects, nodes, workItems, deps, _, _, _ := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("MixedTest")
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	now := time.Now().UTC()
	future := now.AddDate(0, 0, 7)

	// Predecessor (will block wi2 via dependency)
	wi1 := testutil.NewTestWorkItem(node.ID, "Predecessor",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	// Blocked by dependency
	wi2 := testutil.NewTestWorkItem(node.ID, "DepBlocked",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	// Blocked by NotBefore
	wi3 := testutil.NewTestWorkItem(node.ID, "NotBeforeBlocked",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
		testutil.WithNotBefore(future),
	)
	// Blocked by work complete
	wi4 := testutil.NewTestWorkItem(node.ID, "WorkComplete",
		testutil.WithPlannedMin(60),
		testutil.WithLoggedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	// Free
	wi5 := testutil.NewTestWorkItem(node.ID, "Free",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)

	require.NoError(t, workItems.Create(ctx, wi1))
	require.NoError(t, workItems.Create(ctx, wi2))
	require.NoError(t, workItems.Create(ctx, wi3))
	require.NoError(t, workItems.Create(ctx, wi4))
	require.NoError(t, workItems.Create(ctx, wi5))

	require.NoError(t, deps.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: wi1.ID,
		SuccessorWorkItemID:  wi2.ID,
	}))

	resolver := &BlockResolver{deps: deps}
	candidates := []repository.SchedulableCandidate{
		{WorkItem: *wi1, ProjectID: proj.ID, ProjectName: proj.Name},
		{WorkItem: *wi2, ProjectID: proj.ID, ProjectName: proj.Name},
		{WorkItem: *wi3, ProjectID: proj.ID, ProjectName: proj.Name},
		{WorkItem: *wi4, ProjectID: proj.ID, ProjectName: proj.Name},
		{WorkItem: *wi5, ProjectID: proj.ID, ProjectName: proj.Name},
	}

	unblocked, blockers, err := resolver.Resolve(ctx, candidates, now)
	require.NoError(t, err)
	assert.Len(t, unblocked, 2, "only wi1 and wi5 should pass through")
	assert.Len(t, blockers, 3, "wi2, wi3, wi4 should each have a blocker")

	blockerCodes := make(map[app.ConstraintBlockerCode]bool)
	for _, b := range blockers {
		blockerCodes[b.Code] = true
	}
	assert.True(t, blockerCodes[app.BlockerDependency])
	assert.True(t, blockerCodes[app.BlockerNotBefore])
	assert.True(t, blockerCodes[app.BlockerWorkComplete])
}

func TestScoreCandidates_DelegatesCorrectly(t *testing.T) {
	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	candidates := []repository.SchedulableCandidate{
		{
			WorkItem: domain.WorkItem{
				ID:                "wi-1",
				Seq:               1,
				Title:             "Task 1",
				Status:            domain.WorkItemTodo,
				PlannedMin:        100,
				LoggedMin:         20,
				MinSessionMin:     15,
				MaxSessionMin:     60,
				DefaultSessionMin: 30,
			},
			ProjectID:         "proj-1",
			ProjectName:       "Test Project",
			NodeTitle:         "Week 1",
			ProjectTargetDate: &target,
		},
	}

	agg := ProjectAggregates{
		Risks: map[string]scheduler.RiskResult{
			"proj-1": {Level: domain.RiskOnTrack},
		},
	}
	weights := scheduler.ScoringWeights{
		DeadlinePressure: 1.0,
		BehindPace:       0.8,
		Spacing:          0.5,
		Variation:        0.3,
	}

	scored := ScoreCandidates(candidates, nil, agg, weights, domain.ModeBalanced, now)
	require.Len(t, scored, 1)
	assert.Equal(t, "wi-1", scored[0].Input.WorkItemID)
	assert.False(t, scored[0].Blocked)
}

func TestAssembleResponse_AllocatedMinSum(t *testing.T) {
	now := time.Now().UTC()
	slices := []app.WorkSlice{
		{WorkItemID: "wi-1", AllocatedMin: 30, ProjectID: "p1"},
		{WorkItemID: "wi-2", AllocatedMin: 25, ProjectID: "p2"},
	}
	agg := ProjectAggregates{
		Risks:      map[string]scheduler.RiskResult{},
		Names:      map[string]string{},
		Planned:    map[string]int{},
		Logged:     map[string]int{},
		RecentMin:  map[string]int{},
		TargetDate: map[string]*time.Time{},
		StartDate:  map[string]*time.Time{},
	}

	resp := AssembleResponse(now, domain.ModeBalanced, 90, slices, nil, agg)
	assert.Equal(t, 55, resp.AllocatedMin)
	assert.Equal(t, 35, resp.UnallocatedMin)
	assert.Equal(t, 90, resp.RequestedMin)
}

func TestAssembleResponse_PolicyMessages(t *testing.T) {
	now := time.Now().UTC()
	target := now.AddDate(0, 3, 0)
	agg := ProjectAggregates{
		Risks: map[string]scheduler.RiskResult{
			"p1": {Level: domain.RiskOnTrack},
			"p2": {Level: domain.RiskAtRisk},
		},
		Names:      map[string]string{"p1": "Alpha", "p2": "Beta"},
		Planned:    map[string]int{"p1": 100, "p2": 200},
		Logged:     map[string]int{"p1": 50, "p2": 30},
		RecentMin:  map[string]int{"p1": 60, "p2": 20},
		TargetDate: map[string]*time.Time{"p1": &target, "p2": &target},
		StartDate:  map[string]*time.Time{},
	}

	resp := AssembleResponse(now, domain.ModeBalanced, 60, nil, nil, agg)

	// Only on-track projects generate policy messages.
	require.Len(t, resp.PolicyMessages, 1)
	assert.Contains(t, resp.PolicyMessages[0], "Alpha")
	assert.Contains(t, resp.PolicyMessages[0], "on track")
}

func TestComputeAggregates_RiskLevels(t *testing.T) {
	now := time.Now().UTC()
	tomorrow := now.AddDate(0, 0, 1)
	farFuture := now.AddDate(0, 6, 0)

	startDate := now.AddDate(0, -1, 0)

	rctx := &RecommendationContext{
		Now: now,
		Candidates: []repository.SchedulableCandidate{
			{
				WorkItem: domain.WorkItem{
					ID:         "wi-critical",
					PlannedMin: 500,
					LoggedMin:  0,
				},
				ProjectID:         "proj-critical",
				ProjectName:       "Critical",
				ProjectTargetDate: &tomorrow,
				ProjectStartDate:  &startDate,
			},
			{
				WorkItem: domain.WorkItem{
					ID:         "wi-safe",
					PlannedMin: 60,
					LoggedMin:  30,
				},
				ProjectID:         "proj-safe",
				ProjectName:       "Safe",
				ProjectTargetDate: &farFuture,
				ProjectStartDate:  &startDate,
			},
		},
		CompletedSummaries: nil,
		RecentSessions:     nil,
		BufferPct:          0.1,
		BaselineDailyMin:   30,
	}

	agg := ComputeAggregates(rctx)
	assert.Contains(t, agg.Risks, "proj-critical")
	assert.Contains(t, agg.Risks, "proj-safe")
	// Critical project due tomorrow with 500 min remaining should be critical.
	assert.Equal(t, domain.RiskCritical, agg.Risks["proj-critical"].Level)
	// Safe project 6 months out should not be critical.
	assert.NotEqual(t, domain.RiskCritical, agg.Risks["proj-safe"].Level)
}
