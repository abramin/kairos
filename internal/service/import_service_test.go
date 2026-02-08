package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeImportJSON(t *testing.T, schema *importer.ImportSchema) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "import.json")
	data, err := json.MarshalIndent(schema, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

func ptrStr(s string) *string       { return &s }
func ptrInt(i int) *int             { return &i }
func ptrFloat(f float64) *float64   { return &f }
func ptrBool(b bool) *bool          { return &b }

func TestImportProject_FullStructure(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps)

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:    "MATH01",
			Name:       "Mathematics",
			Domain:     "education",
			StartDate:  "2025-02-01",
			TargetDate: ptrStr("2025-06-01"),
		},
		Nodes: []importer.NodeImport{
			{Ref: "ch1", Title: "Chapter 1", Kind: "module", Order: 0},
			{Ref: "ch1_s1", ParentRef: ptrStr("ch1"), Title: "Section 1.1", Kind: "section", Order: 0},
			{Ref: "ch2", Title: "Chapter 2", Kind: "module", Order: 1},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "ch1_s1", Title: "Read 1.1", Type: "reading", PlannedMin: ptrInt(45),
				EstimateConfidence: ptrFloat(0.7),
				Units:              &importer.UnitsImport{Kind: "pages", Total: 20}},
			{Ref: "w2", NodeRef: "ch1_s1", Title: "Exercises 1.1", Type: "assignment", PlannedMin: ptrInt(30)},
			{Ref: "w3", NodeRef: "ch2", Title: "Read Ch2", Type: "reading"},
			{Ref: "w4", NodeRef: "ch2", Title: "Quiz Ch2", Type: "quiz", PlannedMin: ptrInt(15)},
		},
		Dependencies: []importer.DependencyImport{
			{PredecessorRef: "w1", SuccessorRef: "w2"},
			{PredecessorRef: "w3", SuccessorRef: "w4"},
		},
	}

	path := writeImportJSON(t, schema)
	result, err := svc.ImportProject(ctx, path)
	require.NoError(t, err)

	// Verify result counts
	assert.Equal(t, "Mathematics", result.Project.Name)
	assert.Equal(t, "MATH01", result.Project.ShortID)
	assert.Equal(t, 3, result.NodeCount)
	assert.Equal(t, 4, result.WorkItemCount)
	assert.Equal(t, 2, result.DependencyCount)

	// Verify project persisted
	proj, err := projects.GetByID(ctx, result.Project.ID)
	require.NoError(t, err)
	assert.Equal(t, "Mathematics", proj.Name)
	assert.Equal(t, domain.ProjectActive, proj.Status)
	assert.NotNil(t, proj.TargetDate)

	// Verify nodes persisted
	allNodes, err := nodes.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Len(t, allNodes, 3)

	// Verify root nodes
	roots, err := nodes.ListRoots(ctx, proj.ID)
	require.NoError(t, err)
	assert.Len(t, roots, 2, "should have 2 root nodes (ch1, ch2)")

	// Verify work items persisted
	allItems, err := workItems.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Len(t, allItems, 4)

	// Verify w1 has correct fields
	var readItem *domain.WorkItem
	for _, wi := range allItems {
		if wi.Title == "Read 1.1" {
			readItem = wi
			break
		}
	}
	require.NotNil(t, readItem, "should find 'Read 1.1' work item")
	assert.Equal(t, 45, readItem.PlannedMin)
	assert.Equal(t, "pages", readItem.UnitsKind)
	assert.Equal(t, 20, readItem.UnitsTotal)
	assert.Equal(t, domain.SourceManual, readItem.DurationSource)

	// Verify dependencies persisted
	w1Succs, err := deps.ListSuccessors(ctx, readItem.ID)
	require.NoError(t, err)
	assert.Len(t, w1Succs, 1, "w1 should have 1 successor (w2)")
}

func TestImportProject_MinimalWithDefaults(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps)

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   "TST01",
			Name:      "Minimal",
			Domain:    "test",
			StartDate: "2025-01-01",
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Node 1", Kind: "generic"},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task 1", Type: "task"},
		},
	}

	path := writeImportJSON(t, schema)
	result, err := svc.ImportProject(ctx, path)
	require.NoError(t, err)

	assert.Equal(t, 1, result.NodeCount)
	assert.Equal(t, 1, result.WorkItemCount)
	assert.Equal(t, 0, result.DependencyCount)

	// Verify defaults on work item
	items, err := workItems.ListByProject(ctx, result.Project.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	wi := items[0]
	assert.Equal(t, domain.WorkItemTodo, wi.Status)
	assert.Equal(t, domain.DurationEstimate, wi.DurationMode)
	assert.Equal(t, domain.SourceManual, wi.DurationSource)
	assert.Equal(t, 0, wi.PlannedMin)
	assert.Equal(t, 15, wi.MinSessionMin)
	assert.Equal(t, 60, wi.MaxSessionMin)
	assert.Equal(t, 30, wi.DefaultSessionMin)
	assert.True(t, wi.Splittable)
}

func TestImportProject_ValidationFailure(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps)

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			Name:      "Bad Project",
			Domain:    "test",
			StartDate: "2025-01-01",
			// Missing ShortID
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Node", Kind: "generic"},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task", Type: "task"},
		},
	}

	path := writeImportJSON(t, schema)
	_, err := svc.ImportProject(ctx, path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")

	// Verify nothing was persisted
	allProjects, listErr := projects.List(ctx, true)
	require.NoError(t, listErr)
	assert.Empty(t, allProjects, "no project should be persisted on validation failure")
}

func TestImportProject_MalformedJSON(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps)

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid json"), 0644))

	_, err := svc.ImportProject(ctx, path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading import file")
}

func TestImportProject_FileNotFound(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps)

	_, err := svc.ImportProject(ctx, "/nonexistent/path/import.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading import file")
}

func TestImportProject_SchemaDefaults(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps)

	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   "DEF01",
			Name:      "Defaults Test",
			Domain:    "test",
			StartDate: "2025-01-01",
		},
		Defaults: &importer.DefaultsImport{
			DurationMode: "fixed",
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     ptrInt(20),
				MaxSessionMin:     ptrInt(90),
				DefaultSessionMin: ptrInt(45),
				Splittable:        ptrBool(false),
			},
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Node", Kind: "generic"},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task", Type: "task"},
		},
	}

	path := writeImportJSON(t, schema)
	result, err := svc.ImportProject(ctx, path)
	require.NoError(t, err)

	items, err := workItems.ListByProject(ctx, result.Project.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	wi := items[0]
	assert.Equal(t, domain.DurationFixed, wi.DurationMode)
	assert.Equal(t, 20, wi.MinSessionMin)
	assert.Equal(t, 90, wi.MaxSessionMin)
	assert.Equal(t, 45, wi.DefaultSessionMin)
	assert.False(t, wi.Splittable)
}
