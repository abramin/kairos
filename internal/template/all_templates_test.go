package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllTemplates_LoadAndValidate loads every JSON file in the templates/
// directory and verifies each one parses correctly and passes schema validation.
// This prevents a malformed template from breaking `kairos project init` at runtime.
func TestAllTemplates_LoadAndValidate(t *testing.T) {
	templatesDir := findTemplatesDir(t)

	entries, err := os.ReadDir(templatesDir)
	require.NoError(t, err, "failed to read templates directory")

	jsonCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		jsonCount++
		name := entry.Name()

		t.Run(name, func(t *testing.T) {
			path := filepath.Join(templatesDir, name)

			// Step 1: File must parse as valid JSON into TemplateSchema.
			schema, err := LoadSchema(path)
			require.NoError(t, err, "template %s failed to load", name)
			require.NotNil(t, schema)

			// Step 2: Schema must have required identity fields.
			assert.NotEmpty(t, schema.ID, "template %s missing ID", name)
			assert.NotEmpty(t, schema.Name, "template %s missing Name", name)

			// Step 3: Schema must pass structural validation.
			errs := ValidateSchema(schema)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("validation error in %s: %v", name, e)
				}
			}

			// Step 4: Template must have at least one node.
			assert.NotEmpty(t, schema.Nodes, "template %s has no nodes", name)
		})
	}

	require.Greater(t, jsonCount, 0, "no JSON template files found in %s", templatesDir)
}

// TestAllTemplates_ExecuteWithDefaults attempts to execute every template
// with default variable values and verifies the output is non-empty.
func TestAllTemplates_ExecuteWithDefaults(t *testing.T) {
	templatesDir := findTemplatesDir(t)

	entries, err := os.ReadDir(templatesDir)
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()

		t.Run(name, func(t *testing.T) {
			path := filepath.Join(templatesDir, name)
			schema, err := LoadSchema(path)
			require.NoError(t, err)

			// Build default vars from variable declarations.
			vars := make(map[string]string)
			for _, v := range schema.Variables {
				if len(v.Default) > 0 {
					vars[v.Key] = strings.Trim(string(v.Default), "\"")
				} else if v.Min != nil && *v.Min > 0 {
					vars[v.Key] = "1"
				}
			}

			due := "2026-06-30"
			generated, err := Execute(schema, "Test "+schema.Name, "2026-01-15", &due, vars)
			require.NoError(t, err, "template %s failed to execute with defaults", name)
			require.NotNil(t, generated)

			assert.NotNil(t, generated.Project, "template %s produced nil project", name)
			assert.NotEmpty(t, generated.Nodes, "template %s produced no nodes", name)
			assert.NotEmpty(t, generated.WorkItems, "template %s produced no work items", name)
		})
	}
}

// findTemplatesDir locates the templates directory relative to the test file.
func findTemplatesDir(t *testing.T) string {
	t.Helper()
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
