package scheduler

import (
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
)

type ScoringWeights struct {
	DeadlinePressure float64
	BehindPace       float64
	Spacing          float64
	Variation        float64
}

func DefaultWeights() ScoringWeights {
	return ScoringWeights{
		DeadlinePressure: 1.0,
		BehindPace:       0.8,
		Spacing:          0.5,
		Variation:        0.3,
	}
}

type ScoringInput struct {
	WorkItemID          string
	ProjectID           string
	ProjectName         string
	NodeTitle           string
	Title               string
	DueDate             *time.Time // work item or node due date (whichever is earliest)
	ProjectRisk         domain.RiskLevel
	Now                 time.Time
	LastSessionDaysAgo  *int   // nil if never worked
	ProjectSlicesInPlan int    // how many slices from this project already allocated
	Weights             ScoringWeights
	Mode                domain.PlanMode

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
	Reasons []contract.RecommendationReason
	Blocked bool
	Blocker *contract.ConstraintBlocker
}

func ScoreWorkItem(input ScoringInput) ScoredCandidate {
	result := ScoredCandidate{
		Input: input,
	}

	// In critical mode, block non-critical items entirely
	if input.Mode == domain.ModeCritical && input.ProjectRisk != domain.RiskCritical {
		result.Blocked = true
		result.Blocker = &contract.ConstraintBlocker{
			EntityType: "work_item",
			EntityID:   input.WorkItemID,
			Code:       contract.BlockerStatusDone,
			Message:    "Item skipped: not in critical scope during critical mode",
		}
		return result
	}

	var score float64

	// 1. DEADLINE_PRESSURE
	if input.DueDate != nil {
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
		score += delta
		result.Reasons = append(result.Reasons, contract.RecommendationReason{
			Code:        contract.ReasonDeadlinePressure,
			Message:     formatDeadlineMessage(daysUntil),
			WeightDelta: &delta,
		})
	}

	// 2. BEHIND_PACE
	switch input.ProjectRisk {
	case domain.RiskCritical:
		delta := 30.0 * input.Weights.BehindPace
		score += delta
		result.Reasons = append(result.Reasons, contract.RecommendationReason{
			Code:        contract.ReasonBehindPace,
			Message:     "Project is in critical risk",
			WeightDelta: &delta,
		})
	case domain.RiskAtRisk:
		delta := 15.0 * input.Weights.BehindPace
		score += delta
		result.Reasons = append(result.Reasons, contract.RecommendationReason{
			Code:        contract.ReasonBehindPace,
			Message:     "Project is at risk",
			WeightDelta: &delta,
		})
	}

	// 3. SPACING
	if input.LastSessionDaysAgo != nil {
		daysAgo := *input.LastSessionDaysAgo
		var spacingDelta float64
		var code contract.RecommendationReasonCode
		var msg string

		switch {
		case daysAgo == 0:
			spacingDelta = -10.0 * input.Weights.Spacing
			code = contract.ReasonSpacingBlocked
			msg = "Already worked on this project today"
		case daysAgo >= 1 && daysAgo <= 3:
			spacingDelta = 5.0 * input.Weights.Spacing
			code = contract.ReasonSpacingOK
			msg = "Good spacing since last session"
		default: // > 3 days ago
			spacingDelta = 3.0 * input.Weights.Spacing
			code = contract.ReasonSpacingOK
			msg = "Haven't worked on this recently"
		}
		score += spacingDelta
		result.Reasons = append(result.Reasons, contract.RecommendationReason{
			Code:        code,
			Message:     msg,
			WeightDelta: &spacingDelta,
		})
	}

	// 4. VARIATION
	switch {
	case input.ProjectSlicesInPlan == 0:
		delta := 10.0 * input.Weights.Variation
		score += delta
		result.Reasons = append(result.Reasons, contract.RecommendationReason{
			Code:        contract.ReasonVariationBonus,
			Message:     "Adds variety across projects",
			WeightDelta: &delta,
		})
	case input.ProjectSlicesInPlan >= 2:
		delta := -5.0 * input.Weights.Variation * float64(input.ProjectSlicesInPlan)
		score += delta
		result.Reasons = append(result.Reasons, contract.RecommendationReason{
			Code:        contract.ReasonVariationPenalty,
			Message:     "Project already well-represented in plan",
			WeightDelta: &delta,
		})
	}
	// ProjectSlicesInPlan == 1 is neutral

	// 5. Critical mode bonus
	if input.Mode == domain.ModeCritical && input.ProjectRisk == domain.RiskCritical {
		delta := 50.0
		score += delta
		result.Reasons = append(result.Reasons, contract.RecommendationReason{
			Code:        contract.ReasonCriticalFocus,
			Message:     "Critical mode: focusing on critical work",
			WeightDelta: &delta,
		})
	}

	// 6. Safe mix indicator (balanced mode, on-track project)
	if input.Mode == domain.ModeBalanced && input.ProjectRisk == domain.RiskOnTrack {
		zero := 0.0
		result.Reasons = append(result.Reasons, contract.RecommendationReason{
			Code:        contract.ReasonOnTrackSafeMix,
			Message:     "Project is on track, safe to include",
			WeightDelta: &zero,
		})
	}

	result.Score = score
	return result
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
