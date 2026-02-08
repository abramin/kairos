# Testing Recommendations

Remaining items from the Inverted Testing Pyramid Coverage Review.

## Week 1 (Completed)

- [x] #1 Concurrent DB access test (`internal/repository/concurrent_test.go`)
- [x] #2 Template directory validation test (`internal/template/all_templates_test.go`)
- [x] #5 ImportSchema cross-producer contract test (`internal/service/schema_contract_test.go`)
- [x] Bug fix: `cali_move_1.json` order field was number instead of string

## Week 1 (Remaining)

### #3 — Scheduler determinism property test
**Package:** `internal/scheduler`
**Priority:** High
**What:** Given identical `ScoringInput`, `ScoreWorkItem` + `CanonicalSort` + `AllocateSlices` must produce byte-identical output across 1000 randomized runs. This is the core determinism guarantee.
```
TestDeterminism_SameInputSameOutput
- Generate a fixed ScoringInput with 20+ candidates
- Run full pipeline 1000 times
- Assert output slices are identical every time
```

### #4 — WhatNow E2E round-trip test
**Package:** `internal/service`
**Priority:** High
**What:** Full round-trip from seeded DB through `WhatNowService.Recommend()` verifying all contract invariants hold: `allocated_min <= requested_min`, session bounds respected, critical mode only returns critical items, `safe_for_secondary_work` correct.
```
TestWhatNow_E2E_ContractInvariants
- Seed DB with 3 projects (1 critical, 1 at-risk, 1 on-track)
- Call Recommend with various available minutes
- Assert all contract invariants from docs/contracts.md
```

## Week 2

### #6 — Replan idempotency test
**Package:** `internal/service`
**Priority:** Medium
**What:** Verify `ReplanService` is idempotent: calling replan twice with unchanged input produces identical output.
```
TestReplan_Idempotent
- Seed project with sessions logged
- Call Replan twice
- Assert outputs are identical
```

### #7 — Risk classification boundary tests
**Package:** `internal/scheduler`
**Priority:** Medium
**What:** Test exact boundaries where projects transition between `on_track`, `at_risk`, and `critical` risk levels.
```
TestRisk_Boundaries
- Construct RiskInput at exact boundary values
- Assert correct classification at each boundary
- Test one unit above/below each threshold
```

### #8 — Session bounds enforcement test
**Package:** `internal/scheduler`
**Priority:** Medium
**What:** Verify `AllocateSlices` never produces allocations outside `[min_session_min, max_session_min]` bounds, even with edge-case inputs (1 minute available, 999 minutes available, etc.).
```
TestAllocate_SessionBoundsAlwaysRespected
- Fuzz available_min from 1 to 500
- Assert every allocation satisfies bounds
- Assert no allocation exceeds available_min
```

### #9 — Dependency chain ordering test
**Package:** `internal/service`
**Priority:** Medium
**What:** When work items have dependencies (predecessor/successor), recommendations must respect ordering. A successor should not be recommended while its predecessor is incomplete.
```
TestWhatNow_DependencyChainRespected
- Seed chain: A -> B -> C
- Assert only A is recommended initially
- Mark A done, assert B recommended
- Mark B done, assert C recommended
```

### #10 — Soft-delete exclusion test
**Package:** `internal/repository`
**Priority:** Medium
**What:** Archived entities (non-nil `archived_at`) must be excluded from all recommendation queries.
```
TestSoftDelete_ExcludedFromSchedulable
- Create work item, verify it appears in ListSchedulable
- Archive it (set archived_at)
- Verify it no longer appears in ListSchedulable
```

## Week 3

### #11 — Re-estimation smoothing test
**Package:** `internal/scheduler`
**Priority:** Medium
**What:** Verify `SmoothReEstimate` applies `0.7*old + 0.3*implied` formula correctly and never goes below logged time.
```
TestReEstimate_SmoothingFormula
- Test with various old/implied combinations
- Assert formula: 0.7*old + 0.3*implied
- Assert result >= logged time (floor)
```

### #12 — Template expression evaluation edge cases
**Package:** `internal/template`
**Priority:** Medium
**What:** `EvalExpr` and `ExpandTemplate` with edge cases: division by zero, negative results, missing variables, nested expressions.
```
TestEvalExpr_EdgeCases
- Division by zero -> error
- Missing variable -> error
- Nested arithmetic -> correct result
- Negative result -> handled correctly
```

### #13 — Import validation exhaustive negative tests
**Package:** `internal/importer`
**Priority:** Low
**What:** `ValidateImportSchema` should reject all known invalid schemas: missing project name, duplicate refs, orphan node refs, circular dependencies, invalid dates.
```
TestValidateImportSchema_RejectsInvalid
- Missing project short_id -> error
- Duplicate node refs -> error
- Work item referencing nonexistent node -> error
- Circular dependency -> error
- Invalid date format -> error
```

### #14 — CLI output formatter snapshot tests
**Package:** `internal/cli/formatter`
**Priority:** Low
**What:** Golden-file tests for terminal output formatters to catch unintended rendering changes.
```
TestFormatter_WhatNowOutput_Snapshot
- Construct known WhatNowResponse
- Format to string
- Compare against golden file
```

## Mock Audit Notes

Current test suite uses **zero mocks** — all tests use real in-memory SQLite via `testutil.NewTestDB()`. This is a strength for the repository and service layers. Consider adding interface-based mocks only for:
- LLM client (`internal/llm`) — to test intelligence services without a running Ollama
- Ollama HTTP responses — to test error handling, timeouts, malformed JSON
