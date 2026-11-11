package classifier

import (
	"testing"

	"github.com/thanhmaitran/compatlab/internal/compatibility/rules"
	"github.com/thanhmaitran/compatlab/internal/domain"
)

func TestClassifyProducesOneEntryPerDirection(t *testing.T) {
	changes := []domain.ChangeRecord{
		{ChangeType: domain.ChangeEndpointRemoved, Scope: domain.ScopeEndpoint, Path: "/a", Method: "GET", Location: "endpoint"},
	}
	got := Classify(changes, rules.Default())
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if got[0].Direction != domain.DirectionBackward || got[0].Classification != domain.Breaking || got[0].RuleID != "EP-001" {
		t.Errorf("unexpected backward entry: %+v", got[0])
	}
	if got[1].Direction != domain.DirectionForward || got[1].Classification != domain.NonBreaking {
		t.Errorf("unexpected forward entry: %+v", got[1])
	}
}

func TestUnknownChangesAreNeverIgnored(t *testing.T) {
	changes := []domain.ChangeRecord{{ChangeType: domain.ChangeType("MYSTERY"), Path: "/x"}}
	got := Classify(changes, rules.Default())
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	for _, c := range got {
		if c.RuleID != UnclassifiedRuleID || c.Classification != domain.Conditional || c.Severity != domain.SeverityHigh {
			t.Errorf("unexpected fallback entry: %+v", c)
		}
	}
}

func TestClassifyOrderIsStable(t *testing.T) {
	a := domain.ChangeRecord{ChangeType: domain.ChangeEndpointAdded, Scope: domain.ScopeEndpoint, Path: "/b", Method: "GET"}
	b := domain.ChangeRecord{ChangeType: domain.ChangeEndpointAdded, Scope: domain.ScopeEndpoint, Path: "/a", Method: "GET"}
	got := Classify([]domain.ChangeRecord{a, b}, rules.Default())
	if got[0].Path != "/a" || got[len(got)-1].Path != "/b" {
		t.Errorf("results not sorted by path: %v ... %v", got[0].Path, got[len(got)-1].Path)
	}
}

func TestMaxSeverity(t *testing.T) {
	if MaxSeverity(nil) != domain.SeverityInfo {
		t.Error("empty list must yield INFO")
	}
	list := []domain.ClassifiedChange{{Severity: domain.SeverityLow}, {Severity: domain.SeverityCritical}, {Severity: domain.SeverityMedium}}
	if MaxSeverity(list) != domain.SeverityCritical {
		t.Error("expected CRITICAL")
	}
}
