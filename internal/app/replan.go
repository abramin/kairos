package app

import (
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

type ReplanRequest struct {
	Trigger                     domain.ReplanTrigger
	Now                         *time.Time
	ProjectScope                []string
	Strategy                    string // "rebalance" or "deadline_first"
	PreserveExistingAssignments bool
	IncludeArchived             bool
	Explain                     bool
}

func NewReplanRequest(trigger domain.ReplanTrigger) ReplanRequest {
	return ReplanRequest{
		Trigger:                     trigger,
		Strategy:                    "rebalance",
		PreserveExistingAssignments: true,
		Explain:                     true,
	}
}

type ProjectReplanDelta struct {
	ProjectID              string
	ProjectName            string
	RiskBefore             domain.RiskLevel
	RiskAfter              domain.RiskLevel
	RequiredDailyMinBefore float64
	RequiredDailyMinAfter  float64
	RemainingMinBefore     int
	RemainingMinAfter      int
	ChangedItemsCount      int
	Notes                  []string
}

type ReplanResponse struct {
	GeneratedAt        time.Time
	Trigger            domain.ReplanTrigger
	Strategy           string
	RecomputedProjects int
	Deltas             []ProjectReplanDelta
	GlobalModeAfter    domain.PlanMode
	Warnings           []string
	Explanation        *ReplanExplanation
}

type ReplanExplanation struct {
	CriticalProjects []string
	RulesApplied     []string
}

type ReplanErrorCode string

const (
	ReplanErrInvalidTrigger   ReplanErrorCode = "INVALID_TRIGGER"
	ReplanErrNoActiveProjects ReplanErrorCode = "NO_ACTIVE_PROJECTS"
	ReplanErrDataIntegrity    ReplanErrorCode = "DATA_INTEGRITY"
	ReplanErrInternal         ReplanErrorCode = "INTERNAL_ERROR"
)

type ReplanError struct {
	Code    ReplanErrorCode
	Message string
}

func (e *ReplanError) Error() string {
	return string(e.Code) + ": " + e.Message
}
