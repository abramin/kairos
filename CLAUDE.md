# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kairos is a single-user CLI project planner and session recommender. It answers: "I have X minutes now, what should I do?" by recommending work sessions across multiple projects, respecting hard deadlines, on-track status, anti-cram spacing, and cross-project variation.

**Status**: Core v1 packages (domain, contracts, repository, scheduler, services) and v2 intelligence layer (LLM-backed NL parsing, explanations, template drafting, interactive help, guided draft wizard) are implemented. Shell-only: `kairos` always launches the interactive shell; all commands run within the REPL.

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

## Local Dev Setup

```bash
export KAIROS_DB="$PWD/.kairos/kairos.db"
export KAIROS_TEMPLATES="$PWD/templates"
```

## Architecture

### Package Dependency Graph

```
cmd/kairos/main.go              (CLI entry point — wires all deps, runs interactive shell)
  ↓
internal/cli/                    (Shell command dispatch + App struct + bubbletea TUI)
  ├→ internal/cli/formatter/     (terminal output: tables, trees, colors, progress bars)
  ├→ internal/app/               (use-case interfaces + domain-oriented request/response types)
  ├→ internal/service/           (orchestrates repos + scheduler, implements app/ interfaces)
  │    ├→ internal/contract/     (transport-agnostic request/response DTOs)
  │    ├→ internal/repository/   (SQLite data access via interfaces, accepts db.DBTX)
  │    │    ├→ internal/domain/  (entity structs + enums)
  │    │    └→ internal/db/      (OpenDB, migrations, WAL+FK config, DBTX, UnitOfWork)
  │    ├→ internal/scheduler/    (pure scoring/allocation functions — NO DB calls)
  │    │    └→ internal/domain/
  │    ├→ internal/template/     (JSON template parsing + expression evaluation + validation)
  │    ├→ internal/importer/     (JSON import schema validation + domain conversion)
  │    └→ internal/generation/   (shared defaults resolution + dependency inference)
  └→ internal/intelligence/      (v2 LLM-powered services: intent, explain, template/project draft)
       ├→ internal/llm/          (Ollama HTTP client, structured JSON extraction, config)
       └→ internal/importer/     (validates draft output against import schema)

templates/                       (JSON template files loaded by TemplateService)
internal/testutil/               (in-memory DB helpers + builder-pattern fixtures)
internal/teatest/                (synchronous bubbletea test driver — deterministic, goroutine-free)
```

### Key Packages

**`internal/domain`** — Value objects and enums. All timestamps are `time.Time` (UTC). Nullable fields use pointers. String UUIDs for IDs. `Project` has a `ShortID` field for human-friendly identification (e.g., `PHI01`). `UserProfile` includes tuning weights plus `BaselineDailyMin` for daily commitment target. `SessionSummaryByType` aggregates session minutes per work item with type info (used by weekly review).

**`internal/contract`** — Request/response types for three core operations (`WhatNow`, `Status`, `Replan`). Builder constructors like `NewWhatNowRequest(availableMin)`. Custom error types with `Code` + `Message` fields.

**`internal/scheduler`** — Pure, deterministic functions with no DB access:
- `scorer.go` — `ScoreWorkItem(ScoringInput) ScoredCandidate` (6 weighted factors)
- `allocator.go` — `AllocateSlices()` two-pass: enforce variation, then fill; respects session bounds
- `risk.go` — `ComputeRisk(RiskInput) RiskResult` classifies projects as critical/at_risk/on_track
- `sorter.go` — `CanonicalSort()` deterministic ordering: risk level → due date → score → name → ID
- `reestimate.go` — `SmoothReEstimate()` applies `0.7*old + 0.3*implied`, never below logged

**`internal/repository`** — Seven interfaces (`ProjectRepo`, `PlanNodeRepo`, `ProjectSequenceRepo`, `WorkItemRepo`, `DependencyRepo`, `SessionRepo`, `UserProfileRepo`) with SQLite implementations prefixed `SQLite*Repo`. `ProjectSequenceRepo` provides atomic project-scoped `seq` allocation backed by `project_sequences`. Key query: `WorkItemRepo.ListSchedulable()` joins work_items + plan_nodes + projects for scoring input. `SessionRepo` also provides `ListRecentByProject()` and `ListRecentSummaryByType()` for review/replan features.

**`internal/service`** — Eight service interfaces wired via constructor injection (`NewWhatNowService(repos...)`). `ImportService` validates and converts JSON import files into domain objects. Core orchestration flow in `WhatNowService.Recommend()`: load candidates → compute risk per project → determine mode → score → sort → allocate. The pipeline is decomposed into reusable stages in `recommend_pipeline.go` (`ContextLoader`, `ComputeAggregates`, `BlockResolver`, `ScoreCandidates`, `AssembleResponse`) — these stages are shared by Status and Replan services.

**`internal/db`** — `OpenDB(path)` opens SQLite (`:memory:` for tests), enables WAL mode + foreign keys, runs migrations. Schema has 7 tables with indexes, soft-delete via `archived_at`, plus `project_sequences` for atomic project-wide `seq` allocation across nodes/work items. Migrations include `short_id` column on `projects` (unique index), `baseline_daily_min` on `user_profile`, and sequence backfills for legacy rows. `DBTX` interface abstracts over `*sql.DB` and `*sql.Tx` so repositories can operate within transactions. `UnitOfWork` (`SQLiteUnitOfWork`) provides `WithinTx()` for atomic multi-entity operations (e.g., import creates project + nodes + items + dependencies in one transaction).

**`internal/app`** — Use-case layer with domain-oriented request/response types and interface definitions. Provides `WhatNowUseCase`, `StatusUseCase`, `ReplanUseCase`, `LogSessionUseCase`, `InitProjectUseCase`, `ImportProjectUseCase` interfaces. Types (`WhatNowRequest`/`WhatNowResponse`, `StatusRequest`/`StatusResponse`, `ReplanRequest`/`ReplanResponse`) are the canonical API contracts; `internal/contract` re-exports them. Domain-aware error types (`WhatNowError`, `StatusError`, `ReplanError`) with structured codes.

**`internal/generation`** — Shared helpers for resolving work-item defaults and dependencies across both template and import paths. `ResolveWorkItemDefaults()` applies a 3-level cascade (item → node/defaults → hardcoded). `InferLinearDependencies()` creates predecessor→successor links from node/position ordering. `SessionPolicy` interface bridges template and import schema types without circular dependencies.

**`internal/template`** — JSON template schema types (`TemplateSchema`, `NodeConfig`, `WorkItemConfig`) and expression evaluation. `EvalExpr()` handles arithmetic with variables (e.g., `(i-1)*7`), `ExpandTemplate()` expands `{expr}` placeholders in template strings. Used by `TemplateService` to scaffold project structures from JSON files in `templates/`.

**`internal/testutil`** — `NewTestDB()` for in-memory databases. Builder fixtures: `NewTestProject(name, opts...)`, `NewTestNode(projectID, title, opts...)`, `NewTestWorkItem(nodeID, title, opts...)` with option functions like `WithTargetDate`, `WithPlannedMin`.

**`internal/teatest`** — Synchronous test driver for bubbletea models. `Driver` replaces `tea.Program` in tests — calls `Update()` directly and synchronously drains returned `Cmd`s. Cursor blink `Cmd`s (which block on timer channels) are skipped via a 10ms timeout. `MaxDrainDepth` (100) prevents infinite loops. Provides helpers: `PressKey()`, `PressEnter()`, `PressEsc()`, `Type()`, `Send()`, `View()`.

**`internal/importer`** — JSON import schema (`ImportSchema`, `NodeImport`, `WorkItemImport`) with validation (`ValidateImportSchema`) and conversion to domain objects (`Convert`). Used by both `ImportService` (file-based import) and `ProjectDraftService` (LLM-generated drafts).

**`internal/llm`** — Ollama HTTP client (`NewOllamaClient`), structured JSON extraction (`ExtractJSON[T]` — generic, strips markdown fences, validates via `SchemaValidator[T]`), config from env vars, and observability hooks (`Observer` interface). All LLM calls go through this package.

**`internal/intelligence`** — Five LLM-powered services:
- `IntentService` — NL→structured intent parsing (`ask` command). Pipeline: LLM parse → `ExtractJSON[ParsedIntent]` → `EnforceWriteSafety` → `ValidateIntentArguments` → `ConfirmationPolicy.Evaluate`
- `ExplainService` — Generates faithful narrative explanations from engine traces. Falls back to `Deterministic*` functions when LLM fails or evidence bindings are invalid
- `TemplateDraftService` — NL→template JSON generation. LLM output is validated against `template.ValidateSchema`
- `ProjectDraftService` — Multi-turn NL→project structure drafting. Interactive conversation produces `ImportSchema`, validated via `importer.ValidateImportSchema`, then imported via `ImportService`
- `HelpService` — LLM-powered Q&A about using Kairos. Supports one-shot questions and multi-turn chat (`StartChat`/`NextTurn`). Uses grounding validation to filter hallucinated commands/flags and a domain glossary embedded in the system prompt. Falls back to `DeterministicHelp()` (fuzzy-matching against the command spec) when LLM is unavailable

**`internal/cli`** — Bubbletea TUI with view-stack navigation and direct command dispatch. `App` struct (`root.go`) holds all service interfaces; v2 intelligence fields (`Intent`, `Explain`, `TemplateDraft`, `ProjectDraft`, `Help`) are nil when LLM is disabled. **Shell-only**: `kairos` always launches the interactive shell. All commands route through `command_dispatch.go` with inline implementations or delegates. Supported commands: built-in (`projects`, `use`, `inspect`, `status`, `what-now`, `log`, `start`, `finish`, `add`, `ask`, `explain`, `review`, `replan`, `context`, `draft`, `help`, `import`), entity groups (`project`, `node`, `work`, `session`, `template` with subcommands), and shell utilities (`clear`, `exit`/`quit`).

**TUI Architecture** (view-stack pattern):
- **`app_model.go`** — Root bubbletea `appModel`: owns a `viewStack []View`, a persistent `commandBar`, and `SharedState`. Handles `pushViewMsg`/`popViewMsg`/`replaceViewMsg` navigation messages and `wizardCompleteMsg` for multi-step flows. Wizard completion batches the follow-up command with `refreshViewMsg` so views reload after mutations.
- **`view.go`** — `View` interface (extends `tea.Model` with `ID()`, `ShortHelp()`, `Title()`). Eight `ViewID` constants: `ViewDashboard`, `ViewProjectList`, `ViewTaskList`, `ViewActionMenu`, `ViewRecommendation`, `ViewForm`, `ViewDraft`, `ViewHelpChat`.
- **`shared_state.go`** — `SharedState` holds active project/item context, terminal dimensions, project cache, and transient recommendation state. Shared across all views via pointer.
- **`command_bar.go`** — Persistent text input at the bottom of the TUI with autocomplete suggestions and history navigation.
- **`navigate.go`** — Navigation message types (`pushViewMsg`, `popViewMsg`, `replaceViewMsg`, `cmdOutputMsg`, `wizardCompleteMsg`, `refreshViewMsg`) and helper constructors (`pushView()`, `popView()`, `replaceView()`). `refreshViewMsg` notifies views to reload data after state mutations.
- **`command_dispatch.go`** — `commandBar.executeCommand()` dispatches text input to command handlers. Routes built-in commands (`projects`, `use`, `inspect`, `status`, `what-now`, `log`, `start`, `finish`, `add`, `ask`, `explain`, `review`, `replan`, `context`, `draft`, `help`, `import`, `clear`, `exit`/`quit`) and delegates to `cmdEntityGroup()` for entity commands (`project`, `node`, `work`, `session`, `template`).

**View files**:
- `view_dashboard.go` — Split-pane home screen: left pane has selectable project list with cursor, right pane shows project detail (stats: total/done/in-progress/todo). Async detail loading via `dashboardDetailLoadedMsg`.
- `view_project_list.go` — Navigable project list with cursor + `/` filtering
- `view_task_list.go` — Flattened plan tree (`taskRow`) for a project with cursor navigation. Supports node collapse/expand (`collapsedNodes` map) and digit-jump-to-sequence (`jumpBuf`). Handles `refreshViewMsg` to reload data after mutations.
- `view_recommendation.go` — Interactive what-now results with action selection
- `view_action_menu.go` — Action menu for selected work item with single-key shortcuts: start (s), log (l), adjust logged (a), mark done (d), edit (e), delete (x). Uses `replaceView()` for form-based actions.
- `view_log_form.go` — Form-based views: `newLogFormView()` (duration/units/notes), `newAdjustLoggedView()` (correct logged minutes), `newEditWorkItemView()` (title/planned/type), `newAddWorkItemView()` (add new item).
- `view_wizard.go` — Wraps `huh.Form` as a `View` on the stack; sends `wizardCompleteMsg` with chained callback on completion
- `view_draft.go` — Draft mode: wizard flow (no-LLM) or LLM conversational flow; produces `ImportSchema`
- `view_help_chat.go` — Interactive help chat view

**Command implementation files**:
- `command_dispatch.go` — Main command dispatcher (`executeCommand()`): routes built-in commands (`projects`, `use`, `inspect`, `status`, `what-now`, `log`, `start`, `finish`, `add`, `ask`, `explain`, `review`, `replan`, `context`, `draft`, `help`, `import`, `clear`, `exit`/`quit`) and entity groups. Most command handlers are implemented inline. Also includes `replan` command implementation.
- `cmd_entity.go` — Entity group routing (`cmdEntityGroup`): handles `project`, `node`, `work`, `session`, `template` commands. Routes to wizard for bare creation, confirmation for destructive ops, and delegates to `cmd_entity_dispatch.go`.
- `cmd_entity_dispatch.go` — Entity subcommand dispatch (`dispatchEntityCommand`, `dispatchProject`, `dispatchNode`, `dispatchWork`, `dispatchSession`, `dispatchTemplate`): direct service calls with flag parsing. Subcommands: project (list, inspect, add, update, archive, unarchive, remove, init, import), node (add, inspect, update, remove), work (add, inspect, update, done, archive, remove), session (log, list, remove), template (list, show).
- `cmd_navigation.go` — Navigation commands: `projects`, `use`, `inspect`, `status`, `what-now` handlers
- `cmd_work.go` — Work commands: `log`, `start`, `finish` with wizard chaining for missing args
- `cmd_intelligence.go` — LLM command handlers: `ask`, `explain`, `review`
- `intent_helpers.go` — Intent argument extraction helpers (`intArg`, `boolArg`, `stringArg`) for parsing LLM-generated intent arguments
- `work_actions.go` — Extracted action handlers reused across command bar and action menu: `execLogSession()`, `execStartItem()`, `execMarkDone()`. Each takes `context`, `App`, `SharedState` and returns formatted output or error.

**Supporting files**:
- `wizard.go` — Reusable huh form builders (`wizardSelectProject`, `wizardSelectWorkItem`, `wizardInputDuration`, etc.). Gruvbox-themed via `kairosHuhTheme()`.
- `resolve.go` — ID resolution helpers (`resolveNodeID`, `resolveWorkItemID`, `resolveProjectID`) that accept numeric seq IDs or UUIDs and resolve to full UUIDs using project context.
- `shell_history.go` — Persistent command history at `~/.kairos/shell_history` (max 500 lines). Arrow keys navigate history.
- `shell_completer.go` — Tab autocomplete for the command bar.
- `shell_cmd.go` — `runShell()` entrypoint, `destructiveCommands` map, utility functions.
- `command_hint.go` — Maps `ParsedIntent` (from LLM intent parsing) to concrete CLI command strings.
- `draft_wizard.go` — Interactive structure wizard for guided project creation without LLM. `generateShortID()` creates human-friendly IDs (e.g., `"PHYS01"`).
- `cmdspec.go` — `CommandSpec` describing available shell commands for help and grounding validation.

**`internal/cli/formatter`** — Terminal output formatting with lipgloss: tables, tree views, progress bars, color helpers, animated spinner (`spinner.go`). Separate formatters for what-now, status, explain, ask, draft, review, and help output. `review_fmt.go` includes Zettelkasten backlog nudge (flags reading items not yet processed into notes).

### Data Flow: what-now Recommendation Pipeline

```
WhatNowRequest
  → ContextLoader.Load() → RecommendationContext (candidates, sessions, profile)
  → ComputeAggregates() → ProjectAggregates (per-project risk, planned/logged)
  → DetermineMode() → Critical | Balanced
  → BlockResolver.Resolve() → (unblocked candidates, blockers)
  → ScoreCandidates() → []ScoredCandidate (6 weighted factors + reasons)
  → CanonicalSort() → deterministic ordering
  → AllocateSlices() → []WorkSlice + allocation blockers
  → AssembleResponse() → WhatNowResponse
```

### CLI ↔ App Layer Adapters

`app_ports.go` provides adapter methods on `App` that resolve to use-case interfaces (e.g., `a.logSessionUseCase()` returns `App.LogSession` if set, else falls back to `App.Sessions`). This enables dependency injection and gradual refactoring.

### v2 Intelligence: LLM Integration Pattern

All LLM features are **optional** — the `App.Intent`, `App.Explain`, `App.TemplateDraft`, `App.ProjectDraft`, and `App.Help` fields are nil when `KAIROS_LLM_ENABLED=false`. CLI commands check for nil before use and return a helpful error pointing to explicit commands. The draft wizard provides a fully functional no-LLM path for project creation.

Key design patterns:
- **Graceful fallback**: `ExplainService` falls back to `Deterministic*` functions (pure Go, no LLM) on any failure: connection error, timeout, invalid JSON, or unfaithful evidence bindings
- **Evidence binding validation**: LLM explanations must reference only valid `evidence_ref_key` values derived from the engine trace (`TraceKeys()` / `WeeklyTraceKeys()`). Invalid references trigger fallback
- **Write safety enforcement**: `EnforceWriteSafety()` overrides LLM-classified risk for known write intents — the LLM cannot bypass confirmation for mutations
- **Structured extraction**: `llm.ExtractJSON[T]()` handles markdown fences, brace matching, and schema validation generically
- **Grounding validation**: `HelpService` filters LLM responses to remove hallucinated commands/flags not present in the actual shell command spec
- **Wizard-to-LLM handoff**: Draft wizard collects structure interactively, then optionally hands off to LLM for refinement — both paths produce the same `ImportSchema` type

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
8. **DBTX abstraction** — repositories accept `db.DBTX` (satisfied by both `*sql.DB` and `*sql.Tx`), enabling transactional composition via `UnitOfWork.WithinTx()`
9. **Pipeline decomposition** — WhatNow recommendation broken into reusable stages (`ContextLoader`, `BlockResolver`, `ScoreCandidates`, etc.) shared by Status/Replan services

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
| `KAIROS_LLM_HELP_TIMEOUT_MS` | `10000` | help task timeout override |
| `KAIROS_LLM_MAX_RETRIES` | `1` | LLM retry count |
| `KAIROS_LLM_CONFIDENCE_THRESHOLD` | `0.85` | Auto-execute threshold for read-only intents |
| `KAIROS_LLM_LOG_CALLS` | `false` | Enable verbose LLM call logging to stderr |
| `KAIROS_LOG_USECASES` | `false` | Enable lightweight use-case execution logs (what-now, replan, log-session, init/import) to stderr |

## Key Dependencies

- `modernc.org/sqlite` — pure Go SQLite driver (**no CGO** — this is intentional, do not switch to `mattn/go-sqlite3`)
- `github.com/stretchr/testify` — test assertions use `assert`/`require` packages (not `testing` stdlib alone)
- `github.com/charmbracelet/bubbletea` — TUI framework powering the interactive shell (Elm-architecture Model/Update/View)
- `github.com/charmbracelet/bubbles` — text input component with suggestions and history
- `github.com/charmbracelet/huh` — form/wizard components for interactive input (selects, text inputs, confirms)
- `github.com/charmbracelet/lipgloss` — terminal styling (colors, bold, dim)

## Testing Patterns

Tests use in-memory SQLite via `testutil.NewTestDB()`. Builder-pattern fixtures create test data:

```go
db := testutil.NewTestDB(t)
proj := testutil.NewTestProject("My Project", testutil.WithTargetDate(deadline))
node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeKindModule))
item := testutil.NewTestWorkItem(node.ID, "Reading", testutil.WithPlannedMin(60))
```

Scheduler tests use pure functions with no DB setup needed — just construct `ScoringInput`/`RiskInput` structs directly.

TUI tests use `internal/teatest.Driver` for synchronous, deterministic testing. The CLI package wraps this in `TestDriver` (`tui_driver_test.go`) with Kairos-specific helpers:

```go
drv := NewTestDriver(t, app)
drv.Command("use PHI01")           // type + execute in command bar
assert.Equal(t, ViewTaskList, drv.ActiveViewID())
drv.PressKey("enter")              // interact with current view
assert.Equal(t, ViewActionMenu, drv.ActiveViewID())
```

Intelligence package tests use `httptest` servers to validate the full HTTP serialization path against real Ollama response shapes, preventing mock-drift.

## Reference Documents

- `docs/prd.md` — Domain model, features, duration/progress rules, CLI UX requirements
- `docs/prd-v2.md` — v2 intelligence layer requirements
- `docs/contracts.md` — WhatNow/Status/Replan request/response types and invariants
- `docs/llm-contracts.md` — LLM service contracts and prompt specifications
- `docs/orchestrator.md` — Build strategy, technical guardrails, checkpoints, mandatory tests
- `docs/orchestrator-v2.md` — v2 intelligence build strategy
- `docs/design.md` — System design notes
- `docs/repl.md` — Interactive shell design and command reference
- `docs/shell-first-prd.md` — Shell-first product direction and UX strategy
- `docs/template-sample.json` — Example template schema for project scaffolding
- `docs/testing-recommendations.md` — Testing strategy and recommendations
