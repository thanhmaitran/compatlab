# CLI Reference

Build and run from the repository root:

```powershell
go build -o bin/compatlab.exe ./apps/cli/cmd/compatlab
./bin/compatlab.exe help
```

| Command | Purpose |
|---|---|
| `compatlab spec validate <file>` | Parse and validate an OpenAPI 3.x file |
| `compatlab spec inspect <file>` | List endpoints and schemas |
| `compatlab diff --base A --target B [--format text\|json]` | Raw changes without classification |
| `compatlab analyze --base A --target B [--format text\|json\|html] [--output file]` | Classified changes, scores |
| `compatlab contract verify --spec API --contract C [--contract C2]` | Verify consumer contracts |
| `compatlab gate --base A --target B --policy P [--contract C ...]` | Full release check |
| `compatlab test generate --spec API [--max-per-endpoint N] [--seed N] [--priority KEY]` | Generate a reproducible test suite (JSON) |
| `compatlab test run --spec API [--suite F] [--target URL --allow-host H]` | Run tests on an in-process mock, or on an allowlisted target |
| `compatlab mock --spec API [--addr 127.0.0.1:8081]` | Serve a mock of the API (loopback only) |
| `compatlab workflow run --file W.json --target URL --allow-host H` | Run a stateful workflow |
| `compatlab version` | Print the version |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success, gate passed (or passed with warnings) |
| 1 | Analysis or execution error |
| 2 | Invalid input or configuration (bad spec, missing file, bad flags) |
| 3 | Release rejected by policy, or a contract failed verification |

## Example

```powershell
./bin/compatlab.exe gate `
  --base testdata/compatible/users-v1.yaml `
  --target testdata/breaking/users-v2-breaking.yaml `
  --policy configs/release-policy.yaml `
  --contract tests/fixtures/contracts/mobile-app.json `
  --contract tests/fixtures/contracts/analytics.json
echo $LASTEXITCODE
```
Run it yourself and save the output as evidence in `docs/images/local_run/`.

## Workflow file format
```json
{
  "name": "order lifecycle",
  "on_failure": "stop",
  "steps": [
    {"name": "create", "method": "POST", "path": "/orders", "body": {"sku": "abc"},
     "extract": {"order_id": "id"},
     "assertions": [{"target": "status", "operator": "eq", "expected": "201"}],
     "timeout_ms": 2000, "retry": {"max_attempts": 2, "backoff_ms": 100}},
    {"name": "get", "method": "GET", "path": "/orders/{{order_id}}",
     "assertions": [{"target": "body.sku", "operator": "eq", "expected": "abc"}]}
  ],
  "cleanup": [{"name": "remove", "method": "DELETE", "path": "/orders/{{order_id}}"}]
}
```
Assertion targets: `status`, `header.<name>`, `body.<json.path>`. Operators: `eq`, `ne`, `contains`, `exists`, `gt`, `lt`. Targets must be allowed with `--allow-host`; nothing is allowed by default.
