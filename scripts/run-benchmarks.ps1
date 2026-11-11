# Runs the benchmarks and the accuracy evaluation and stores everything needed
# to reproduce the numbers: hardware, OS, Go version and the raw output.
# Usage: ./scripts/run-benchmarks.ps1 [-Count 5]
param([int]$Count = 5)

$stamp = Get-Date -Format "yyyyMMdd-HHmm"
$out = "docs/benchmarks/results-$stamp.txt"
$cpu = (Get-CimInstance Win32_Processor | Select-Object -First 1)
$ramGb = [math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB, 1)
$os = (Get-CimInstance Win32_OperatingSystem).Caption

@(
  "CompatLab benchmark run $stamp",
  "CPU: $($cpu.Name) ($($cpu.NumberOfCores) cores, $($cpu.NumberOfLogicalProcessors) threads)",
  "RAM: $ramGb GB",
  "OS: $os",
  "Go: $(go version)",
  "Command: go test ./benchmarks -run '^$' -bench . -benchmem -count $Count",
  "Datasets: synthetic, see benchmarks/dataset.go (resources = 10, 100, 500; 3 operations each)",
  ""
) | Set-Content -Path $out

go test ./benchmarks -run '^$' -bench . -benchmem -count $Count | Tee-Object -FilePath $out -Append

$env:COMPATLAB_WRITE_RESULTS = "1"
go test ./benchmarks -run TestAccuracy -count 1 -v | Tee-Object -FilePath $out -Append
Remove-Item Env:COMPATLAB_WRITE_RESULTS
Write-Host "`nSaved to $out and docs/benchmarks/accuracy-latest.md"
