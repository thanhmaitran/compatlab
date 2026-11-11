# Development tasks for Windows (no `make` required).
# Usage: ./scripts/dev.ps1 [fmt|vet|test|check]
param([ValidateSet("fmt", "vet", "test", "check")][string]$Task = "check")

switch ($Task) {
  "fmt"  { gofmt -w . }
  "vet"  { go vet ./... }
  "test" { go test ./... -count=1 }
  "check" {
    $unformatted = gofmt -l .
    if ($unformatted) { Write-Host "Unformatted files:" -ForegroundColor Yellow; $unformatted; exit 1 }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go test ./... -count=1
    exit $LASTEXITCODE
  }
}
