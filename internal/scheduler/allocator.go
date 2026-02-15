package scheduler

import (
	"github.com/alexanderramin/kairos/internal/app"
)

// AllocateSlices takes sorted scored candidates and available time,
// returns WorkSlices respecting session bounds.
func AllocateSlices(
	candidates []ScoredCandidate,
	availableMin int,
	maxSlices int,
	enforceVariation bool,
) ([]app.WorkSlice, []app.ConstraintBlocker) {
	var slices []app.WorkSlice
	var blockers []app.ConstraintBlocker
	var pass1Candidates []ScoredCandidate // parallel to slices — tracks pass-1 origins for extension
	remaining := availableMin
	projectsUsed := make(map[string]bool)

	// First pass: prefer variation (one item per project)
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
			pass1Candidates = append(pass1Candidates, c)
			remaining -= slice.AllocatedMin
			projectsUsed[c.Input.ProjectID] = true
		}
	}

	// Extension pass: when variation deferred same-project items, extend
	// pass-1 slices up to maxSessionMin before filling with deferred items.
	for i, c := range pass1Candidates {
		if !enforceVariation || len(deferred) == 0 {
			break
		}
		if remaining <= 0 {
			break
		}
		workLeft := c.Input.PlannedMin - c.Input.LoggedMin
		ceiling := min(c.Input.MaxSessionMin, workLeft)
		headroom := ceiling - slices[i].AllocatedMin
		if headroom > 0 {
			extend := min(headroom, remaining)
			slices[i].AllocatedMin += extend
			remaining -= extend
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

func tryAllocate(c ScoredCandidate, remaining int) (*app.WorkSlice, *app.ConstraintBlocker) {
	minS := c.Input.MinSessionMin
	maxS := c.Input.MaxSessionMin
	defS := c.Input.DefaultSessionMin

	// Can't fit minimum session
	if remaining < minS {
		return nil, &app.ConstraintBlocker{
			EntityType: "work_item",
			EntityID:   c.Input.WorkItemID,
			Code:       app.BlockerSessionMinExceedsAvail,
			Message:    "Not enough time for minimum session",
		}
	}

	// Clamp allocation to [min, min(max, remaining)]
	upper := min(maxS, remaining)
	allocated := clamp(defS, minS, upper)

	// No remaining work — item is fully logged
	workRemaining := c.Input.PlannedMin - c.Input.LoggedMin
	if c.Input.PlannedMin > 0 && workRemaining <= 0 {
		return nil, &app.ConstraintBlocker{
			EntityType: "work_item",
			EntityID:   c.Input.WorkItemID,
			Code:       app.BlockerWorkComplete,
			Message:    "No remaining work to allocate",
		}
	}

	// Don't over-allocate past remaining planned work
	if workRemaining > 0 && workRemaining < allocated {
		allocated = clamp(workRemaining, minS, upper)
	}

	reasons := make([]app.RecommendationReason, len(c.Reasons))
	copy(reasons, c.Reasons)
	if allocated != defS {
		delta := 0.0
		reasons = append(reasons, app.RecommendationReason{
			Code:        app.ReasonBoundsApplied,
			Message:     "Session duration adjusted to fit constraints",
			WeightDelta: &delta,
		})
	}

	var dueDateStr *string
	if c.Input.DueDate != nil {
		s := c.Input.DueDate.Format("2006-01-02")
		dueDateStr = &s
	}

	slice := &app.WorkSlice{
		WorkItemID:        c.Input.WorkItemID,
		WorkItemSeq:       c.Input.WorkItemSeq,
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
