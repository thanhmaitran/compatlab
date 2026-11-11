# Contributing

1. Fork and create a feature branch.
2. Keep business logic in `internal/`; handlers in `apps/api` stay thin.
3. Every module needs unit tests. No placeholder implementations, hardcoded results or unmeasured benchmark numbers.
4. Run `./scripts/dev.ps1 check` (and `npm test` in `apps/web`) before opening a pull request.
5. Use Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
6. Record significant architectural decisions as ADRs in `docs/design-decisions/`.

See `docs/developer-guide/` for details.
