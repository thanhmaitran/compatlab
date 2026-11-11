# Test Generation

`internal/generation` turns a normalized API into a reproducible `TestSuite`.

| Category | Cases generated |
|---|---|
| AUTHENTICATION | Request without credentials is rejected (401) on secured endpoints |
| STATUS | Documented success status is returned |
| SCHEMA_VALIDATION | Success response matches its schema |
| BOUNDARY | Numbers at, below and above min/max; string lengths at and beyond min/max |
| PARAMETER_VALIDATION | Required non-path parameters are enforced |
| NEGATIVE | Missing required body field, wrong type, value outside an enum |

Priority (lower first): contract 0, authentication 10, status 20, schema 30, boundary 40, parameter 45, negative 50, workflow 60. Endpoints listed in `priority_endpoints` (for example those touched by breaking changes) get a 10-point boost.

## Reproducibility and limits
- No randomness: equal specification and configuration give equal suites; the generator version, spec hash and seed are stored in the suite.
- `max_cases_per_endpoint` (default 50) keeps the highest-priority cases and prevents combinatorial explosion.
- String boundaries above 10,000 characters are skipped so a hostile specification cannot force large allocations.

## Scope and limitations
Only top-level body properties are covered; nested objects and arrays of objects are not expanded. Expected statuses use the documented 2xx and 400 (or 422). Generated cases describe requests; executing them against an allowed target (mock server first) is part of Phase 07, and the target allowlist from ADR-0004 applies.
