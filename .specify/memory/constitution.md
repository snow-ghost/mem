<!--
  Sync Impact Report
  ==================
  Version change: (new) → 1.0.0
  Modified principles: N/A (initial ratification)
  Added sections:
    - Core Principles (4): Code Quality, Testing Standards,
      API Consistency, Performance Requirements
    - Architectural Constraints
    - Development Workflow
    - Governance
  Removed sections: N/A
  Templates requiring updates:
    - .specify/templates/plan-template.md — ✅ no update needed
      (Constitution Check section is generic; gates derived at plan time)
    - .specify/templates/spec-template.md — ✅ no update needed
      (Success Criteria already covers measurable performance outcomes)
    - .specify/templates/tasks-template.md — ✅ no update needed
      (Phase structure accommodates testing and polish phases)
    - .specify/templates/checklist-template.md — ✅ no update needed
      (Generic; checklist items generated per feature)
  Follow-up TODOs: None
-->

# Warehouse Goods API Constitution

## Core Principles

### I. Code Quality & Clean Architecture

Every module MUST respect the four-layer dependency rule:
`interfaces → application → domain ← infrastructure`.
No layer may import from one that depends on it.

- Domain layer MUST have zero external dependencies (stdlib only).
- Application layer MUST NOT import infrastructure or interface types.
- Infrastructure repositories MUST include a compile-time interface
  check: `var _ domainrepo.Repository = (*ConcreteRepo)(nil)`.
- Interface Segregation Principle (ISP) MUST be applied: handler
  files declare private interfaces scoped to exactly the use cases
  they consume; cross-domain consumers use `Finder` (read-only)
  interfaces, never full `Repository`.
- Entity fields MUST be unexported with getter methods. Construction
  MUST go through `NewXxx()` (validates invariants) or
  `ReconstructXxx()` (persistence hydration only).
- Functions and methods MUST have a single, clear responsibility.
  Use cases are one-struct-per-operation with an `Execute` method.
- Errors MUST be wrapped with context:
  `fmt.Errorf("operation: %w", err)`.
- Maximum line length: 150 characters. Code MUST pass
  `golangci-lint` with the project `.golangci.yml` configuration.

### II. Testing Standards

All production code MUST be covered by tests appropriate to its
layer and risk profile.

- **Unit tests** MUST run with `-race -shuffle=on` to detect data
  races and order-dependent failures.
- **Integration tests** MUST use testcontainers (PostgreSQL) and
  reside under `tests/integration/`. They MUST NOT depend on
  shared external databases or services.
- `sql.ErrNoRows` MUST be mapped to domain sentinel errors in
  repository implementations. Tests MUST verify this mapping.
- Mocks MUST be generated via Mockery (`.mockery.yaml` config).
  Hand-written mocks are prohibited to prevent drift.
- New use cases MUST have unit tests that cover: happy path, at
  least one validation/invariant failure, and repository error
  propagation.
- New HTTP handlers MUST have tests that verify: correct status
  codes, request validation rejection, and error response format.
- Tests MUST NOT rely on execution order. Each test MUST set up
  its own state and clean up after itself.

### III. API Consistency & User Experience

All HTTP endpoints MUST present a uniform, predictable interface
to consumers across both Client and External API audiences.

- Request parsing MUST use `api.ShouldBindJSON`,
  `api.ShouldBindQuery`, or `api.ShouldBindURI`. Direct
  `c.Bind` calls are prohibited.
- Responses MUST use `commonapi.RespondOK(c, status, data, meta)`.
  Domain entities MUST NOT leak into HTTP responses; all output
  goes through interface-layer DTOs.
- Error responses MUST be mapped through per-domain
  `serve{Domain}Error` functions to ensure consistent error
  structure.
- Pagination MUST use `api.Meta{Pages, Entries}` for metadata.
- Swagger documentation (`make docs`) MUST be regenerated whenever
  handler signatures, request DTOs, or response DTOs change.
  Client and External docs MUST both be updated.
- URI patterns MUST follow RESTful conventions: plural resource
  nouns, nested resources for parent-child relationships,
  consistent use of path parameters for identity.

### IV. Performance Requirements

The API MUST meet latency and resource budgets under expected
warehouse operational load.

- Single-entity read endpoints MUST respond within 100ms at p95
  under normal load.
- List/paginated endpoints MUST respond within 300ms at p95.
  `Limit=0` MUST be guarded against — only append `qm.Limit()`
  when `params.Limit > 0`.
- Write operations (create, update, delete) MUST respond within
  500ms at p95 including transaction commit.
- Database queries MUST use appropriate indexes. New migrations
  that add queries without corresponding indexes MUST be justified.
- Eager-loading (`qm.Load`) MUST be used deliberately. Always
  check `m.R != nil` before accessing eager-loaded relations to
  prevent nil-pointer panics.
- Connection pooling via pgBouncer MUST be used for all database
  access. Direct connections bypassing the pool are prohibited in
  production code.
- Transaction scope MUST be minimized. Use cases own transactions
  via `db.Transactor.RunInTransaction` and MUST NOT hold
  transactions open across external service calls.

## Architectural Constraints

- **Go version**: 1.26.0. Dependencies MUST NOT require a newer
  version.
- **Generated code**: `internal/db/postgres/model/` is generated
  by SQLBoiler. These files MUST NOT be edited manually. Run
  `make model` to regenerate.
- **Migrations**: Goose SQL format with `+goose` markers. New
  migrations MUST be created via `make migrate-create name=<name>`.
  Every migration MUST include both Up and Down directions.
- **Domain boundaries**: Each bounded context (`container`,
  `supply`, `storage`, etc.) MUST remain isolated. Cross-domain
  access MUST go through `Finder` interfaces or domain events,
  never direct repository imports.
- **Nullable fields**: Use `helper.Null.String()` /
  `helper.Null.Time()` for nullable fields in DB mappers. Raw
  `sql.NullString` in domain code is prohibited.
- **Observability**: All new services MUST emit structured logs
  via zerolog. Critical error paths MUST be instrumented for
  Sentry capture when `SENTRY_ENABLED` is set. Elastic APM
  spans MUST wrap external calls when `ELASTIC_APM_ACTIVE` is
  set.

## Development Workflow

- **Branching**: Feature branches from `main`. Merge via PR with
  at least one reviewer.
- **Pre-merge gates**: `make lint`, `make test`, and
  `make test-integration` MUST pass before merge. Swagger docs
  MUST be up to date (`make docs` produces no diff).
- **Code generation**: After schema changes, run `make model`
  then `make generate-mockery`. Generated files MUST be committed
  alongside the code that requires them.
- **Commit discipline**: Each commit SHOULD represent a single
  logical change. Migration commits MUST be separate from
  application code commits.
- **Dependency additions**: New Go module dependencies MUST be
  justified in the PR description. Prefer stdlib and existing
  dependencies over new ones.

## Governance

This constitution is the authoritative source for architectural
and quality decisions in the Warehouse Goods API project. It
supersedes informal practices and ad-hoc conventions.

- **Amendments** MUST be documented with a version bump, rationale,
  and migration plan for any affected code.
- **Versioning** follows semantic versioning:
  - MAJOR: Principle removal or backward-incompatible redefinition.
  - MINOR: New principle or materially expanded guidance.
  - PATCH: Clarifications, wording, and non-semantic refinements.
- **Compliance review**: PRs MUST be checked against these
  principles. Violations require explicit justification in the PR
  description with a reference to this constitution.
- **Complexity justification**: Any deviation from the dependency
  rule or introduction of new architectural patterns MUST be
  recorded in the plan's Complexity Tracking table.
- **Runtime guidance**: Refer to `CLAUDE.md` for development
  commands, project structure, and layer-specific implementation
  patterns.

**Version**: 1.0.0 | **Ratified**: 2026-03-20 | **Last Amended**: 2026-03-20
