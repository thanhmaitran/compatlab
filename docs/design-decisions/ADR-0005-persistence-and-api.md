# ADR-0005: Persistence model and REST API design

- Status: Accepted
- Date: 2026-11-08

## Decision
- **Store interface.** The backend depends on `persistence.Store`. An in-memory implementation serves development and tests; a PostgreSQL implementation (pgx, parameterized queries only) serves production. Both run the same contract test (`storeContract`).
- **Tables.** Identity and structure use typed tables (`organizations`, `users`, `api_projects`, `api_specifications`, `api_versions`, `audit_events`, `change_records`). Results that are only ever read back whole (analyses, contracts, suites, runs, policies, reports) are stored as JSONB documents in one table per kind with `project_id` and `created_at` columns. Change records are also stored as rows to support SQL analytics.
- **Migrations.** SQL files follow the golang-migrate naming convention. The server applies them at start-up with a small built-in runner (`compatlab_migrations` table), so the migrate CLI remains usable.
- **Tenancy.** Organization is the tenant boundary; foreign resources return 404, not 403.
- **Auth.** bcrypt password hashes; HS256 tokens built with the standard library (algorithm pinned, expiry checked). Roles: `admin` (creates policies and users) and `member`.
- **Limits.** Request-size limit, per-IP rate limits (stricter on register/login), `X-Forwarded-For` trusted only from configured proxies.
- **Test runs.** Run on an in-process mock unless the operator configures `ALLOWED_TARGET_HOSTS`; clients can never extend the allowlist.

## Deviations from the first design
No separate `consumers`, `test_cases`, `mock_scenarios` or `workflows` tables: consumers are part of contract documents, cases live inside suite documents, and mock/workflow features are CLI-only for now.

## Consequences
Fewer tables and simpler code at the cost of limited SQL queries over documents. Moving a document kind to typed tables later only requires a new migration and a Store change.
