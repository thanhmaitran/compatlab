# Evidence images

Only screenshots of real runs belong here. Nothing in this folder is a mock-up.

Run `./scripts/capture-evidence.ps1` to produce the command output, then take the screenshots:

| File | Source |
|---|---|
| `local_run/environment-check.png` | `./scripts/check-env.ps1` |
| `local_run/parser-validation.png` | `compatlab spec validate ...` |
| `local_run/semantic-diff.png` | `compatlab diff ...` |
| `local_run/compatibility-analysis.png` | `compatlab analyze ...` |
| `local_run/contract-verification.png` | `compatlab contract verify ...` |
| `local_run/generated-tests.png` | `compatlab test generate ...` |
| `local_run/test-execution.png` | `compatlab test run ...` |
| `local_run/cli-analysis.png` | `compatlab analyze --format json` |
| `local_run/ci-quality-gate.png` | `compatlab gate ...` and its exit code (3) |
| `local_run/benchmark-results.png` | `./scripts/run-benchmarks.ps1` |
| `dashboard/overview.png` ... `dashboard/analytics.png` | the running web dashboard (`docker compose up --build`, then the six pages) |

`architecture-layers.svg`, `workflow.svg` and `module-dependencies.svg` are diagrams generated for the documentation.
