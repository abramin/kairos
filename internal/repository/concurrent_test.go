package repository

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newConcurrentTestDB creates a file-backed SQLite database in a temp directory.
// Unlike :memory:, a file-backed DB shares state across all connections in the
// pool, which is required to test real concurrent access with WAL mode.
func newConcurrentTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "concurrent_test.db")
	database, err := db.OpenDB(dbPath)
	require.NoError(t, err, "failed to create concurrent test database")
	t.Cleanup(func() { database.Close() })
	return database
}

// TestConcurrentAccess_ReadDuringWrite verifies that concurrent ListSchedulable
// calls do not block or corrupt data while writes are in progress.
// SQLite WAL mode allows concurrent readers with a single writer, which is the
// normal operating mode for Kairos (single-user CLI with occasional writes).
func TestConcurrentAccess_ReadDuringWrite(t *testing.T) {
	database := newConcurrentTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(database)
	nodeRepo := NewSQLitePlanNodeRepo(database)
	wiRepo := NewSQLiteWorkItemRepo(database)

	// Seed initial data.
	proj := testutil.NewTestProject("ReadWrite")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	var wg sync.WaitGroup

	// Writer goroutine: create 20 work items sequentially.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			wi := testutil.NewTestWorkItem(node.ID, fmt.Sprintf("Item-%d", i),
				testutil.WithPlannedMin(30),
				testutil.WithSessionBounds(15, 60, 30),
			)
			if err := wiRepo.Create(ctx, wi); err != nil {
				t.Errorf("writer: create work item %d: %v", i, err)
				return
			}
		}
	}()

	// Reader goroutines: repeatedly list schedulable while writes happen.
	for r := 0; r < 5; r++ {
		wg.Add(1)
		go func(reader int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				candidates, err := wiRepo.ListSchedulable(ctx, false)
				if err != nil {
					t.Errorf("reader %d: list schedulable: %v", reader, err)
					return
				}
				// Candidates should be a consistent snapshot (not half-written).
				for _, c := range candidates {
					if c.ProjectID == "" || c.WorkItem.ID == "" {
						t.Errorf("reader %d: got candidate with empty ID", reader)
					}
				}
			}
		}(r)
	}

	wg.Wait()

	// Final check: all 20 items should be present.
	candidates, err := wiRepo.ListSchedulable(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, 20, len(candidates))
}

// TestConcurrentAccess_SequentialWritesConcurrentReads verifies that building
// up state through sequential writes while multiple readers query concurrently
// produces correct, consistent results with no data races.
func TestConcurrentAccess_SequentialWritesConcurrentReads(t *testing.T) {
	database := newConcurrentTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(database)
	nodeRepo := NewSQLitePlanNodeRepo(database)
	wiRepo := NewSQLiteWorkItemRepo(database)
	sessRepo := NewSQLiteSessionRepo(database)

	const projectCount = 10

	// Phase 1: Sequentially create projects + nodes + work items + sessions.
	// This simulates normal CLI usage (one operation at a time).
	for i := 0; i < projectCount; i++ {
		proj := testutil.NewTestProject(fmt.Sprintf("Project-%d", i),
			testutil.WithShortID(fmt.Sprintf("CC%02d", i)))
		require.NoError(t, projRepo.Create(ctx, proj))

		node := testutil.NewTestNode(proj.ID, fmt.Sprintf("Node-%d", i))
		require.NoError(t, nodeRepo.Create(ctx, node))

		wi := testutil.NewTestWorkItem(node.ID, fmt.Sprintf("Task-%d", i),
			testutil.WithPlannedMin(60),
			testutil.WithSessionBounds(15, 60, 30),
		)
		require.NoError(t, wiRepo.Create(ctx, wi))

		sess := testutil.NewTestSession(wi.ID, 30)
		require.NoError(t, sessRepo.Create(ctx, sess))
	}

	// Phase 2: Launch many concurrent readers to stress-test read consistency.
	var wg sync.WaitGroup
	const readers = 20

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(reader int) {
			defer wg.Done()

			// List projects
			projects, err := projRepo.List(ctx, false)
			if err != nil {
				t.Errorf("reader %d: list projects: %v", reader, err)
				return
			}
			if len(projects) != projectCount {
				t.Errorf("reader %d: expected %d projects, got %d", reader, projectCount, len(projects))
			}

			// List schedulable
			candidates, err := wiRepo.ListSchedulable(ctx, false)
			if err != nil {
				t.Errorf("reader %d: list schedulable: %v", reader, err)
				return
			}
			if len(candidates) != projectCount {
				t.Errorf("reader %d: expected %d candidates, got %d", reader, projectCount, len(candidates))
			}

			// List recent sessions
			sessions, err := sessRepo.ListRecent(ctx, 7)
			if err != nil {
				t.Errorf("reader %d: list sessions: %v", reader, err)
				return
			}
			if len(sessions) != projectCount {
				t.Errorf("reader %d: expected %d sessions, got %d", reader, projectCount, len(sessions))
			}
		}(r)
	}

	wg.Wait()
}

func TestConcurrentAccess_ProjectSequence_NoDuplicateSeq(t *testing.T) {
	database := newConcurrentTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(database)
	nodeRepo := NewSQLitePlanNodeRepo(database)
	wiRepo := NewSQLiteWorkItemRepo(database)
	uow := db.NewSQLiteUnitOfWork(database)

	proj := testutil.NewTestProject("Seq Concurrency")
	require.NoError(t, projRepo.Create(ctx, proj))

	// Seed one existing node to force allocator bootstrap from existing seq.
	root := testutil.NewTestNode(proj.ID, "Root")
	root.Seq = 1
	require.NoError(t, nodeRepo.Create(ctx, root))

	retryTx := func(fn func() error) error {
		const maxRetries = 10
		for attempt := 0; attempt < maxRetries; attempt++ {
			err := fn()
			if err == nil {
				return nil
			}
			if attempt == maxRetries-1 {
				return err
			}
			time.Sleep(time.Millisecond * time.Duration(1<<attempt))
		}
		return nil
	}

	const workers = 40
	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := retryTx(func() error {
				return uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
					txSeq := NewSQLiteProjectSequenceRepo(tx)
					txNode := NewSQLitePlanNodeRepo(tx)
					txWork := NewSQLiteWorkItemRepo(tx)

					seq, err := txSeq.NextProjectSeq(ctx, proj.ID)
					if err != nil {
						return err
					}

					if i%2 == 0 {
						node := testutil.NewTestNode(proj.ID, fmt.Sprintf("Node-%d", i))
						node.Seq = seq
						return txNode.Create(ctx, node)
					}

					work := testutil.NewTestWorkItem(root.ID, fmt.Sprintf("Work-%d", i))
					work.Seq = seq
					return txWork.Create(ctx, work)
				})
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	nodes, err := nodeRepo.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	items, err := wiRepo.ListByProject(ctx, proj.ID)
	require.NoError(t, err)

	seen := make(map[int]bool, len(nodes)+len(items))
	for _, n := range nodes {
		assert.Falsef(t, seen[n.Seq], "duplicate seq %d on node %s", n.Seq, n.ID)
		seen[n.Seq] = true
	}
	for _, w := range items {
		assert.Falsef(t, seen[w.Seq], "duplicate seq %d on work item %s", w.Seq, w.ID)
		seen[w.Seq] = true
	}

	assert.Equal(t, len(nodes)+len(items), len(seen), "all created entities should have unique sequence IDs")
}
