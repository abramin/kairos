# ADR 0001: Enforce Layer Boundaries

- Status: Accepted
- Date: 2026-02-15

## Context

The original service layer mixed orchestration, policy, and adapter-facing contracts. This created coupling across `internal/cli`, `internal/service`, and persistence concerns.

## Decision

Kairos enforces four explicit layers:

1. Adapter layer (`internal/cli`): parses input, formats output, and maps transport concerns.
2. Application/use-case layer (`internal/app`, `internal/service`): orchestrates repositories and domain behavior.
3. Domain layer (`internal/domain` + pure scheduler policy): owns business invariants and deterministic decision rules.
4. Infrastructure layer (`internal/repository`, `internal/db`): persistence and transaction implementations.

Additional rules:

1. Application interfaces use app/domain types, not adapter DTOs.
2. CLI does not call repositories directly.
3. Scheduler remains pure (no DB calls).
4. Domain invariants are enforced in domain methods.

## Consequences

1. Service constructors carry only use-case dependencies.
2. CLI and service mapping remains explicit and testable.
3. Business rules become reusable across shell commands and future adapters.

