package repository

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCascadeDelete_ProjectToNodes verifies that deleting a project cascades to its plan nodes.
func TestCascadeDelete_ProjectToNodes(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)

	proj := testutil.NewTestProject("CascadeProj")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Child Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	require.NoError(t, projRepo.Delete(ctx, proj.ID))

	_, err := nodeRepo.GetByID(ctx, node.ID)
	assert.Error(t, err, "node should be cascade-deleted when project is deleted")
}

// TestCascadeDelete_NodeToWorkItems verifies plan_nodes -> work_items cascade.
func TestCascadeDelete_NodeToWorkItems(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)

	proj := testutil.NewTestProject("CascadeProj2")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task")
	require.NoError(t, wiRepo.Create(ctx, wi))

	require.NoError(t, nodeRepo.Delete(ctx, node.ID))

	_, err := wiRepo.GetByID(ctx, wi.ID)
	assert.Error(t, err, "work item should be cascade-deleted when node is deleted")
}

// TestCascadeDelete_WorkItemToSessions verifies work_items -> work_session_logs cascade.
func TestCascadeDelete_WorkItemToSessions(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	sessRepo := NewSQLiteSessionRepo(db)

	proj := testutil.NewTestProject("CascadeProj3")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task")
	require.NoError(t, wiRepo.Create(ctx, wi))

	sess := testutil.NewTestSession(wi.ID, 30)
	require.NoError(t, sessRepo.Create(ctx, sess))

	require.NoError(t, wiRepo.Delete(ctx, wi.ID))

	_, err := sessRepo.GetByID(ctx, sess.ID)
	assert.Error(t, err, "session should be cascade-deleted when work item is deleted")
}

// TestCascadeDelete_WorkItemToDependencies verifies work_items -> dependencies cascade.
func TestCascadeDelete_WorkItemToDependencies(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	depRepo := NewSQLiteDependencyRepo(db)

	proj := testutil.NewTestProject("CascadeProj4")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Pred")
	wi2 := testutil.NewTestWorkItem(node.ID, "Succ")
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))

	dep := &domain.Dependency{PredecessorWorkItemID: wi1.ID, SuccessorWorkItemID: wi2.ID}
	require.NoError(t, depRepo.Create(ctx, dep))

	// Delete the predecessor work item.
	require.NoError(t, wiRepo.Delete(ctx, wi1.ID))

	// Dependency should be gone.
	preds, err := depRepo.ListPredecessors(ctx, wi2.ID)
	require.NoError(t, err)
	assert.Empty(t, preds, "dependency should be cascade-deleted when predecessor is deleted")
}

// TestCascadeDelete_FullChain verifies project -> nodes -> work_items -> sessions/dependencies.
func TestCascadeDelete_FullChain(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	sessRepo := NewSQLiteSessionRepo(db)
	depRepo := NewSQLiteDependencyRepo(db)

	proj := testutil.NewTestProject("FullChain")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Task1")
	wi2 := testutil.NewTestWorkItem(node.ID, "Task2")
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))

	sess := testutil.NewTestSession(wi1.ID, 45)
	require.NoError(t, sessRepo.Create(ctx, sess))

	dep := &domain.Dependency{PredecessorWorkItemID: wi1.ID, SuccessorWorkItemID: wi2.ID}
	require.NoError(t, depRepo.Create(ctx, dep))

	// Delete the project — everything should cascade.
	require.NoError(t, projRepo.Delete(ctx, proj.ID))

	_, err := nodeRepo.GetByID(ctx, node.ID)
	assert.Error(t, err, "node should be gone")

	_, err = wiRepo.GetByID(ctx, wi1.ID)
	assert.Error(t, err, "work item 1 should be gone")

	_, err = wiRepo.GetByID(ctx, wi2.ID)
	assert.Error(t, err, "work item 2 should be gone")

	_, err = sessRepo.GetByID(ctx, sess.ID)
	assert.Error(t, err, "session should be gone")

	preds, err := depRepo.ListPredecessors(ctx, wi2.ID)
	require.NoError(t, err)
	assert.Empty(t, preds, "dependency should be gone")
}

// TestCascadeDelete_ParentNodeToChildNodes verifies plan_nodes parent -> child cascade.
func TestCascadeDelete_ParentNodeToChildNodes(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)

	proj := testutil.NewTestProject("ParentChildCascade")
	require.NoError(t, projRepo.Create(ctx, proj))

	parent := testutil.NewTestNode(proj.ID, "Parent")
	require.NoError(t, nodeRepo.Create(ctx, parent))

	child := testutil.NewTestNode(proj.ID, "Child", testutil.WithParentID(parent.ID))
	require.NoError(t, nodeRepo.Create(ctx, child))

	require.NoError(t, nodeRepo.Delete(ctx, parent.ID))

	_, err := nodeRepo.GetByID(ctx, child.ID)
	assert.Error(t, err, "child node should be cascade-deleted when parent is deleted")
}

// TestForeignKey_WorkItemRequiresNode verifies FK constraint on work_items.node_id.
func TestForeignKey_WorkItemRequiresNode(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	wiRepo := NewSQLiteWorkItemRepo(db)

	wi := testutil.NewTestWorkItem("nonexistent-node", "Orphan Task")
	err := wiRepo.Create(ctx, wi)
	assert.Error(t, err, "creating work item with nonexistent node should fail FK constraint")
}

// TestForeignKey_NodeRequiresProject verifies FK constraint on plan_nodes.project_id.
func TestForeignKey_NodeRequiresProject(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	nodeRepo := NewSQLitePlanNodeRepo(db)

	node := testutil.NewTestNode("nonexistent-project", "Orphan Node")
	err := nodeRepo.Create(ctx, node)
	assert.Error(t, err, "creating node with nonexistent project should fail FK constraint")
}

// TestForeignKey_SessionRequiresWorkItem verifies FK constraint on work_session_logs.work_item_id.
func TestForeignKey_SessionRequiresWorkItem(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	sessRepo := NewSQLiteSessionRepo(db)

	sess := testutil.NewTestSession("nonexistent-wi", 30)
	err := sessRepo.Create(ctx, sess)
	assert.Error(t, err, "creating session with nonexistent work item should fail FK constraint")
}
