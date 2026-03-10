package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

type nodeService struct {
	nodes repository.PlanNodeRepo
	deps  repository.DependencyRepo
	uow   db.UnitOfWork
}

func NewNodeService(nodes repository.PlanNodeRepo, deps repository.DependencyRepo, uow db.UnitOfWork) NodeService {
	return &nodeService{
		nodes: nodes,
		deps:  deps,
		uow:   uow,
	}
}

func (s *nodeService) Create(ctx context.Context, n *domain.PlanNode) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	n.CreatedAt = now
	n.UpdatedAt = now

	return s.uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
		txNodes := repository.NewSQLitePlanNodeRepo(tx)
		txSeqs := repository.NewSQLiteProjectSequenceRepo(tx)

		if n.Seq == 0 {
			seq, err := txSeqs.NextProjectSeq(ctx, n.ProjectID)
			if err != nil {
				return fmt.Errorf("assigning seq: %w", err)
			}
			n.Seq = seq
		}

		return txNodes.Create(ctx, n)
	})
}

func (s *nodeService) GetByID(ctx context.Context, id string) (*domain.PlanNode, error) {
	return s.nodes.GetByID(ctx, id)
}

func (s *nodeService) GetBySeq(ctx context.Context, projectID string, seq int) (*domain.PlanNode, error) {
	return s.nodes.GetBySeq(ctx, projectID, seq)
}

func (s *nodeService) ListByProject(ctx context.Context, projectID string) ([]*domain.PlanNode, error) {
	return s.nodes.ListByProject(ctx, projectID)
}

func (s *nodeService) ListChildren(ctx context.Context, parentID string) ([]*domain.PlanNode, error) {
	return s.nodes.ListChildren(ctx, parentID)
}

func (s *nodeService) ListRoots(ctx context.Context, projectID string) ([]*domain.PlanNode, error) {
	return s.nodes.ListRoots(ctx, projectID)
}

func (s *nodeService) Update(ctx context.Context, n *domain.PlanNode) error {
	n.UpdatedAt = time.Now().UTC()
	return s.nodes.Update(ctx, n)
}

func (s *nodeService) Delete(ctx context.Context, id string) error {
	return s.uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
		// Explicitly delete dependencies for work items under this node
		// (and its descendant nodes) before deleting the node itself.
		// This is defense-in-depth: ON DELETE CASCADE should handle this,
		// but explicit cleanup ensures no orphaned deps remain.
		txDeps := repository.NewSQLiteDependencyRepo(tx)
		if err := deleteNodeTreeDeps(ctx, tx, txDeps, id); err != nil {
			return fmt.Errorf("cleaning up dependencies: %w", err)
		}

		txNodes := repository.NewSQLitePlanNodeRepo(tx)
		return txNodes.Delete(ctx, id)
	})
}

// deleteNodeTreeDeps removes all dependency rows referencing work items
// under the given node and its descendant nodes (recursive).
func deleteNodeTreeDeps(ctx context.Context, tx db.DBTX, deps *repository.SQLiteDependencyRepo, nodeID string) error {
	// Collect all node IDs in the subtree (the node itself + all descendants).
	nodeIDs := []string{nodeID}
	if err := collectDescendantNodeIDs(ctx, tx, nodeID, &nodeIDs); err != nil {
		return err
	}

	// Delete dependencies where any work item under these nodes appears as
	// either predecessor or successor.
	for _, nid := range nodeIDs {
		_, err := tx.ExecContext(ctx, `
			DELETE FROM dependencies
			WHERE predecessor_work_item_id IN (SELECT id FROM work_items WHERE node_id = ?)
			   OR successor_work_item_id IN (SELECT id FROM work_items WHERE node_id = ?)`,
			nid, nid)
		if err != nil {
			return fmt.Errorf("deleting dependencies for node %s: %w", nid, err)
		}
	}
	return nil
}

// collectDescendantNodeIDs recursively collects all child node IDs.
func collectDescendantNodeIDs(ctx context.Context, tx db.DBTX, parentID string, out *[]string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM plan_nodes WHERE parent_id = ?`, parentID)
	if err != nil {
		return fmt.Errorf("listing child nodes: %w", err)
	}
	defer rows.Close()

	var childIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scanning child node: %w", err)
		}
		childIDs = append(childIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, cid := range childIDs {
		*out = append(*out, cid)
		if err := collectDescendantNodeIDs(ctx, tx, cid, out); err != nil {
			return err
		}
	}
	return nil
}
