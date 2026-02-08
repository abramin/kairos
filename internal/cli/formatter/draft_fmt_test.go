package formatter

import (
	"fmt"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/stretchr/testify/assert"
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func testDraft() *importer.ImportSchema {
	return &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "PHYS01",
			Name:       "Physics 101",
			Domain:     "education",
			StartDate:  "2025-02-01",
			TargetDate: strPtr("2025-06-15"),
		},
		Defaults: &importer.DefaultsImport{
			DurationMode: "estimate",
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     intPtr(20),
				MaxSessionMin:     intPtr(90),
				DefaultSessionMin: intPtr(45),
			},
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Chapter 1: Mechanics", Kind: "module", Order: 0},
			{Ref: "n2", Title: "Chapter 2: Thermo", Kind: "module", Order: 1},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Read Chapter 1", Type: "reading", PlannedMin: intPtr(90)},
			{Ref: "w2", NodeRef: "n1", Title: "Practice Problems 1", Type: "practice", PlannedMin: intPtr(60)},
			{Ref: "w3", NodeRef: "n2", Title: "Read Chapter 2", Type: "reading", PlannedMin: intPtr(90)},
		},
		Dependencies: []importer.DependencyImport{
			{PredecessorRef: "w1", SuccessorRef: "w2"},
		},
	}
}

func TestFormatDraftTurn_ShowsMessageAndSummary(t *testing.T) {
	conv := &intelligence.DraftConversation{
		LLMMessage: "What is the target date for this project?",
		Draft:      testDraft(),
		Status:     intelligence.DraftStatusGathering,
	}

	out := FormatDraftTurn(conv)
	assert.Contains(t, out, "What is the target date")
	assert.Contains(t, out, "2 nodes")
	assert.Contains(t, out, "3 work items")
}

func TestFormatDraftTurn_NoDraft(t *testing.T) {
	conv := &intelligence.DraftConversation{
		LLMMessage: "Tell me about your project.",
		Status:     intelligence.DraftStatusGathering,
	}

	out := FormatDraftTurn(conv)
	assert.Contains(t, out, "Tell me about your project")
	assert.NotContains(t, out, "nodes")
}

func TestFormatDraftPreview_ShowsTree(t *testing.T) {
	conv := &intelligence.DraftConversation{
		Draft:  testDraft(),
		Status: intelligence.DraftStatusGathering,
	}

	out := FormatDraftPreview(conv)
	assert.Contains(t, out, "DRAFT PREVIEW")
	assert.Contains(t, out, "Physics 101")
	assert.Contains(t, out, "PHYS01")
	assert.Contains(t, out, "Chapter 1: Mechanics")
	assert.Contains(t, out, "Read Chapter 1")
	assert.Contains(t, out, "1h 30m")
}

func TestFormatDraftPreview_NilDraft(t *testing.T) {
	conv := &intelligence.DraftConversation{}
	out := FormatDraftPreview(conv)
	assert.Contains(t, out, "No draft yet")
}

func TestFormatDraftReview_ShowsReadyMessage(t *testing.T) {
	conv := &intelligence.DraftConversation{
		Draft:  testDraft(),
		Status: intelligence.DraftStatusReady,
	}

	out := FormatDraftReview(conv)
	assert.Contains(t, out, "DRAFT REVIEW")
	assert.Contains(t, out, "Draft is ready for review")
	assert.Contains(t, out, "Physics 101")
}

func TestFormatDraftValidationErrors_ShowsAllErrors(t *testing.T) {
	errs := []error{
		fmt.Errorf("project.short_id is required"),
		fmt.Errorf("node[0].kind is invalid"),
	}

	out := FormatDraftValidationErrors(errs)
	assert.Contains(t, out, "2 errors")
	assert.Contains(t, out, "project.short_id is required")
	assert.Contains(t, out, "node[0].kind is invalid")
}

func TestFormatDraftAccepted_ShowsResult(t *testing.T) {
	result := &service.ImportResult{
		Project: &domain.Project{
			Name:    "Physics 101",
			ShortID: "PHYS01",
		},
		NodeCount:       3,
		WorkItemCount:   12,
		DependencyCount: 5,
	}

	out := FormatDraftAccepted(result)
	assert.Contains(t, out, "Project created successfully")
	assert.Contains(t, out, "Physics 101")
	assert.Contains(t, out, "PHYS01")
	assert.Contains(t, out, "3 nodes")
	assert.Contains(t, out, "12 work items")
	assert.Contains(t, out, "5 dependencies")
}

func TestFormatDraftPreview_NestedNodes(t *testing.T) {
	draft := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   "TEST01",
			Name:      "Test Project",
			Domain:    "testing",
			StartDate: "2025-01-01",
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Phase 1", Kind: "stage", Order: 0},
			{Ref: "n1s1", ParentRef: strPtr("n1"), Title: "Section 1.1", Kind: "section", Order: 0},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1s1", Title: "Task A", Type: "task", PlannedMin: intPtr(30)},
		},
	}

	conv := &intelligence.DraftConversation{Draft: draft}
	out := FormatDraftPreview(conv)
	assert.Contains(t, out, "Phase 1")
	assert.Contains(t, out, "Section 1.1")
	assert.Contains(t, out, "Task A")
}
