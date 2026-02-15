package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests verify that every way of producing an ImportSchema — wizard,
// LLM draft, and JSON file — satisfies the same contract: validates, converts,
// imports into DB, and yields schedulable what-now candidates.

func TestSchemaContract_WizardStyleProducesValidSchema(t *testing.T) {
	// Reproduce what buildSchemaFromWizard creates: a multi-group schema with
	// work items stamped on every node plus a special assessment node.
	targetDate := "2026-06-01"
	pm60 := 60
	pm45 := 45
	pm120 := 120
	dueExam := "2026-05-28"

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "PHYS01",
			Name:       "Physics Exam Prep",
			Domain:     "education",
			StartDate:  "2026-02-01",
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
			{Ref: "n3", Title: "Module 3", Kind: "module", Order: 3},
			{Ref: "n4", Title: "Lab 1", Kind: "section", Order: 4},
			{Ref: "n5", Title: "Lab 2", Kind: "section", Order: 5},
			{Ref: "n6", Title: "Final Exam", Kind: "assessment", Order: 6, DueDate: &dueExam},
		},
		WorkItems: []importer.WorkItemImport{
			// Work items stamped on each module (3 modules * 2 items = 6)
			{Ref: "w1", NodeRef: "n1", Title: "Reading", Type: "reading", PlannedMin: &pm60},
			{Ref: "w2", NodeRef: "n1", Title: "Exercises", Type: "practice", PlannedMin: &pm45},
			{Ref: "w3", NodeRef: "n2", Title: "Reading", Type: "reading", PlannedMin: &pm60},
			{Ref: "w4", NodeRef: "n2", Title: "Exercises", Type: "practice", PlannedMin: &pm45},
			{Ref: "w5", NodeRef: "n3", Title: "Reading", Type: "reading", PlannedMin: &pm60},
			{Ref: "w6", NodeRef: "n3", Title: "Exercises", Type: "practice", PlannedMin: &pm45},
			// Work items on labs (2 labs * 2 items = 4)
			{Ref: "w7", NodeRef: "n4", Title: "Reading", Type: "reading", PlannedMin: &pm60},
			{Ref: "w8", NodeRef: "n4", Title: "Exercises", Type: "practice", PlannedMin: &pm45},
			{Ref: "w9", NodeRef: "n5", Title: "Reading", Type: "reading", PlannedMin: &pm60},
			{Ref: "w10", NodeRef: "n5", Title: "Exercises", Type: "practice", PlannedMin: &pm45},
			// Special node work item
			{Ref: "w11", NodeRef: "n6", Title: "Exam Review", Type: "review", PlannedMin: &pm120},
		},
	}

	assertSchemaContractHolds(t, schema, "wizard-style", 6, 11)
}

func TestSchemaContract_SampleJSONProducesValidSchema(t *testing.T) {
	samplePath := findProjectSample(t)
	schema, err := importer.LoadImportSchema(samplePath)
	require.NoError(t, err, "sample JSON should load")

	assertSchemaContractHolds(t, schema, "sample-json", 3, 6)
}

func TestSchemaContract_LLMDraftStyleProducesValidSchema(t *testing.T) {
	// Simulate what an LLM draft produces: a compact schema with dependencies.
	targetDate := "2026-05-15"
	pm60 := 60
	pm120 := 120

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "LLM01",
			Name:       "LLM-Drafted Study Plan",
			Domain:     "education",
			StartDate:  "2026-01-10",
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
			{Ref: "n1", Title: "Week 1", Kind: "week", Order: 1},
			{Ref: "n2", Title: "Week 2", Kind: "week", Order: 2},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Read Chapter 1", Type: "reading", PlannedMin: &pm120},
			{Ref: "w2", NodeRef: "n1", Title: "Exercises 1", Type: "practice", PlannedMin: &pm60},
			{Ref: "w3", NodeRef: "n2", Title: "Read Chapter 2", Type: "reading", PlannedMin: &pm120},
		},
		Dependencies: []importer.DependencyImport{
			{PredecessorRef: "w1", SuccessorRef: "w2"},
			{PredecessorRef: "w2", SuccessorRef: "w3"},
		},
	}

	assertSchemaContractHolds(t, schema, "llm-draft-style", 2, 3)
}

// assertSchemaContractHolds verifies the full ImportSchema contract:
// validation → conversion → import → schedulable → what-now recommendations.
func assertSchemaContractHolds(
	t *testing.T,
	schema *importer.ImportSchema,
	producer string,
	expectedNodes, expectedWorkItems int,
) {
	t.Helper()
	ctx := context.Background()

	// Contract 1: Schema must pass validation.
	errs := importer.ValidateImportSchema(schema)
	require.Empty(t, errs, "%s: schema validation failed: %v", producer, errs)

	// Contract 2: Schema must convert to domain objects without error.
	generated, err := importer.Convert(schema)
	require.NoError(t, err, "%s: conversion failed", producer)
	require.NotNil(t, generated.Project, "%s: nil project after conversion", producer)
	assert.Equal(t, expectedNodes, len(generated.Nodes),
		"%s: unexpected node count after conversion", producer)
	assert.Equal(t, expectedWorkItems, len(generated.WorkItems),
		"%s: unexpected work item count after conversion", producer)

	// Contract 3: All generated entities must have non-empty IDs.
	assert.NotEmpty(t, generated.Project.ID, "%s: project has empty ID", producer)
	for _, n := range generated.Nodes {
		assert.NotEmpty(t, n.ID, "%s: node %q has empty ID", producer, n.Title)
		assert.Equal(t, generated.Project.ID, n.ProjectID,
			"%s: node %q has wrong project ID", producer, n.Title)
	}
	for _, wi := range generated.WorkItems {
		assert.NotEmpty(t, wi.ID, "%s: work item %q has empty ID", producer, wi.Title)
		assert.NotEmpty(t, wi.NodeID, "%s: work item %q has empty node ID", producer, wi.Title)
	}

	// Contract 4: Schema must import into the DB via ImportService.
	projRepo, nodeRepo, wiRepo, depRepo, sessRepo, profRepo, uow := setupRepos(t)
	importSvc := NewImportService(projRepo, nodeRepo, wiRepo, depRepo, uow)
	result, err := importSvc.ImportProjectFromSchema(ctx, schema)
	require.NoError(t, err, "%s: import failed", producer)
	require.NotNil(t, result.Project, "%s: import returned nil project", producer)
	assert.Equal(t, expectedNodes, result.NodeCount,
		"%s: import node count mismatch", producer)
	assert.Equal(t, expectedWorkItems, result.WorkItemCount,
		"%s: import work item count mismatch", producer)

	// Contract 5: Imported items must appear as schedulable candidates.
	assertSchedulableAfterImport(t, ctx, wiRepo, result.Project.ID, producer)

	// Contract 6: What-now pipeline must produce recommendations for the imported project.
	assertWhatNowRecommends(t, ctx, wiRepo, sessRepo, projRepo, depRepo, profRepo, result.Project.ID, producer)
}

func assertSchedulableAfterImport(
	t *testing.T, ctx context.Context,
	wiRepo repository.WorkItemRepo,
	projectID, producer string,
) {
	t.Helper()

	candidates, err := wiRepo.ListSchedulable(ctx, false)
	require.NoError(t, err, "%s: list schedulable failed", producer)

	schedulableCount := 0
	for _, c := range candidates {
		if c.ProjectID == projectID {
			schedulableCount++
			assert.NotEmpty(t, c.WorkItem.ID,
				"%s: schedulable candidate missing work item ID", producer)
			assert.NotEmpty(t, c.ProjectID,
				"%s: schedulable candidate missing project ID", producer)
		}
	}
	assert.Greater(t, schedulableCount, 0,
		"%s: no schedulable candidates from imported project", producer)
}

func assertWhatNowRecommends(
	t *testing.T, ctx context.Context,
	wiRepo repository.WorkItemRepo,
	sessRepo repository.SessionRepo,
	projRepo repository.ProjectRepo,
	depRepo repository.DependencyRepo,
	profRepo repository.UserProfileRepo,
	projectID, producer string,
) {
	t.Helper()

	whatNowSvc := NewWhatNowService(wiRepo, sessRepo, depRepo, profRepo)
	now := time.Now().UTC()
	req := contract.NewWhatNowRequest(90)
	req.Now = &now
	req.ProjectScope = []string{projectID}

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err, "%s: what-now failed", producer)
	require.NotEmpty(t, resp.Recommendations,
		"%s: what-now produced no recommendations for imported project", producer)
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

// findProjectSample locates docs/project-sample.json relative to the test file.
func findProjectSample(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		candidate := filepath.Join(dir, "docs", "project-sample.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find docs/project-sample.json")
		}
		dir = parent
	}
}
