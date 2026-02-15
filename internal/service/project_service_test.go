package service

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectService_Create_ValidShortID(t *testing.T) {
	projects, _, _, _, _, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewProjectService(projects)

	proj := &domain.Project{
		Name:    "Philosophy Essay",
		ShortID: "PHI01",
		Domain:  "edu",
	}

	err := svc.Create(ctx, proj)
	require.NoError(t, err)
	assert.NotEmpty(t, proj.ID, "UUID should be generated")
	assert.Equal(t, domain.ProjectActive, proj.Status, "status should default to active")

	// Verify roundtrip
	fetched, err := svc.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, "Philosophy Essay", fetched.Name)
	assert.Equal(t, "PHI01", fetched.ShortID)
}

func TestProjectService_Create_InvalidShortID(t *testing.T) {
	projects, _, _, _, _, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewProjectService(projects)

	tests := []struct {
		name    string
		shortID string
	}{
		{"empty", ""},
		{"lowercase", "phi01"},
		{"no digits", "PHILO"},
		{"too short letters", "PH01"},
		{"too long letters", "PHILOSO01"},
		{"only digits", "12345"},
		{"special chars", "PH!01"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proj := &domain.Project{
				Name:    "Test",
				ShortID: tc.shortID,
				Domain:  "test",
			}
			err := svc.Create(ctx, proj)
			assert.Error(t, err, "short ID %q should be rejected", tc.shortID)
		})
	}
}

func TestProjectService_Delete_RequiresArchiveFirst(t *testing.T) {
	projects, _, _, _, _, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewProjectService(projects)

	proj := testutil.NewTestProject("Active Project")
	require.NoError(t, projects.Create(ctx, proj))

	// Delete without archiving should fail (force=false)
	err := svc.Delete(ctx, proj.ID, false)
	assert.Error(t, err, "should require archive before delete")
	assert.Contains(t, err.Error(), "archived before deletion")

	// Project should still exist
	_, err = svc.GetByID(ctx, proj.ID)
	require.NoError(t, err)
}

func TestProjectService_Delete_ForceBypassesGuard(t *testing.T) {
	projects, _, _, _, _, _, _ := setupRepos(t)
	ctx := context.Background()

	svc := NewProjectService(projects)

	proj := testutil.NewTestProject("Active Project")
	require.NoError(t, projects.Create(ctx, proj))

	// Force delete should work without archiving
	err := svc.Delete(ctx, proj.ID, true)
	require.NoError(t, err)

	// Project should be gone
	_, err = svc.GetByID(ctx, proj.ID)
	assert.Error(t, err, "project should be deleted")
}
