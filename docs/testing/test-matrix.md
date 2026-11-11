# Compatibility Test Matrix

Backward-direction expectations asserted by `TestBackwardClassificationMatrix` and the labelled dataset.

| Scenario | Rule | Expected |
|---|---|---|
| Remove endpoint | EP-001 | BREAKING |
| Add endpoint | EP-002 | NON_BREAKING |
| Remove HTTP method | EP-003 | BREAKING |
| Add required request field | REQ-001 | BREAKING |
| Add optional request field | REQ-001 | NON_BREAKING |
| Remove request field | REQ-002 | CONDITIONAL |
| Add required parameter | PAR-002 | BREAKING |
| Optional to required parameter | PAR-003 | BREAKING |
| Change request type (integer to number) | REQ-004 | NON_BREAKING |
| Remove values from a request enum | REQ-005 | BREAKING |
| Tighten a request constraint | REQ-006 | BREAKING |
| Remove response field | RES-001 | BREAKING |
| Add response field | RES-002 | CONDITIONAL |
| Response field becomes optional | RES-003 | BREAKING |
| Change response type | RES-004 | BREAKING |
| Add values to a response enum | RES-005 | CONDITIONAL |
| Response becomes nullable | RES-006 | BREAKING |
| Remove a 2xx status | RES-007 | BREAKING |
| Remove a non-2xx status | RES-007 | CONDITIONAL |
| Authentication added | SEC-001 | BREAKING |
| Additional scopes or schemes | SEC-001 | CONDITIONAL |
| Rename a path parameter | (no change) | none |
