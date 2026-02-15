# ADR 0003: Atomic Project Sequence Allocation

- Status: Accepted
- Date: 2026-02-15

## Context

Allocating `seq` values via `MAX(seq)+1` is race-prone under concurrent creates and can produce duplicate sequence numbers.

## Decision

Adopt project-scoped atomic allocation backed by `project_sequences(project_id, next_seq)`.

Implementation details:

1. Allocate inside the same transaction as node/work-item creation.
2. Increment `next_seq` atomically per project.
3. Expose allocation via `ProjectSequenceRepo`.
4. Backfill and maintain `project_sequences` via migrations.

## Consequences

1. Concurrent writers receive unique monotonic sequence numbers per project.
2. Sequence allocation logic is centralized and testable.
3. Legacy `MAX(seq)+1` behavior is removed from write paths.

