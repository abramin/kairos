# PRD v3: Time Chart & Workout Logging

## Problem

Kairos tracks project session minutes but provides no visual summary of how time is distributed across projects and weeks. There is also no way to log physical training — an activity with its own categories and time investment — so the user has no unified picture of where their hours go.

## Goals

1. Provide a stacked horizontal bar chart rendered in the TUI showing weekly time breakdowns by category.
2. Introduce workout logging as a first-class concept, separate from the project/plan-node/work-item hierarchy.
3. Surface both project sessions and workout logs in the chart, colored by category.

## Non-Goals

- Workout planning, scheduling, or recommendation (no risk scoring, deadlines, or spacing for workouts).
- Export or image generation of charts.
- LLM integration for this feature.
- Calorie, heart rate, or any biometric tracking.

---

## Domain Model

### Workout Log

A workout log is a standalone time entry not tied to any project or work item.

```
WorkoutLog {
  ID            string        // UUID
  Category      WorkoutCategory
  Minutes       int           // > 0
  PerformedAt   time.Time     // UTC, defaults to now
  Notes         *string       // optional free text
  CreatedAt     time.Time
}
```

### WorkoutCategory (enum)

Predefined categories representing training modalities:

| Value          | Label          |
|----------------|----------------|
| `qigong`       | Qigong         |
| `calisthenics` | Calisthenics   |
| `running`      | Running        |
| `kettlebell`   | Kettlebell     |
| `gmb`          | GMB Movement   |
| `stretching`   | Stretching     |
| `other`        | Other          |

The enum is extensible in code but the initial set covers the user's established routine. Adding a new category requires a code change (no dynamic user-defined categories in v3).

### Chart Data

The chart is a read-only aggregation. No new domain entity is needed — it is assembled at query time from two sources:

1. **Project sessions** — existing `sessions` table, grouped by project name and ISO week.
2. **Workout logs** — new `workout_logs` table, grouped by category and ISO week.

```
WeeklyBreakdown {
  ISOWeek     string                    // e.g. "2026-W08"
  WeekLabel   string                    // e.g. "Feb 16–22"
  Segments    []CategorySegment         // ordered by minutes desc
  TotalMin    int
}

CategorySegment {
  Label       string                    // project name or workout category label
  Minutes     int
  Kind        SegmentKind               // "project" | "workout"
}
```

---

## Schema

### New Table: `workout_logs`

```sql
CREATE TABLE workout_logs (
    id            TEXT PRIMARY KEY,
    category      TEXT NOT NULL,
    minutes       INTEGER NOT NULL CHECK (minutes > 0),
    performed_at  TEXT NOT NULL,     -- RFC3339
    notes         TEXT,
    created_at    TEXT NOT NULL      -- RFC3339
);

CREATE INDEX idx_workout_logs_performed ON workout_logs(performed_at);
CREATE INDEX idx_workout_logs_category  ON workout_logs(category);
```

No `archived_at` — workout logs are either present or hard-deleted. They have no lifecycle beyond creation.

### Existing Table: `sessions`

No schema changes. The chart reads `sessions.logged_min`, joining through `work_items → plan_nodes → projects` to get the project name and `sessions.created_at` for week grouping.

---

## Repository

### WorkoutLogRepo (new interface)

```go
type WorkoutLogRepo interface {
    Create(ctx context.Context, tx db.DBTX, log domain.WorkoutLog) error
    Delete(ctx context.Context, tx db.DBTX, id string) error
    ListByDateRange(ctx context.Context, tx db.DBTX, from, to time.Time) ([]domain.WorkoutLog, error)
    ListRecent(ctx context.Context, tx db.DBTX, limit int) ([]domain.WorkoutLog, error)
}
```

Implementation: `SQLiteWorkoutLogRepo` in `internal/repository/`.

### SessionRepo (existing, new query)

Add a query method for chart aggregation:

```go
// ListSessionMinutesByWeek returns (project_name, iso_week, total_minutes) tuples
// for all sessions with created_at in [from, to].
ListSessionMinutesByWeek(ctx context.Context, tx db.DBTX, from, to time.Time) ([]ProjectWeekMinutes, error)
```

This avoids pulling all sessions into memory and aggregating in Go.

---

## Service

### WorkoutService (new)

```go
type WorkoutService interface {
    LogWorkout(ctx context.Context, req LogWorkoutRequest) (domain.WorkoutLog, error)
    DeleteWorkout(ctx context.Context, id string) error
    ListRecent(ctx context.Context, limit int) ([]domain.WorkoutLog, error)
}
```

`LogWorkoutRequest`:

```go
type LogWorkoutRequest struct {
    Category    domain.WorkoutCategory
    Minutes     int
    PerformedAt *time.Time  // nil = now
    Notes       *string
}
```

Validation: category must be a known enum value, minutes > 0.

### ChartService (new)

```go
type ChartService interface {
    WeeklyBreakdown(ctx context.Context, numWeeks int) ([]domain.WeeklyBreakdown, error)
}
```

Implementation:
1. Compute `from` = start of ISO week `(now - numWeeks*7 days)`, `to` = end of current ISO week.
2. Call `SessionRepo.ListSessionMinutesByWeek(from, to)`.
3. Call `WorkoutLogRepo.ListByDateRange(from, to)`, aggregate by category and ISO week in Go.
4. Merge both into `[]WeeklyBreakdown`, sorted most recent week first.

Default `numWeeks`: 6.

---

## CLI Commands

### `workout log <category> <minutes> [--date YYYY-MM-DD] [--notes "..."]`

Log a workout entry. Category is matched case-insensitively and supports unambiguous prefix matching (`cal` → `calisthenics`, `run` → `running`, `q` → `qigong`).

If no arguments are given, launch a wizard (huh form) with category select + duration input.

**Examples:**
```
workout log calisthenics 45
workout log qigong 20 --date 2026-02-18
workout log running 30 --notes "easy 5k"
workout log                              # → wizard
```

### `workout list [--weeks N]`

Show recent workout logs. Default: current week. With `--weeks 3`, show last 3 weeks grouped by date.

### `workout delete <id>`

Delete a workout log by ID. Requires confirmation.

### `chart [--weeks N]`

Render the stacked horizontal bar chart in the terminal. Default: 6 weeks.

**Output format:**

```
                      Time Breakdown (6 weeks)

  Feb 16–22  ▓▓▓▓▓▓▓▓▓▓▓▓▒▒▒▒▒▒▒░░░░░▓▓▓▒▒░           11h 25m
  Feb 09–15  ▓▓▓▓▓▓▓▓▒▒▒▒▒▒▒▒▒░░░░▓▓▒▒░                 9h 50m
  Feb 02–08  ▓▓▓▓▓▓▓▓▓▓▒▒▒▒▒▒░░░░░░▓▓▓▒▒░░             11h 00m
  Jan 26–01  ▓▓▓▓▓▓▓▒▒▒▒▒▒░░░░▓▓▒░                       8h 05m
  Jan 19–25  ▓▓▓▓▓▓▓▓▓▒▒▒▒▒▒▒▒░░░░░▓▓▓▒▒░              10h 45m
  Jan 12–18  ▓▓▓▓▓▓▓▓▒▒▒▒▒▒░░░▓▓▒░                       8h 35m

  ■ Credo  ■ OU Psychology  ■ DELE Prep  ■ Kairos
  ■ Qigong  ■ Calisthenics  ■ Running  ■ Kettlebell
```

**Rendering rules:**
- Bar width scales to terminal width (minus label and total columns). Query terminal width from `SharedState.Width`.
- Each segment uses the category color via lipgloss. Block characters: `█` for segments (colored), `░` for empty remainder to scale max.
- Segments within a bar are ordered by minutes descending.
- Legend rendered below the chart, split into "Projects" and "Training" groups.
- If terminal width < 60, fall back to a compact table format (no bars).

### Entity Group: `workout`

Follows the existing entity group pattern in `cmd_entity.go`:

```
workout log [args]        → log a workout
workout list [--weeks N]  → list recent
workout delete <id>       → delete with confirmation
```

### Integration with Dashboard

The dashboard view (`view_dashboard.go`) gains an optional summary line showing total training minutes for the current week, rendered below the project list.

---

## Color Palette

Colors follow the Gruvbox theme already used by Kairos:

| Category       | Hex       | Lipgloss        |
|----------------|-----------|-----------------|
| Credo          | `#d79921` | Yellow           |
| OU Psychology  | `#458588` | Blue             |
| DELE Prep      | `#b16286` | Purple           |
| Kairos         | `#98971a` | Green            |
| Qigong         | `#fb4934` | Red              |
| Calisthenics   | `#fe8019` | Orange           |
| Running        | `#fabd2f` | Bright Yellow    |
| Kettlebell     | `#d65d0e` | Dark Orange      |
| GMB Movement   | `#cc241d` | Dark Red         |
| Stretching     | `#689d6a` | Aqua             |
| Other          | `#928374` | Gray             |

Project colors are assigned dynamically from a fixed palette based on project name hash, so new projects get consistent colors without hardcoding. The table above shows likely assignments for the current project set.

---

## Data Flow

```
chart command
  → ChartService.WeeklyBreakdown(numWeeks)
    → SessionRepo.ListSessionMinutesByWeek(from, to)  // project minutes by week
    → WorkoutLogRepo.ListByDateRange(from, to)         // workout minutes by week
    → merge + sort → []WeeklyBreakdown
  → formatter.RenderChart(breakdown, termWidth)        // lipgloss bar rendering
  → cmdOutputMsg → display
```

```
workout log command
  → parse args or launch wizard
  → WorkoutService.LogWorkout(req)
    → validate category + minutes
    → WorkoutLogRepo.Create(...)
  → format confirmation → cmdOutputMsg
```

---

## Migration

Single migration adding the `workout_logs` table. Added to the existing migration sequence in `internal/db/migrations.go`.

---

## Testing Strategy

### Unit Tests

- `WorkoutCategory` enum: validation, prefix matching, case normalization.
- `ChartService`: mock both repos, verify merge logic with overlapping weeks, empty weeks, mixed project/workout data.
- Chart formatter: test bar width calculation at various terminal widths, segment ordering, compact fallback at narrow width.
- `WorkoutService`: validation (bad category, zero minutes, negative minutes).

### Integration Tests

- `SQLiteWorkoutLogRepo`: CRUD + date range queries using `testutil.NewTestDB()`.
- `ListSessionMinutesByWeek`: verify aggregation SQL with known session data.
- Full chart pipeline: create sessions + workout logs → call chart command → verify output contains expected categories and totals.

### TUI Tests

- `workout log` wizard: drive the huh form via `teatest.Driver`, verify the correct `WorkoutLog` is created.
- `chart` command: verify the command dispatches and produces output (content correctness tested at the formatter level).

---

## Implementation Order

1. **Domain types** — `WorkoutLog`, `WorkoutCategory` enum, `WeeklyBreakdown`, `CategorySegment` in `internal/domain/`.
2. **Migration** — `workout_logs` table in `internal/db/`.
3. **Repository** — `SQLiteWorkoutLogRepo` + `ListSessionMinutesByWeek` on `SessionRepo`.
4. **Services** — `WorkoutService`, `ChartService` in `internal/service/`.
5. **CLI wiring** — `workout` entity group in `cmd_entity.go`, `chart` command in `command_dispatch.go`.
6. **Wizard** — `wizardWorkoutLog()` huh form in `wizard.go`.
7. **Formatter** — `chart_fmt.go` in `internal/cli/formatter/` — lipgloss bar rendering.
8. **Dashboard integration** — training summary line.
9. **Tests** — unit, integration, TUI at each layer.

---

## Open Questions

1. **Week start day** — ISO weeks start Monday. Confirm this is the desired grouping.
2. **Backfill** — should there be an import path for historical workout data (e.g. from a CSV or manual bulk entry)?
3. **Chart as a view vs. command output** — the chart could be a persistent TUI view on the view stack (scrollable, interactive hover) rather than a one-shot command output. The one-shot approach is simpler and proposed here; a full view could follow in a later iteration.
4. **Project color assignment** — the dynamic hash approach means colors shift if projects are added. An alternative is storing color assignments in `user_profile` or a dedicated table. Worth doing if the palette proves unstable in practice.
