package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionLogReEstimation_E2E exercises the full lifecycle:
// import project → what-now → log session with units → verify re-estimation → replan → verify convergence.
// This catches integration issues between session logging, re-estimation smoothing, and replan.
func TestSessionLogReEstimation_E2E(t *testing.T) {
	projects, _, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	// === Step 1: Import a project with units tracking ===
	importSvc := NewImportService(uow)
	targetDate := "2026-06-01"
	pm120 := 120
	pm90 := 90

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "REEST1",
			Name:       "Re-Estimation E2E",
			Domain:     "education",
			StartDate:  "2026-01-01",
			TargetDate: &targetDate,
		},
		Defaults: &importer.DefaultsImport{
			DurationMode: "estimate",
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     intPtr(15),
				MaxSessionMin:     intPtr(60),
				DefaultSessionMin: intPtr(30),
				Splittable:        boolPtr(true),
			},
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Module 1", Kind: "module", Order: 1},
			{Ref: "n2", Title: "Module 2", Kind: "module", Order: 2},
		},
		WorkItems: []importer.WorkItemImport{
			{
				Ref:        "w1",
				NodeRef:    "n1",
				Title:      "Read Chapters",
				Type:       "reading",
				PlannedMin: &pm120,
				Units:      &importer.UnitsImport{Kind: "chapters", Total: 10},
			},
			{
				Ref:        "w2",
				NodeRef:    "n2",
				Title:      "Exercises",
				Type:       "practice",
				PlannedMin: &pm90,
			},
		},
	}

	result, err := importSvc.ImportProjectFromSchema(ctx, schema)
	require.NoError(t, err)
	require.Equal(t, 2, result.WorkItemCount)

	// Find the work item IDs.
	allItems, err := workItems.ListByProject(ctx, result.Project.ID)
	require.NoError(t, err)
	require.Len(t, allItems, 2)

	var readingItem, exerciseItem *domain.WorkItem
	for _, wi := range allItems {
		if wi.Title == "Read Chapters" {
			readingItem = wi
		} else {
			exerciseItem = wi
		}
	}
	require.NotNil(t, readingItem)
	require.NotNil(t, exerciseItem)

	now := time.Now().UTC()
	originalPlannedMin := readingItem.PlannedMin

	// === Step 2: What-now should recommend the project ===
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now
	req.ProjectScope = []string{result.Project.ID}

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations, "should produce recommendations for imported project")

	// === Step 3: Log session with units on the reading item ===
	// 30 min for 2 chapters → implied pace = 15 min/chapter → 150 min for 10 chapters
	// Smooth: round(0.7*120 + 0.3*150) = round(84 + 45) = 129
	sessionSvc := NewSessionService(sessions, uow)
	sess := &domain.WorkSessionLog{
		WorkItemID:     readingItem.ID,
		StartedAt:      now.Add(-time.Hour),
		Minutes:        30,
		UnitsDoneDelta: 2,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess))

	// Verify re-estimation happened.
	updatedReading, err := workItems.GetByID(ctx, readingItem.ID)
	require.NoError(t, err)
	assert.Equal(t, 30, updatedReading.LoggedMin, "logged_min should reflect session")
	assert.Equal(t, 2, updatedReading.UnitsDone, "units_done should reflect session")
	assert.Equal(t, domain.WorkItemInProgress, updatedReading.Status, "should auto-transition to in_progress")
	assert.NotEqual(t, originalPlannedMin, updatedReading.PlannedMin,
		"planned_min should be re-estimated after session with units")

	// Verify the smoothing formula: round(0.7*120 + 0.3*150) = 129
	// implied = (30 / 2) * 10 = 150
	assert.Equal(t, 129, updatedReading.PlannedMin,
		"re-estimated planned_min should follow smoothing formula")

	// === Step 4: Log another session (verify cumulative re-estimation) ===
	// Now: logged=60, unitsDone=4 → implied = (60/4)*10 = 150
	// Smooth: round(0.7*129 + 0.3*150) = round(90.3 + 45) = 135
	sess2 := &domain.WorkSessionLog{
		WorkItemID:     readingItem.ID,
		StartedAt:      now.Add(-30 * time.Minute),
		Minutes:        30,
		UnitsDoneDelta: 2,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess2))

	updated2, err := workItems.GetByID(ctx, readingItem.ID)
	require.NoError(t, err)
	assert.Equal(t, 60, updated2.LoggedMin)
	assert.Equal(t, 4, updated2.UnitsDone)
	assert.Equal(t, 135, updated2.PlannedMin,
		"second re-estimation should use cumulative logged/units")

	// === Step 5: Log session WITHOUT units (no re-estimation expected for exercise item) ===
	exerciseOrigPlanned := exerciseItem.PlannedMin
	sess3 := &domain.WorkSessionLog{
		WorkItemID: exerciseItem.ID,
		StartedAt:  now.Add(-2 * time.Hour),
		Minutes:    45,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess3))

	updatedExercise, err := workItems.GetByID(ctx, exerciseItem.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, updatedExercise.LoggedMin)
	assert.Equal(t, exerciseOrigPlanned, updatedExercise.PlannedMin,
		"planned_min should NOT change for items without units tracking")

	// === Step 6: Replan — verify convergence ===
	replanSvc := NewReplanService(projects, workItems, sessions, profiles, uow)
	replanReq := contract.NewReplanRequest(domain.TriggerManual)
	replanReq.Now = &now

	// Run until convergence.
	for i := 0; i < 30; i++ {
		replanResp, err := replanSvc.Replan(ctx, replanReq)
		require.NoError(t, err)
		require.Len(t, replanResp.Deltas, 1)
		if replanResp.Deltas[0].ChangedItemsCount == 0 {
			break
		}
	}

	// Verify stability: two more replans should produce zero changes.
	stableResp, err := replanSvc.Replan(ctx, replanReq)
	require.NoError(t, err)
	assert.Equal(t, 0, stableResp.Deltas[0].ChangedItemsCount,
		"replan should converge to zero changes")

	// === Step 7: What-now should still work after all the re-estimation ===
	finalResp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, finalResp.Recommendations,
		"should still produce recommendations after session logging and replan")

	// Verify determinism.
	finalResp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.Equal(t, len(finalResp.Recommendations), len(finalResp2.Recommendations))
	for i := range finalResp.Recommendations {
		assert.Equal(t, finalResp.Recommendations[i].WorkItemID, finalResp2.Recommendations[i].WorkItemID,
			"recommendations should be deterministic after convergence")
	}
}
