package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmoothReEstimate_BasicSmoothing(t *testing.T) {
	// planned=100, logged=60, total=10, done=3
	// implied = (60/3)*10 = 200
	// new = 0.7*100 + 0.3*200 = 70 + 60 = 130
	result := SmoothReEstimate(100, 60, 10, 3)
	assert.Equal(t, 130, result)
}

func TestSmoothReEstimate_NoUnitsNoChange(t *testing.T) {
	result := SmoothReEstimate(100, 30, 0, 0)
	assert.Equal(t, 100, result)
}

func TestSmoothReEstimate_NeverBelowLogged(t *testing.T) {
	// planned=10, logged=50, total=10, done=9
	// implied = (50/9)*10 = 55.5
	// new = 0.7*10 + 0.3*55.5 = 7 + 16.65 = 23.65 ≈ 24
	// But loggedMin=50, so result should be 50
	result := SmoothReEstimate(10, 50, 10, 9)
	assert.GreaterOrEqual(t, result, 50, "should never be less than logged minutes")
}

func TestSmoothReEstimate_PaceMatchesPlan(t *testing.T) {
	// planned=100, logged=30, total=10, done=3
	// implied = (30/3)*10 = 100
	// new = 0.7*100 + 0.3*100 = 100
	result := SmoothReEstimate(100, 30, 10, 3)
	assert.Equal(t, 100, result)
}

// ============ EDGE CASE TESTS ============

func TestSmoothReEstimate_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		planned      int
		logged       int
		totalUnits   int
		doneUnits    int
		description  string
	}{
		{
			name:        "zero planned min",
			planned:     0,
			logged:      50,
			totalUnits:  10,
			doneUnits:   3,
			description: "zero planned should return logged as floor",
		},
		{
			name:        "zero logged min",
			planned:     100,
			logged:      0,
			totalUnits:  10,
			doneUnits:   1,
			description: "zero logged min should not cause issues",
		},
		{
			name:        "all units done",
			planned:     100,
			logged:      120,
			totalUnits:  10,
			doneUnits:   10,
			description: "when all units done, pace equals logged",
		},
		{
			name:        "implied far exceeds planned",
			planned:     100,
			logged:      90,
			totalUnits:  10,
			doneUnits:   1,
			description: "implied=900, smoothing dampens the jump",
		},
		{
			name:        "very slow pace",
			planned:     1000,
			logged:      10,
			totalUnits:  100,
			doneUnits:   1,
			description: "very slow early pace, smoothing dampens extrapolation",
		},
		{
			name:        "equal planned and implied",
			planned:     100,
			logged:      50,
			totalUnits:  5,
			doneUnits:   2,
			description: "when pace matches plan, smoothing should keep similar value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SmoothReEstimate(tc.planned, tc.logged, tc.totalUnits, tc.doneUnits)
			// The key invariant: result should never be less than logged
			assert.GreaterOrEqual(t, result, tc.logged,
				"%s: result %d should never be less than logged %d", tc.description, result, tc.logged)
		})
	}
}

func TestSmoothReEstimate_FormulaCombinations(t *testing.T) {
	// Test various combinations to verify the formula: 0.7*planned + 0.3*implied
	tests := []struct {
		name     string
		planned  int
		logged   int
		total    int
		done     int
		expected int
	}{
		{
			name:     "basic example",
			planned:  100,
			logged:   60,
			total:    10,
			done:     3,
			expected: 130, // 0.7*100 + 0.3*200 = 70 + 60 = 130
		},
		{
			name:     "pace matches plan",
			planned:  100,
			logged:   30,
			total:    10,
			done:     3,
			expected: 100, // 0.7*100 + 0.3*100 = 100
		},
		{
			name:     "converged",
			planned:  200,
			logged:   200,
			total:    10,
			done:     10,
			expected: 200, // 0.7*200 + 0.3*200 = 200
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SmoothReEstimate(tc.planned, tc.logged, tc.total, tc.done)
			assert.Equal(t, tc.expected, result,
				"formula for planned=%d logged=%d total=%d done=%d",
				tc.planned, tc.logged, tc.total, tc.done)
		})
	}
}
