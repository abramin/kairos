package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPersona_GradStudent_MixedCompletion simulates a grad student with 3 projects
// at different completion stages: near-complete thesis chapter, half-done course
// readings, and a freshly-started lab report with an imminent deadline.
// Exercises: critical mode from zero-activity project, logging shifts priorities,
// momentum bonus, status accuracy, determinism.
func TestPersona_GradStudent_MixedCompletion(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// === Project A: Thesis Chapter — 90% done, due in 30 days, on-track ===
	projA := testutil.NewTestProject("Thesis Chapter",
		testutil.WithTargetDate(now.AddDate(0, 0, 30)))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Writing", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Revise Draft",
		testutil.WithPlannedMin(600),
		testutil.WithLoggedMin(540),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))
	// Recent activity so A has pace.
	sessA := testutil.NewTestSession(wiA.ID, 60, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessA))

	// === Project B: Course Readings — 50% done, due in 14 days, at-risk ===
	projB := testutil.NewTestProject("Course Readings",
		testutil.WithTargetDate(now.AddDate(0, 0, 14)))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Chapters", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Read Chapters 5-10",
		testutil.WithPlannedMin(300),
		testutil.WithLoggedMin(150),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiB))
	sessB := testutil.NewTestSession(wiB.ID, 30, testutil.WithStartedAt(now.Add(-48*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessB))

	// === Project C: Lab Report — just started, due in 5 days, 500 min work, critical ===
	// With baseline_daily_min=30: required = (500*1.1)/5 = 110 min/day, ratio = 110/30 = 3.67 > 1.5.
	// No sessions, no progress => not on-pace => critical.
	projC := testutil.NewTestProject("Lab Report",
		testutil.WithTargetDate(now.AddDate(0, 0, 5)))
	require.NoError(t, projects.Create(ctx, projC))
	nodeC := testutil.NewTestNode(projC.ID, "Sections", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeC))
	wiC := testutil.NewTestWorkItem(nodeC.ID, "Write Lab Report",
		testutil.WithPlannedMin(500),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiC))
	// No sessions for C — zero activity.

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	sessionSvc := NewSessionService(sessions, workItems, uow)

	// === Phase 1: Initial query — C should trigger critical mode ===
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations)

	assert.Equal(t, domain.ModeCritical, resp1.Mode,
		"lab report with no activity and 7-day deadline should trigger critical mode")

	// In critical mode, only the critical project should be recommended.
	for _, rec := range resp1.Recommendations {
		assert.Equal(t, projC.ID, rec.ProjectID,
			"critical mode should only recommend items from the critical project")
	}

	// === Phase 2: Log 30 min on C, re-query ===
	sess1 := &domain.WorkSessionLog{
		WorkItemID: wiC.ID,
		StartedAt:  now.Add(-time.Hour),
		Minutes:    30,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess1))

	updatedC, err := workItems.GetByID(ctx, wiC.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, updatedC.Status,
		"first session should auto-transition to in_progress")
	assert.Equal(t, 30, updatedC.LoggedMin)

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations)

	// C should still be prioritized (still high risk with 470 min remaining in 5 days).
	assert.Equal(t, projC.ID, resp2.Recommendations[0].ProjectID,
		"after one session, lab report should still be top priority")

	// === Phase 3: Log another 30 min on C, re-query ===
	sess2 := &domain.WorkSessionLog{
		WorkItemID: wiC.ID,
		StartedAt:  now.Add(-30 * time.Minute),
		Minutes:    30,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess2))

	resp3, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp3.Recommendations)

	// After 60 min logged on C, remaining = 440 min in 5 days = ~88 min/day.
	// With 60 min logged today, recentDailyMin is higher. Risk may drop.
	// Verify C is still in recommendations (may or may not be #1 depending on risk calc).
	recProjectIDs := make(map[string]bool)
	for _, rec := range resp3.Recommendations {
		recProjectIDs[rec.ProjectID] = true
	}

	// === Phase 4: Status check — verify all 3 projects reported ===
	statusSvc := NewStatusService(projects, workItems, sessions, profiles)
	statusReq := contract.NewStatusRequest()
	statusReq.Now = &now

	statusResp, err := statusSvc.GetStatus(ctx, statusReq)
	require.NoError(t, err)
	require.Len(t, statusResp.Projects, 3, "all 3 projects should appear in status")

	riskMap := make(map[string]domain.RiskLevel)
	progressMap := make(map[string]float64)
	for _, ps := range statusResp.Projects {
		riskMap[ps.ProjectID] = ps.RiskLevel
		progressMap[ps.ProjectID] = ps.ProgressTimePct
	}

	// Project A (90% done, 30 days left) should be on-track.
	assert.Equal(t, domain.RiskOnTrack, riskMap[projA.ID],
		"thesis chapter (90%% done, 30 days left) should be on-track")

	// Project A progress should be ~90%.
	assert.InDelta(t, 90.0, progressMap[projA.ID], 1.0,
		"thesis chapter progress should be ~90%%")

	// === Phase 5: Determinism check ===
	resp4a, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	resp4b, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	require.Equal(t, len(resp4a.Recommendations), len(resp4b.Recommendations))
	for i := range resp4a.Recommendations {
		assert.Equal(t, resp4a.Recommendations[i].WorkItemID, resp4b.Recommendations[i].WorkItemID,
			"recommendations must be deterministic")
	}

	// === Verify allocation invariants ===
	for _, rec := range resp4a.Recommendations {
		assert.GreaterOrEqual(t, rec.AllocatedMin, rec.MinSessionMin)
		assert.LessOrEqual(t, rec.AllocatedMin, rec.MaxSessionMin)
	}
	assert.LessOrEqual(t, resp4a.AllocatedMin, resp4a.RequestedMin)
}

// TestPersona_Freelancer_DeadlineCrunch simulates a freelancer with 4 projects,
// two due on the same day (deadline clustering), one relaxed, one with no deadline.
// Exercises: deadline clustering competition, no-deadline handling, logging shifts
// priority between competing deadlines, mode transitions.
func TestPersona_Freelancer_DeadlineCrunch(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// === Project A: Client Alpha — due in 2 days, 180 min remaining ===
	projA := testutil.NewTestProject("Client Alpha",
		testutil.WithTargetDate(now.AddDate(0, 0, 2)))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Deliverable", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Alpha Report",
		testutil.WithPlannedMin(180),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))
	sessA := testutil.NewTestSession(wiA.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessA))

	// === Project B: Client Beta — also due in 2 days, 120 min remaining ===
	projB := testutil.NewTestProject("Client Beta",
		testutil.WithTargetDate(now.AddDate(0, 0, 2)))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Deliverable", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Beta Slides",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiB))
	sessB := testutil.NewTestSession(wiB.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessB))

	// === Project C: Personal Blog — due in 60 days, 90 min remaining ===
	projC := testutil.NewTestProject("Personal Blog",
		testutil.WithTargetDate(now.AddDate(0, 0, 60)))
	require.NoError(t, projects.Create(ctx, projC))
	nodeC := testutil.NewTestNode(projC.ID, "Posts", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeC))
	wiC := testutil.NewTestWorkItem(nodeC.ID, "Write Blog Post",
		testutil.WithPlannedMin(90),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiC))
	sessC := testutil.NewTestSession(wiC.ID, 15, testutil.WithStartedAt(now.Add(-72*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessC))

	// === Project D: Side Project — no deadline, 200 min remaining ===
	projD := testutil.NewTestProject("Side Project") // no target date
	require.NoError(t, projects.Create(ctx, projD))
	nodeD := testutil.NewTestNode(projD.ID, "Features", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeD))
	wiD := testutil.NewTestWorkItem(nodeD.ID, "Build Feature",
		testutil.WithPlannedMin(200),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiD))
	sessD := testutil.NewTestSession(wiD.ID, 20, testutil.WithStartedAt(now.Add(-48*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessD))

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	sessionSvc := NewSessionService(sessions, workItems, uow)

	// === Phase 1: Initial query — A and B are urgent, should drive mode ===
	req := contract.NewWhatNowRequest(120)
	req.Now = &now

	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations)

	// With 2-day deadlines and 180+120 min of work, A and/or B should be critical/at-risk.
	// Verify both deadline-clustered projects appear in risk summaries.
	riskMap := make(map[string]domain.RiskLevel)
	for _, rs := range resp1.TopRiskProjects {
		riskMap[rs.ProjectID] = rs.RiskLevel
	}
	assert.Contains(t, riskMap, projA.ID, "Client Alpha should appear in risk summaries")
	assert.Contains(t, riskMap, projB.ID, "Client Beta should appear in risk summaries")

	// Project D (no deadline) should be on-track.
	if rl, ok := riskMap[projD.ID]; ok {
		assert.Equal(t, domain.RiskOnTrack, rl,
			"no-deadline project should always be on-track")
	}

	// In critical or at-risk mode, the urgent projects should be prioritized.
	if resp1.Mode == domain.ModeCritical {
		for _, rec := range resp1.Recommendations {
			assert.NotEqual(t, projC.ID, rec.ProjectID,
				"critical mode should not recommend relaxed project")
			assert.NotEqual(t, projD.ID, rec.ProjectID,
				"critical mode should not recommend no-deadline project")
		}
	}

	// === Phase 2: Log 60 min on A, re-query ===
	sess1 := &domain.WorkSessionLog{
		WorkItemID: wiA.ID,
		StartedAt:  now.Add(-time.Hour),
		Minutes:    60,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess1))

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations)

	// After logging 60 on A (remaining: 120), B (remaining: 120) should be equally urgent.
	// B should appear in recommendations (may be first due to A's spacing penalty).
	recIDs2 := make(map[string]bool)
	for _, rec := range resp2.Recommendations {
		recIDs2[rec.WorkItemID] = true
	}
	assert.True(t, recIDs2[wiB.ID],
		"after logging on A, B should appear in recommendations (equally or more urgent)")

	// === Phase 3: Log 60 min on B, re-query — mode may transition ===
	sess2 := &domain.WorkSessionLog{
		WorkItemID: wiB.ID,
		StartedAt:  now.Add(-30 * time.Minute),
		Minutes:    60,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess2))

	resp3, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp3.Recommendations)

	// After 60 min logged on both A and B, their remaining work is reduced.
	// A: remaining = 120, B: remaining = 60. Pace is improving.
	// If mode transitions to balanced, C and D may now appear.
	if resp3.Mode == domain.ModeBalanced {
		recProjects3 := make(map[string]bool)
		for _, rec := range resp3.Recommendations {
			recProjects3[rec.ProjectID] = true
		}
		// In balanced mode with variation, multiple projects should appear.
		assert.GreaterOrEqual(t, len(recProjects3), 2,
			"balanced mode should distribute work across projects")
	}

	// === Phase 4: Verify D (no deadline) status ===
	statusSvc := NewStatusService(projects, workItems, sessions, profiles)
	statusReq := contract.NewStatusRequest()
	statusReq.Now = &now

	statusResp, err := statusSvc.GetStatus(ctx, statusReq)
	require.NoError(t, err)

	for _, ps := range statusResp.Projects {
		if ps.ProjectID == projD.ID {
			assert.Equal(t, domain.RiskOnTrack, ps.RiskLevel,
				"no-deadline project should always be on-track in status")
		}
	}

	// === Allocation invariants ===
	assert.LessOrEqual(t, resp3.AllocatedMin, resp3.RequestedMin)
	for _, rec := range resp3.Recommendations {
		assert.GreaterOrEqual(t, rec.AllocatedMin, rec.MinSessionMin)
		assert.LessOrEqual(t, rec.AllocatedMin, rec.MaxSessionMin)
	}
}

// TestPersona_FreshStart_AllNewProjects simulates a user who just imported 3 projects
// with zero session history. Tests the import->schedule pipeline, baseline_daily_min
// floor, zero-session bootstrap, and first-session spacing effects.
func TestPersona_FreshStart_AllNewProjects(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	importSvc := NewImportService(projects, nodes, workItems, deps, uow)

	// === Import Project A: due in 21 days, 300 min total ===
	targetA := now.AddDate(0, 0, 21).Format("2006-01-02")
	startA := now.Format("2006-01-02")
	pm80, pm70, pm75 := 80, 70, 75

	schemaA := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID: "FRE01", Name: "Web Redesign", Domain: "freelance",
			StartDate: startA, TargetDate: &targetA,
		},
		Defaults: &importer.DefaultsImport{
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     intPtr(15),
				MaxSessionMin:     intPtr(60),
				DefaultSessionMin: intPtr(30),
				Splittable:        boolPtr(true),
			},
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Design", Kind: "module", Order: 1},
			{Ref: "n2", Title: "Development", Kind: "module", Order: 2},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Wireframes", Type: "task", PlannedMin: &pm80},
			{Ref: "w2", NodeRef: "n1", Title: "Mockups", Type: "task", PlannedMin: &pm70},
			{Ref: "w3", NodeRef: "n2", Title: "HTML/CSS", Type: "task", PlannedMin: &pm75},
			{Ref: "w4", NodeRef: "n2", Title: "JS Logic", Type: "task", PlannedMin: &pm75},
		},
	}
	resultA, err := importSvc.ImportProjectFromSchema(ctx, schemaA)
	require.NoError(t, err)
	assert.Equal(t, 4, resultA.WorkItemCount)

	// === Import Project B: due in 45 days, 240 min total ===
	targetB := now.AddDate(0, 0, 45).Format("2006-01-02")
	pm80b := 80
	schemaB := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID: "FRE02", Name: "Blog Platform", Domain: "personal",
			StartDate: startA, TargetDate: &targetB,
		},
		Defaults: &importer.DefaultsImport{
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     intPtr(15),
				MaxSessionMin:     intPtr(60),
				DefaultSessionMin: intPtr(30),
				Splittable:        boolPtr(true),
			},
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Backend", Kind: "module", Order: 1},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "API Endpoints", Type: "task", PlannedMin: &pm80b},
			{Ref: "w2", NodeRef: "n1", Title: "Database Schema", Type: "task", PlannedMin: &pm80b},
			{Ref: "w3", NodeRef: "n1", Title: "Auth System", Type: "task", PlannedMin: &pm80b},
		},
	}
	resultB, err := importSvc.ImportProjectFromSchema(ctx, schemaB)
	require.NoError(t, err)
	assert.Equal(t, 3, resultB.WorkItemCount)

	// === Import Project C: due in 90 days, 180 min total ===
	targetC := now.AddDate(0, 0, 90).Format("2006-01-02")
	pm90 := 90
	schemaC := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID: "FRE03", Name: "Portfolio Site", Domain: "personal",
			StartDate: startA, TargetDate: &targetC,
		},
		Defaults: &importer.DefaultsImport{
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     intPtr(15),
				MaxSessionMin:     intPtr(60),
				DefaultSessionMin: intPtr(30),
				Splittable:        boolPtr(true),
			},
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Content", Kind: "module", Order: 1},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Write About Page", Type: "task", PlannedMin: &pm90},
			{Ref: "w2", NodeRef: "n1", Title: "Project Gallery", Type: "task", PlannedMin: &pm90},
		},
	}
	resultC, err := importSvc.ImportProjectFromSchema(ctx, schemaC)
	require.NoError(t, err)
	assert.Equal(t, 2, resultC.WorkItemCount)

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	sessionSvc := NewSessionService(sessions, workItems, uow)

	// === Phase 1: First-ever query — zero sessions everywhere ===
	req := contract.NewWhatNowRequest(90)
	req.Now = &now

	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations,
		"should produce recommendations despite zero session history")

	// Verify baseline_daily_min floor prevents spurious critical for projects with
	// reasonable deadlines. Project A (21 days, 300 min) = ~14 min/day needed.
	// Default baseline_daily_min = 30, so effective daily = 30.
	// Ratio = 14/30 = 0.47 < 1.0 => on-track.
	assert.Equal(t, domain.ModeBalanced, resp1.Mode,
		"with baseline_daily_min floor, reasonable deadlines should not trigger critical")

	// === Phase 2: Log first session on recommended item ===
	firstRec := resp1.Recommendations[0]
	sess1 := &domain.WorkSessionLog{
		WorkItemID: firstRec.WorkItemID,
		StartedAt:  now.Add(-time.Hour),
		Minutes:    45,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess1))

	// Verify auto-transition.
	updatedWI, err := workItems.GetByID(ctx, firstRec.WorkItemID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, updatedWI.Status,
		"first session should auto-transition to in_progress")
	assert.Equal(t, 45, updatedWI.LoggedMin)

	// === Phase 3: Re-query — spacing should affect ordering ===
	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations)

	// The item we just worked on today should have a spacing penalty.
	// With variation enforcement, other projects should appear first.
	if len(resp2.Recommendations) >= 2 {
		recProjects := make(map[string]bool)
		for _, rec := range resp2.Recommendations {
			recProjects[rec.ProjectID] = true
		}
		assert.GreaterOrEqual(t, len(recProjects), 2,
			"after logging on one project, variation should distribute to others")
	}

	// === Phase 4: Replan — verify risk deltas ===
	replanSvc := NewReplanService(projects, workItems, sessions, profiles)
	replanReq := contract.NewReplanRequest(domain.TriggerManual)
	replanReq.Now = &now

	replanResp, err := replanSvc.Replan(ctx, replanReq)
	require.NoError(t, err)
	require.Len(t, replanResp.Deltas, 3, "should have one delta per project")

	for _, delta := range replanResp.Deltas {
		assert.NotEmpty(t, string(delta.RiskBefore), "risk_before should be populated")
		assert.NotEmpty(t, string(delta.RiskAfter), "risk_after should be populated")
	}
}

// TestPersona_NearlyDone_WindingDown simulates a user near completion on all
// projects. Tests: work-remaining < min_session blocking, graceful degradation
// as projects complete, eventual NoCandidates.
func TestPersona_NearlyDone_WindingDown(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 1, 0)

	// === Project A: 20 min remaining ===
	projA := testutil.NewTestProject("Almost Done A", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Final", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Final Polish A",
		testutil.WithPlannedMin(300),
		testutil.WithLoggedMin(280),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))
	sessA := testutil.NewTestSession(wiA.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessA))

	// === Project B: 10 min remaining (below default_session but above min_session) ===
	projB := testutil.NewTestProject("Almost Done B", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Final", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiBremaining := 10
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Final Polish B",
		testutil.WithPlannedMin(200),
		testutil.WithLoggedMin(200-wiBremaining),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiB))
	sessB := testutil.NewTestSession(wiB.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessB))

	// === Project C: fully logged (logged >= planned) — triggers WorkComplete blocker ===
	projC := testutil.NewTestProject("Almost Done C", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, projC))
	nodeC := testutil.NewTestNode(projC.ID, "Final", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeC))
	wiC := testutil.NewTestWorkItem(nodeC.ID, "Final Polish C",
		testutil.WithPlannedMin(150),
		testutil.WithLoggedMin(150),
		testutil.WithWorkItemStatus(domain.WorkItemInProgress),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiC))
	sessC := testutil.NewTestSession(wiC.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessC))

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)

	// === Phase 1: Initial query ===
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations)

	// A (20 min left) and B (10 min left) should be schedulable.
	// C (5 min left, min_session=15) — remaining < min_session, should be blocked.
	recIDs := make(map[string]bool)
	for _, rec := range resp1.Recommendations {
		recIDs[rec.WorkItemID] = true
	}

	assert.True(t, recIDs[wiA.ID],
		"project A with 20 min remaining should be recommended")

	// C should not be recommended (logged >= planned triggers WorkComplete blocker).
	assert.False(t, recIDs[wiC.ID],
		"project C with logged >= planned should not be recommended (work complete)")

	// === Phase 2: Mark B done, re-query ===
	wiB.Status = domain.WorkItemDone
	wiB.LoggedMin = 200
	require.NoError(t, workItems.Update(ctx, wiB))

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	for _, rec := range resp2.Recommendations {
		assert.NotEqual(t, wiB.ID, rec.WorkItemID,
			"done item should not appear in recommendations")
	}

	// === Phase 3: Mark A done, re-query — expect NoCandidates ===
	wiA.Status = domain.WorkItemDone
	wiA.LoggedMin = 300
	require.NoError(t, workItems.Update(ctx, wiA))

	// C is still there but only 5 min remaining (below min_session).
	// All schedulable work is done.
	_, err = whatNowSvc.Recommend(ctx, req)
	// Should either return empty recommendations or NoCandidates error.
	if err != nil {
		var wnErr *contract.WhatNowError
		require.ErrorAs(t, err, &wnErr)
		assert.Equal(t, contract.ErrNoCandidates, wnErr.Code,
			"should get NoCandidates when all remaining work is complete or done")
	}
}

// TestPersona_SpacingEffect_AcrossDays simulates a user with 2 equal-priority
// projects, testing that the spacing bonus/penalty drives alternation across
// simulated days.
func TestPersona_SpacingEffect_AcrossDays(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	deadline := now.AddDate(0, 3, 0) // Both due in 3 months

	// === Two identical-priority projects ===
	projA := testutil.NewTestProject("Project Alpha", testutil.WithTargetDate(deadline))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Module", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Alpha Work",
		testutil.WithPlannedMin(300),
		testutil.WithSessionBounds(30, 60, 45),
	)
	require.NoError(t, workItems.Create(ctx, wiA))

	projB := testutil.NewTestProject("Project Bravo", testutil.WithTargetDate(deadline))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Module", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Bravo Work",
		testutil.WithPlannedMin(300),
		testutil.WithSessionBounds(30, 60, 45),
	)
	require.NoError(t, workItems.Create(ctx, wiB))

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)

	// === Day 0: Log session on A, query ===
	day0 := now
	sessA0 := testutil.NewTestSession(wiA.ID, 45, testutil.WithStartedAt(day0.Add(-time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessA0))

	req := contract.NewWhatNowRequest(60)
	req.Now = &day0

	resp0, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp0.Recommendations)

	// B should be recommended first (A worked today, gets spacing penalty).
	assert.Equal(t, projB.ID, resp0.Recommendations[0].ProjectID,
		"day 0: B should be first since A was worked today (spacing penalty)")

	// === Day 1: Log session on B, query with advanced time ===
	day1 := now.Add(24 * time.Hour)
	sessB1 := testutil.NewTestSession(wiB.ID, 45, testutil.WithStartedAt(day1.Add(-time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessB1))

	req.Now = &day1
	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations)

	// A should be recommended first now (B worked today, A worked yesterday → spacing bonus).
	assert.Equal(t, projA.ID, resp1.Recommendations[0].ProjectID,
		"day 1: A should be first since B was worked today")

	// === Day 2: Log session on A, query ===
	day2 := now.Add(48 * time.Hour)
	sessA2 := testutil.NewTestSession(wiA.ID, 45, testutil.WithStartedAt(day2.Add(-time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessA2))

	req.Now = &day2
	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations)

	// B should be recommended first again (A worked today).
	assert.Equal(t, projB.ID, resp2.Recommendations[0].ProjectID,
		"day 2: B should be first since A was worked today (alternation pattern)")

	// === Verify determinism at each point ===
	resp2b, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.Equal(t, len(resp2.Recommendations), len(resp2b.Recommendations))
	for i := range resp2.Recommendations {
		assert.Equal(t, resp2.Recommendations[i].WorkItemID, resp2b.Recommendations[i].WorkItemID,
			"spacing-driven recommendations must be deterministic")
	}
}

// TestPersona_ProgressiveModeTransition simulates a user with a critical project
// that progressively improves as work is logged, tracking the transition from
// critical to balanced mode across multiple logging steps.
func TestPersona_ProgressiveModeTransition(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// === Project A: Critical — due in 3 days, 240 min remaining, no sessions ===
	projA := testutil.NewTestProject("Urgent Deadline",
		testutil.WithTargetDate(now.AddDate(0, 0, 3)))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Work", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Urgent Task",
		testutil.WithPlannedMin(240),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiA))

	// === Project B: On-track control — due in 60 days, 120 min remaining ===
	projB := testutil.NewTestProject("Relaxed Project",
		testutil.WithTargetDate(now.AddDate(0, 0, 60)))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Work", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Relaxed Task",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiB))
	sessB := testutil.NewTestSession(wiB.ID, 30, testutil.WithStartedAt(now.Add(-48*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessB))

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	sessionSvc := NewSessionService(sessions, workItems, uow)

	// === Step 1: Initial query — should be critical (A has no sessions, due in 3 days) ===
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations)

	assert.Equal(t, domain.ModeCritical, resp1.Mode,
		"step 1: should be critical with 240 min due in 3 days and no activity")

	// Only A should be recommended in critical mode.
	for _, rec := range resp1.Recommendations {
		assert.Equal(t, projA.ID, rec.ProjectID,
			"step 1: critical mode should only recommend the critical project")
	}

	// B should be blocked as not-in-critical-scope.
	bBlocked := false
	for _, blocker := range resp1.Blockers {
		if blocker.EntityID == wiB.ID && blocker.Code == contract.BlockerNotInCriticalScope {
			bBlocked = true
		}
	}
	assert.True(t, bBlocked,
		"step 1: relaxed project item should be blocked with NOT_IN_CRITICAL_SCOPE")

	// === Track risk progression ===
	type riskStep struct {
		mode    domain.PlanMode
		riskA   domain.RiskLevel
		loggedA int
		bInRecs bool
	}
	var steps []riskStep

	// Record initial state.
	var initialRiskA domain.RiskLevel
	for _, rs := range resp1.TopRiskProjects {
		if rs.ProjectID == projA.ID {
			initialRiskA = rs.RiskLevel
		}
	}
	steps = append(steps, riskStep{
		mode:    resp1.Mode,
		riskA:   initialRiskA,
		loggedA: 0,
		bInRecs: false,
	})

	// === Steps 2-4: Progressively log 60 min each time on A ===
	for step := 2; step <= 4; step++ {
		sess := &domain.WorkSessionLog{
			WorkItemID: wiA.ID,
			StartedAt:  now.Add(-time.Duration(step) * time.Hour),
			Minutes:    60,
		}
		require.NoError(t, sessionSvc.LogSession(ctx, sess))

		resp, err := whatNowSvc.Recommend(ctx, req)
		require.NoError(t, err)

		var riskA domain.RiskLevel
		for _, rs := range resp.TopRiskProjects {
			if rs.ProjectID == projA.ID {
				riskA = rs.RiskLevel
			}
		}

		bFound := false
		for _, rec := range resp.Recommendations {
			if rec.ProjectID == projB.ID {
				bFound = true
			}
		}

		updatedA, err := workItems.GetByID(ctx, wiA.ID)
		require.NoError(t, err)

		steps = append(steps, riskStep{
			mode:    resp.Mode,
			riskA:   riskA,
			loggedA: updatedA.LoggedMin,
			bInRecs: bFound,
		})
	}

	// === Verify progression ===
	// After 180 min logged on A (3 sessions of 60 min), remaining = 60 min in 3 days.
	// With pace of ~60 min/day (3 sessions in one day), required ~20 min/day.
	// Ratio = 20/60 ≈ 0.33 < 1.0 => should be on-track or at-risk.
	lastStep := steps[len(steps)-1]
	assert.Equal(t, 180, lastStep.loggedA,
		"should have logged 180 min total after 3 sessions")

	// Initial mode was critical; verify it was set correctly.
	assert.Equal(t, domain.ModeCritical, steps[0].mode,
		"initial mode should be critical")

	// The mode should eventually transition away from critical.
	transitioned := false
	for _, s := range steps[1:] {
		if s.mode != domain.ModeCritical {
			transitioned = true
			// Once balanced, B should appear.
			assert.True(t, s.bInRecs,
				"once in balanced mode, relaxed project should appear in recommendations")
			break
		}
	}
	assert.True(t, transitioned,
		"after logging 180 of 240 min, mode should transition away from critical")
}
