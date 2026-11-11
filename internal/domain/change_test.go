package domain

import "testing"

func TestNewChangeIDIsStableAndDistinct(t *testing.T) {
	a := NewChangeID("/users/{id}", "GET", ChangeTypeChanged, "response.200.id")
	b := NewChangeID("/users/{id}", "GET", ChangeTypeChanged, "response.200.id")
	c := NewChangeID("/users/{id}", "GET", ChangeTypeChanged, "response.200.email")
	if a != b {
		t.Error("same input must give same ID")
	}
	if a == c {
		t.Error("different location must give different ID")
	}
	if len(a) != 16 {
		t.Errorf("expected 16 hex chars, got %d", len(a))
	}
}

func TestAnalysisSummaryCountsByClassification(t *testing.T) {
	a := &CompatibilityAnalysis{Changes: []ClassifiedChange{
		{Classification: Breaking}, {Classification: Breaking},
		{Classification: Conditional}, {Classification: NonBreaking},
		{Classification: NonBreaking}, {Classification: NonBreaking},
	}}
	got := a.Summary()
	want := AnalysisSummary{Breaking: 2, NonBreaking: 3, Conditional: 1}
	if got != want {
		t.Errorf("Summary() = %+v, want %+v", got, want)
	}
}
