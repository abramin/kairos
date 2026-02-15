package contract

import "github.com/alexanderramin/kairos/internal/app"

type RecommendationReasonCode = app.RecommendationReasonCode

const (
	ReasonDeadlinePressure  RecommendationReasonCode = app.ReasonDeadlinePressure
	ReasonBehindPace        RecommendationReasonCode = app.ReasonBehindPace
	ReasonSpacingOK         RecommendationReasonCode = app.ReasonSpacingOK
	ReasonSpacingBlocked    RecommendationReasonCode = app.ReasonSpacingBlocked
	ReasonVariationBonus    RecommendationReasonCode = app.ReasonVariationBonus
	ReasonVariationPenalty  RecommendationReasonCode = app.ReasonVariationPenalty
	ReasonBoundsApplied     RecommendationReasonCode = app.ReasonBoundsApplied
	ReasonDependencyBlocked RecommendationReasonCode = app.ReasonDependencyBlocked
	ReasonOnTrackSafeMix    RecommendationReasonCode = app.ReasonOnTrackSafeMix
	ReasonCriticalFocus     RecommendationReasonCode = app.ReasonCriticalFocus
	ReasonMomentum          RecommendationReasonCode = app.ReasonMomentum
)

type RecommendationReason = app.RecommendationReason

type WorkSlice = app.WorkSlice

type RiskSummary = app.RiskSummary

type ConstraintBlockerCode = app.ConstraintBlockerCode

const (
	BlockerNotBefore              ConstraintBlockerCode = app.BlockerNotBefore
	BlockerDependency             ConstraintBlockerCode = app.BlockerDependency
	BlockerStatusDone             ConstraintBlockerCode = app.BlockerStatusDone
	BlockerNotInCriticalScope     ConstraintBlockerCode = app.BlockerNotInCriticalScope
	BlockerSessionMinExceedsAvail ConstraintBlockerCode = app.BlockerSessionMinExceedsAvail
	BlockerWorkComplete           ConstraintBlockerCode = app.BlockerWorkComplete
)

type ConstraintBlocker = app.ConstraintBlocker
