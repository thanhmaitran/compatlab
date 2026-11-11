# Component Architecture

| Layer | Responsibility | Package |
|---|---|---|
| Specification Engine | Decode JSON/YAML, validate, resolve internal `$ref`, normalize | `internal/specification` |
| Compatibility Engine | Semantic diff, rule registry, classification, scoring | `internal/compatibility/{analyzer,rules,classifier,scoring}` |
| Contract Engine | Contract parsing, verification, consumer impact | `internal/contracts` |
| Test Generation | Boundary, negative, authentication, status and schema cases | `internal/generation` |
| Validation | JSON-schema subset used by the mock and the runner | `internal/validation` |
| Mock Engine | Deterministic mock server with request validation | `internal/mocking` |
| Execution | Bounded request runner, differential execution | `internal/execution` |
| Replay | Recorder with masking, deterministic replay | `internal/replay` |
| Workflows | Stateful multi-step scenarios | `internal/workflows` |
| Policy | Release decision | `internal/policies` |
| Reporting | Text, JSON and HTML reports | `internal/reporting` |
| Security | Target allowlist and connection guard | `internal/security` |
| Analytics | Historical metrics (pure computation) | `internal/analytics` |
| Authentication | Password hashing, signed tokens | `internal/auth` |
| Persistence | Store interface, in-memory and PostgreSQL stores, migrations | `internal/persistence`, `database/` |
| Service | Use cases for the backend | `internal/service` |
| Backend | Gin REST API: handlers, middleware, routes, DTOs | `apps/api` |
| CLI | `compatlab` command | `apps/cli` |
| Dashboard | React + TypeScript | `apps/web` |

## How callers use the engine
- **Backend**: a handler validates the DTO and calls `internal/service`, which calls the engine packages and stores results through the `Store` interface.
- **CLI**: commands call the same engine packages directly. Exit codes: 0 success, 1 error, 2 invalid input, 3 rejected.
- **Dashboard**: calls the REST API only; it never reaches the database.

## Where tenant isolation is enforced
Every project belongs to an organization. Services authorize through `Store.GetProject(orgID, id)`; documents (analyses, contracts, suites, runs, policies, reports) are reached through their project, so another organization receives 404.
