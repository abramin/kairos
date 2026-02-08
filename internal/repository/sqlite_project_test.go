package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectRepo_CreateAndGetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	target := time.Now().UTC().AddDate(0, 2, 0)
	proj := testutil.NewTestProject("Algebra", testutil.WithTargetDate(target))
	require.NoError(t, repo.Create(ctx, proj))

	fetched, err := repo.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, proj.ID, fetched.ID)
	assert.Equal(t, "Algebra", fetched.Name)
	assert.Equal(t, domain.ProjectActive, fetched.Status)
	require.NotNil(t, fetched.TargetDate)
	assert.Equal(t, target.Format("2006-01-02"), fetched.TargetDate.Format("2006-01-02"))
}

func TestProjectRepo_GetByShortID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	proj := testutil.NewTestProject("Biology", testutil.WithShortID("BIO01"))
	require.NoError(t, repo.Create(ctx, proj))

	// Case-insensitive lookup.
	fetched, err := repo.GetByShortID(ctx, "bio01")
	require.NoError(t, err)
	assert.Equal(t, proj.ID, fetched.ID)
	assert.Equal(t, "BIO01", fetched.ShortID)
}

func TestProjectRepo_GetByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProjectRepo_List_ExcludesArchived(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	p1 := testutil.NewTestProject("Active1")
	p2 := testutil.NewTestProject("Active2")
	p3 := testutil.NewTestProject("Archived")
	require.NoError(t, repo.Create(ctx, p1))
	require.NoError(t, repo.Create(ctx, p2))
	require.NoError(t, repo.Create(ctx, p3))
	require.NoError(t, repo.Archive(ctx, p3.ID))

	// Without archived
	list, err := repo.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// With archived
	listAll, err := repo.List(ctx, true)
	require.NoError(t, err)
	assert.Len(t, listAll, 3)
}

func TestProjectRepo_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	proj := testutil.NewTestProject("OrigName")
	require.NoError(t, repo.Create(ctx, proj))

	proj.Name = "NewName"
	proj.Domain = "math"
	proj.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.Update(ctx, proj))

	fetched, err := repo.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, "NewName", fetched.Name)
	assert.Equal(t, "math", fetched.Domain)
}

func TestProjectRepo_ArchiveAndUnarchive(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	proj := testutil.NewTestProject("ArchTest")
	require.NoError(t, repo.Create(ctx, proj))

	require.NoError(t, repo.Archive(ctx, proj.ID))
	fetched, err := repo.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ProjectArchived, fetched.Status)
	assert.NotNil(t, fetched.ArchivedAt)

	require.NoError(t, repo.Unarchive(ctx, proj.ID))
	fetched, err = repo.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ProjectActive, fetched.Status)
	assert.Nil(t, fetched.ArchivedAt)
}

func TestProjectRepo_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	proj := testutil.NewTestProject("DelTest")
	require.NoError(t, repo.Create(ctx, proj))

	require.NoError(t, repo.Delete(ctx, proj.ID))
	_, err := repo.GetByID(ctx, proj.ID)
	assert.Error(t, err)
}

func TestProjectRepo_UniqueShortID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	p1 := testutil.NewTestProject("Proj1", testutil.WithShortID("DUP01"))
	p2 := testutil.NewTestProject("Proj2", testutil.WithShortID("DUP01"))
	require.NoError(t, repo.Create(ctx, p1))

	err := repo.Create(ctx, p2)
	assert.Error(t, err, "duplicate short_id should violate unique index")
}

func TestProjectRepo_NullTargetDate(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteProjectRepo(db)
	ctx := context.Background()

	// Project without target date.
	proj := testutil.NewTestProject("NoDeadline")
	proj.TargetDate = nil
	require.NoError(t, repo.Create(ctx, proj))

	fetched, err := repo.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Nil(t, fetched.TargetDate)
}
