package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplateService_MissingDirectory verifies graceful behavior when the
// template directory does not exist.
func TestTemplateService_MissingDirectory(t *testing.T) {
	svc := NewTemplateService("/nonexistent/templates/path", nil, nil, nil, nil)

	list, err := svc.List(context.Background())
	require.NoError(t, err, "List should not error on missing directory (Glob returns nil)")
	assert.Empty(t, list, "should return empty list for missing directory")
}

// TestTemplateService_EmptyDirectory verifies behavior when the template
// directory exists but contains no JSON files.
func TestTemplateService_EmptyDirectory(t *testing.T) {
	emptyDir := t.TempDir()
	svc := NewTemplateService(emptyDir, nil, nil, nil, nil)

	list, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, list, "should return empty list for empty directory")
}

// TestTemplateService_MalformedJSONFile verifies that a single malformed template
// file does not prevent valid templates from loading.
func TestTemplateService_MalformedJSONFile(t *testing.T) {
	dir := t.TempDir()

	// Valid template
	validJSON := `{
		"id": "valid_template",
		"name": "Valid Template",
		"version": "1.0.0",
		"domain": "test"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.json"), []byte(validJSON), 0644))

	// Malformed JSON (truncated)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "malformed.json"), []byte(`{"id": "bad`), 0644))

	// Empty file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.json"), []byte(``), 0644))

	// Non-JSON file (should be ignored by Glob pattern)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a template"), 0644))

	svc := NewTemplateService(dir, nil, nil, nil, nil)

	list, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 1, "should load only the valid template")
	assert.Equal(t, "Valid Template", list[0].Name)
}

// TestTemplateService_Get_MissingDirectory verifies Get returns an appropriate
// error when the template directory doesn't exist.
func TestTemplateService_Get_MissingDirectory(t *testing.T) {
	svc := NewTemplateService("/nonexistent/path", nil, nil, nil, nil)

	_, err := svc.Get(context.Background(), "anything")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestTemplateService_InitProject_MissingTemplate verifies that InitProject
// returns a clear error when referencing a non-existent template.
func TestTemplateService_InitProject_MissingTemplate(t *testing.T) {
	projects, nodes, workItems, deps, _, _ := setupRepos(t)
	svc := NewTemplateService(t.TempDir(), projects, nodes, workItems, deps)

	_, err := svc.InitProject(context.Background(), "nonexistent_template", "Test", "TST01", "2026-01-01", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestTemplateService_DirectoryWithSubdirectories verifies that subdirectories
// inside the template directory are ignored (only .json files at root level).
func TestTemplateService_DirectoryWithSubdirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory with a JSON file (should be ignored)
	subDir := filepath.Join(dir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.json"), []byte(`{
		"id": "nested",
		"name": "Nested Template",
		"version": "1.0.0",
		"domain": "test"
	}`), 0644))

	// Root-level template
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.json"), []byte(`{
		"id": "root_template",
		"name": "Root Template",
		"version": "1.0.0",
		"domain": "test"
	}`), 0644))

	svc := NewTemplateService(dir, nil, nil, nil, nil)

	list, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 1, "should only list root-level templates")
	assert.Equal(t, "Root Template", list[0].Name)
}
