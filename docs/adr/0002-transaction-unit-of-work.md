# ADR 0002: Transaction Boundary via Unit of Work

- Status: Accepted
- Date: 2026-02-15

## Context

Multi-entity write flows (session logging, template init, project import) can partially persist data if an intermediate step fails.

## Decision

Use a `UnitOfWork` abstraction (`internal/db`) with `WithinTx(ctx, fn)` as the transaction boundary for all multi-write use cases.

Implementation details:

1. SQLite adapter: `SQLiteUnitOfWork`.
2. Repository methods accept `db.DBTX` so they can run on `*sql.DB` or `*sql.Tx`.
3. Use-case services create tx-scoped repositories inside `WithinTx`.
4. Any error returned from the transaction function aborts and rolls back.

## Consequences

1. No partial writes for transactional use cases.
2. Transaction orchestration stays in use-case services, not CLI or repository callers.
3. Integration tests can validate rollback semantics directly at the service boundary.

