# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kairos is a single-user CLI project planner and session recommender. It answers: "I have X minutes now, what should I do?" by recommending work sessions across multiple projects, respecting hard deadlines, on-track status, anti-cram spacing, and cross-project variation.

**Status**: Core packages implemented (domain, contracts, repository, scheduler, services). CLI wiring and tests are the main remaining work.

## Build Commands

```bash
go build -o kairos ./cmd/kairos
go test ./...
go test -run TestFunctionName ./internal/scheduler/   # single test
go vet ./...
golangci-lint run
```

## Architecture

### Package Dependency Graph

```
cmd/kairos/main.go          (CLI entry point — stubbed, needs cobra wiring)
  ↓
internal/service/            (orchestrates repos + scheduler)
  ├→ internal/contract/      (transport-agnostic request/response DTOs)
  ├→ internal/repository/    (SQLite data access via interfaces)
  │    ├→ internal/domain/   (entity structs + enums)
  │    └→ internal/db/       (OpenDB, migrations, WAL+FK config)
  ├→ internal/scheduler/     (pure scoring/allocation functions — NO DB calls)
  │    └→ internal/domain/
  └→ internal/template/      (JSON template parsing + expression evaluation)

templates/                   (JSON template files loaded by TemplateService)
internal/testutil/           (in-memory DB helpers + builder-pattern fixtures)
```

### Key Packages

**`internal/domain`** — Value objects and enums. All timestamps are `time.Time` (UTC). Nullable fields use pointers. String UUIDs for IDs.

**`internal/contract`** — Request/response types for three core operations (`WhatNow`, `Status`, `Replan`). Builder constructors like `NewWhatNowRequest(availableMin)`. Custom error types with `Code` + `Message` fields.

**`internal/scheduler`** — Pure, deterministic functions with no DB access:
- `scorer.go` — `ScoreWorkItem(ScoringInput) ScoredCandidate` (6 weighted factors)
- `allocator.go` — `AllocateSlices()` two-pass: enforce variation, then fill; respects session bounds
- `risk.go` — `ComputeRisk(RiskInput) RiskResult` classifies projects as critical/at_risk/on_track
- `sorter.go` — `CanonicalSort()` deterministic ordering: risk level → due date → score → name → ID
- `reestimate.go` — `SmoothReEstimate()` applies `0.7*old + 0.3*implied`, never below logged

**`internal/repository`** — Six interfaces (`ProjectRepo`, `PlanNodeRepo`, `WorkItemRepo`, `DependencyRepo`, `SessionRepo`, `UserProfileRepo`) with SQLite implementations prefixed `SQLite*Repo`. Key query: `WorkItemRepo.ListSchedulable()` joins work_items + plan_nodes + projects for scoring input.

**`internal/service`** — Seven service interfaces wired via constructor injection (`NewWhatNowService(repos...)`). Core orchestration flow in `WhatNowService.Recommend()`: load candidates → compute risk per project → determine mode → score → sort → allocate.

**`internal/db`** — `OpenDB(path)` opens SQLite (`:memory:` for tests), enables WAL mode + foreign keys, runs migrations. Schema has 6 tables with indexes, soft-delete via `archived_at`.

**`internal/template`** — JSON template schema types (`TemplateSchema`, `NodeConfig`, `WorkItemConfig`) and expression evaluation. `EvalExpr()` handles arithmetic with variables (e.g., `(i-1)*7`), `ExpandTemplate()` expands `{expr}` placeholders in template strings. Used by `TemplateService` to scaffold project structures from JSON files in `templates/`.

**`internal/testutil`** — `NewTestDB()` for in-memory databases. Builder fixtures: `NewTestProject(name, opts...)`, `NewTestNode(projectID, title, opts...)`, `NewTestWorkItem(nodeID, title, opts...)` with option functions like `WithTargetDate`, `WithPlannedMin`.

### Data Flow: what-now Recommendation Pipeline

```
SchedulableCandidate (from repo)
  → ScoreWorkItem() → ScoredCandidate (with score + reasons)
  → CanonicalSort() → deterministic ordering
  → AllocateSlices() → []WorkSlice + []ConstraintBlocker
```

## Naming Conventions

- Repository implementations: `SQLite<Entity>Repo` (e.g., `SQLiteProjectRepo`)
- Service implementations: unexported struct in `<name>_service_impl.go` files
- Error constants: `Err...` or `<Service>Err...` (e.g., `ErrNoCandidates`, `StatusErrInvalidScope`)
- Reason codes: `Reason...` (e.g., `ReasonDeadlinePressure`)
- Blocker codes: `Blocker...` (e.g., `BlockerSessionMinExceedsAvail`)
- Date layouts: `"2006-01-02"` for project dates, `time.RFC3339` for timestamps

## Key Design Principles

1. **Scorer is pure** — no DB calls inside scoring logic; accept data, return scores
2. **Deterministic output** — same input snapshot always produces same recommendations; canonical sorting enforces this
3. **Soft-delete via `archived_at`** — archived entities excluded from recommendations
4. **Module boundaries** — CLI never bypasses services to hit DB directly
5. **Replanning is explicit** — user-triggered or command-triggered, no background daemon
6. **Templates scaffold upfront** — full node/work-item tree generated at init, not lazy-loaded
7. **Re-estimation via smoothing** — `new_planned = 0.7*old + 0.3*implied` from unit pace; never hard-jump

## Contract Invariants

- `allocated_min <= requested_min`
- Each allocation satisfies session bounds (`min_session_min` ≤ `allocated_min` ≤ `max_session_min`)
- Critical mode only recommends critical-scope items
- `safe_for_secondary_work` is true only when no critical project is off-track
- `progress_time_pct` can exceed 100% (logged > planned is valid)
- Replan is idempotent over unchanged input

## Mandatory Acceptance Tests

Defined in `docs/orchestrator.md` — must pass before v1:

1. Critical deadline → critical mode recommendations only
2. Balanced mode allows secondary project when primary is safe
3. Session bounds never violated
4. Unit-based re-estimation smoothing works correctly
5. Template generation produces full expected structure
6. Deadline update changes risk level and recommendation ranking
7. Archived/removed entities excluded from suggestions
8. Deterministic output stability on repeated runs

## Dependencies

- `modernc.org/sqlite` — pure Go SQLite driver (no CGO)
- `github.com/google/uuid` — UUID generation for entity IDs
- `github.com/stretchr/testify` — test assertions (`assert`/`require` packages)

## Reference Documents

- `docs/prd.md` — Domain model, features, duration/progress rules, CLI UX requirements
- `docs/contracts.md` — WhatNow/Status/Replan request/response types and invariants
- `docs/orchestrator.md` — Build strategy, technical guardrails, checkpoints, mandatory tests
- `docs/template-sample.json` — Example template schema for project scaffolding
