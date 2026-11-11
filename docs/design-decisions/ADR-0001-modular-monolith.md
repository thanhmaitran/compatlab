# ADR-0001: Modular monolith with an independent core engine

- Status: Accepted
- Date: 2026-11-04

## Context
CompatLab combines parsing, compatibility analysis, contract testing, mocking and policy evaluation. Microservices would add operational complexity before module boundaries are proven.

## Decision
One Go module at the repository root. Core logic lives in `internal/` packages behind interfaces defined in `internal/domain`. `apps/api` and `apps/cli` are thin delivery layers over the same engine.

## Consequences
Simple local development and deployment. Modules can be extracted later because they communicate through interfaces. Requires discipline: no compatibility logic in HTTP handlers.
