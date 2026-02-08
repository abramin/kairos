package scheduler

import (
	"math"

	"github.com/alexanderramin/kairos/internal/contract"
)

// AllocateSlices takes sorted scored candidates and available time,
// returns WorkSlices respecting session bounds.
func AllocateSlices(
	candidates []ScoredCandidate,
	availableMin int,
	maxSlices int,
	enforceVariation bool,
) ([]contract.WorkSlice, []contract.ConstraintBlocker) {
	var slices []contract.WorkSlice
	var blockers []contract.ConstraintBlocker
	remaining := availableMin
	projectsUsed := make(map[string]bool)

	// First pass: prefer variation
	var deferred []ScoredCandidate
	for _, c := range candidates {
		if len(slices) >= maxSlices || remaining <= 0 {
			break
		}
		if c.Blocked {
			if c.Blocker != nil {
				blockers = append(blockers, *c.Blocker)
			}
			continue
		}

		// Skip same-project for variation in first pass
		if enforceVariation && projectsUsed[c.Input.ProjectID] {
			deferred = append(deferred, c)
			continue
		}

		slice, blocker := tryAllocate(c, remaining)
		if blocker != nil {
			blockers = append(blockers, *blocker)
			continue
		}
		if slice != nil {
			slices = append(slices, *slice)
			remaining -= slice.AllocatedMin
			projectsUsed[c.Input.ProjectID] = true
		}
	}

	// Second pass: fill remaining with deferred candidates
	for _, c := range deferred {
		if len(slices) >= maxSlices || remaining <= 0 {
			break
		}
		slice, blocker := tryAllocate(c, remaining)
		if blocker != nil {
			blockers = append(blockers, *blocker)
			continue
		}
		if slice != nil {
			slices = append(slices, *slice)
			remaining -= slice.AllocatedMin
		}
	}

	return slices, blockers
}

func tryAllocate(c ScoredCandidate, remaining int) (*contract.WorkSlice, *contract.ConstraintBlocker) {
	minS := c.Input.MinSessionMin
	maxS := c.Input.MaxSessionMin
	defS := c.Input.DefaultSessionMin

	// Can't fit minimum session
	if remaining < minS {
		return nil, &contract.ConstraintBlocker{
			EntityType: "work_item",
			EntityID:   c.Input.WorkItemID,
			Code:       contract.BlockerSessionMinExceedsAvail,
			Message:    "Not enough time for minimum session",
		}
	}

	// Clamp allocation to [min, min(max, remaining)]
	upper := int(math.Min(float64(maxS), float64(remaining)))
	allocated := clamp(defS, minS, upper)

	// Don't over-allocate past remaining planned work
	workRemaining := c.Input.PlannedMin - c.Input.LoggedMin
	if workRemaining > 0 && workRemaining < allocated {
		allocated = clamp(workRemaining, minS, upper)
	}

	reasons := make([]contract.RecommendationReason, len(c.Reasons))
	copy(reasons, c.Reasons)
	if allocated != defS {
		delta := 0.0
		reasons = append(reasons, contract.RecommendationReason{
			Code:        contract.ReasonBoundsApplied,
			Message:     "Session duration adjusted to fit constraints",
			WeightDelta: &delta,
		})
	}

	var dueDateStr *string
	if c.Input.DueDate != nil {
		s := c.Input.DueDate.Format("2006-01-02")
		dueDateStr = &s
	}

	slice := &contract.WorkSlice{
		WorkItemID:        c.Input.WorkItemID,
		ProjectID:         c.Input.ProjectID,
		NodeID:            c.Input.NodeID,
		Title:             c.Input.Title,
		AllocatedMin:      allocated,
		MinSessionMin:     minS,
		MaxSessionMin:     maxS,
		DefaultSessionMin: defS,
		Splittable:        c.Input.Splittable,
		DueDate:           dueDateStr,
		RiskLevel:         c.Input.ProjectRisk,
		Score:             c.Score,
		Reasons:           reasons,
	}

	return slice, nil
}

func clamp(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}
