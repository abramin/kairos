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

// ============ EDGE CASE TESTS ============

// TestAllocateSlices_BoundsEnforcement_ExtremeCases tests session bounds
// with extreme available time values (1 minute, 999 minutes) and edge conditions.
func TestAllocateSlices_BoundsEnforcement_ExtremeCases(t *testing.T) {
	due := time.Now().AddDate(0, 0, 7)

	tests := []struct {
		name         string
		availableMin int
		minSession   int
		maxSession   int
		defSession   int
		plannedMin   int
		loggedMin    int
		expectSlice  bool
		expectMin    int // expected allocation if slice created
		expectMax    int // expected allocation if slice created
	}{
		// Extreme: 1 minute available, minimum bounds
		{
			name:         "1 min available, min=5 - insufficient",
			availableMin: 1,
			minSession:   5,
			maxSession:   60,
			defSession:   30,
			plannedMin:   100,
			loggedMin:    0,
			expectSlice:  false,
			expectMin:    0,
			expectMax:    0,
		},

		// Edge: exact minimum match
		{
			name:         "5 min available, min=5 - exact fit",
			availableMin: 5,
			minSession:   5,
			maxSession:   60,
			defSession:   30,
			plannedMin:   100,
			loggedMin:    0,
			expectSlice:  true,
			expectMin:    5,
			expectMax:    5,
		},

		// Extreme: 999 minutes available, allocate default (bounded by default or max)
		{
			name:         "999 min available, default=30, max=60 - uses default",
			availableMin: 999,
			minSession:   15,
			maxSession:   60,
			defSession:   30,
			plannedMin:   100,
			loggedMin:    0,
			expectSlice:  true,
			expectMin:    30,
			expectMax:    30,
		},

		// Bounded by remaining work
		{
			name:         "30 min available, work remaining=10, min=5 - capped at remaining",
			availableMin: 30,
			minSession:   5,
			maxSession:   60,
			defSession:   30,
			plannedMin:   100,
			loggedMin:    90, // Only 10 min remaining
			expectSlice:  true,
			expectMin:    5,
			expectMax:    10,
		},

		// min == max (exact session only)
		{
			name:         "min=max=30 - exact session",
			availableMin: 60,
			minSession:   30,
			maxSession:   30,
			defSession:   30,
			plannedMin:   100,
			loggedMin:    0,
			expectSlice:  true,
			expectMin:    30,
			expectMax:    30,
		},

		// Edge: just enough for minimum
		{
			name:         "work remaining=5, min=5, max=60 - exact for min",
			availableMin: 100,
			minSession:   5,
			maxSession:   60,
			defSession:   30,
			plannedMin:   100,
			loggedMin:    95, // Exactly 5 min remaining
			expectSlice:  true,
			expectMin:    5,
			expectMax:    5,
		},

		// Default between min and max
		{
			name:         "default between min and max",
			availableMin: 100,
			minSession:   10,
			maxSession:   50,
			defSession:   25,
			plannedMin:   200,
			loggedMin:    0,
			expectSlice:  true,
			expectMin:    25,
			expectMax:    25,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidates := []ScoredCandidate{
				{
					Input: ScoringInput{
						WorkItemID:        "wi-1",
						ProjectID:         "p-1",
						ProjectName:       "Project",
						Title:             "Task",
						DueDate:           &due,
						ProjectRisk:       domain.RiskOnTrack,
						MinSessionMin:     tc.minSession,
						MaxSessionMin:     tc.maxSession,
						DefaultSessionMin: tc.defSession,
						PlannedMin:        tc.plannedMin,
						LoggedMin:         tc.loggedMin,
						NodeID:            "n-1",
					},
					Score: 50.0,
				},
			}

			slices, _ := AllocateSlices(candidates, tc.availableMin, 5, false)

			if tc.expectSlice {
				assert.Len(t, slices, 1, "should allocate exactly one slice")

				s := slices[0]
				assert.GreaterOrEqual(t, s.AllocatedMin, tc.expectMin,
					"allocated %d should be >= min %d", s.AllocatedMin, tc.expectMin)
				assert.LessOrEqual(t, s.AllocatedMin, tc.expectMax,
					"allocated %d should be <= max %d", s.AllocatedMin, tc.expectMax)
				assert.GreaterOrEqual(t, s.AllocatedMin, tc.minSession,
					"allocated %d should respect session min %d", s.AllocatedMin, tc.minSession)
				assert.LessOrEqual(t, s.AllocatedMin, tc.maxSession,
					"allocated %d should respect session max %d", s.AllocatedMin, tc.maxSession)
				assert.LessOrEqual(t, s.AllocatedMin, tc.availableMin,
					"allocated %d should not exceed available %d", s.AllocatedMin, tc.availableMin)
			} else {
				assert.Empty(t, slices, "should not allocate when constraints can't be met")
				// Blockers may or may not be generated depending on the allocator implementation
				// The key invariant is: when no slices are returned, constraints were not satisfiable
			}
		})
	}
}
