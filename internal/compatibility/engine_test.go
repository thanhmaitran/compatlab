package compatibility_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

func load(t *testing.T, rel string) *domain.NormalizedAPI {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	api, err := specification.NewParser().Parse(data)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return api
}

func ruleIDs(a *domain.CompatibilityAnalysis, dir domain.CompatibilityDirection, cl domain.ChangeClassification) map[string]bool {
	out := map[string]bool{}
	for _, c := range a.Changes {
		if c.Direction == dir && c.Classification == cl {
			out[c.RuleID] = true
		}
	}
	return out
}

func TestCompatibleEvolutionHasNoBackwardBreakingChanges(t *testing.T) {
	a, err := compatibility.NewEngine().Compare(load(t, "compatible/users-v1.yaml"), load(t, "compatible/users-v2.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if n := a.SummaryFor(domain.DirectionBackward).Breaking; n != 0 {
		t.Errorf("expected 0 backward breaking changes, got %d: %+v", n, a.Changes)
	}
	if a.SummaryFor(domain.DirectionBackward).NonBreaking == 0 || a.SummaryFor(domain.DirectionBackward).Conditional == 0 {
		t.Errorf("expected non-breaking and conditional changes: %+v", a.SummaryFor(domain.DirectionBackward))
	}
	conditional := ruleIDs(a, domain.DirectionBackward, domain.Conditional)
	for _, id := range []string{"RES-002", "RES-005", "RES-008"} {
		if !conditional[id] {
			t.Errorf("expected conditional rule %s", id)
		}
	}
	if a.Scores == nil || a.Scores.Backward < 80 {
		t.Errorf("compatible change should score >= 80 backward, got %+v", a.Scores)
	}
}

func TestBreakingEvolutionIsDetected(t *testing.T) {
	a, err := compatibility.NewEngine().Compare(load(t, "compatible/users-v1.yaml"), load(t, "breaking/users-v2-breaking.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	breaking := ruleIDs(a, domain.DirectionBackward, domain.Breaking)
	for _, id := range []string{"EP-003", "RES-001", "RES-004", "REQ-001", "REQ-005", "REQ-006", "PAR-003", "SEC-001"} {
		if !breaking[id] {
			t.Errorf("expected breaking rule %s, got %v", id, breaking)
		}
	}
	if !ruleIDs(a, domain.DirectionBackward, domain.Conditional)["RES-007"] {
		t.Error("removed 404 should be a conditional RES-007 change")
	}
	if a.Scores == nil || a.Scores.Backward > 69 {
		t.Errorf("severe breaking changes must cap the score at 69, got %+v", a.Scores)
	}
}

func TestAnalysisIsReproducible(t *testing.T) {
	run := func() []byte {
		a, err := compatibility.NewEngine().Compare(load(t, "compatible/users-v1.yaml"), load(t, "breaking/users-v2-breaking.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	first := run()
	for i := 0; i < 5; i++ {
		if string(run()) != string(first) {
			t.Fatal("analysis output differs between runs")
		}
	}
}

func TestIdenticalVersionsProduceNoChanges(t *testing.T) {
	a, err := compatibility.NewEngine().Compare(load(t, "compatible/users-v1.yaml"), load(t, "compatible/users-v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Changes) != 0 || a.Scores.Backward != 100 || a.Scores.Forward != 100 {
		t.Errorf("unexpected result: %+v", a)
	}
}

func TestNilInputsAreRejected(t *testing.T) {
	if _, err := compatibility.NewEngine().Compare(nil, nil); err == nil {
		t.Error("expected an error for nil inputs")
	}
}

func TestConditionalOnlyEvolution(t *testing.T) {
	a, err := compatibility.NewEngine().Compare(load(t, "conditional/items-v1.yaml"), load(t, "conditional/items-v2.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sum := a.SummaryFor(domain.DirectionBackward)
	if sum.Breaking != 0 || sum.Conditional < 3 {
		t.Errorf("expected only conditional backward changes (new field, new enum value, new status): %+v", sum)
	}
	ids := ruleIDs(a, domain.DirectionBackward, domain.Conditional)
	for _, id := range []string{"RES-002", "RES-005", "RES-008"} {
		if !ids[id] {
			t.Errorf("expected conditional rule %s, got %v", id, ids)
		}
	}
}
