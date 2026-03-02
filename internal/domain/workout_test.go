package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidWorkoutCategories(t *testing.T) {
	assert.True(t, ValidWorkoutCategories["qigong"])
	assert.True(t, ValidWorkoutCategories["calisthenics"])
	assert.True(t, ValidWorkoutCategories["running"])
	assert.True(t, ValidWorkoutCategories["kettlebell"])
	assert.True(t, ValidWorkoutCategories["gmb"])
	assert.True(t, ValidWorkoutCategories["stretching"])
	assert.True(t, ValidWorkoutCategories["other"])

	assert.False(t, ValidWorkoutCategories["yoga"])
	assert.False(t, ValidWorkoutCategories[""])
}

func TestWorkoutCategoryLabel(t *testing.T) {
	assert.Equal(t, "Qigong", WorkoutCategoryLabel[WorkoutQigong])
	assert.Equal(t, "GMB Movement", WorkoutCategoryLabel[WorkoutGMB])
	assert.Equal(t, "Other", WorkoutCategoryLabel[WorkoutOther])
}
