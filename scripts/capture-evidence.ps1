# Runs the main commands and stores their real output as text files in
# docs/images/local_run/ so that screenshots can be taken from genuine runs.
# Usage: ./scripts/capture-evidence.ps1
$dir = "docs/images/local_run"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
go build -o bin/compatlab.exe ./apps/cli/cmd/compatlab

./scripts/check-env.ps1 *> "$dir/environment-check.txt"
./bin/compatlab.exe spec validate testdata/valid/payment-api.yaml *> "$dir/parser-validation.txt"
./bin/compatlab.exe diff --base testdata/compatible/users-v1.yaml --target testdata/breaking/users-v2-breaking.yaml *> "$dir/semantic-diff.txt"
./bin/compatlab.exe analyze --base testdata/compatible/users-v1.yaml --target testdata/breaking/users-v2-breaking.yaml *> "$dir/compatibility-analysis.txt"
./bin/compatlab.exe contract verify --spec testdata/breaking/users-v2-breaking.yaml --contract tests/fixtures/contracts/mobile-app.json --contract tests/fixtures/contracts/analytics.json *> "$dir/contract-verification.txt"
./bin/compatlab.exe test generate --spec testdata/compatible/users-v1.yaml --max-per-endpoint 5 *> "$dir/generated-tests.txt"
./bin/compatlab.exe test run --spec testdata/compatible/users-v1.yaml *> "$dir/test-execution.txt"
./bin/compatlab.exe gate --base testdata/compatible/users-v1.yaml --target testdata/breaking/users-v2-breaking.yaml --policy configs/release-policy.yaml --contract tests/fixtures/contracts/mobile-app.json *> "$dir/ci-quality-gate.txt"
"exit code of the gate: $LASTEXITCODE" | Add-Content "$dir/ci-quality-gate.txt"
Write-Host "Saved transcripts to $dir. Open them in a terminal or editor and take screenshots (see docs/images/README.md)."
