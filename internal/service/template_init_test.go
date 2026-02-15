package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateInit_PersistsFullStructure(t *testing.T) {
	projects, nodes, workItems, deps, _, _, uow := setupRepos(t)
	ctx := context.Background()

	// Use real templates directory
	templateDir := findTemplatesDir(t)

	svc := NewTemplateService(templateDir, projects, nodes, workItems, deps, uow)

	// Use weeks=3, assignment_count=2 for a small but realistic test
	due := "2026-06-01"
	proj, err := svc.InitProject(ctx, "course_weekly_generic", "Test Module", "TST01", "2026-02-10", &due, map[string]string{
		"weeks":            "3",
		"assignment_count": "2",
	})
	require.NoError(t, err)
	require.NotNil(t, proj)

	assert.Equal(t, "Test Module", proj.Name)
	assert.Equal(t, "TST01", proj.ShortID)
	assert.Equal(t, "education", proj.Domain)

	// Verify nodes persisted: course_root + 3 weeks + assignments_root + 2 assignments = 7 nodes
	projectNodes, err := nodes.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 7, len(projectNodes), "expected 7 nodes: course_root + 3 weeks + assignments_root + 2 assignments")

	// Verify work items persisted:
	// 3 weeks * (reading + review) = 6 weekly items
	// 2 assignments * (draft + submit) = 4 assignment items
	// Total: 10 work items
	projectItems, err := workItems.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 10, len(projectItems), "expected 10 work items: 6 weekly + 4 assignment")

	// Verify all work items have correct attributes from template
	for _, item := range projectItems {
		assert.Equal(t, domain.WorkItemTodo, item.Status, "all items should start as todo")
		assert.Greater(t, item.PlannedMin, 0, "all items should have planned minutes from template")
	}
}

func TestTemplateInit_WithVariableOverride(t *testing.T) {
	projects, nodes, workItems, deps, _, _, uow := setupRepos(t)
	ctx := context.Background()

	templateDir := findTemplatesDir(t)
	svc := NewTemplateService(templateDir, projects, nodes, workItems, deps, uow)

	// Override weeks=2 (minimal)
	proj, err := svc.InitProject(ctx, "course_weekly_generic", "Mini Module", "MIN01", "2026-02-10", nil, map[string]string{
		"weeks":            "2",
		"assignment_count": "1",
	})
	require.NoError(t, err)

	// Nodes: course_root + 2 weeks + assignments_root + 1 assignment = 5
	projectNodes, err := nodes.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, len(projectNodes), "expected 5 nodes with 2 weeks and 1 assignment")

	// Work items: 2*(reading+review) + 1*(draft+submit) = 6
	projectItems, err := workItems.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 6, len(projectItems), "expected 6 work items with 2 weeks and 1 assignment")
}

func TestTemplateInit_WithDueDate(t *testing.T) {
	projects, nodes, workItems, deps, _, _, uow := setupRepos(t)
	ctx := context.Background()

	templateDir := findTemplatesDir(t)
	svc := NewTemplateService(templateDir, projects, nodes, workItems, deps, uow)

	due := "2026-12-01"
	proj, err := svc.InitProject(ctx, "course_weekly_generic", "Deadline Module", "DED01", "2026-02-10", &due, map[string]string{
		"weeks":            "1",
		"assignment_count": "1",
	})
	require.NoError(t, err)
	require.NotNil(t, proj.TargetDate)
	assert.Equal(t, "2026-12-01", proj.TargetDate.Format("2006-01-02"))
}

// findTemplatesDir locates the templates directory relative to the test file.
func findTemplatesDir(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find the repo root
	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		candidate := filepath.Join(dir, "templates")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find templates directory")
		}
		dir = parent
	}
}
