# ADR-0002: Decode with standard libraries, extract with our own model

- Status: Accepted (revised)
- Date: 2026-11-05

## Context
The original plan was to use libopenapi. Its API could not be verified in the offline environment where this code was written, and the engine only needs a small, well-defined subset of OpenAPI 3.x.

## Decision
Documents are decoded with `encoding/json` and `gopkg.in/yaml.v3` into a generic tree. CompatLab then resolves internal `$ref`s and extracts the normalized model itself. The parser is hidden behind `domain.SpecificationParser`, so a library such as libopenapi can replace it without touching the analysis engine. External `$ref`s are rejected by policy.

## Consequences
- Small dependency surface (one YAML library) and full control over limits (size, nesting depth, reference chains).
- Supported subset: paths, operations, parameters, request bodies, responses, security, schemas with `$ref`, enum, required, nullable, format, numeric/string bounds, `oneOf`/`anyOf`/`allOf` (compared structurally).
- Known limits: one content type per body/response (application/json preferred); `allOf` is not flattened; security AND/OR structure is flattened to scheme names; no external references; no callbacks or webhooks.
