package scheduler

import (
	"math"
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

	// Habit-specific fields (zero/false for regular work items)
	IsHabit          bool
	HabitID          string // clean habit ID (without "habit:" prefix)
	HabitCadenceDays int
	HabitDaysSince   int // days since last completion

	// Domain fields for domain-aware variation
	ProjectDomain      string
	DomainSlicesInPlan int
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
		scoreDomainVariation,
		scoreMomentum,
		scoreCriticalBonus,
		scoreSafeMix,
		scoreHabitUrgency,
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
	case daysAgo <= 7:
		// Logarithmic bonus: rewards 2-3 day gaps more than 1-day
		delta = (3.0 + 2.0*math.Log2(float64(daysAgo))) * input.Weights.Spacing
		code = app.ReasonSpacingOK
		msg = "Good spacing since last session"
	default: // > 7 days ago
		delta = 8.0 * input.Weights.Spacing
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

func scoreHabitUrgency(input ScoringInput) (float64, *app.RecommendationReason) {
	if !input.IsHabit {
		return 0, nil
	}
	// fraction: 0.0 = just done, 1.0 = due now, >1.0 = overdue
	fraction := float64(input.HabitDaysSince) / float64(input.HabitCadenceDays)
	if input.HabitCadenceDays <= 0 {
		fraction = float64(input.HabitDaysSince) // treat as daily
	}

	var delta float64
	var code app.RecommendationReasonCode
	var msg string

	switch {
	case fraction >= 1.5:
		delta = 50.0
		code = app.ReasonHabitOverdue
		msg = "Habit significantly overdue"
	case fraction >= 1.0:
		delta = 30.0
		code = app.ReasonHabitDue
		msg = "Habit due today"
	case fraction >= 0.8:
		delta = 15.0
		code = app.ReasonHabitApproaching
		msg = "Habit due soon"
	default:
		return 0, nil // not due yet
	}

	return delta, &app.RecommendationReason{
		Code:        code,
		Message:     msg,
		WeightDelta: &delta,
	}
}

func scoreDomainVariation(input ScoringInput) (float64, *app.RecommendationReason) {
	if input.ProjectDomain == "" {
		return 0, nil
	}
	switch {
	case input.DomainSlicesInPlan == 0:
		delta := 5.0 * input.Weights.Variation
		return delta, &app.RecommendationReason{
			Code:        app.ReasonDomainVariationBonus,
			Message:     "Adds variety across domains",
			WeightDelta: &delta,
		}
	case input.DomainSlicesInPlan >= 2:
		delta := -3.0 * input.Weights.Variation * float64(input.DomainSlicesInPlan)
		return delta, &app.RecommendationReason{
			Code:        app.ReasonDomainVariationPenalty,
			Message:     "Domain already well-represented in plan",
			WeightDelta: &delta,
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
