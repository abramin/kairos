package repository

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserProfileRepo_Get_DefaultSeededProfile(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteUserProfileRepo(db)
	ctx := context.Background()

	profile, err := repo.Get(ctx)
	require.NoError(t, err)

	assert.Equal(t, "default", profile.ID)
	assert.Equal(t, 0.1, profile.BufferPct)
	assert.Equal(t, 1.0, profile.WeightDeadlinePressure)
	assert.Equal(t, 0.8, profile.WeightBehindPace)
	assert.Equal(t, 0.5, profile.WeightSpacing)
	assert.Equal(t, 0.3, profile.WeightVariation)
	assert.Equal(t, 3, profile.DefaultMaxSlices)
	assert.Equal(t, 30, profile.BaselineDailyMin)
}

func TestUserProfileRepo_Upsert_UpdatesProfile(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteUserProfileRepo(db)
	ctx := context.Background()

	updated := &domain.UserProfile{
		ID:                     "default",
		BufferPct:              0.2,
		WeightDeadlinePressure: 1.4,
		WeightBehindPace:       0.9,
		WeightSpacing:          0.7,
		WeightVariation:        0.4,
		DefaultMaxSlices:       5,
		BaselineDailyMin:       45,
	}
	require.NoError(t, repo.Upsert(ctx, updated))

	got, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, updated.ID, got.ID)
	assert.Equal(t, updated.BufferPct, got.BufferPct)
	assert.Equal(t, updated.WeightDeadlinePressure, got.WeightDeadlinePressure)
	assert.Equal(t, updated.WeightBehindPace, got.WeightBehindPace)
	assert.Equal(t, updated.WeightSpacing, got.WeightSpacing)
	assert.Equal(t, updated.WeightVariation, got.WeightVariation)
	assert.Equal(t, updated.DefaultMaxSlices, got.DefaultMaxSlices)
	assert.Equal(t, updated.BaselineDailyMin, got.BaselineDailyMin)
}

func TestUserProfileRepo_Get_NotFoundWhenDefaultDeleted(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteUserProfileRepo(db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `DELETE FROM user_profile WHERE id = 'default'`)
	require.NoError(t, err)

	_, err = repo.Get(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}
