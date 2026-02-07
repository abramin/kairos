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
