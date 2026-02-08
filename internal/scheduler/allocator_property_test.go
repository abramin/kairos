package scheduler

import (
	"math/rand"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestAllocateSlices_Invariants_AllocatedNeverExceedsRequested property-tests
// the core allocation invariant: allocated_min ≤ requested_min and session bounds.
func TestAllocateSlices_Invariants_AllocatedNeverExceedsRequested(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for trial := 0; trial < 200; trial++ {
		availableMin := rng.Intn(240) + 1 // 1–240 min
		maxSlices := rng.Intn(5) + 1      // 1–5 slices
		enforceVar := rng.Intn(2) == 1

		numCandidates := rng.Intn(8) + 1
		candidates := make([]ScoredCandidate, numCandidates)
		for i := range candidates {
			minS := rng.Intn(30) + 5   // 5–34
			maxS := minS + rng.Intn(90) // minS to minS+89
			defS := minS + rng.Intn(maxS-minS+1)
			planned := rng.Intn(300) + 30
			logged := rng.Intn(planned)
			due := time.Now().AddDate(0, 0, rng.Intn(90)+1)

			candidates[i] = ScoredCandidate{
				Input: ScoringInput{
					WorkItemID:        "wi-" + string(rune('A'+i)),
					ProjectID:         "p-" + string(rune('0'+i%3)),
					ProjectName:       "Project",
					Title:             "Task",
					DueDate:           &due,
					ProjectRisk:       domain.RiskOnTrack,
					MinSessionMin:     minS,
					MaxSessionMin:     maxS,
					DefaultSessionMin: defS,
					PlannedMin:        planned,
					LoggedMin:         logged,
					NodeID:            "n-1",
				},
				Score: float64(rng.Intn(100)),
			}
		}

		slices, _ := AllocateSlices(candidates, availableMin, maxSlices, enforceVar)

		// Invariant 1: total allocated ≤ available
		totalAllocated := 0
		for _, s := range slices {
			totalAllocated += s.AllocatedMin
		}
		assert.LessOrEqual(t, totalAllocated, availableMin,
			"trial %d: total allocated (%d) must not exceed available (%d)", trial, totalAllocated, availableMin)

		// Invariant 2: each slice respects session bounds
		for j, s := range slices {
			assert.GreaterOrEqual(t, s.AllocatedMin, s.MinSessionMin,
				"trial %d slice %d: allocated (%d) must be >= min session (%d)", trial, j, s.AllocatedMin, s.MinSessionMin)
			assert.LessOrEqual(t, s.AllocatedMin, s.MaxSessionMin,
				"trial %d slice %d: allocated (%d) must be <= max session (%d)", trial, j, s.AllocatedMin, s.MaxSessionMin)
		}

		// Invariant 3: no negative allocations
		for j, s := range slices {
			assert.Greater(t, s.AllocatedMin, 0,
				"trial %d slice %d: allocated must be positive", trial, j)
		}

		// Invariant 4: number of slices ≤ maxSlices
		assert.LessOrEqual(t, len(slices), maxSlices,
			"trial %d: number of slices (%d) must not exceed maxSlices (%d)", trial, len(slices), maxSlices)
	}
}

// TestAllocateSlices_Invariant_NoOverAllocatePastRemaining verifies that allocations
// don't assign more time than the work item's remaining planned minutes.
func TestAllocateSlices_Invariant_NoOverAllocatePastRemaining(t *testing.T) {
	due := time.Now().AddDate(0, 0, 14)

	// Work item with only 10 min remaining but session bounds allow up to 60
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "wi-1",
				ProjectID:         "p-1",
				ProjectName:       "A",
				Title:             "Nearly Done",
				DueDate:           &due,
				ProjectRisk:       domain.RiskOnTrack,
				MinSessionMin:     5,
				MaxSessionMin:     60,
				DefaultSessionMin: 30,
				PlannedMin:        100,
				LoggedMin:         90, // Only 10 min remaining
				NodeID:            "n-1",
			},
			Score: 50.0,
		},
	}

	slices, _ := AllocateSlices(candidates, 60, 3, false)

	if len(slices) > 0 {
		// Should not allocate more than remaining work (10 min)
		assert.LessOrEqual(t, slices[0].AllocatedMin, 10,
			"should not allocate more than remaining planned work")
		assert.GreaterOrEqual(t, slices[0].AllocatedMin, 5,
			"should still respect min session bound")
	}
}
