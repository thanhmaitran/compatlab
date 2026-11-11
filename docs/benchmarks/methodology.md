# Benchmark Methodology

- **What is measured:** parse latency, diff latency, full analysis (diff + classification + scoring), classification alone, test generation, test execution on the in-process mock, report rendering (text and JSON). Memory is reported as allocated bytes and allocations per operation (`-benchmem`).
- **Datasets:** synthetic specifications from `benchmarks/dataset.go` with 10, 100 and 500 resources (3 operations each, enums, numeric constraints, referenced nested schemas). Variant 1 contains type changes, removed and added fields and removed operations so that diffs are non-trivial. Generation is deterministic.
- **How to run:** `./scripts/run-benchmarks.ps1` (default 5 repetitions). It records CPU, RAM, OS, Go version, the exact command and the raw output in `docs/benchmarks/results-<timestamp>.txt`, and writes `accuracy-latest.md`.
- **Reporting rules:** numbers are quoted only from a saved run together with its environment; compare medians of repeated runs; do not compare across machines.
- **Scalability:** compare results for 10, 100 and 500 resources; growth should be close to linear for parsing and diffing. Treat any other claim as unverified until measured.
- **Concurrency/reliability:** `go test ./tests/integration -race` runs the engine, mock handler and in-memory store from many goroutines; the analysis determinism test repeats runs and compares byte-identical output.
