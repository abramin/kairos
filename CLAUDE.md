# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kairos is a single-user CLI project planner and session recommender. It answers: "I have X minutes now, what should I do?" by recommending work sessions across multiple projects, respecting hard deadlines, on-track status, anti-cram spacing, and cross-project variation.

**Status**: Core v1 packages (domain, contracts, repository, scheduler, services) and v2 intelligence layer (LLM-backed NL parsing, explanations, template drafting) are implemented. CLI is fully wired with Cobra.

**Requires**: Go 1.25+

## Build Commands

```bash
make build                                            # build binary → ./kairos
make test                                             # go test ./... -count=1
make test-race                                        # tests with race detector
make vet                                              # go vet ./...
make lint                                             # vet alias
make test-cover                                       # tests with coverage report → coverage.out
make all                                              # vet + test + build
make install                                          # build + copy to $GOPATH/bin
make clean                                            # remove built binary + coverage.out
go test -run TestFunctionName ./internal/scheduler/   # single test
```

## Architecture

### Package Dependency Graph

```
cmd/kairos/main.go              (CLI entry point — wires all deps, runs Cobra root)
  ↓
internal/cli/                    (Cobra command definitions + App struct + shell REPL)
  ├→ internal/cli/formatter/     (terminal output: tables, trees, colors, progress bars)
  ├→ internal/service/           (orchestrates repos + scheduler)
  │    ├→ internal/contract/     (transport-agnostic request/response DTOs)
  │    ├→ internal/repository/   (SQLite data access via interfaces)
  │    │    ├→ internal/domain/  (entity structs + enums)
  │    │    └→ internal/db/      (OpenDB, migrations, WAL+FK config)
  │    ├→ internal/scheduler/    (pure scoring/allocation functions — NO DB calls)
  │    │    └→ internal/domain/
  │    ├→ internal/template/     (JSON template parsing + expression evaluation + validation)
  │    └→ internal/importer/     (JSON import schema validation + domain conversion)
  └→ internal/intelligence/      (v2 LLM-powered services: intent, explain, template/project draft)
       ├→ internal/llm/          (Ollama HTTP client, structured JSON extraction, config)
       └→ internal/importer/     (validates draft output against import schema)

templates/                       (JSON template files loaded by TemplateService)
internal/testutil/               (in-memory DB helpers + builder-pattern fixtures)
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

**`internal/service`** — Eight service interfaces wired via constructor injection (`NewWhatNowService(repos...)`). `ImportService` validates and converts JSON import files into domain objects. Core orchestration flow in `WhatNowService.Recommend()`: load candidates → compute risk per project → determine mode → score → sort → allocate.

**`internal/db`** — `OpenDB(path)` opens SQLite (`:memory:` for tests), enables WAL mode + foreign keys, runs migrations. Schema has 6 tables with indexes, soft-delete via `archived_at`.

**`internal/template`** — JSON template schema types (`TemplateSchema`, `NodeConfig`, `WorkItemConfig`) and expression evaluation. `EvalExpr()` handles arithmetic with variables (e.g., `(i-1)*7`), `ExpandTemplate()` expands `{expr}` placeholders in template strings. Used by `TemplateService` to scaffold project structures from JSON files in `templates/`.

**`internal/testutil`** — `NewTestDB()` for in-memory databases. Builder fixtures: `NewTestProject(name, opts...)`, `NewTestNode(projectID, title, opts...)`, `NewTestWorkItem(nodeID, title, opts...)` with option functions like `WithTargetDate`, `WithPlannedMin`.

**`internal/importer`** — JSON import schema (`ImportSchema`, `NodeImport`, `WorkItemImport`) with validation (`ValidateImportSchema`) and conversion to domain objects (`Convert`). Used by both `ImportService` (file-based import) and `ProjectDraftService` (LLM-generated drafts).

**`internal/llm`** — Ollama HTTP client (`NewOllamaClient`), structured JSON extraction (`ExtractJSON[T]` — generic, strips markdown fences, validates via `SchemaValidator[T]`), config from env vars, and observability hooks (`Observer` interface). All LLM calls go through this package.

**`internal/intelligence`** — Four LLM-powered services:
- `IntentService` — NL→structured intent parsing (`ask` command). Pipeline: LLM parse → `ExtractJSON[ParsedIntent]` → `EnforceWriteSafety` → `ValidateIntentArguments` → `ConfirmationPolicy.Evaluate`
- `ExplainService` — Generates faithful narrative explanations from engine traces. Falls back to `Deterministic*` functions when LLM fails or evidence bindings are invalid
- `TemplateDraftService` — NL→template JSON generation. LLM output is validated against `template.ValidateSchema`
- `ProjectDraftService` — Multi-turn NL→project structure drafting. Interactive conversation produces `ImportSchema`, validated via `importer.ValidateImportSchema`, then imported via `ImportService`

**`internal/cli`** — Cobra command tree. `App` struct holds all service interfaces; v2 intelligence fields are nil when LLM is disabled. Commands: `project` (incl. `init`, `import`, `draft`), `node`, `work`, `session`, `what-now`, `status`, `replan`, `template`, `shell`, `ask`, `explain` (subcommands: `now`, `why-not`), `review` (subcommand: `weekly`). Note: `kairos 45` is a shortcut for `kairos what-now --minutes 45`.

**`internal/cli/shell_cmd.go`** — Interactive REPL via `kairos shell`. Uses `go-prompt` for autocomplete. Built-in commands: `projects`, `use <id>`, `inspect [id]`, `status`, `what-now [min]`, `clear`, `help`, `exit`. Unrecognized input falls through to the full Cobra command tree. Prompt shows active project context: `kairos (proj_id) ❯`.

**`internal/cli/formatter`** — Terminal output formatting with lipgloss: tables, tree views, progress bars, color helpers. Separate formatters for what-now, status, explain, ask, and draft output.

### Data Flow: what-now Recommendation Pipeline

```
SchedulableCandidate (from repo)
  → ScoreWorkItem() → ScoredCandidate (with score + reasons)
  → CanonicalSort() → deterministic ordering
  → AllocateSlices() → []WorkSlice + []ConstraintBlocker
```

### v2 Intelligence: LLM Integration Pattern

All LLM features are **optional** — the `App.Intent`, `App.Explain`, `App.TemplateDraft`, and `App.ProjectDraft` fields are nil when `KAIROS_LLM_ENABLED=false`. CLI commands check for nil before use and return a helpful error pointing to explicit commands.

Key design patterns:
- **Graceful fallback**: `ExplainService` falls back to `Deterministic*` functions (pure Go, no LLM) on any failure: connection error, timeout, invalid JSON, or unfaithful evidence bindings
- **Evidence binding validation**: LLM explanations must reference only valid `evidence_ref_key` values derived from the engine trace (`TraceKeys()` / `WeeklyTraceKeys()`). Invalid references trigger fallback
- **Write safety enforcement**: `EnforceWriteSafety()` overrides LLM-classified risk for known write intents — the LLM cannot bypass confirmation for mutations
- **Structured extraction**: `llm.ExtractJSON[T]()` handles markdown fences, brace matching, and schema validation generically

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

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `KAIROS_DB` | `~/.kairos/kairos.db` | SQLite database path |
| `KAIROS_TEMPLATES` | `~/.kairos/templates` | Template JSON directory |
| `KAIROS_LLM_ENABLED` | `false` | Enable v2 intelligence features (ask, explain, review) |
| `KAIROS_LLM_ENDPOINT` | `http://localhost:11434` | Ollama server URL |
| `KAIROS_LLM_MODEL` | `llama3.2` | Ollama model name |
| `KAIROS_LLM_TIMEOUT_MS` | `10000` | Global LLM timeout |
| `KAIROS_LLM_PARSE_TIMEOUT_MS` | `10000` | `ask`/intent parser timeout override |
| `KAIROS_LLM_EXPLAIN_TIMEOUT_MS` | `6000` | explain/review timeout override |
| `KAIROS_LLM_TEMPLATE_DRAFT_TIMEOUT_MS` | `8000` | template draft timeout override |
| `KAIROS_LLM_PROJECT_DRAFT_TIMEOUT_MS` | `30000` | project draft timeout override |
| `KAIROS_LLM_MAX_RETRIES` | `1` | LLM retry count |
| `KAIROS_LLM_CONFIDENCE_THRESHOLD` | `0.85` | Auto-execute threshold for read-only intents |

## Key Dependencies

- `modernc.org/sqlite` — pure Go SQLite driver (**no CGO** — this is intentional, do not switch to `mattn/go-sqlite3`)
- `github.com/stretchr/testify` — test assertions use `assert`/`require` packages (not `testing` stdlib alone)
- `github.com/c-bata/go-prompt` — interactive REPL autocomplete for `kairos shell`

## Testing Patterns

Tests use in-memory SQLite via `testutil.NewTestDB()`. Builder-pattern fixtures create test data:

```go
db := testutil.NewTestDB(t)
proj := testutil.NewTestProject("My Project", testutil.WithTargetDate(deadline))
node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeKindModule))
item := testutil.NewTestWorkItem(node.ID, "Reading", testutil.WithPlannedMin(60))
```

Scheduler tests use pure functions with no DB setup needed — just construct `ScoringInput`/`RiskInput` structs directly.

## Reference Documents

- `docs/prd.md` — Domain model, features, duration/progress rules, CLI UX requirements
- `docs/prd-v2.md` — v2 intelligence layer requirements
- `docs/contracts.md` — WhatNow/Status/Replan request/response types and invariants
- `docs/llm-contracts.md` — LLM service contracts and prompt specifications
- `docs/orchestrator.md` — Build strategy, technical guardrails, checkpoints, mandatory tests
- `docs/orchestrator-v2.md` — v2 intelligence build strategy
- `docs/design.md` — System design notes
- `docs/repl.md` — Interactive shell design and command reference
- `docs/template-sample.json` — Example template schema for project scaffolding
