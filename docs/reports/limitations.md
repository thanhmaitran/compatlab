# Limitations

## Specification support
- OpenAPI 3.x only (Swagger 2.0 is rejected). One content type per request body and response (`application/json` preferred). `allOf` is compared structurally but not flattened. Security requirements are flattened to scheme names (AND/OR structure is lost). External `$ref`, callbacks and webhooks are not supported by design or not modelled.

## Compatibility analysis
- Classification is rule-based and reflects common consumer behaviour. `CONDITIONAL` changes need contracts or human review. Consumers without registered contracts are unknown and are never reported as unaffected.
- The score is a CompatLab-defined indicator, not a standard.

## Contracts, tests, mock
- Contract verification checks endpoint, status and body fields/types against the specification, not against a live provider.
- Generated tests cover top-level body fields only and assume the documented 2xx and 400/422 statuses. The mock validates a JSON-schema subset (types, required, enum, bounds, lengths, pattern, combinators); `format`, `uniqueItems` and `additionalProperties` are not validated.
- Replay sends masked placeholders instead of secrets, so it is meant for mocks and test environments.

## Backend and dashboard
- No e-mail verification, password reset or invitations; the role is carried in the token until it expires (12 hours). Rate limiting is per process. The in-memory store loses data on restart. The PostgreSQL store is covered by a contract test that needs a database (`COMPATLAB_TEST_DATABASE_URL`).
- The dashboard has no pagination yet and no pages for mock/replay or workflows (CLI only). The session token lives in `sessionStorage`; the shipped nginx configuration sets a strict Content-Security-Policy.

## Evaluation
- The labelled dataset is small and was labelled together with the fixtures. Benchmarks use synthetic specifications. No result is recorded until it is produced by a real run.

## Development status
- The code base was written without access to a Go or Node toolchain. Treat the first `go test ./...`, `npm test` and `npm run build` on a developer machine as the verification step and fix any reported issue.
