# Verifies the local toolchain required by CompatLab.
$tools = @("go", "node", "npm", "git", "docker")
$missing = $false
foreach ($tool in $tools) {
  Write-Host "`n========== $tool ==========" -ForegroundColor Cyan
  $command = Get-Command $tool -ErrorAction SilentlyContinue
  if ($command) {
    Write-Host "Installed: $($command.Source)" -ForegroundColor Green
    if ($tool -eq "go") { & $tool version } else { & $tool --version }
  } else {
    Write-Host "Not installed or not in PATH" -ForegroundColor Yellow
    $missing = $true
  }
}
Write-Host "`n========== docker compose ==========" -ForegroundColor Cyan
docker compose version
if ($missing) { exit 1 }
