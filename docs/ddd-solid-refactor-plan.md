# DDD/SOLID Refactor Technical Plan

## Goal

Reduce coupling and complexity by enforcing clear DDD/SOLID boundaries, transactional consistency, and smaller use-case-focused components.

## Scope

- `internal/service`
- `internal/domain`
- `internal/repository`
- `internal/template`
- `internal/importer`
- `internal/cli`

## Expected Outcome

- Services become orchestrators.
- Domain owns invariants and business behavior.
- Persistence is transactional for multi-entity writes.
- High-complexity flows (`what-now`, template/import) are decomposed into focused components.

## Target End State

- Application layer: use-case services with narrow dependencies.
- Domain layer: entities/value objects with behavior.
- Infrastructure layer: repositories plus transaction/UoW implementation.
- Adapter layer: CLI/contract mapping only (no business policy).
- Shared policy modules: defaults cascade, sequence assignment, dependency inference.

## Phase 1: Architectural Skeleton (1 week)

- Create explicit application package (for example `internal/app`) and define use-case ports for:
  - `Status`
  - `WhatNow`
  - `Replan`
  - `LogSession`
  - `InitProject`
  - `ImportProject`
- Keep `contract` DTOs in adapter boundary; add mappers in CLI layer (`internal/cli`) from app DTOs to contract/output.
- Remove unused dependency injection in `what-now` service (`internal/service/whatnow_service_impl.go:18`).

### Acceptance Criteria

- App/use-case interfaces no longer import `internal/contract`.
- CLI compiles and works via explicit mapping.

## Phase 2: Transaction Boundary / Unit of Work (1-2 weeks)

- Introduce `UnitOfWork` interface with `WithinTx(ctx, fn)` and tx-scoped repositories.
- Implement SQLite UoW adapter and tx-aware repo constructors.
- Refactor multi-write use cases to run in one transaction:
  - `LogSession` (`internal/service/session_service_impl.go`)
  - `InitProject` (`internal/service/template_service_impl.go`)
  - `ImportProject` (`internal/service/import_service_impl.go`)
- Add rollback integration tests for failure in the middle of each flow.

### Acceptance Criteria

- No partial writes when any step fails.
- Existing behavior is preserved for successful paths.

## Phase 3: Domain Behavior and Invariants (1-2 weeks)

- Add domain behavior methods such as:
  - `Project.ValidateShortID(...)`
  - `WorkItem.MarkDone(now)`
  - `WorkItem.MarkInProgress(now)`
  - `WorkItem.ApplySession(minutes, unitsDelta, now)`
  - `WorkItem.ReestimateIfNeeded(...)`
- Move invariant logic from service methods into entities/value objects.
- Keep services focused on orchestration only (load aggregate, invoke behavior, persist).

### Acceptance Criteria

- Service methods shrink and stop embedding core business rules.
- Domain unit tests cover state transitions and invalid transitions.

## Phase 4: Decompose What-Now Flow (2 weeks)

- Split `Recommend` flow (`internal/service/whatnow_service_impl.go`) into:
  - `RecommendationContextLoader`
  - `RiskCalculator`
  - `DependencyBlockResolver` (batch predecessor check)
  - `CandidateScorer`
  - `SliceAllocator`
  - `RecommendationAssembler`
- Replace per-item dependency checks with bulk repository query (for example `ListBlockedSuccessors(ids)`).

### Acceptance Criteria

- `Recommend` orchestration is broken into focused, testable units.
- Recommendation outputs remain compatible with current behavior.
- Query count is reduced versus N+1 checks.

## Phase 5: Unify Template and Import Generation Policies (1-2 weeks)

- Extract shared generation module (for example `internal/generation`) for:
  - defaults cascade for work item fields
  - sequence assignment policy
  - dependency inference policy
  - date parsing helpers with explicit error handling
- Refactor both generators to consume shared policies:
  - `internal/template/engine.go`
  - `internal/importer/convert.go`

### Acceptance Criteria

- Duplicate policy logic is removed.
- Existing template/import tests (including golden/regression tests) pass.

## Phase 6: Sequence Allocation Hardening (1 week)

- Replace `MAX(seq)+1` allocation (`internal/repository/sqlite_plannode.go:68`) with atomic allocator.
- Recommended approach:
  - add `project_sequences(project_id, next_seq)` table
  - increment atomically in transaction
- Add migration and allocator repository port.
- Update node/work-item create flows to request sequence in same transaction.

### Acceptance Criteria

- Concurrent creates do not produce duplicate sequence numbers.
- Concurrency stress tests pass.

## Phase 7: Cleanup, ADRs, and Observability (3-4 days)

- Write ADRs for:
  - layer boundaries
  - transaction/UoW strategy
  - sequence allocation design
- Add lightweight metrics/logging around critical use cases:
  - `what-now`
  - `replan`
  - `log-session`
  - `init/import`
- Remove dead code and tighten constructors to required dependencies only.

## Suggested PR Slices

1. App ports plus contract mappers plus remove unused dependencies.
2. UoW abstraction plus SQLite implementation.
3. Transactional `LogSession`.
4. Transactional template init/import.
5. Domain behavior methods plus service simplification.
6. What-now decomposition scaffolding.
7. Batch dependency blocking plus performance tests.
8. Shared generation policy module plus template/import migration.
9. Sequence allocator migration plus adoption.
10. ADR/docs/cleanup.

## Testing Strategy

- Unit tests: domain behavior, policy modules, scoring/risk components.
- Integration tests: transaction rollback and cross-repo consistency.
- Concurrency tests: sequence allocator under parallel creates.
- Regression tests: `status`, `what-now`, and `replan` output parity.

## Definition of Done

- No service method performs multi-entity writes outside a transaction.
- Application layer does not depend directly on adapter contracts.
- Domain invariants are enforced in domain methods with tests.
- `what-now` is decomposed and no longer monolithic.
- Template/import policy duplication is eliminated.
- Sequence collisions are prevented under concurrency.

