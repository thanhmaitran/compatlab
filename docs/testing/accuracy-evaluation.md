# Accuracy Evaluation

## Method
`testdata/accuracy/manifest.json` contains pairs of small specifications with hand-written labels: expected change type, location (and constraint name where relevant) and expected **backward** classification. `TestAccuracy` runs the engine on every pair and computes, per change type:

- Precision = TP / (TP + FP)
- Recall = TP / (TP + FN)
- F1 = 2 * Precision * Recall / (Precision + Recall)

A change is a true positive when the engine reports the same (method, path, change type, location, constraint). Classification accuracy is measured on the detected changes. Every mismatch fails the test, so the dataset also works as a regression suite.

## Running
```powershell
go test ./benchmarks -run TestAccuracy -v
$env:COMPATLAB_WRITE_RESULTS = "1"; go test ./benchmarks -run TestAccuracy -count=1   # writes docs/benchmarks/accuracy-latest.md
```

## Limitations
- The dataset is small (see the manifest) and its labels were written together with the fixtures; independent review or a larger labelled corpus is needed before the figures are cited as evidence of general accuracy.
- Real-world OpenAPI documents (large, vendor extensions, `allOf` compositions) are not represented.
- Results measure agreement with the labelling, not with what any specific consumer would experience.
