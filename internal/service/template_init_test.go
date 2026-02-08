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
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	// Use real templates directory
	templateDir := findTemplatesDir(t)

	svc := NewTemplateService(templateDir, projects, nodes, workItems, deps)

	// Use study_weeks=3, tma_count=2 for a small but realistic test
	due := "2026-06-01"
	proj, err := svc.InitProject(ctx, "ou_module_weekly", "Test Module", "TST01", "2026-02-10", &due, map[string]string{
		"study_weeks": "3",
		"tma_count":   "2",
	})
	require.NoError(t, err)
	require.NotNil(t, proj)

	assert.Equal(t, "Test Module", proj.Name)
	assert.Equal(t, "TST01", proj.ShortID)
	assert.Equal(t, "education", proj.Domain)

	// Verify nodes persisted: study_root + 3 weeks + tma_root + 2 TMAs = 7 nodes
	projectNodes, err := nodes.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 7, len(projectNodes), "expected 7 nodes: study_root + 3 weeks + tma_root + 2 TMAs")

	// Verify work items persisted:
	// 3 weeks * (reading + activities) = 6 weekly items
	// 2 TMAs * (draft + review + submit) = 6 TMA items
	// Total: 12 work items
	projectItems, err := workItems.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 12, len(projectItems), "expected 12 work items: 6 weekly + 6 TMA")

	// Verify all work items have correct attributes from template
	for _, item := range projectItems {
		assert.Equal(t, domain.WorkItemTodo, item.Status, "all items should start as todo")
		assert.Greater(t, item.PlannedMin, 0, "all items should have planned minutes from template")
	}
}

func TestTemplateInit_WithVariableOverride(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	templateDir := findTemplatesDir(t)
	svc := NewTemplateService(templateDir, projects, nodes, workItems, deps)

	// Override study_weeks=2 (minimal)
	proj, err := svc.InitProject(ctx, "ou_module_weekly", "Mini Module", "MIN01", "2026-02-10", nil, map[string]string{
		"study_weeks": "2",
		"tma_count":   "1",
	})
	require.NoError(t, err)

	// Nodes: study_root + 2 weeks + tma_root + 1 TMA = 5
	projectNodes, err := nodes.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, len(projectNodes), "expected 5 nodes with 2 weeks and 1 TMA")

	// Work items: 2*(reading+activities) + 1*(draft+review+submit) = 7
	projectItems, err := workItems.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 7, len(projectItems), "expected 7 work items with 2 weeks and 1 TMA")
}

func TestTemplateInit_WithDueDate(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	ctx := context.Background()

	templateDir := findTemplatesDir(t)
	svc := NewTemplateService(templateDir, projects, nodes, workItems, deps)

	due := "2026-12-01"
	proj, err := svc.InitProject(ctx, "ou_module_weekly", "Deadline Module", "DED01", "2026-02-10", &due, map[string]string{
		"study_weeks": "1",
		"tma_count":   "1",
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
