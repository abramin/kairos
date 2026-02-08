package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSchema(t *testing.T) {
	dir := t.TempDir()

	validPath := filepath.Join(dir, "valid.json")
	require.NoError(t, os.WriteFile(validPath, []byte(`{
  "id":"test_template",
  "name":"Test Template",
  "version":"1.0.0",
  "domain":"education",
  "nodes":[],
  "work_items":[]
}`), 0o644))

	schema, err := LoadSchema(validPath)
	require.NoError(t, err)
	assert.Equal(t, "test_template", schema.ID)
	assert.Equal(t, "Test Template", schema.Name)

	invalidPath := filepath.Join(dir, "invalid.json")
	require.NoError(t, os.WriteFile(invalidPath, []byte(`{"id":"broken"`), 0o644))

	_, err = LoadSchema(invalidPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing template")

	_, err = LoadSchema(filepath.Join(dir, "missing.json"))
	require.Error(t, err)
}
