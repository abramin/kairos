package scheduler

import "math"

// SmoothReEstimate computes an updated planned_min based on unit progress.
// Returns the current planned_min unchanged if insufficient data.
// Formula: new = round(0.7 * currentPlanned + 0.3 * impliedTotal)
// Never returns less than loggedMin (can't plan less than already done).
func SmoothReEstimate(currentPlannedMin, loggedMin, unitsTotal, unitsDone int) int {
	if unitsDone <= 0 || unitsTotal <= 0 {
		return currentPlannedMin
	}

	pacePerUnit := float64(loggedMin) / float64(unitsDone)
	impliedTotal := pacePerUnit * float64(unitsTotal)

	newPlanned := 0.7*float64(currentPlannedMin) + 0.3*impliedTotal
	result := int(math.Round(newPlanned))

	// Never plan less than what's already logged
	if result < loggedMin {
		return loggedMin
	}
	return result
}
