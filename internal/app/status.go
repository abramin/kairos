package app

import (
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

type StatusRequest struct {
	Now                      *time.Time
	ProjectScope             []string
	IncludeArchived          bool
	Recalc                   bool
	IncludeBlockers          bool
	IncludeRecentSessionDays int
}

func NewStatusRequest() StatusRequest {
	return StatusRequest{
		Recalc:                   true,
		IncludeRecentSessionDays: 7,
	}
}

type ProjectStatusView struct {
	ProjectID             string
	ProjectName           string
	Status                domain.ProjectStatus
	RiskLevel             domain.RiskLevel
	DueDate               *string
	DaysLeft              *int
	ProgressTimePct       float64
	ProgressStructuralPct float64
	PlannedMinTotal       int
	LoggedMinTotal        int
	RemainingMinTotal     int
	RequiredDailyMin      float64
	RecentDailyMin        float64
	SlackMinPerDay        float64
	SafeForSecondaryWork  bool
	Notes                 []string
}

type GlobalStatusSummary struct {
	GeneratedAt      time.Time
	CountsTotal      int
	CountsOnTrack    int
	CountsAtRisk     int
	CountsCritical   int
	GlobalModeIfNow  domain.PlanMode
	PolicyMessage    string
}

type StatusResponse struct {
	Summary  GlobalStatusSummary
	Projects []ProjectStatusView
	Blockers []ConstraintBlocker
	Warnings []string
}

type StatusErrorCode string

const (
	StatusErrInvalidScope StatusErrorCode = "INVALID_SCOPE"
)

type StatusError struct {
	Code    StatusErrorCode
	Message string
}

func (e *StatusError) Error() string {
	return string(e.Code) + ": " + e.Message
}
