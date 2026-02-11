package cli

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/stretchr/testify/assert"
)

func TestCommandHint_WhatNow(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentWhatNow,
		Arguments: map[string]interface{}{"available_min": float64(45)},
	}
	assert.Equal(t, "what-now --minutes 45", CommandHint(intent))
}

func TestCommandHint_Status(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentStatus,
		Arguments: map[string]interface{}{},
	}
	assert.Equal(t, "status", CommandHint(intent))
}

func TestCommandHint_Replan(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentReplan,
		Arguments: map[string]interface{}{"strategy": "deadline_first"},
	}
	assert.Equal(t, "replan --strategy deadline_first", CommandHint(intent))
}

func TestCommandHint_ProjectAdd(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentProjectAdd,
		Arguments: map[string]interface{}{
			"name":        "Spanish A2 B1",
			"domain":      "Language Learning",
			"start_date":  "2026-02-09",
			"target_date": "2026-12-31",
		},
	}
	got := CommandHint(intent)
	assert.Contains(t, got, "project add")
	assert.Contains(t, got, `--name "Spanish A2 B1"`)
	assert.Contains(t, got, `--domain "Language Learning"`)
	assert.Contains(t, got, "--start 2026-02-09")
	assert.Contains(t, got, "--due 2026-12-31")
	assert.Contains(t, got, "--id <SHORT_ID>")
}

func TestCommandHint_ProjectImport(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentProjectImport,
		Arguments: map[string]interface{}{"file_path": "spanish_a2_b1_modules.json"},
	}
	assert.Equal(t, "project import spanish_a2_b1_modules.json", CommandHint(intent))
}

func TestCommandHint_ProjectImportMissingArg(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentProjectImport,
		Arguments: map[string]interface{}{},
	}
	assert.Equal(t, "project import <FILE>", CommandHint(intent))
}

func TestCommandHint_ProjectUpdate(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentProjectUpdate,
		Arguments: map[string]interface{}{
			"project_id":  "PHI01",
			"name":        "New Name",
			"target_date": "2026-06-01",
			"status":      "done",
		},
	}
	got := CommandHint(intent)
	assert.Contains(t, got, "project update PHI01")
	assert.Contains(t, got, `--name "New Name"`)
	assert.Contains(t, got, "--due 2026-06-01")
	assert.Contains(t, got, "--status done")
}

func TestCommandHint_ProjectArchive(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentProjectArchive,
		Arguments: map[string]interface{}{"project_id": "PHI01"},
	}
	assert.Equal(t, "project archive PHI01", CommandHint(intent))
}

func TestCommandHint_ProjectRemove(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentProjectRemove,
		Arguments: map[string]interface{}{"project_id": "PHI01"},
	}
	assert.Equal(t, "project remove PHI01", CommandHint(intent))
}

func TestCommandHint_NodeAdd(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentNodeAdd,
		Arguments: map[string]interface{}{
			"project_id": "PHI01",
			"title":      "Week 1",
			"kind":       "week",
		},
	}
	got := CommandHint(intent)
	assert.Contains(t, got, "node add")
	assert.Contains(t, got, "--project PHI01")
	assert.Contains(t, got, `--title "Week 1"`)
	assert.Contains(t, got, "--kind week")
}

func TestCommandHint_WorkDone(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentWorkDone,
		Arguments: map[string]interface{}{"work_item_id": "abc-123"},
	}
	assert.Equal(t, "work done abc-123", CommandHint(intent))
}

func TestCommandHint_SessionLog(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentSessionLog,
		Arguments: map[string]interface{}{
			"work_item_id": "abc-123",
			"minutes":      float64(30),
			"note":         "Finished chapter 3",
		},
	}
	got := CommandHint(intent)
	assert.Contains(t, got, "session log")
	assert.Contains(t, got, "--work-item abc-123")
	assert.Contains(t, got, "--minutes 30")
	assert.Contains(t, got, `--note "Finished chapter 3"`)
}

func TestCommandHint_TemplateList(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentTemplateList,
		Arguments: map[string]interface{}{},
	}
	assert.Equal(t, "template list", CommandHint(intent))
}

func TestCommandHint_ExplainNow(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentExplainNow,
		Arguments: map[string]interface{}{"minutes": float64(90)},
	}
	assert.Equal(t, "explain now --minutes 90", CommandHint(intent))
}

func TestCommandHint_ReviewWeekly(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentReviewWeekly,
		Arguments: map[string]interface{}{},
	}
	assert.Equal(t, "review weekly", CommandHint(intent))
}

func TestCommandHint_ProjectInitFromTemplate(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentProjectInitFromTmpl,
		Arguments: map[string]interface{}{
			"template_id":  "physics_101",
			"project_name": "Physics Fall 2026",
			"start_date":   "2026-09-01",
			"target_date":  "2026-12-20",
		},
	}
	got := CommandHint(intent)
	assert.Contains(t, got, "project init")
	assert.Contains(t, got, "--template physics_101")
	assert.Contains(t, got, `--name "Physics Fall 2026"`)
	assert.Contains(t, got, "--start 2026-09-01")
	assert.Contains(t, got, "--due 2026-12-20")
}

func TestCommandHint_Simulate_ReturnsEmpty(t *testing.T) {
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentSimulate,
		Arguments: map[string]interface{}{"scenario_text": "what if deadline moves"},
	}
	assert.Equal(t, "", CommandHint(intent))
}

func TestCommandHint_NilIntent(t *testing.T) {
	assert.Equal(t, "", CommandHint(nil))
}
