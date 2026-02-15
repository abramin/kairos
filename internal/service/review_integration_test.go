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

// TestWeeklyReview_FullPipeline exercises the full weekly review pipeline:
// seed projects with sessions → StatusService.GetStatus → build WeeklyReviewTrace →
// DeterministicWeeklyReview → verify explanation + zettelkasten backlog via SessionSummaryByType.
func TestWeeklyReview_FullPipeline(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// === Project A: At-risk study project with reading + practice items ===
	projA := testutil.NewTestProject("Linear Algebra",
		testutil.WithTargetDate(now.AddDate(0, 0, 14)))
	require.NoError(t, projects.Create(ctx, projA))

	nodeA := testutil.NewTestNode(projA.ID, "Week 3", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodes.Create(ctx, nodeA))

	wiReading := testutil.NewTestWorkItem(nodeA.ID, "Read Chapter 5",
		testutil.WithPlannedMin(180),
		testutil.WithLoggedMin(75),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithWorkItemType("reading"),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiReading))

	wiPractice := testutil.NewTestWorkItem(nodeA.ID, "Problem Set 5",
		testutil.WithPlannedMin(120),
		testutil.WithLoggedMin(30),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithWorkItemType("practice"),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiPractice))

	// === Project B: On-track side project ===
	projB := testutil.NewTestProject("Blog Posts",
		testutil.WithTargetDate(now.AddDate(0, 3, 0)))
	require.NoError(t, projects.Create(ctx, projB))

	nodeB := testutil.NewTestNode(projB.ID, "Drafts", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeB))

	wiBlog := testutil.NewTestWorkItem(nodeB.ID, "Write Go Concurrency Post",
		testutil.WithPlannedMin(90),
		testutil.WithLoggedMin(45),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithWorkItemType("task"),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiBlog))

	// === Seed sessions across 7 days ===
	// Reading sessions: 5 sessions totaling 75 min
	for i := 0; i < 5; i++ {
		sess := testutil.NewTestSession(wiReading.ID, 15,
			testutil.WithStartedAt(now.Add(-time.Duration(i)*24*time.Hour)))
		require.NoError(t, sessions.Create(ctx, sess))
	}

	// Practice sessions: 2 sessions totaling 30 min
	sess1 := testutil.NewTestSession(wiPractice.ID, 15,
		testutil.WithStartedAt(now.Add(-1*24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess1))
	sess2 := testutil.NewTestSession(wiPractice.ID, 15,
		testutil.WithStartedAt(now.Add(-3*24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess2))

	// Blog sessions: 3 sessions totaling 45 min
	for i := 0; i < 3; i++ {
		sess := testutil.NewTestSession(wiBlog.ID, 15,
			testutil.WithStartedAt(now.Add(-time.Duration(i*2)*24*time.Hour)))
		require.NoError(t, sessions.Create(ctx, sess))
	}

	// === Step 1: Get project status (as the review command does) ===
	statusSvc := NewStatusService(projects, workItems, sessions, profiles)
	statusReq := contract.NewStatusRequest()
	statusReq.Now = &now

	statusResp, err := statusSvc.GetStatus(ctx, statusReq)
	require.NoError(t, err)
	require.Len(t, statusResp.Projects, 2, "should have 2 active projects")

	// === Step 2: Build WeeklyReviewTrace (matches CLI review_cmd.go logic) ===
	trace := intelligence.WeeklyReviewTrace{
		PeriodDays: 7,
	}
	totalLogged := 0
	for _, p := range statusResp.Projects {
		trace.ProjectSummaries = append(trace.ProjectSummaries, intelligence.ProjectWeeklySummary{
			ProjectID:   p.ProjectID,
			ProjectName: p.ProjectName,
			PlannedMin:  p.PlannedMinTotal,
			LoggedMin:   p.LoggedMinTotal,
			RiskLevel:   string(p.RiskLevel),
		})
		totalLogged += p.LoggedMinTotal
	}
	trace.TotalLoggedMin = totalLogged

	require.NotEmpty(t, trace.ProjectSummaries, "trace should contain project summaries")
	assert.Greater(t, trace.TotalLoggedMin, 0, "should have logged minutes")

	// === Step 3: Generate deterministic explanation (no LLM) ===
	explanation := intelligence.DeterministicWeeklyReview(trace)

	require.NotNil(t, explanation)
	assert.Equal(t, intelligence.ExplainContextWeeklyReview, explanation.Context)
	assert.Equal(t, 1.0, explanation.Confidence, "deterministic fallback is always 1.0")
	assert.NotEmpty(t, explanation.SummaryShort)
	assert.Contains(t, explanation.SummaryShort, "2 project(s)")

	// Verify one factor per project
	assert.Len(t, explanation.Factors, 2, "should have 1 factor per project")
	for _, factor := range explanation.Factors {
		assert.Equal(t, "push_for", factor.Direction)
		assert.Equal(t, intelligence.EvidenceHistory, factor.EvidenceRefType)
		assert.NotEmpty(t, factor.Summary)
	}

	// === Step 4: Verify evidence keys are valid ===
	validKeys := trace.WeeklyTraceKeys()
	for _, factor := range explanation.Factors {
		assert.True(t, validKeys[factor.EvidenceRefKey],
			"evidence key %q should be valid", factor.EvidenceRefKey)
	}

	// === Step 5: Verify zettelkasten backlog via SessionSummaryByType ===
	summaries, err := sessions.ListRecentSummaryByType(ctx, 7)
	require.NoError(t, err)
	require.NotEmpty(t, summaries, "should have session summaries")

	// Check that reading sessions are captured
	var readingTotal int
	for _, s := range summaries {
		if s.WorkItemType == "reading" {
			readingTotal += s.TotalMinutes
		}
	}
	assert.Equal(t, 75, readingTotal, "reading sessions should total 75 min (5 x 15)")

	// No zettel processing sessions → backlog should show
	var zettelTotal int
	for _, s := range summaries {
		if s.WorkItemType == "zettel" {
			zettelTotal += s.TotalMinutes
		}
	}
	assert.Equal(t, 0, zettelTotal, "no zettel sessions seeded")
}

// TestWeeklyReview_NoSessions_ProducesEmptyReview verifies the review pipeline
// handles the cold-start case (no sessions logged yet).
func TestWeeklyReview_NoSessions_ProducesEmptyReview(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	proj := testutil.NewTestProject("New Project",
		testutil.WithTargetDate(now.AddDate(0, 1, 0)))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "First Task",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wi))

	// Get status (no sessions)
	statusSvc := NewStatusService(projects, workItems, sessions, profiles)
	statusReq := contract.NewStatusRequest()
	statusReq.Now = &now

	statusResp, err := statusSvc.GetStatus(ctx, statusReq)
	require.NoError(t, err)

	// Build trace
	trace := intelligence.WeeklyReviewTrace{PeriodDays: 7}
	for _, p := range statusResp.Projects {
		trace.ProjectSummaries = append(trace.ProjectSummaries, intelligence.ProjectWeeklySummary{
			ProjectID:   p.ProjectID,
			ProjectName: p.ProjectName,
			PlannedMin:  p.PlannedMinTotal,
			LoggedMin:   p.LoggedMinTotal,
			RiskLevel:   string(p.RiskLevel),
		})
	}

	// Deterministic review should handle zero sessions gracefully
	explanation := intelligence.DeterministicWeeklyReview(trace)
	require.NotNil(t, explanation)
	assert.Contains(t, explanation.SummaryShort, "0 sessions")
	assert.Contains(t, explanation.SummaryShort, "0 minutes")

	// Session summary should be empty
	summaries, err := sessions.ListRecentSummaryByType(ctx, 7)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

// TestWeeklyReview_ZettelBacklogRatio verifies the zettelkasten backlog nudge
// respects the 3:1 reading:zettel threshold.
func TestWeeklyReview_ZettelBacklogRatio(t *testing.T) {
	projects, nodes, workItems, _, sessions, _, _ := setupRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	proj := testutil.NewTestProject("Study", testutil.WithTargetDate(now.AddDate(0, 3, 0)))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Module")
	require.NoError(t, nodes.Create(ctx, node))

	wiReading := testutil.NewTestWorkItem(node.ID, "Read Textbook",
		testutil.WithPlannedMin(200),
		testutil.WithWorkItemType("reading"),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiReading))

	wiZettel := testutil.NewTestWorkItem(node.ID, "Process Notes",
		testutil.WithPlannedMin(100),
		testutil.WithWorkItemType("zettel"),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiZettel))

	// Reading: 60 min
	readSess := testutil.NewTestSession(wiReading.ID, 60,
		testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, readSess))

	// Zettel: 30 min (ratio = 60/30 = 2.0 < 3.0 → no backlog nudge)
	zettelSess := testutil.NewTestSession(wiZettel.ID, 30,
		testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, zettelSess))

	summaries, err := sessions.ListRecentSummaryByType(ctx, 7)
	require.NoError(t, err)

	var readingMin, zettelMin int
	for _, s := range summaries {
		switch s.WorkItemType {
		case "reading":
			readingMin += s.TotalMinutes
		case "zettel":
			zettelMin += s.TotalMinutes
		}
	}

	assert.Equal(t, 60, readingMin)
	assert.Equal(t, 30, zettelMin)
	// Ratio is 2.0, below 3.0 threshold → backlog nudge should NOT show
	assert.False(t, float64(readingMin)/float64(zettelMin) > 3.0,
		"2:1 ratio should not trigger backlog nudge")
}
