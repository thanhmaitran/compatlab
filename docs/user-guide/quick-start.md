# Quick Start

1. Compare two API versions from the command line:
   ```powershell
   compatlab analyze --base testdata/compatible/users-v1.yaml --target testdata/breaking/users-v2-breaking.yaml
   ```
2. Check consumer contracts and the release policy:
   ```powershell
   compatlab gate --base testdata/compatible/users-v1.yaml --target testdata/breaking/users-v2-breaking.yaml `
     --policy configs/release-policy.yaml --contract tests/fixtures/contracts/mobile-app.json
   echo $LASTEXITCODE    # 3 = release rejected
   ```
3. Generate and run tests against a mock: `compatlab test run --spec testdata/compatible/users-v1.yaml`.
4. Use the dashboard: register, create a project, upload two versions, run an analysis, register contracts, create a policy (admin) and evaluate the release.
