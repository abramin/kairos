# CLAUDE.md

## Project Overview

**Kairos**: CLI project planner & session recommender. "I have X minutes, what should I do?"
**Tech**: Go 1.25+, SQLite (modernc.org/sqlite), Bubbletea TUI, Ollama (LLM).
**Status**: v1 Core (Scheduler/Repo) + v2 Intelligence (LLM) complete. Shell-only REPL.

## Build & Environment

* **Commands**: `make [build|test|test-race|vet|install|clean|all]`
* **Coverage**: `make test-cover` -> `coverage.out`
* **Env**: `KAIROS_DB`, `KAIROS_TEMPLATES`, `KAIROS_LLM_ENABLED` (bool), `KAIROS_LLM_MODEL`.

## Architecture & Package Map

* `cmd/kairos`: Entry point; wires dependencies; launches REPL.
* `internal/domain`: Pure entities, enums, and UUIDs. UTC timestamps.
* `internal/app`: Use-case interfaces & DTOs (the API boundary).
* `internal/service`: Business logic; orchestrates repos & scheduler.
* `internal/scheduler`: **Pure functions only.** Scorer (6 weighted factors), Allocator (two-pass), Risk (classification).
* `internal/repository`: SQLite implementations (`db.DBTX` for transactions). Soft-delete via `archived_at`.
* `internal/intelligence`: LLM services (Ollama). Intent parsing, explanations, and project drafting.
* `internal/cli`: Bubbletea TUI.
* `view-stack`: Navigation via `pushView`/`popView`.
* `command_dispatch`: Routes REPL input to services.
* `wizard`: `huh`-based interactive forms.


* `internal/db`: SQLite config (WAL, FKs), migrations, and `UnitOfWork`.
* `internal/template/importer`: JSON schema validation and domain conversion.

## Core Design Principles

1. **Pure Scorer**: No DB calls in `internal/scheduler`.
2. **Deterministic**: Same input + same time = same recommendation (enforced by `CanonicalSort`).
3. **Boundary Integrity**: CLI calls Services; Services call Repos; Repos call DB.
4. **Graceful Fallback**: LLM features (Ask/Explain) must revert to deterministic Go logic if the LLM fails or returns unfaithful data.
5. **Write Safety**: LLM cannot bypass confirmation for mutations (`EnforceWriteSafety`).

## Key Invariants

* `allocated_min` ≤ `requested_min`.
* Sessions must respect `[min_session_min, max_session_min]`.
* Re-estimation uses smoothing: `0.7*old + 0.3*implied`.

## Testing Patterns

* **Unit**: Use `testutil.NewTestDB()` for in-memory SQLite.
* **Mocking**: Use builder-pattern fixtures in `internal/testutil`.
* **TUI**: Use `internal/teatest.Driver` for synchronous, deterministic TUI testing.
* **LLM**: Use `httptest` to validate JSON extraction against real Ollama shapes.

## Command Reference

* **Core**: `what-now`, `status`, `log`, `start`, `finish`, `replan`.
* **Admin**: `projects`, `use`, `inspect`, `add`, `import`.
* **v2**: `ask`, `explain`, `review`, `draft`.