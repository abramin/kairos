package scheduler

import (
	"time"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/domain"
)

type ScoringWeights struct {
	DeadlinePressure float64
	BehindPace       float64
	Spacing          float64
	Variation        float64
}

func defaultWeights() ScoringWeights {
	return ScoringWeights{
		DeadlinePressure: 1.0,
		BehindPace:       0.8,
		Spacing:          0.5,
		Variation:        0.3,
	}
}

type ScoringInput struct {
	WorkItemID          string
	WorkItemSeq         int
	ProjectID           string
	ProjectName         string
	NodeTitle           string
	Title               string
	DueDate             *time.Time // work item or node due date (whichever is earliest)
	ProjectRisk         domain.RiskLevel
	Now                 time.Time
	LastSessionDaysAgo  *int // nil if never worked
	ProjectSlicesInPlan int  // how many slices from this project already allocated
	Weights             ScoringWeights
	Mode                domain.PlanMode

	// Work item status for momentum scoring
	Status domain.WorkItemStatus

	// Work item fields for allocation
	MinSessionMin     int
	MaxSessionMin     int
	DefaultSessionMin int
	Splittable        bool
	PlannedMin        int
	LoggedMin         int
	NodeID            string
}

type ScoredCandidate struct {
	Input   ScoringInput
	Score   float64
	Reasons []app.RecommendationReason
	Blocked bool
	Blocker *app.ConstraintBlocker
}

func ScoreWorkItem(input ScoringInput) ScoredCandidate {
	result := ScoredCandidate{
		Input: input,
	}

	// In critical mode, block non-critical items entirely
	if input.Mode == domain.ModeCritical && input.ProjectRisk != domain.RiskCritical {
		result.Blocked = true
		result.Blocker = &app.ConstraintBlocker{
			EntityType: "work_item",
			EntityID:   input.WorkItemID,
			Code:       app.BlockerNotInCriticalScope,
			Message:    "Item skipped: not in critical scope during critical mode",
		}
		return result
	}

	var score float64
	factors := []func(ScoringInput) (float64, *app.RecommendationReason){
		scoreDeadlinePressure,
		scoreBehindPace,
		scoreSpacing,
		scoreVariation,
		scoreMomentum,
		scoreCriticalBonus,
		scoreSafeMix,
	}
	for _, f := range factors {
		delta, reason := f(input)
		score += delta
		if reason != nil {
			result.Reasons = append(result.Reasons, *reason)
		}
	}

	result.Score = score
	return result
}

func scoreDeadlinePressure(input ScoringInput) (float64, *app.RecommendationReason) {
	if input.DueDate == nil {
		return 0, nil
	}
	daysUntil := int(input.DueDate.Sub(input.Now).Hours() / 24)
	var pressure float64
	switch {
	case daysUntil <= 0:
		pressure = 100.0
	case daysUntil <= 3:
		pressure = 80.0 / float64(daysUntil)
	case daysUntil <= 7:
		pressure = 40.0 / float64(daysUntil)
	case daysUntil <= 14:
		pressure = 20.0 / float64(daysUntil)
	default:
		pressure = 10.0 / float64(daysUntil)
	}
	delta := pressure * input.Weights.DeadlinePressure
	return delta, &app.RecommendationReason{
		Code:        app.ReasonDeadlinePressure,
		Message:     formatDeadlineMessage(daysUntil),
		WeightDelta: &delta,
	}
}

func scoreBehindPace(input ScoringInput) (float64, *app.RecommendationReason) {
	switch input.ProjectRisk {
	case domain.RiskCritical:
		delta := 30.0 * input.Weights.BehindPace
		return delta, &app.RecommendationReason{
			Code:        app.ReasonBehindPace,
			Message:     "Project is in critical risk",
			WeightDelta: &delta,
		}
	case domain.RiskAtRisk:
		delta := 15.0 * input.Weights.BehindPace
		return delta, &app.RecommendationReason{
			Code:        app.ReasonBehindPace,
			Message:     "Project is at risk",
			WeightDelta: &delta,
		}
	}
	return 0, nil
}

func scoreSpacing(input ScoringInput) (float64, *app.RecommendationReason) {
	if input.LastSessionDaysAgo == nil {
		return 0, nil
	}
	daysAgo := *input.LastSessionDaysAgo
	var delta float64
	var code app.RecommendationReasonCode
	var msg string

	switch {
	case daysAgo == 0:
		delta = -10.0 * input.Weights.Spacing
		code = app.ReasonSpacingBlocked
		msg = "Already worked on this project today"
	case daysAgo >= 1 && daysAgo <= 3:
		delta = 5.0 * input.Weights.Spacing
		code = app.ReasonSpacingOK
		msg = "Good spacing since last session"
	default: // > 3 days ago
		delta = 3.0 * input.Weights.Spacing
		code = app.ReasonSpacingOK
		msg = "Haven't worked on this recently"
	}
	return delta, &app.RecommendationReason{
		Code:        code,
		Message:     msg,
		WeightDelta: &delta,
	}
}

func scoreVariation(input ScoringInput) (float64, *app.RecommendationReason) {
	switch {
	case input.ProjectSlicesInPlan == 0:
		delta := 10.0 * input.Weights.Variation
		return delta, &app.RecommendationReason{
			Code:        app.ReasonVariationBonus,
			Message:     "Adds variety across projects",
			WeightDelta: &delta,
		}
	case input.ProjectSlicesInPlan >= 2:
		delta := -5.0 * input.Weights.Variation * float64(input.ProjectSlicesInPlan)
		return delta, &app.RecommendationReason{
			Code:        app.ReasonVariationPenalty,
			Message:     "Project already well-represented in plan",
			WeightDelta: &delta,
		}
	}
	return 0, nil // ProjectSlicesInPlan == 1 is neutral
}

func scoreMomentum(input ScoringInput) (float64, *app.RecommendationReason) {
	if input.Status == domain.WorkItemInProgress {
		delta := 15.0
		return delta, &app.RecommendationReason{
			Code:        app.ReasonMomentum,
			Message:     "Item already in progress — continue momentum",
			WeightDelta: &delta,
		}
	}
	return 0, nil
}

func scoreCriticalBonus(input ScoringInput) (float64, *app.RecommendationReason) {
	if input.Mode == domain.ModeCritical && input.ProjectRisk == domain.RiskCritical {
		delta := 50.0
		return delta, &app.RecommendationReason{
			Code:        app.ReasonCriticalFocus,
			Message:     "Critical mode: focusing on critical work",
			WeightDelta: &delta,
		}
	}
	return 0, nil
}

func scoreSafeMix(input ScoringInput) (float64, *app.RecommendationReason) {
	if input.Mode == domain.ModeBalanced && input.ProjectRisk == domain.RiskOnTrack {
		zero := 0.0
		return 0, &app.RecommendationReason{
			Code:        app.ReasonOnTrackSafeMix,
			Message:     "Project is on track, safe to include",
			WeightDelta: &zero,
		}
	}
	return 0, nil
}

func formatDeadlineMessage(daysUntil int) string {
	switch {
	case daysUntil <= 0:
		return "Past due!"
	case daysUntil == 1:
		return "Due tomorrow"
	case daysUntil <= 7:
		return "Due this week"
	default:
		return "Upcoming deadline"
	}
}
