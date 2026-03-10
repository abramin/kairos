package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

// =============================================================================
// RunOneShot integration tests — exercise the full CLI dispatch:
// App (services + in-memory DB) → RunOneShot → formatter → stdout
// =============================================================================

func TestRunOneShot_WhatNow_WithData(t *testing.T) {
	app := testApp(t)
	seedCriticalAndOnTrack(t, app)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "what-now", []string{"60"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Write Introduction", "should contain work item from critical project")
	assert.Contains(t, out, "Read Chapter 1", "should contain work item from on-track project")
}

func TestRunOneShot_WhatNow_DefaultMinutes(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "what-now", nil)
		require.NoError(t, err)
	})

	assert.NotEmpty(t, out, "should produce output with default 60 minutes")
	assert.Contains(t, out, "Reading")
}

func TestRunOneShot_WhatNow_InvalidDuration(t *testing.T) {
	app := testApp(t)

	err := RunOneShot(app, "what-now", []string{"abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration")
}

func TestRunOneShot_WhatNow_EmptyDB(t *testing.T) {
	app := testApp(t)

	// With no schedulable items, what-now returns NO_CANDIDATES error.
	err := RunOneShot(app, "what-now", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NO_CANDIDATES")
}

func TestRunOneShot_Status_WithData(t *testing.T) {
	app := testApp(t)
	seedCriticalAndOnTrack(t, app)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "status", nil)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Urgent Paper", "should show critical project")
	assert.Contains(t, out, "Leisurely Reading", "should show on-track project")
}

func TestRunOneShot_Status_EmptyDB(t *testing.T) {
	app := testApp(t)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "status", nil)
		require.NoError(t, err)
	})

	assert.NotEmpty(t, out, "should produce output even with no projects")
}

func TestRunOneShot_Projects_WithData(t *testing.T) {
	app := testApp(t)
	seedCriticalAndOnTrack(t, app)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "projects", nil)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Urgent Paper")
	assert.Contains(t, out, "Leisurely Reading")
}

func TestRunOneShot_Projects_EmptyDB(t *testing.T) {
	app := testApp(t)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "projects", nil)
		require.NoError(t, err)
	})

	assert.NotEmpty(t, out, "should produce output even with no projects")
}

func TestRunOneShot_Chart_WithData(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "chart", nil)
		require.NoError(t, err)
	})

	assert.NotEmpty(t, out, "chart should produce output")
}

func TestRunOneShot_Chart_WeeksFlag(t *testing.T) {
	app := testApp(t)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "chart", []string{"--weeks", "2"})
		require.NoError(t, err)
	})

	assert.NotEmpty(t, out, "chart with --weeks flag should produce output")
}

func TestRunOneShot_Chart_WeeksEqualsFlag(t *testing.T) {
	app := testApp(t)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "chart", []string{"--weeks=3"})
		require.NoError(t, err)
	})

	assert.NotEmpty(t, out, "chart with --weeks= flag should produce output")
}

func TestRunOneShot_Help(t *testing.T) {
	app := testApp(t)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "help", nil)
		require.NoError(t, err)
	})

	assert.NotEmpty(t, out, "help should produce output")
}

func TestRunOneShot_UnknownCommand(t *testing.T) {
	app := testApp(t)

	err := RunOneShot(app, "frobnicate", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestRunOneShot_WhatNow_AlternateAlias(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	out := captureStdout(t, func() {
		err := RunOneShot(app, "whatnow", []string{"60"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Reading", "whatnow alias should work the same as what-now")
}

func TestRunOneShot_WhatNow_DurationFormats(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	// Test various duration formats via RunOneShot
	for _, dur := range []string{"30", "1h", "1h30m"} {
		t.Run(dur, func(t *testing.T) {
			out := captureStdout(t, func() {
				err := RunOneShot(app, "what-now", []string{dur})
				require.NoError(t, err)
			})
			assert.NotEmpty(t, out, "should produce output for duration %q", dur)
		})
	}
}

// =============================================================================
// E2E pipeline: import JSON fixture → what-now → verify imported items appear
// =============================================================================

func TestE2E_Import_ThenWhatNow(t *testing.T) {
	app := testAppFull(t)
	ctx := context.Background()

	// Create a JSON fixture with two work items across a project.
	importJSON := `{
		"project": {
			"short_id": "E2E01",
			"name": "E2E Pipeline Project",
			"domain": "education",
			"start_date": "2026-01-15"
		},
		"nodes": [
			{"ref": "n1", "title": "Module Alpha", "kind": "module", "order": 0}
		],
		"work_items": [
			{"ref": "w1", "node_ref": "n1", "title": "Watch Lecture", "type": "reading", "planned_min": 90},
			{"ref": "w2", "node_ref": "n1", "title": "Complete Exercises", "type": "practice", "planned_min": 60}
		]
	}`

	dir := t.TempDir()
	path := filepath.Join(dir, "e2e_import.json")
	require.NoError(t, os.WriteFile(path, []byte(importJSON), 0644))

	// Step 1: Import the fixture via the service.
	state := &SharedState{App: app}
	cb := &commandBar{state: state}

	result, err := cb.dispatchProject(ctx, "import", []string{path}, map[string]string{})
	require.NoError(t, err)
	assert.Contains(t, result, "Imported")

	// Step 2: Verify what-now includes the imported items via RunOneShot.
	out := captureStdout(t, func() {
		err := RunOneShot(app, "what-now", []string{"120"})
		require.NoError(t, err)
	})

	// At least one imported item should appear (allocator may not select both
	// depending on session bounds and max-slices).
	assert.Condition(t, func() bool {
		return bytes.Contains([]byte(out), []byte("Watch Lecture")) ||
			bytes.Contains([]byte(out), []byte("Complete Exercises"))
	}, "at least one imported work item should appear in what-now recommendations")

	// Step 3: Verify status shows the imported project.
	out = captureStdout(t, func() {
		err := RunOneShot(app, "status", nil)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "E2E Pipeline Project",
		"imported project should appear in status output")
}
