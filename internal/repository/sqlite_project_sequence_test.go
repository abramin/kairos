package repository

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectSequenceRepo_NextProjectSeq_EmptyProjectStartsAtOne(t *testing.T) {
	database := testutil.NewTestDB(t)
	ctx := context.Background()

	projectRepo := NewSQLiteProjectRepo(database)
	seqRepo := NewSQLiteProjectSequenceRepo(database)

	proj := testutil.NewTestProject("Seq Project")
	require.NoError(t, projectRepo.Create(ctx, proj))

	seq1, err := seqRepo.NextProjectSeq(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, seq1)

	seq2, err := seqRepo.NextProjectSeq(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, seq2)
}

func TestProjectSequenceRepo_NextProjectSeq_BootstrapsFromExistingRows(t *testing.T) {
	database := testutil.NewTestDB(t)
	ctx := context.Background()

	projectRepo := NewSQLiteProjectRepo(database)
	nodeRepo := NewSQLitePlanNodeRepo(database)
	workRepo := NewSQLiteWorkItemRepo(database)
	seqRepo := NewSQLiteProjectSequenceRepo(database)

	proj := testutil.NewTestProject("Seq Bootstrap")
	require.NoError(t, projectRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	node.Seq = 4
	require.NoError(t, nodeRepo.Create(ctx, node))

	work := testutil.NewTestWorkItem(node.ID, "Task")
	work.Seq = 9
	require.NoError(t, workRepo.Create(ctx, work))

	next, err := seqRepo.NextProjectSeq(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 10, next)
}
