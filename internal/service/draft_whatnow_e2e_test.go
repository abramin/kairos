package service

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDraftWizard_ImportSchema_ThenWhatNow_E2E simulates the no-LLM draft wizard
// producing an ImportSchema, importing it, and then running what-now to verify
// the drafted project is schedulable. This is the primary onboarding flow.
func TestDraftWizard_ImportSchema_ThenWhatNow_E2E(t *testing.T) {
	_, _, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	// Simulate what buildSchemaFromWizard() produces: a multi-node project
	// with work items stamped on each node.
	targetDate := "2026-09-01"
	pm60 := 60
	pm45 := 45

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "DW01",
			Name:       "Draft Wizard E2E",
			Domain:     "education",
			StartDate:  "2026-03-01",
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
			{Ref: "n1", Title: "Chapter 1", Kind: "module", Order: 1},
			{Ref: "n2", Title: "Chapter 2", Kind: "module", Order: 2},
			{Ref: "n3", Title: "Chapter 3", Kind: "module", Order: 3},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Reading", Type: "reading", PlannedMin: &pm60},
			{Ref: "w2", NodeRef: "n1", Title: "Exercises", Type: "practice", PlannedMin: &pm45},
			{Ref: "w3", NodeRef: "n2", Title: "Reading", Type: "reading", PlannedMin: &pm60},
			{Ref: "w4", NodeRef: "n2", Title: "Exercises", Type: "practice", PlannedMin: &pm45},
			{Ref: "w5", NodeRef: "n3", Title: "Reading", Type: "reading", PlannedMin: &pm60},
			{Ref: "w6", NodeRef: "n3", Title: "Exercises", Type: "practice", PlannedMin: &pm45},
		},
	}

	// Validate before import (same as wizard does).
	errs := importer.ValidateImportSchema(schema)
	require.Empty(t, errs, "wizard-produced schema should be valid: %v", errs)

	// Import.
	importSvc := NewImportService(uow)
	result, err := importSvc.ImportProjectFromSchema(ctx, schema)
	require.NoError(t, err)
	assert.Equal(t, "DW01", result.Project.ShortID)
	assert.Equal(t, 3, result.NodeCount)
	assert.Equal(t, 6, result.WorkItemCount)

	// Run what-now → verify items are schedulable.
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(90)

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations,
		"draft wizard output should produce schedulable work items")

	// Verify allocation invariants.
	assert.LessOrEqual(t, resp.AllocatedMin, resp.RequestedMin)
	for _, rec := range resp.Recommendations {
		assert.GreaterOrEqual(t, rec.AllocatedMin, rec.MinSessionMin)
		assert.LessOrEqual(t, rec.AllocatedMin, rec.MaxSessionMin)
	}

	// Verify all recommendations come from our drafted project.
	for _, rec := range resp.Recommendations {
		assert.Equal(t, result.Project.ID, rec.ProjectID,
			"all recommendations should come from the draft project")
	}
}

// TestDraftWizard_WithSpecialNode_ThenStatus_E2E tests a draft schema
// that includes a special assessment node (mimicking the wizard's special
// node phase) and verifies status reporting works correctly.
func TestDraftWizard_WithSpecialNode_ThenStatus_E2E(t *testing.T) {
	projects, _, workItems, _, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	targetDate := "2026-12-01"
	examDue := "2026-11-25"
	pm60 := 60
	pm120 := 120

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "DW02",
			Name:       "Draft With Exam",
			Domain:     "education",
			StartDate:  "2026-03-01",
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
			{Ref: "n2", Title: "Final Exam", Kind: "assessment", Order: 2, DueDate: &examDue},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Study", Type: "reading", PlannedMin: &pm60},
			{Ref: "w2", NodeRef: "n2", Title: "Exam Prep", Type: "review", PlannedMin: &pm120},
		},
	}

	errs := importer.ValidateImportSchema(schema)
	require.Empty(t, errs)

	importSvc := NewImportService(uow)
	result, err := importSvc.ImportProjectFromSchema(ctx, schema)
	require.NoError(t, err)

	// Status should show the project.
	statusSvc := NewStatusService(projects, workItems, sessions, profiles)
	statusReq := contract.NewStatusRequest()
	statusResp, err := statusSvc.GetStatus(ctx, statusReq)
	require.NoError(t, err)
	require.Len(t, statusResp.Projects, 1)
	assert.Equal(t, result.Project.ID, statusResp.Projects[0].ProjectID)
}
