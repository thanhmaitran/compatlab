# Developer Guide: Architecture

Read [system overview](../architecture/system-overview.md) and [module dependencies](../architecture/module-dependencies.md). Rules: `internal/domain` imports only the standard library (enforced by a test); handlers stay thin; engine packages never import `apps/*` or `persistence`; only services import the store.

## Adding a compatibility rule
1. If a new kind of difference is needed, add a `ChangeType` (domain), detect it in `internal/compatibility/analyzer` with machine-readable `Attributes`.
2. Add a rule in `internal/compatibility/rules/*_rules.go` returning one backward and one forward verdict, with explanation and remediation.
3. Add rows to `TestBackwardClassificationMatrix`, the change type to `TestEveryChangeTypeHasARule`, and a labelled case to `testdata/accuracy/manifest.json`.
4. Document it in `docs/specifications/compatibility-model.md`.

## Adding an endpoint
DTO in `apps/api/dto`, service method in `internal/service`, thin handler, route, and a test in `apps/api/routes/routes_test.go` covering authorization and tenancy.
