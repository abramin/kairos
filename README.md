# Kairos

Kairos is a personal command-line planner built for shell-first use.

Start Kairos in a terminal, set an active project once, then run fast planning and logging commands without repeating flags.

## What Kairos does

- Tracks projects, plan nodes, and work items
- Recommends what to work on for the time you have (`what-now`)
- Shows risk/progress across projects (`status`)
- Logs sessions and updates progress over time
- Replans when you ask (`replan`)

## Install

### Prerequisites

- Go 1.25+
- macOS/Linux terminal

### Build and install

```bash
make install
```

This installs `kairos` into your Go bin path.

## Runtime paths

Kairos reads these environment variables:

- `KAIROS_DB`: SQLite file path
- `KAIROS_TEMPLATES`: templates directory

If not set:

- DB defaults to `~/.kairos/kairos.db`
- Templates default to `./templates` if present, otherwise `~/.kairos/templates`

Recommended local-dev setup:

```bash
export KAIROS_DB="$PWD/.kairos/kairos.db"
export KAIROS_TEMPLATES="$PWD/templates"
```

## Shell-first quickstart

Run:

```bash
kairos
```

With no arguments, Kairos opens the interactive shell.

Typical first session:

```text
kairos ❯ projects
kairos ❯ use PHI01
kairos (PHI01) ❯ inspect
kairos (PHI01) ❯ what-now 45
kairos (PHI01) ❯ log
kairos (PHI01) ❯ status
kairos (PHI01) ❯ replan
```

## Create a project

### Option 1: Template init (recommended)

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

### Option 2: Import a concrete JSON plan

```bash
kairos project import docs/project-sample.json
```

Then enter shell and continue there:

```bash
kairos
```

## Shell behavior (current)

- Active project context:
  - `use <id>` sets the active project
  - `use` (no args) clears active project context
  - `inspect`/`what-now`/`status` scope to active project when set
- Quick shell-native commands:
  - `projects`, `use`, `inspect`, `status`, `what-now`, `replan`
  - `add`, `log`, `start`, `finish`, `context`, `draft`
- Wizard behavior:
  - Bare `session log`, `work add`, and `node add` trigger guided prompts when flags are omitted
  - `log` also uses context + prompts for missing project/item/duration
- Safety behavior:
  - Destructive commands (`project remove/archive`, `node remove`, `work remove/archive`, `session remove`) ask for confirmation in shell
  - `--yes`/`-y` bypasses confirmation
- Shell quality-of-life:
  - Tab autocomplete and persisted history
  - `help` for command reference
  - `help chat [question]` for interactive help
  - `clear` clears screen, `exit`/`quit` leaves shell

## One-shot CLI (automation/scripts)

All standard commands still work non-interactively, for example:

```bash
kairos project list
kairos status --project PHI01 --recalc
kairos what-now --minutes 60 --project PHI01
kairos session log --work-item 5 --project PHI01 --minutes 45 --units-done 1
```

Use one-shot commands for scripts/CI; use shell mode for daily planning loops.

## Common commands

```bash
kairos project list
kairos project inspect PHI01
kairos node update 3 --project PHI01 --title "Week 4 - Ethics"
kairos work update 5 --project PHI01 --planned-min 75
kairos work done 5 --project PHI01
kairos session list --work-item 5 --project PHI01
kairos template list
```

## Starter templates

- `course_weekly_generic`
- `self_paced_work_items`
- `certification_exam_prep`
- `writing_project_stages`
- `interview_prep_track`
- `language_immersion_sprint`
- `portfolio_milestones`

## Documentation map

- `docs/prd.md`: behavior and planning rules
- `docs/contracts.md`: `what-now`, `status`, `replan` contracts
- `docs/orchestrator.md`: implementation workflow and quality gates
- `docs/shell-first-prd.md`: shell-first product direction

## Why "Kairos"?

In Greek, *kairos* refers to the right or opportune moment.
