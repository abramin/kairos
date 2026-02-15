package app

import "github.com/alexanderramin/kairos/internal/domain"

type RecommendationReasonCode string

const (
	ReasonDeadlinePressure  RecommendationReasonCode = "DEADLINE_PRESSURE"
	ReasonBehindPace        RecommendationReasonCode = "BEHIND_PACE"
	ReasonSpacingOK         RecommendationReasonCode = "SPACING_OK"
	ReasonSpacingBlocked    RecommendationReasonCode = "SPACING_BLOCKED"
	ReasonVariationBonus    RecommendationReasonCode = "VARIATION_BONUS"
	ReasonVariationPenalty  RecommendationReasonCode = "VARIATION_PENALTY"
	ReasonBoundsApplied     RecommendationReasonCode = "BOUNDS_APPLIED"
	ReasonDependencyBlocked RecommendationReasonCode = "DEPENDENCY_BLOCKED"
	ReasonOnTrackSafeMix    RecommendationReasonCode = "ON_TRACK_SAFE_MIX"
	ReasonCriticalFocus     RecommendationReasonCode = "CRITICAL_FOCUS"
	ReasonMomentum          RecommendationReasonCode = "MOMENTUM"
)

type RecommendationReason struct {
	Code        RecommendationReasonCode
	Message     string
	WeightDelta *float64
}

type WorkSlice struct {
	WorkItemID        string
	WorkItemSeq       int
	ProjectID         string
	NodeID            string
	Title             string
	AllocatedMin      int
	MinSessionMin     int
	MaxSessionMin     int
	DefaultSessionMin int
	Splittable        bool
	DueDate           *string
	RiskLevel         domain.RiskLevel
	Score             float64
	Reasons           []RecommendationReason
}

type RiskSummary struct {
	ProjectID         string
	ProjectName       string
	RiskLevel         domain.RiskLevel
	DueDate           *string
	DaysLeft          *int
	PlannedMinTotal   int
	LoggedMinTotal    int
	RemainingMinTotal int
	RequiredDailyMin  float64
	RecentDailyMin    float64
	SlackMinPerDay    float64
	ProgressTimePct   float64
}

type ConstraintBlockerCode string

const (
	BlockerNotBefore              ConstraintBlockerCode = "NOT_BEFORE"
	BlockerDependency             ConstraintBlockerCode = "DEPENDENCY"
	BlockerStatusDone             ConstraintBlockerCode = "STATUS_DONE"
	BlockerNotInCriticalScope     ConstraintBlockerCode = "NOT_IN_CRITICAL_SCOPE"
	BlockerSessionMinExceedsAvail ConstraintBlockerCode = "SESSION_MIN_EXCEEDS_AVAILABLE"
	BlockerWorkComplete           ConstraintBlockerCode = "WORK_COMPLETE"
)

type ConstraintBlocker struct {
	EntityType string
	EntityID   string
	Code       ConstraintBlockerCode
	Message    string
}
