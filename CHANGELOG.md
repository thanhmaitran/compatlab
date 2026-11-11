# Changelog

All notable changes are documented here. Format: [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- Phase 00-01: repository foundation, `internal/domain`, architecture docs, ADR-0001 to ADR-0004.
- Phase 02: OpenAPI 3.x specification engine (JSON/YAML, internal `$ref`, circular schemas, limits).
- Phase 03-04: semantic diff engine, 20-rule registry, backward/forward classification, scoring.
- Phase 05: consumer contracts, verification and consumer impact analysis.
- Phase 06: deterministic, bounded test generation.
- Phase 07: mock server, replay with masking, stateful workflows, allowlisted bounded execution, differential execution.
- Phase 08: Gin REST API, PostgreSQL and in-memory stores, migrations, authentication and authorization, rate and size limits, audit trail.
- Phase 09: React + TypeScript dashboard with Vitest and Playwright tests.
- Phase 10: `compatlab` CLI (`spec`, `diff`, `analyze`, `contract verify`, `test generate|run`, `mock`, `workflow run`, `gate`) and CI workflows.
- Phase 11: historical analytics (API and dashboard).
- Phase 12: benchmarks, labelled accuracy dataset and harness, security/integration/E2E/concurrency tests.
- Phase 13: Docker images and compose file, release workflow, documentation, technical report skeleton.
- OpenAPI description of the REST API (`api/openapi/compatlab.yaml`) with a test that keeps it in sync with the registered routes; conditional-change fixtures; sample workflow file; compatibility-check workflow that analyzes the API's own description.
