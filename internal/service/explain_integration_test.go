package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExplainIntegration_RealTrace_DeterministicExplainNow exercises the full pipeline:
// create project + items → what-now → build trace → DeterministicExplainNow → verify
// output references only valid TraceKeys.
func TestExplainIntegration_RealTrace_DeterministicExplainNow(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 0, 14) // 2 weeks out

	// Create a project with multiple work items at different priorities.
	proj := testutil.NewTestProject("Explain Test", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodes.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "High Priority Reading",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi1))

	wi2 := testutil.NewTestWorkItem(node.ID, "Practice Exercises",
		testutil.WithPlannedMin(90),
		testutil.WithSessionBounds(15, 45, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi2))

	// Step 1: Run what-now to get a real response.
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations, "should produce recommendations")

	// Step 2: Build trace from real response.
	trace := intelligence.BuildRecommendationTrace(resp)

	assert.NotEmpty(t, trace.Mode, "trace must have a mode")
	assert.Greater(t, trace.RequestedMin, 0, "trace must have requested_min")
	assert.NotEmpty(t, trace.Recommendations, "trace must have recommendations")
	assert.NotEmpty(t, trace.RiskProjects, "trace must have risk projects")

	// Step 3: Get trace keys — these are the only valid evidence references.
	validKeys := trace.TraceKeys()
	assert.NotEmpty(t, validKeys, "trace should produce valid keys")

	// Verify standard keys are present.
	assert.True(t, validKeys["mode"], "mode must be a valid key")
	assert.True(t, validKeys["requested_min"], "requested_min must be a valid key")
	assert.True(t, validKeys["allocated_min"], "allocated_min must be a valid key")

	// Verify recommendation-specific keys are present.
	for _, rec := range trace.Recommendations {
		prefix := "rec." + rec.WorkItemID
		assert.True(t, validKeys[prefix+".score"], "score key must exist for each recommendation")
		assert.True(t, validKeys[prefix+".risk_level"], "risk_level key must exist")
		assert.True(t, validKeys[prefix+".allocated_min"], "allocated_min key must exist")
	}

	// Verify risk project keys.
	for _, rp := range trace.RiskProjects {
		prefix := "risk." + rp.ProjectID
		assert.True(t, validKeys[prefix+".risk_level"], "risk project risk_level key must exist")
	}

	// Step 4: Run DeterministicExplainNow and verify output.
	explanation := intelligence.DeterministicExplainNow(trace)
	require.NotNil(t, explanation)

	assert.Equal(t, intelligence.ExplainContextWhatNow, explanation.Context)
	assert.Equal(t, float64(1.0), explanation.Confidence,
		"deterministic explanations should have confidence 1.0")
	assert.NotEmpty(t, explanation.SummaryShort, "must produce a summary")
	assert.NotEmpty(t, explanation.Factors, "must produce factors")

	// Verify all evidence references in the explanation are valid trace keys.
	for _, factor := range explanation.Factors {
		if factor.EvidenceRefKey != "" {
			assert.True(t, validKeys[factor.EvidenceRefKey],
				"factor %q references invalid key %q", factor.Name, factor.EvidenceRefKey)
		}
	}
}

// TestExplainIntegration_WhyNot_BlockedByDependency exercises the why-not pipeline
// when an item is blocked by an unfinished dependency.
func TestExplainIntegration_WhyNot_BlockedByDependency(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Dependency Explain", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	prerequisite := testutil.NewTestWorkItem(node.ID, "Foundation Reading",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, prerequisite))

	dependent := testutil.NewTestWorkItem(node.ID, "Advanced Topic",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, dependent))

	// Create dependency: prerequisite → dependent.
	require.NoError(t, deps.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: prerequisite.ID,
		SuccessorWorkItemID:   dependent.ID,
	}))

	// Run what-now — dependent should be blocked.
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(120)
	req.Now = &now

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	// Verify the dependent item is not in recommendations.
	for _, rec := range resp.Recommendations {
		assert.NotEqual(t, dependent.ID, rec.WorkItemID,
			"dependent item should be blocked by unfinished dependency")
	}

	// Build trace and run DeterministicWhyNot.
	trace := intelligence.BuildRecommendationTrace(resp)
	explanation := intelligence.DeterministicWhyNot(trace, dependent.ID)
	require.NotNil(t, explanation)

	assert.Equal(t, intelligence.ExplainContextWhyNot, explanation.Context)
	assert.NotEmpty(t, explanation.SummaryShort, "why-not must produce a summary")

	// If the item appears as a blocker, verify the blocker explanation.
	hasBlocker := false
	for _, b := range trace.Blockers {
		if b.EntityID == dependent.ID {
			hasBlocker = true
			assert.NotEmpty(t, b.Code, "blocker must have a code")
			assert.NotEmpty(t, b.Message, "blocker must have a message")
		}
	}

	if hasBlocker {
		assert.Contains(t, explanation.SummaryShort, "Blocked",
			"blocked item explanation should mention blocking")
	} else {
		// Item may not appear as a blocker if filtered at schedulable level.
		assert.Contains(t, explanation.SummaryShort, "not in the top recommendations",
			"non-blocked but excluded item should explain it wasn't recommended")
	}
}
