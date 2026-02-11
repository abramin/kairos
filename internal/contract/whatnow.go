package contract

import (
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

type WhatNowRequest struct {
	AvailableMin     int
	Now              *time.Time
	ProjectScope     []string
	IncludeArchived  bool
	DryRun           bool
	MaxSlices        int
	EnforceVariation bool
	Explain          bool
}

func NewWhatNowRequest(availableMin int) WhatNowRequest {
	return WhatNowRequest{
		AvailableMin:     availableMin,
		MaxSlices:        3,
		EnforceVariation: true,
		Explain:          true,
	}
}

type WhatNowResponse struct {
	GeneratedAt     time.Time
	Mode            domain.PlanMode
	RequestedMin    int
	AllocatedMin    int
	UnallocatedMin  int
	Recommendations []WorkSlice
	Blockers        []ConstraintBlocker
	TopRiskProjects []RiskSummary
	PolicyMessages  []string
	Warnings        []string
}

type WhatNowErrorCode string

const (
	ErrInvalidAvailableMin WhatNowErrorCode = "INVALID_AVAILABLE_MIN"
	ErrNoCandidates        WhatNowErrorCode = "NO_CANDIDATES"
	ErrDataIntegrity       WhatNowErrorCode = "DATA_INTEGRITY"
	ErrInternalError       WhatNowErrorCode = "INTERNAL_ERROR"
)

type WhatNowError struct {
	Code    WhatNowErrorCode
	Message string
}

func (e *WhatNowError) Error() string {
	return string(e.Code) + ": " + e.Message
}
