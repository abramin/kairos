package contract

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

// --- WhatNowRequest constructor defaults ---

func TestNewWhatNowRequest_SetsDefaults(t *testing.T) {
	req := NewWhatNowRequest(45)

	assert.Equal(t, 45, req.AvailableMin)
	assert.Equal(t, 3, req.MaxSlices)
	assert.True(t, req.EnforceVariation)
	assert.True(t, req.Explain)
	assert.Nil(t, req.Now)
	assert.Nil(t, req.ProjectScope)
	assert.False(t, req.IncludeArchived)
	assert.False(t, req.DryRun)
}

func TestNewWhatNowRequest_ZeroMinutes_Preserved(t *testing.T) {
	// Zero is preserved in the DTO — validation happens in the service layer
	req := NewWhatNowRequest(0)
	assert.Equal(t, 0, req.AvailableMin)
}

func TestNewWhatNowRequest_NegativeMinutes_Preserved(t *testing.T) {
	// Negative is preserved — service layer validates
	req := NewWhatNowRequest(-10)
	assert.Equal(t, -10, req.AvailableMin)
}

// --- StatusRequest constructor defaults ---

func TestNewStatusRequest_SetsDefaults(t *testing.T) {
	req := NewStatusRequest()

	assert.True(t, req.Recalc)
	assert.Equal(t, 7, req.IncludeRecentSessionDays)
	assert.Nil(t, req.Now)
	assert.Nil(t, req.ProjectScope)
	assert.False(t, req.IncludeArchived)
	assert.False(t, req.IncludeBlockers)
}

// --- ReplanRequest constructor defaults ---

func TestNewReplanRequest_SetsDefaults(t *testing.T) {
	req := NewReplanRequest(domain.TriggerManual)

	assert.Equal(t, domain.TriggerManual, req.Trigger)
	assert.Equal(t, "rebalance", req.Strategy)
	assert.True(t, req.PreserveExistingAssignments)
	assert.True(t, req.Explain)
	assert.Nil(t, req.Now)
	assert.Nil(t, req.ProjectScope)
	assert.False(t, req.IncludeArchived)
}

// --- Error types ---

func TestWhatNowError_ErrorString(t *testing.T) {
	err := &WhatNowError{
		Code:    ErrInvalidAvailableMin,
		Message: "available_min must be > 0",
	}
	assert.Equal(t, "INVALID_AVAILABLE_MIN: available_min must be > 0", err.Error())
}

func TestStatusError_ErrorString(t *testing.T) {
	err := &StatusError{
		Code:    StatusErrInvalidScope,
		Message: "invalid project scope",
	}
	assert.Equal(t, "INVALID_SCOPE: invalid project scope", err.Error())
}

func TestReplanError_ErrorString(t *testing.T) {
	err := &ReplanError{
		Code:    ReplanErrNoActiveProjects,
		Message: "no active projects to replan",
	}
	assert.Equal(t, "NO_ACTIVE_PROJECTS: no active projects to replan", err.Error())
}

// --- Error codes are distinct ---

func TestWhatNowErrorCodes_AreDistinct(t *testing.T) {
	codes := []WhatNowErrorCode{
		ErrInvalidAvailableMin,
		ErrNoCandidates,
		ErrDataIntegrity,
		ErrInternalError,
	}
	seen := make(map[WhatNowErrorCode]bool)
	for _, c := range codes {
		assert.False(t, seen[c], "duplicate error code: %s", c)
		seen[c] = true
	}
}

func TestReplanErrorCodes_AreDistinct(t *testing.T) {
	codes := []ReplanErrorCode{
		ReplanErrInvalidTrigger,
		ReplanErrNoActiveProjects,
		ReplanErrDataIntegrity,
		ReplanErrInternal,
	}
	seen := make(map[ReplanErrorCode]bool)
	for _, c := range codes {
		assert.False(t, seen[c], "duplicate error code: %s", c)
		seen[c] = true
	}
}
