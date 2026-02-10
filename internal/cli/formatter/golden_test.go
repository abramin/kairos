package formatter

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ansiPattern matches ANSI escape sequences for stripping before golden comparison.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape codes from a string so golden files
// are terminal-independent.
func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// goldenTest compares got against a golden file in testdata/<name>.golden.
// Set GOLDEN_UPDATE=1 to regenerate golden files.
func goldenTest(t *testing.T, name, got string) {
	t.Helper()

	goldenDir := filepath.Join("testdata")
	goldenPath := filepath.Join(goldenDir, name+".golden")

	stripped := stripANSI(got)

	if os.Getenv("GOLDEN_UPDATE") == "1" {
		require.NoError(t, os.MkdirAll(goldenDir, 0755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(stripped), 0644))
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		t.Fatalf("golden file %s does not exist; run with GOLDEN_UPDATE=1 to create it", goldenPath)
	}
	require.NoError(t, err)

	assert.Equal(t, string(expected), stripped,
		"output does not match golden file %s; run with GOLDEN_UPDATE=1 to update", goldenPath)
}

func TestFormatWhatNow_Golden_CriticalMode(t *testing.T) {
	resp := &contract.WhatNowResponse{
		Mode:         domain.ModeCritical,
		RequestedMin: 60,
		AllocatedMin: 45,
		Recommendations: []contract.WorkSlice{
			{
				Title:        "Write Introduction",
				AllocatedMin: 30,
				MinSessionMin: 15,
				MaxSessionMin: 60,
				ProjectID:    "p-urgent",
				RiskLevel:    domain.RiskCritical,
			},
			{
				Title:        "Research Sources",
				AllocatedMin: 15,
				MinSessionMin: 15,
				MaxSessionMin: 60,
				ProjectID:    "p-urgent",
				RiskLevel:    domain.RiskCritical,
			},
		},
		TopRiskProjects: []contract.RiskSummary{
			{
				ProjectID:         "p-urgent",
				ProjectName:       "Urgent Paper",
				RiskLevel:         domain.RiskCritical,
				PlannedMinTotal:   300,
				LoggedMinTotal:    50,
				RemainingMinTotal: 275,
			},
		},
	}

	out := FormatWhatNowWithProjectIDs(resp, map[string]string{
		"p-urgent": "URG01",
	})
	goldenTest(t, "whatnow_critical", out)
}

func TestFormatWhatNow_Golden_BalancedMode(t *testing.T) {
	resp := &contract.WhatNowResponse{
		Mode:         domain.ModeBalanced,
		RequestedMin: 90,
		AllocatedMin: 75,
		Recommendations: []contract.WorkSlice{
			{
				Title:        "Read Chapter 3",
				AllocatedMin: 30,
				MinSessionMin: 15,
				MaxSessionMin: 60,
				ProjectID:    "p-study",
				RiskLevel:    domain.RiskAtRisk,
			},
			{
				Title:        "Practice Set 2",
				AllocatedMin: 30,
				MinSessionMin: 15,
				MaxSessionMin: 60,
				ProjectID:    "p-study",
				RiskLevel:    domain.RiskAtRisk,
			},
			{
				Title:        "Weekly Reading",
				AllocatedMin: 15,
				MinSessionMin: 15,
				MaxSessionMin: 45,
				ProjectID:    "p-leisure",
				RiskLevel:    domain.RiskOnTrack,
			},
		},
		TopRiskProjects: []contract.RiskSummary{
			{
				ProjectID:         "p-study",
				ProjectName:       "Study Plan",
				RiskLevel:         domain.RiskAtRisk,
				PlannedMinTotal:   600,
				LoggedMinTotal:    200,
				RemainingMinTotal: 440,
			},
			{
				ProjectID:         "p-leisure",
				ProjectName:       "Leisure Reading",
				RiskLevel:         domain.RiskOnTrack,
				PlannedMinTotal:   300,
				LoggedMinTotal:    150,
				RemainingMinTotal: 165,
			},
		},
	}

	out := FormatWhatNowWithProjectIDs(resp, map[string]string{
		"p-study":   "STD01",
		"p-leisure": "LEI01",
	})
	goldenTest(t, "whatnow_balanced", out)
}

func TestFormatStatus_Golden_MultiProject(t *testing.T) {
	dueDate1 := "2026-03-15"
	dueDate2 := "2026-06-01"
	dueDate3 := "2026-09-30"

	resp := &contract.StatusResponse{
		Summary: contract.GlobalStatusSummary{
			CountsCritical: 1,
			CountsAtRisk:   1,
			CountsOnTrack:  1,
			CountsTotal:    3,
			PolicyMessage:  "Critical work requires immediate attention",
		},
		Projects: []contract.ProjectStatusView{
			{
				ProjectID:       "p-crit",
				ProjectName:     "Urgent Paper",
				Status:          domain.ProjectActive,
				RiskLevel:       domain.RiskCritical,
				DueDate:         &dueDate1,
				PlannedMinTotal: 300,
				LoggedMinTotal:  50,
				ProgressTimePct: 16.7,
			},
			{
				ProjectID:       "p-risk",
				ProjectName:     "Midterm Prep",
				Status:          domain.ProjectActive,
				RiskLevel:       domain.RiskAtRisk,
				DueDate:         &dueDate2,
				PlannedMinTotal: 600,
				LoggedMinTotal:  200,
				ProgressTimePct: 33.3,
			},
			{
				ProjectID:       "p-ok",
				ProjectName:     "Leisure Reading",
				Status:          domain.ProjectActive,
				RiskLevel:       domain.RiskOnTrack,
				DueDate:         &dueDate3,
				PlannedMinTotal: 300,
				LoggedMinTotal:  150,
				ProgressTimePct: 50.0,
			},
		},
	}

	out := FormatStatus(resp)
	goldenTest(t, "status_multiproject", out)
}

func TestFormatWhatNow_Golden_NoRecommendations(t *testing.T) {
	resp := &contract.WhatNowResponse{
		Mode:           domain.ModeBalanced,
		RequestedMin:   60,
		AllocatedMin:   0,
		UnallocatedMin: 60,
		Warnings:       []string{"No eligible work items found"},
		Blockers: []contract.ConstraintBlocker{
			{
				Code:     contract.BlockerSessionMinExceedsAvail,
				EntityID: "wi-1",
				Message:  "Minimum session (30 min) exceeds available time (15 min)",
			},
		},
	}

	out := FormatWhatNow(resp)
	goldenTest(t, "whatnow_empty", out)
}
