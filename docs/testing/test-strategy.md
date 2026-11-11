# Test Strategy

| Level | Where | Focus |
|---|---|---|
| Unit | `internal/*/*_test.go` | Domain validation, parser, each analyzer facet, rules, scoring, policies, reports, validation, security policy, auth, analytics |
| Matrix | `internal/compatibility/rules/rules_test.go` | Classification matrix |
| Component | `internal/mocking`, `internal/execution`, `internal/replay`, `internal/workflows` | Mock, runner, replay, workflows against local HTTP servers |
| Consistency | `internal/execution/execution_test.go` | A suite generated from a spec must fully pass against a mock of the same spec |
| Store contract | `internal/persistence` | Same behaviour for memory and PostgreSQL (`COMPATLAB_TEST_DATABASE_URL`) |
| API integration | `apps/api/routes/routes_test.go` | Workflow, authentication, tenancy, roles, limits |
| Integration | `tests/integration` | Pipeline, backend equals engine, concurrency |
| End to end | `tests/e2e` (API over HTTP), `apps/web/e2e` (browser, Playwright) | Complete user workflow |
| Security | `tests/security` | Hostile documents, SSRF variants, injection, token forgery, error leakage, parameterized SQL |
| Accuracy | `benchmarks/accuracy_test.go` | Precision/recall/F1 against `testdata/accuracy/manifest.json` |
| Performance | `benchmarks/*_benchmark_test.go` | Parse, diff, classify, generate, execute, report |
| Frontend unit | `apps/web/src/**/*.test.ts(x)` | API client, hooks, formatting |

Properties tested explicitly: deterministic output (repeated runs, reordered sources), circular schemas, external `$ref` rejection, size and depth limits, alias bombs, concurrent use of the engine, mock and store, masking of secrets in recordings.

Commands:
```powershell
./scripts/dev.ps1 check                     # gofmt, vet, Go tests
go test ./... -race -count=1                # Linux/macOS or Windows with gcc
cd apps/web; npm test; npm run test:e2e
COMPATLAB_TEST_DATABASE_URL=postgres://... go test ./internal/persistence
```
No coverage or benchmark figure is recorded in the repository until it is produced by a real run.
