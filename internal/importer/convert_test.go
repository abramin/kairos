package importer

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvert_MinimalProject(t *testing.T) {
	schema := validMinimalSchema()

	gen, err := Convert(schema)
	require.NoError(t, err)

	// Project
	assert.NotEmpty(t, gen.Project.ID)
	assert.Equal(t, "PHI01", gen.Project.ShortID)
	assert.Equal(t, "Test Project", gen.Project.Name)
	assert.Equal(t, "test", gen.Project.Domain)
	assert.Equal(t, domain.ProjectActive, gen.Project.Status)
	assert.Nil(t, gen.Project.TargetDate)

	// Nodes
	require.Len(t, gen.Nodes, 1)
	assert.NotEmpty(t, gen.Nodes[0].ID)
	assert.Equal(t, gen.Project.ID, gen.Nodes[0].ProjectID)
	assert.Equal(t, "Node 1", gen.Nodes[0].Title)
	assert.Equal(t, domain.NodeModule, gen.Nodes[0].Kind)
	assert.Nil(t, gen.Nodes[0].ParentID)

	// Work items
	require.Len(t, gen.WorkItems, 1)
	assert.NotEmpty(t, gen.WorkItems[0].ID)
	assert.Equal(t, gen.Nodes[0].ID, gen.WorkItems[0].NodeID)
	assert.Equal(t, "Task 1", gen.WorkItems[0].Title)
	assert.Equal(t, "reading", gen.WorkItems[0].Type)

	// No dependencies
	assert.Empty(t, gen.Dependencies)
}

func TestConvert_FullProjectWithHierarchy(t *testing.T) {
	schema := &ImportSchema{
		Project: ProjectImport{
			ShortID:    "MATH01",
			Name:       "Mathematics",
			Domain:     "education",
			StartDate:  "2025-02-01",
			TargetDate: ptrStr("2025-06-01"),
		},
		Nodes: []NodeImport{
			{Ref: "ch1", Title: "Chapter 1", Kind: "module", Order: 0},
			{Ref: "ch1_s1", ParentRef: ptrStr("ch1"), Title: "Section 1.1", Kind: "section", Order: 0},
			{Ref: "ch2", Title: "Chapter 2", Kind: "module", Order: 1},
		},
		WorkItems: []WorkItemImport{
			{Ref: "w1", NodeRef: "ch1_s1", Title: "Read", Type: "reading", PlannedMin: ptrInt(45)},
			{Ref: "w2", NodeRef: "ch1_s1", Title: "Exercises", Type: "assignment", PlannedMin: ptrInt(30)},
			{Ref: "w3", NodeRef: "ch2", Title: "Read Ch2", Type: "reading"},
		},
		Dependencies: []DependencyImport{
			{PredecessorRef: "w1", SuccessorRef: "w2"},
		},
	}

	gen, err := Convert(schema)
	require.NoError(t, err)

	// Project
	assert.NotNil(t, gen.Project.TargetDate)

	// Nodes
	require.Len(t, gen.Nodes, 3)
	// ch1 is root
	assert.Nil(t, gen.Nodes[0].ParentID)
	// ch1_s1 has ch1 as parent
	require.NotNil(t, gen.Nodes[1].ParentID)
	assert.Equal(t, gen.Nodes[0].ID, *gen.Nodes[1].ParentID)
	// ch2 is root
	assert.Nil(t, gen.Nodes[2].ParentID)

	// Work items
	require.Len(t, gen.WorkItems, 3)
	assert.Equal(t, gen.Nodes[1].ID, gen.WorkItems[0].NodeID) // w1 -> ch1_s1
	assert.Equal(t, gen.Nodes[1].ID, gen.WorkItems[1].NodeID) // w2 -> ch1_s1
	assert.Equal(t, gen.Nodes[2].ID, gen.WorkItems[2].NodeID) // w3 -> ch2

	// Dependencies
	require.Len(t, gen.Dependencies, 1)
	assert.Equal(t, gen.WorkItems[0].ID, gen.Dependencies[0].PredecessorWorkItemID) // w1
	assert.Equal(t, gen.WorkItems[1].ID, gen.Dependencies[0].SuccessorWorkItemID)   // w2
}

func TestConvert_DefaultsApplication(t *testing.T) {
	schema := validMinimalSchema()

	gen, err := Convert(schema)
	require.NoError(t, err)

	wi := gen.WorkItems[0]
	assert.Equal(t, domain.WorkItemTodo, wi.Status)
	assert.Equal(t, domain.DurationEstimate, wi.DurationMode)
	assert.Equal(t, domain.SourceManual, wi.DurationSource)
	assert.Equal(t, 0, wi.PlannedMin)
	assert.Equal(t, 0.5, wi.EstimateConfidence)
	assert.Equal(t, 15, wi.MinSessionMin)
	assert.Equal(t, 60, wi.MaxSessionMin)
	assert.Equal(t, 30, wi.DefaultSessionMin)
	assert.True(t, wi.Splittable)
	assert.Equal(t, 0, wi.LoggedMin)
	assert.Equal(t, 0, wi.UnitsDone)
}

func TestConvert_DefaultsCascade(t *testing.T) {
	schema := &ImportSchema{
		Project: ProjectImport{
			ShortID:   "TST01",
			Name:      "Test",
			Domain:    "test",
			StartDate: "2025-01-01",
		},
		Defaults: &DefaultsImport{
			DurationMode: "fixed",
			SessionPolicy: &SessionPolicyImport{
				MinSessionMin:     ptrInt(20),
				MaxSessionMin:     ptrInt(90),
				DefaultSessionMin: ptrInt(45),
				Splittable:        ptrBool(false),
			},
		},
		Nodes: []NodeImport{
			{Ref: "n1", Title: "Node", Kind: "generic"},
		},
		WorkItems: []WorkItemImport{
			// w1: no overrides, should use schema defaults
			{Ref: "w1", NodeRef: "n1", Title: "Task 1", Type: "task"},
			// w2: overrides session policy partially
			{Ref: "w2", NodeRef: "n1", Title: "Task 2", Type: "task",
				DurationMode: "estimate",
				SessionPolicy: &SessionPolicyImport{
					MinSessionMin: ptrInt(10),
				},
			},
		},
	}

	gen, err := Convert(schema)
	require.NoError(t, err)
	require.Len(t, gen.WorkItems, 2)

	// w1: schema defaults
	w1 := gen.WorkItems[0]
	assert.Equal(t, domain.DurationFixed, w1.DurationMode)
	assert.Equal(t, 20, w1.MinSessionMin)
	assert.Equal(t, 90, w1.MaxSessionMin)
	assert.Equal(t, 45, w1.DefaultSessionMin)
	assert.False(t, w1.Splittable)

	// w2: work item overrides schema defaults
	w2 := gen.WorkItems[1]
	assert.Equal(t, domain.DurationEstimate, w2.DurationMode)
	assert.Equal(t, 10, w2.MinSessionMin)
	// max/default/splittable fall through to schema defaults
	assert.Equal(t, 90, w2.MaxSessionMin)
	assert.Equal(t, 45, w2.DefaultSessionMin)
	assert.False(t, w2.Splittable)
}

func TestConvert_DateParsing(t *testing.T) {
	schema := &ImportSchema{
		Project: ProjectImport{
			ShortID:    "TST01",
			Name:       "Test",
			Domain:     "test",
			StartDate:  "2025-03-15",
			TargetDate: ptrStr("2025-09-30"),
		},
		Nodes: []NodeImport{
			{Ref: "n1", Title: "Node", Kind: "generic",
				DueDate: ptrStr("2025-06-01"), NotBefore: ptrStr("2025-03-15"), NotAfter: ptrStr("2025-06-15")},
		},
		WorkItems: []WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task", Type: "task",
				DueDate: ptrStr("2025-05-01"), NotBefore: ptrStr("2025-04-01")},
		},
	}

	gen, err := Convert(schema)
	require.NoError(t, err)

	// Project dates
	assert.Equal(t, 2025, gen.Project.StartDate.Year())
	assert.Equal(t, 3, int(gen.Project.StartDate.Month()))
	assert.Equal(t, 15, gen.Project.StartDate.Day())
	require.NotNil(t, gen.Project.TargetDate)
	assert.Equal(t, 9, int(gen.Project.TargetDate.Month()))

	// Node dates
	require.NotNil(t, gen.Nodes[0].DueDate)
	assert.Equal(t, 6, int(gen.Nodes[0].DueDate.Month()))
	require.NotNil(t, gen.Nodes[0].NotBefore)
	require.NotNil(t, gen.Nodes[0].NotAfter)

	// Work item dates
	require.NotNil(t, gen.WorkItems[0].DueDate)
	assert.Equal(t, 5, int(gen.WorkItems[0].DueDate.Month()))
	require.NotNil(t, gen.WorkItems[0].NotBefore)
	assert.Equal(t, 4, int(gen.WorkItems[0].NotBefore.Month()))
}

func TestConvert_DurationSourceAlwaysManual(t *testing.T) {
	schema := &ImportSchema{
		Project: ProjectImport{
			ShortID: "TST01", Name: "Test", Domain: "test", StartDate: "2025-01-01",
		},
		Nodes: []NodeImport{
			{Ref: "n1", Title: "Node", Kind: "generic"},
		},
		WorkItems: []WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Fixed Task", Type: "task", DurationMode: "fixed"},
			{Ref: "w2", NodeRef: "n1", Title: "Estimate Task", Type: "task", DurationMode: "estimate"},
			{Ref: "w3", NodeRef: "n1", Title: "Default Task", Type: "task"},
		},
	}

	gen, err := Convert(schema)
	require.NoError(t, err)

	for _, wi := range gen.WorkItems {
		assert.Equal(t, domain.SourceManual, wi.DurationSource, "DurationSource should always be manual for import, got %s for %s", wi.DurationSource, wi.Title)
	}
}

func TestConvert_UnitsTracking(t *testing.T) {
	schema := validMinimalSchema()
	schema.WorkItems[0].Units = &UnitsImport{Kind: "pages", Total: 42}

	gen, err := Convert(schema)
	require.NoError(t, err)

	assert.Equal(t, "pages", gen.WorkItems[0].UnitsKind)
	assert.Equal(t, 42, gen.WorkItems[0].UnitsTotal)
	assert.Equal(t, 0, gen.WorkItems[0].UnitsDone)
}

func TestConvert_ShortIDUppercased(t *testing.T) {
	schema := validMinimalSchema()
	schema.Project.ShortID = "phi01"

	gen, err := Convert(schema)
	require.NoError(t, err)

	assert.Equal(t, "PHI01", gen.Project.ShortID)
}
