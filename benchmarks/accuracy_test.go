package benchmarks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

type expectedChange struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	ChangeType     string `json:"change_type"`
	Location       string `json:"location"`
	Constraint     string `json:"constraint"`
	Classification string `json:"classification"`
}

type labelledCase struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Base        json.RawMessage  `json:"base"`
	Target      json.RawMessage  `json:"target"`
	Expected    []expectedChange `json:"expected"`
}

type manifest struct {
	Description string         `json:"description"`
	Cases       []labelledCase `json:"cases"`
}

func changeKey(method, path, changeType, location, constraint string) string {
	return strings.Join([]string{method, path, changeType, location, constraint}, "|")
}

type counts struct{ tp, fp, fn int }

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func (c counts) precision() float64 { return ratio(c.tp, c.tp+c.fp) }
func (c counts) recall() float64    { return ratio(c.tp, c.tp+c.fn) }
func (c counts) f1() float64 {
	p, r := c.precision(), c.recall()
	if p+r == 0 {
		return 0
	}
	return 2 * p * r / (p + r)
}

func loadManifest(t *testing.T) manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "accuracy", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestAccuracyDatasetIsWellFormed(t *testing.T) {
	m := loadManifest(t)
	if len(m.Cases) < 10 {
		t.Fatalf("expected a meaningful dataset, got %d cases", len(m.Cases))
	}
	names := map[string]bool{}
	for _, c := range m.Cases {
		if c.Name == "" || names[c.Name] {
			t.Errorf("case name %q is empty or duplicated", c.Name)
		}
		names[c.Name] = true
		for _, e := range c.Expected {
			if !domain.ChangeType(e.ChangeType).Valid() || !domain.ChangeClassification(e.Classification).Valid() {
				t.Errorf("%s: invalid label %+v", c.Name, e)
			}
		}
	}
}

// TestAccuracy compares the engine with the hand-labelled dataset. Detection is
// scored per change type with precision, recall and F1; classification accuracy
// is scored on the correctly detected changes (backward direction). Every
// mismatch fails the test, so the dataset doubles as a regression suite.
//
// Set COMPATLAB_WRITE_RESULTS=1 to write docs/benchmarks/accuracy-latest.md.
func TestAccuracy(t *testing.T) {
	m := loadManifest(t)
	perType := map[string]*counts{}
	bucket := func(ct string) *counts {
		if perType[ct] == nil {
			perType[ct] = &counts{}
		}
		return perType[ct]
	}
	var failures []string
	classified, classCorrect, labelled := 0, 0, 0

	for _, c := range m.Cases {
		p := specification.NewParser()
		base, err := p.Parse(c.Base)
		if err != nil {
			t.Fatalf("%s: base: %v", c.Name, err)
		}
		target, err := p.Parse(c.Target)
		if err != nil {
			t.Fatalf("%s: target: %v", c.Name, err)
		}
		analysis, err := compatibility.NewEngine().Compare(base, target)
		if err != nil {
			t.Fatal(err)
		}

		predicted := map[string]string{} // key -> backward classification
		predictedType := map[string]string{}
		for _, ch := range analysis.Changes {
			key := changeKey(ch.Method, ch.Path, string(ch.ChangeType), ch.Location, ch.Attributes["constraint"])
			predictedType[key] = string(ch.ChangeType)
			if ch.Direction == domain.DirectionBackward {
				predicted[key] = string(ch.Classification)
			}
		}
		expected := map[string]expectedChange{}
		for _, e := range c.Expected {
			expected[changeKey(e.Method, e.Path, e.ChangeType, e.Location, e.Constraint)] = e
			labelled++
		}
		for key, e := range expected {
			if _, found := predicted[key]; !found {
				bucket(e.ChangeType).fn++
				failures = append(failures, fmt.Sprintf("%s: missed %s", c.Name, key))
				continue
			}
			bucket(e.ChangeType).tp++
			classified++
			if predicted[key] == e.Classification {
				classCorrect++
			} else {
				failures = append(failures, fmt.Sprintf("%s: %s classified %s, expected %s", c.Name, key, predicted[key], e.Classification))
			}
		}
		for key, ct := range predictedType {
			if _, ok := expected[key]; !ok {
				bucket(ct).fp++
				failures = append(failures, fmt.Sprintf("%s: unexpected %s", c.Name, key))
			}
		}
	}

	types := make([]string, 0, len(perType))
	for ct := range perType {
		types = append(types, ct)
	}
	sort.Strings(types)
	var total counts
	var sb strings.Builder
	sb.WriteString("| Change type | TP | FP | FN | Precision | Recall | F1 |\n|---|---|---|---|---|---|---|\n")
	for _, ct := range types {
		c := *perType[ct]
		total.tp, total.fp, total.fn = total.tp+c.tp, total.fp+c.fp, total.fn+c.fn
		fmt.Fprintf(&sb, "| %s | %d | %d | %d | %.3f | %.3f | %.3f |\n", ct, c.tp, c.fp, c.fn, c.precision(), c.recall(), c.f1())
	}
	fmt.Fprintf(&sb, "| **All** | %d | %d | %d | %.3f | %.3f | %.3f |\n", total.tp, total.fp, total.fn, total.precision(), total.recall(), total.f1())
	summary := fmt.Sprintf("%d cases, %d labelled changes; backward classification accuracy on detected changes: %d / %d", len(m.Cases), labelled, classCorrect, classified)
	t.Logf("\n%s\n%s", sb.String(), summary)

	if os.Getenv("COMPATLAB_WRITE_RESULTS") == "1" {
		out := "# Accuracy evaluation (latest run)\n\nGenerated by `go test ./benchmarks -run TestAccuracy` with COMPATLAB_WRITE_RESULTS=1. Do not edit by hand.\n\n" +
			sb.String() + "\n" + summary + "\n"
		if len(failures) > 0 {
			out += "\nMismatches:\n"
			for _, f := range failures {
				out += "- " + f + "\n"
			}
		}
		if err := os.WriteFile(filepath.Join("..", "docs", "benchmarks", "accuracy-latest.md"), []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range failures {
		t.Error(f)
	}
}
