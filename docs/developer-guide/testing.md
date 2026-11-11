# Developer Guide: Testing

See the [test strategy](../testing/test-strategy.md). Practical notes:
- Fixtures live in `testdata/` (specifications), `tests/fixtures/` (contracts) and `testdata/accuracy/` (labelled pairs).
- Tests that need HTTP use local `httptest` servers; the runner needs the host on its allowlist (`security.TargetPolicy`).
- Keep tests deterministic: no wall-clock dependence except explicit timeouts, no network access.
- When a generated suite fails against its own mock, fix the generator, the mock or the runner so that they agree; `TestGeneratedSuitesPassAgainstTheirOwnMock` is the guard.
