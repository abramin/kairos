````markdown
# docs/contracts.md

## Purpose

This file defines application-level request/response contracts for the three core operations:

- `what-now`
- `status`
- `replan`

These contracts are transport-agnostic and can be used by CLI handlers, tests, and future API adapters.

---

## Common Types

```ts
type UUID = string;          // canonical UUID
type ISODate = string;       // YYYY-MM-DD
type ISODateTime = string;   // RFC3339
````

```ts
type RiskLevel = "on_track" | "at_risk" | "critical";
type ProjectStatus = "active" | "paused" | "done" | "archived";
type WorkItemStatus = "todo" | "in_progress" | "done" | "skipped" | "archived";
type PlanMode = "balanced" | "critical";
```

```ts
interface TimeWindow {
  from?: ISODateTime;
  to?: ISODateTime;
}
```

```ts
interface RecommendationReason {
  code:
    | "DEADLINE_PRESSURE"
    | "BEHIND_PACE"
    | "SPACING_OK"
    | "SPACING_BLOCKED"
    | "VARIATION_BONUS"
    | "VARIATION_PENALTY"
    | "BOUNDS_APPLIED"
    | "DEPENDENCY_BLOCKED"
    | "ON_TRACK_SAFE_MIX"
    | "CRITICAL_FOCUS";
  message: string;
  weight_delta?: number; // optional scoring contribution shown for explainability
}
```

```ts
interface WorkSlice {
  work_item_id: UUID;
  project_id: UUID;
  node_id: UUID;
  title: string;

  allocated_min: number;          // minutes assigned in this recommendation
  min_session_min: number;
  max_session_min: number;
  default_session_min: number;
  splittable: boolean;

  due_date?: ISODate;
  risk_level: RiskLevel;
  score: number;

  reasons: RecommendationReason[];
}
```

```ts
interface RiskSummary {
  project_id: UUID;
  project_name: string;
  risk_level: RiskLevel;

  due_date?: ISODate;
  days_left?: number;

  planned_min_total: number;
  logged_min_total: number;
  remaining_min_total: number;   // includes configured buffer where applicable

  required_daily_min: number;
  recent_daily_min: number;
  slack_min_per_day: number;     // recent - required (can be negative)

  progress_time_pct: number;     // logged/planned * 100
}
```

```ts
interface ConstraintBlocker {
  entity_type: "project" | "node" | "work_item";
  entity_id: UUID;
  code:
    | "NOT_BEFORE"
    | "NOT_AFTER"
    | "DEPENDENCY"
    | "ARCHIVED"
    | "STATUS_DONE"
    | "SESSION_MIN_EXCEEDS_AVAILABLE";
  message: string;
}
```

---

## 1) what-now Contract

### Request

```ts
interface WhatNowRequest {
  available_min: number;                 // required, >0
  now?: ISODateTime;                     // default: system clock
  project_scope?: UUID[];                // optional filter: only these projects
  include_archived?: boolean;            // default false
  dry_run?: boolean;                     // default false, no persistence side effects
  max_slices?: number;                   // default 3
  enforce_variation?: boolean;           // default true
  explain?: boolean;                     // default true
}
```

### Response

```ts
interface WhatNowResponse {
  generated_at: ISODateTime;
  mode: PlanMode;                        // balanced | critical

  requested_min: number;
  allocated_min: number;
  unallocated_min: number;

  recommendations: WorkSlice[];          // ordered selection for this block
  blockers: ConstraintBlocker[];         // optional non-fatal exclusions

  top_risk_projects: RiskSummary[];      // sorted critical -> on_track
  policy_messages: string[];             // e.g. "OU on track, secondary work is safe"

  warnings: string[];
}
```

### Errors

```ts
type WhatNowErrorCode =
  | "INVALID_AVAILABLE_MIN"
  | "NO_CANDIDATES"
  | "DATA_INTEGRITY"
  | "INTERNAL_ERROR";

interface WhatNowError {
  code: WhatNowErrorCode;
  message: string;
  details?: Record<string, unknown>;
}
```

### Invariants

* `allocated_min <= requested_min`
* each `allocated_min` must satisfy session bounds unless exact-fit exception is explicitly enabled (v1: **not enabled**)
* if `mode === "critical"`, all recommendations must belong to critical scope
* recommendations must be deterministic for same input snapshot

---

## 2) status Contract

### Request

```ts
interface StatusRequest {
  now?: ISODateTime;                     // default: system clock
  project_scope?: UUID[];                // optional filter
  include_archived?: boolean;            // default false
  recalc?: boolean;                      // default true
  include_blockers?: boolean;            // default false
  include_recent_sessions_days?: number; // default 7
}
```

### Response

```ts
interface ProjectStatusView {
  project_id: UUID;
  project_name: string;
  status: ProjectStatus;
  risk_level: RiskLevel;

  due_date?: ISODate;
  days_left?: number;

  progress_time_pct: number;             // time-weighted
  progress_structural_pct: number;       // done items / total items

  planned_min_total: number;
  logged_min_total: number;
  remaining_min_total: number;

  required_daily_min: number;
  recent_daily_min: number;
  slack_min_per_day: number;

  safe_for_secondary_work: boolean;      // true when critical obligations are on track
  notes: string[];
}
```

```ts
interface GlobalStatusSummary {
  generated_at: ISODateTime;

  counts: {
    projects_total: number;
    on_track: number;
    at_risk: number;
    critical: number;
  };

  global_mode_if_now: PlanMode;          // predicted mode for what-now at this moment
  policy_message: string;                // concise guidance sentence
}
```

```ts
interface StatusResponse {
  summary: GlobalStatusSummary;
  projects: ProjectStatusView[];         // sorted critical -> on_track, then nearest due_date
  blockers?: ConstraintBlocker[];
  warnings: string[];
}
```

### Errors

```ts
type StatusErrorCode =
  | "INVALID_SCOPE"
  | "DATA_INTEGRITY"
  | "INTERNAL_ERROR";

interface StatusError {
  code: StatusErrorCode;
  message: string;
  details?: Record<string, unknown>;
}
```

### Invariants

* `progress_time_pct` in `[0, 100+]` (can exceed 100 if logged > planned)
* `safe_for_secondary_work` true only if no critical project is currently off-track
* sorting order is deterministic

---

## 3) replan Contract

### Request

```ts
type ReplanTrigger =
  | "MANUAL"
  | "DEADLINE_UPDATED"
  | "ITEM_ADDED"
  | "ITEM_REMOVED"
  | "SESSION_LOGGED"
  | "TEMPLATE_INIT";

interface ReplanRequest {
  trigger: ReplanTrigger;
  now?: ISODateTime;                     // default: system clock
  project_scope?: UUID[];                // optional, default all active projects

  strategy?: "rebalance" | "deadline_first"; // default rebalance
  preserve_existing_assignments?: boolean;    // default true (v1 mostly true)
  include_archived?: boolean;                 // default false
  explain?: boolean;                          // default true
}
```

### Response

```ts
interface ProjectReplanDelta {
  project_id: UUID;
  project_name: string;

  risk_before: RiskLevel;
  risk_after: RiskLevel;

  required_daily_min_before: number;
  required_daily_min_after: number;

  remaining_min_before: number;
  remaining_min_after: number;

  changed_items_count: number;           // items whose planning fields changed
  notes: string[];
}
```

```ts
interface ReplanResponse {
  generated_at: ISODateTime;
  trigger: ReplanTrigger;
  strategy: "rebalance" | "deadline_first";

  recomputed_projects: number;
  deltas: ProjectReplanDelta[];

  global_mode_after: PlanMode;
  warnings: string[];

  // Optional explainability payload for CLI --verbose
  explanation?: {
    critical_projects: UUID[];
    rules_applied: string[];
  };
}
```

### Errors

```ts
type ReplanErrorCode =
  | "INVALID_TRIGGER"
  | "NO_ACTIVE_PROJECTS"
  | "DATA_INTEGRITY"
  | "INTERNAL_ERROR";

interface ReplanError {
  code: ReplanErrorCode;
  message: string;
  details?: Record<string, unknown>;
}
```

### Invariants

* replan must be idempotent over unchanged input snapshot
* no archived entities may be reactivated implicitly
* `risk_after` must be recomputed from canonical formulas, not cached stale values

---

## Canonical Sorting Rules (for deterministic CLI output)

1. Risk: `critical` > `at_risk` > `on_track`
2. Earliest due date first (null due dates last)
3. Higher score first (for recommendations)
4. Stable tie-breaker: lexical `project_name`, then `work_item_id`

---

## Minimal Validation Rules

* `available_min > 0`
* `min_session_min >= 1`
* `max_session_min >= min_session_min`
* `default_session_min` in `[min_session_min, max_session_min]`
* `units_total >= 0`, `units_done >= 0`, `units_done <= units_total` when total known
* due dates must be valid ISO dates

---

## Versioning

Add a contract version in all top-level responses once transport/API is introduced:

```ts
interface ContractEnvelope<T> {
  contract_version: "1.0.0";
  data: T;
}
```

For CLI-internal use, keep plain objects until API extraction is needed.
