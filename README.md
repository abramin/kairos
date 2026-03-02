# Kairos

Kairos is a terminal-first project planner focused on daily execution loops.

It tracks project structure, recommends what to do next (`what-now`), shows risk/progress (`status`), logs sessions, and replans deterministically (`replan`).

## Install

### Prerequisites

- Go 1.25+
- macOS/Linux terminal

### Build and install

```bash
make install
```

This installs `kairos` into your Go bin path.

## Runtime configuration

Kairos reads:

- `KAIROS_DB`: SQLite path
- `KAIROS_TEMPLATES`: templates directory
- `KAIROS_LLM_ENABLED`: enables `ask`/LLM explain/help/draft features (`true`/`false`, default `false`)

Defaults:

- DB: `~/.kairos/kairos.db`
- Templates: `./templates` if present, otherwise `~/.kairos/templates`

Recommended local dev setup:

```bash
export KAIROS_DB="$PWD/.kairos/kairos.db"
export KAIROS_TEMPLATES="$PWD/templates"
```

## TUI/Shell Mode (default)

Run:

```bash
kairos
```

With no args, Kairos launches a full-screen TUI (dashboard + command bar).

Core keys:

- `:` focus command bar
- `esc` back (or blur command bar)
- `q` or `Ctrl+C` quit
- `?` open recommendations view

Dashboard keys:

- `enter` open selected project task tree
- `p` project list view
- `d` draft new project
- `h` help chat view
- `r` refresh

Command bar behavior:

- Prompt shows active context: `kairos (PHI01) ❯`
- History: up/down arrows (persisted at `~/.kairos/shell_history`)
- Suggestions/autocomplete while typing

## Typical shell flow

```text
: projects
: use PHI01
: inspect
: what-now 45
: log
: status
: replan
```

## Shell command behavior

- Active context:
  - `use <id>` sets active project (`short id`, UUID, or UUID prefix)
  - `use` clears active project context
  - `inspect` uses active project when no ID is passed
  - `status` scopes to active project when set
- Shell-native quick commands:
  - `projects`, `use`, `inspect`, `status`, `what-now`, `replan`
  - `add`, `log`, `start`, `finish`, `context`, `draft`
  - `ask`, `explain`, `review`, `help`, `help chat`
- Pass-through command groups:
  - `project *`, `node *`, `work *`, `session *`, `template *`
  - For `node/work/session` commands, active project is auto-applied as `--project` when possible
- Guided flows:
  - Bare `session log`, `work add`, and `node add` open interactive forms
  - `log` also prompts for missing project/item/duration
- Safety:
  - `project archive/remove`, `node remove`, `work archive/remove`, `session remove` ask for confirmation in shell
  - `--yes`/`-y`/`--force` bypasses shell confirmation

## Create a project

### Option 1: Template init

```bash
kairos project init \
  --id PHI01 \
  --template course_weekly_generic \
  --name "Philosophy 101" \
  --start 2026-02-07 \
  --due 2026-05-30 \
  --var weeks=12 \
  --var assignment_count=4
```

### Option 2: Import a plan JSON

```bash
kairos project import docs/project-sample.json
```

### Option 3: Interactive draft (from TUI or CLI)

- In TUI: press `d` or run `: draft`
- CLI: `kairos project draft`

## One-shot mode (scripting)

Pass a command as an argument to skip the TUI and print to stdout:

```bash
kairos what-now 60      # recommendations for 60 min (also: 1h30m)
kairos status           # project progress overview
kairos chart            # weekly time breakdown chart
kairos chart --weeks 4  # last 4 weeks
kairos projects         # list active projects
kairos help             # command reference
```

Useful in cron jobs, shell scripts, or piping into other tools. Set `COLUMNS` to control chart width.

## Global task checklist

Kairos has a lightweight global to-do list independent of any project. Tasks are ordered and persist across sessions.

In the shell:

```text
: tasks             (open checklist view)
```

Checklist view keys:

- `a` add a task
- `enter` mark done / confirm edit
- `e` edit selected task
- `d` delete selected task
- `K` / `J` move task up / down
- `esc` close

Tasks are separate from project work items — use them for personal to-dos, daily intentions, or anything that doesn't belong to a specific plan.

## Documentation map

- `docs/prd.md`: behavior and planning rules
- `docs/contracts.md`: `what-now`, `status`, `replan` contracts
- `docs/shell-first-prd.md`: shell-first product direction
- `docs/orchestrator.md`: implementation workflow and quality gates

## Why "Kairos"?

In Greek, *kairos* refers to the right or opportune moment.
