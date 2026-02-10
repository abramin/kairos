package service

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/importer"
)

// These tests exercise the full import→what-now pipeline with diverse schema shapes
// that go beyond the existing contract tests. Each tests a different real-world
// scenario to catch edge cases in validation, conversion, and scheduling.

// TestSchemaContract_NestedHierarchyProducesValidSchema tests a deeply nested
// node structure: project → chapter → section → subsection.
func TestSchemaContract_NestedHierarchyProducesValidSchema(t *testing.T) {
	targetDate := "2026-09-01"
	pm60 := 60
	pm30 := 30

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "NEST01",
			Name:       "Nested Hierarchy",
			Domain:     "education",
			StartDate:  "2026-01-15",
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
			{Ref: "ch1", Title: "Chapter 1", Kind: "module", Order: 1},
			{Ref: "ch1_s1", ParentRef: ptrStr("ch1"), Title: "Section 1.1", Kind: "section", Order: 1},
			{Ref: "ch1_s2", ParentRef: ptrStr("ch1"), Title: "Section 1.2", Kind: "section", Order: 2},
			{Ref: "ch2", Title: "Chapter 2", Kind: "module", Order: 2},
			{Ref: "ch2_s1", ParentRef: ptrStr("ch2"), Title: "Section 2.1", Kind: "section", Order: 1},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "ch1_s1", Title: "Read 1.1", Type: "reading", PlannedMin: &pm60},
			{Ref: "w2", NodeRef: "ch1_s1", Title: "Exercises 1.1", Type: "practice", PlannedMin: &pm30},
			{Ref: "w3", NodeRef: "ch1_s2", Title: "Read 1.2", Type: "reading", PlannedMin: &pm60},
			{Ref: "w4", NodeRef: "ch2_s1", Title: "Read 2.1", Type: "reading", PlannedMin: &pm60},
		},
	}

	assertSchemaContractHolds(t, schema, "nested-hierarchy", 5, 4)
}

// TestSchemaContract_LargeProjectProducesValidSchema tests a project with many
// nodes and work items to verify the pipeline handles volume correctly.
func TestSchemaContract_LargeProjectProducesValidSchema(t *testing.T) {
	targetDate := "2026-12-31"
	pm45 := 45

	nodes := make([]importer.NodeImport, 12)
	workItems := make([]importer.WorkItemImport, 0, 24)

	for i := 0; i < 12; i++ {
		ref := "n" + string(rune('a'+i))
		nodes[i] = importer.NodeImport{
			Ref:   ref,
			Title: "Week " + string(rune('A'+i)),
			Kind:  "week",
			Order: i + 1,
		}
		// Two work items per node
		workItems = append(workItems,
			importer.WorkItemImport{
				Ref:        ref + "_r",
				NodeRef:    ref,
				Title:      "Reading " + string(rune('A'+i)),
				Type:       "reading",
				PlannedMin: &pm45,
			},
			importer.WorkItemImport{
				Ref:        ref + "_e",
				NodeRef:    ref,
				Title:      "Exercise " + string(rune('A'+i)),
				Type:       "practice",
				PlannedMin: &pm45,
			},
		)
	}

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "LRG01",
			Name:       "Large Course",
			Domain:     "education",
			StartDate:  "2026-01-01",
			TargetDate: &targetDate,
		},
		Defaults: &importer.DefaultsImport{
			DurationMode: "estimate",
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     intPtr(15),
				MaxSessionMin:     intPtr(90),
				DefaultSessionMin: intPtr(45),
				Splittable:        boolPtr(true),
			},
		},
		Nodes:     nodes,
		WorkItems: workItems,
	}

	assertSchemaContractHolds(t, schema, "large-project", 12, 24)
}

// TestSchemaContract_MixedNodeKindsProducesValidSchema tests a project that mixes
// different node kinds (week, module, assessment, generic) with custom session policies.
func TestSchemaContract_MixedNodeKindsProducesValidSchema(t *testing.T) {
	targetDate := "2026-07-15"
	pm90 := 90
	pm120 := 120
	pm30 := 30
	examDue := "2026-07-10"

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "MIX01",
			Name:       "Mixed Kinds Project",
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
			{Ref: "w1", Title: "Week 1", Kind: "week", Order: 1},
			{Ref: "m1", Title: "Module A", Kind: "module", Order: 2},
			{Ref: "s1", Title: "Section A.1", Kind: "section", Order: 3},
			{Ref: "a1", Title: "Midterm", Kind: "assessment", Order: 4, DueDate: &examDue},
			{Ref: "g1", Title: "Miscellaneous", Kind: "generic", Order: 5},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "wi1", NodeRef: "w1", Title: "Weekly Reading", Type: "reading", PlannedMin: &pm90},
			{Ref: "wi2", NodeRef: "m1", Title: "Module Work", Type: "assignment", PlannedMin: &pm120},
			{Ref: "wi3", NodeRef: "s1", Title: "Section Notes", Type: "reading", PlannedMin: &pm30},
			{Ref: "wi4", NodeRef: "a1", Title: "Exam Prep", Type: "review", PlannedMin: &pm120},
			{Ref: "wi5", NodeRef: "g1", Title: "Office Hours", Type: "task", PlannedMin: &pm30},
		},
	}

	assertSchemaContractHolds(t, schema, "mixed-kinds", 5, 5)
}

// TestSchemaContract_UnitsTrackingProducesValidSchema tests that a schema with
// units tracking (chapters, pages) validates and imports correctly.
func TestSchemaContract_UnitsTrackingProducesValidSchema(t *testing.T) {
	targetDate := "2026-04-30"
	pm180 := 180
	pm60 := 60

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "UNT01",
			Name:       "Units Tracking Project",
			Domain:     "education",
			StartDate:  "2026-02-01",
			TargetDate: &targetDate,
		},
		Defaults: &importer.DefaultsImport{
			DurationMode: "estimate",
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     intPtr(20),
				MaxSessionMin:     intPtr(90),
				DefaultSessionMin: intPtr(45),
				Splittable:        boolPtr(true),
			},
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Textbook", Kind: "book", Order: 1},
		},
		WorkItems: []importer.WorkItemImport{
			{
				Ref:        "w1",
				NodeRef:    "n1",
				Title:      "Read Full Textbook",
				Type:       "reading",
				PlannedMin: &pm180,
				Units:      &importer.UnitsImport{Kind: "chapters", Total: 12},
			},
			{
				Ref:        "w2",
				NodeRef:    "n1",
				Title:      "Problem Sets",
				Type:       "practice",
				PlannedMin: &pm60,
				Units:      &importer.UnitsImport{Kind: "sets", Total: 6},
			},
		},
	}

	assertSchemaContractHolds(t, schema, "units-tracking", 1, 2)
}

// TestSchemaContract_NoDueDateProject tests a project with no target date
// (open-ended project) to verify the pipeline handles nil dates.
func TestSchemaContract_NoDueDateProject(t *testing.T) {
	pm60 := 60

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   "OPN01",
			Name:      "Open-Ended Project",
			Domain:    "personal",
			StartDate: "2026-01-01",
			// No TargetDate
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Phase 1", Kind: "stage", Order: 1},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Initial Research", Type: "research", PlannedMin: &pm60},
			{Ref: "w2", NodeRef: "n1", Title: "Write Draft", Type: "writing", PlannedMin: &pm60},
		},
	}

	assertSchemaContractHolds(t, schema, "no-due-date", 1, 2)
}
