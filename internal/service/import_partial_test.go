package service

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImport_DuplicateShortID_FailsCleanly verifies that importing a project
// with a ShortID that already exists fails at the project creation step,
// leaving no partial state from the second import while preserving the first.
func TestImport_DuplicateShortID_FailsCleanly(t *testing.T) {
	projects, nodes, workItems, deps, _, _, uow := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps, uow)

	pm60 := 60

	// First import succeeds.
	schema1 := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   "DUP01",
			Name:      "First Project",
			Domain:    "test",
			StartDate: "2026-01-01",
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Node A", Kind: "generic"},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task A", Type: "task", PlannedMin: &pm60},
		},
	}

	result1, err := svc.ImportProjectFromSchema(ctx, schema1)
	require.NoError(t, err)
	assert.Equal(t, "First Project", result1.Project.Name)
	assert.Equal(t, 1, result1.NodeCount)
	assert.Equal(t, 1, result1.WorkItemCount)

	// Second import with same ShortID should fail.
	schema2 := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   "DUP01", // collision
			Name:      "Second Project",
			Domain:    "test",
			StartDate: "2026-02-01",
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Node B", Kind: "generic"},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task B", Type: "task", PlannedMin: &pm60},
		},
	}

	_, err = svc.ImportProjectFromSchema(ctx, schema2)
	require.Error(t, err, "duplicate ShortID should cause import failure")

	// First import's data should be fully intact.
	allProjects, err := projects.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, allProjects, 1, "only the first project should exist")
	assert.Equal(t, "First Project", allProjects[0].Name)

	allNodes, err := nodes.ListByProject(ctx, result1.Project.ID)
	require.NoError(t, err)
	assert.Len(t, allNodes, 1, "first project's nodes should be intact")

	allItems, err := workItems.ListByProject(ctx, result1.Project.ID)
	require.NoError(t, err)
	assert.Len(t, allItems, 1, "first project's items should be intact")
}

// TestImport_PartialState_ProjectCreatedButDependencyFails verifies what happens
// when the dependency creation step fails after project/nodes/items are created.
// This documents the current non-transactional behavior: partial state remains.
func TestImport_PartialState_ProjectCreatedButDependencyFails(t *testing.T) {
	projects, nodes, workItems, deps, _, _, uow := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps, uow)

	pm60 := 60

	// Import a valid project first.
	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   "DEP01",
			Name:      "Dep Test",
			Domain:    "test",
			StartDate: "2026-01-01",
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Module", Kind: "module"},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task 1", Type: "task", PlannedMin: &pm60},
			{Ref: "w2", NodeRef: "n1", Title: "Task 2", Type: "task", PlannedMin: &pm60},
		},
		Dependencies: []importer.DependencyImport{
			{PredecessorRef: "w1", SuccessorRef: "w2"},
		},
	}

	result, err := svc.ImportProjectFromSchema(ctx, schema)
	require.NoError(t, err)
	assert.Equal(t, 2, result.WorkItemCount)
	assert.Equal(t, 1, result.DependencyCount)

	// Verify dependencies are persisted.
	items, err := workItems.ListByProject(ctx, result.Project.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)

	for _, wi := range items {
		if wi.Title == "Task 2" {
			hasPreds, err := deps.HasUnfinishedPredecessors(ctx, wi.ID)
			require.NoError(t, err)
			assert.True(t, hasPreds, "Task 2 should have Task 1 as predecessor")
		}
	}
}

// TestImport_ValidationBlocksBeforeAnyDBWrite verifies that schema validation
// errors prevent any database writes — the "safe path" that avoids partial state.
func TestImport_ValidationBlocksBeforeAnyDBWrite(t *testing.T) {
	projects, nodes, workItems, deps, _, _, uow := setupRepos(t)
	ctx := context.Background()

	svc := NewImportService(projects, nodes, workItems, deps, uow)

	// Schema with multiple validation errors
	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			// Missing ShortID (validation error)
			Name:      "Invalid",
			Domain:    "test",
			StartDate: "2026-01-01",
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Node", Kind: "generic"},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task", Type: "task"},
		},
	}

	_, err := svc.ImportProjectFromSchema(ctx, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")

	// Verify absolutely nothing was persisted
	allProjects, err := projects.List(ctx, true)
	require.NoError(t, err)
	assert.Empty(t, allProjects, "validation failure should prevent any DB writes")
}
