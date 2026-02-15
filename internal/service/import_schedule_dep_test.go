package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportWithDependencies_SchedulerRespectsDeps verifies that after importing
// a project with a dependency graph, what-now correctly blocks items whose
// predecessors are unfinished.
func TestImportWithDependencies_SchedulerRespectsDeps(t *testing.T) {
	_, _, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(uow)

	// Import a project with chain: w1 → w2 → w3
	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "DEP01",
			Name:       "Dependency Chain",
			Domain:     "education",
			StartDate:  "2026-01-01",
			TargetDate: ptrStr("2026-06-01"),
		},
		Defaults: &importer.DefaultsImport{
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     ptrInt(15),
				MaxSessionMin:     ptrInt(60),
				DefaultSessionMin: ptrInt(30),
				Splittable:        ptrBool(true),
			},
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Module 1", Kind: "module", Order: 0},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Foundation", Type: "reading", PlannedMin: ptrInt(60)},
			{Ref: "w2", NodeRef: "n1", Title: "Practice", Type: "assignment", PlannedMin: ptrInt(45)},
			{Ref: "w3", NodeRef: "n1", Title: "Assessment", Type: "quiz", PlannedMin: ptrInt(30)},
		},
		Dependencies: []importer.DependencyImport{
			{PredecessorRef: "w1", SuccessorRef: "w2"}, // w1 must finish before w2
			{PredecessorRef: "w2", SuccessorRef: "w3"}, // w2 must finish before w3
		},
	}

	path := writeImportJSON(t, schema)
	result, err := svc.ImportProject(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, 3, result.WorkItemCount)
	assert.Equal(t, 2, result.DependencyCount)

	// Schedule: w2 and w3 should be dependency-blocked
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	now := time.Now().UTC()
	req := contract.NewWhatNowRequest(120)
	req.Now = &now
	req.ProjectScope = []string{result.Project.ID}

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	// Only w1 (no predecessors) should be recommended
	for _, rec := range resp.Recommendations {
		assert.Equal(t, "Foundation", rec.Title,
			"only the item with no unfinished predecessors should be recommended")
	}

	// w2 and w3 should appear as dependency-blocked
	depBlockedIDs := make(map[string]bool)
	for _, b := range resp.Blockers {
		if b.Code == contract.BlockerDependency {
			depBlockedIDs[b.EntityID] = true
		}
	}
	assert.Len(t, depBlockedIDs, 2, "w2 and w3 should both be dependency-blocked")

	// Now complete w1 and re-check: w2 should become available
	allItems, err := workItems.ListByProject(ctx, result.Project.ID)
	require.NoError(t, err)

	var w1ID, w2Title string
	for _, wi := range allItems {
		if wi.Title == "Foundation" {
			w1ID = wi.ID
		}
		if wi.Title == "Practice" {
			w2Title = wi.Title
		}
	}
	require.NotEmpty(t, w1ID, "should find w1")
	require.NotEmpty(t, w2Title, "should find w2")

	// Mark w1 as done
	w1, err := workItems.GetByID(ctx, w1ID)
	require.NoError(t, err)
	w1.Status = "done"
	w1.LoggedMin = 60
	require.NoError(t, workItems.Update(ctx, w1))

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	// w2 should now be recommended (w1 is done), but w3 should still be blocked
	foundW2 := false
	for _, rec := range resp2.Recommendations {
		if rec.Title == "Practice" {
			foundW2 = true
		}
		assert.NotEqual(t, "Assessment", rec.Title,
			"w3 should still be blocked (w2 not done yet)")
	}
	assert.True(t, foundW2, "w2 should be recommended after w1 is completed")
}
