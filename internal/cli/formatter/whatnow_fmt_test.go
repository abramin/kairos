package formatter

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestFormatWhatNowWithProjectIDs_UsesFriendlyProjectID(t *testing.T) {
	resp := &contract.WhatNowResponse{
		Mode:         domain.ModeCritical,
		RequestedMin: 30,
		Recommendations: []contract.WorkSlice{
			{
				Title:        "Weekly reading + notes",
				AllocatedMin: 30,
				ProjectID:    "39f351b6-2b6e-4f0e-a1d2-b8e3a40b1f07",
				RiskLevel:    domain.RiskCritical,
			},
		},
	}

	out := FormatWhatNowWithProjectIDs(resp, map[string]string{
		"39f351b6-2b6e-4f0e-a1d2-b8e3a40b1f07": "OU01",
	})

	assert.Contains(t, out, "Project:")
	assert.Contains(t, out, "OU01")
	assert.NotContains(t, out, "39f351b6")
}

func TestFormatWhatNowWithProjectIDs_FallsBackToTruncatedID(t *testing.T) {
	resp := &contract.WhatNowResponse{
		Mode:         domain.ModeCritical,
		RequestedMin: 30,
		Recommendations: []contract.WorkSlice{
			{
				Title:        "Weekly reading + notes",
				AllocatedMin: 30,
				ProjectID:    "12345678-90ab-cdef-1234-567890abcdef",
				RiskLevel:    domain.RiskCritical,
			},
		},
	}

	out := FormatWhatNowWithProjectIDs(resp, nil)

	assert.Contains(t, out, "Project:")
	assert.Contains(t, out, "12345678")
}

func TestFormatWhatNowWithProjectIDs_NoRecommendations_ShowsFallbackAndWarnings(t *testing.T) {
	resp := &contract.WhatNowResponse{
		Mode:           domain.ModeBalanced,
		RequestedMin:   60,
		AllocatedMin:   0,
		UnallocatedMin: 60,
		Warnings:       []string{"No eligible tasks due to constraints"},
		PolicyMessages: []string{"On-track projects may include secondary work."},
	}

	out := FormatWhatNowWithProjectIDs(resp, nil)

	assert.Contains(t, out, "No recommendations available.")
	assert.Contains(t, out, "No eligible tasks due to constraints")
	assert.Contains(t, out, "On-track projects may include secondary work.")
}

func TestFormatWhatNowWithProjectIDs_InvalidDueDateFallsBackToRawValue(t *testing.T) {
	badDate := "tomorrow-ish"
	resp := &contract.WhatNowResponse{
		Mode:         domain.ModeCritical,
		RequestedMin: 30,
		Recommendations: []contract.WorkSlice{
			{
				Title:        "Write summary",
				AllocatedMin: 30,
				ProjectID:    "abc12345-def0-1234-5678-90abcdef1234",
				DueDate:      &badDate,
				RiskLevel:    domain.RiskCritical,
			},
		},
	}

	out := FormatWhatNowWithProjectIDs(resp, nil)
	assert.Contains(t, out, "Due: tomorrow-ish")
}
