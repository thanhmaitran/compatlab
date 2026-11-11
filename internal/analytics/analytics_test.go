package analytics

import (
	"testing"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func analysis(back, fwd int, changes ...domain.ClassifiedChange) *domain.CompatibilityAnalysis {
	return &domain.CompatibilityAnalysis{Changes: changes, Scores: &domain.CompatibilityScores{Backward: back, Forward: fwd}}
}

func ch(dir domain.CompatibilityDirection, cl domain.ChangeClassification) domain.ClassifiedChange {
	return domain.ClassifiedChange{Direction: dir, Classification: cl}
}

func TestComputeOnEmptyInputRendersEmptySlices(t *testing.T) {
	s := Compute(Input{})
	if s.TotalAnalyses != 0 || s.ScoreTrend == nil || s.BreakingByVersion == nil || s.PassRateTrend == nil ||
		s.ContractTrend == nil || s.GateHistory == nil || s.TopChangedEndpoints == nil {
		t.Errorf("empty input must yield empty, non-nil slices: %+v", s)
	}
}

func TestScoreTrendAndBreakingByVersionAreChronological(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	in := Input{Analyses: []AnalysisPoint{
		{ID: "later", BaseLabel: "v2", TargetLabel: "v3", CreatedAt: t0.Add(48 * time.Hour),
			Analysis: analysis(95, 100, ch(domain.DirectionBackward, domain.NonBreaking))},
		{ID: "first", BaseLabel: "v1", TargetLabel: "v2", CreatedAt: t0,
			Analysis: analysis(60, 90,
				ch(domain.DirectionBackward, domain.Breaking), ch(domain.DirectionBackward, domain.Breaking),
				ch(domain.DirectionBackward, domain.Conditional), ch(domain.DirectionForward, domain.Breaking))},
	}}
	s := Compute(in)
	if s.TotalAnalyses != 2 || s.ScoreTrend[0].AnalysisID != "first" || s.ScoreTrend[1].AnalysisID != "later" {
		t.Fatalf("trend must be chronological: %+v", s.ScoreTrend)
	}
	if s.ScoreTrend[0].Backward != 60 || s.ScoreTrend[0].Forward != 90 {
		t.Errorf("scores wrong: %+v", s.ScoreTrend[0])
	}
	first := s.BreakingByVersion[0]
	if first.Target != "v2" || first.Breaking != 2 || first.Conditional != 1 || first.NonBreaking != 0 {
		t.Errorf("only backward changes count: %+v", first)
	}
	if s.Distribution.Breaking != 2 || s.Distribution.Conditional != 1 || s.Distribution.NonBreaking != 1 {
		t.Errorf("distribution wrong: %+v", s.Distribution)
	}
}

func TestPassRateContractAndGateHistory(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	run := &domain.TestRun{Results: []domain.TestResult{{Status: domain.TestStatusPassed}, {Status: domain.TestStatusFailed}}}
	blocked := &domain.CompatibilityReport{
		Contracts:  []domain.ContractVerification{{Interactions: []domain.InteractionResult{{Passed: true}, {Passed: false}}}},
		Evaluation: &domain.PolicyEvaluation{Decision: domain.DecisionBlock},
	}
	passed := &domain.CompatibilityReport{Evaluation: &domain.PolicyEvaluation{Decision: domain.DecisionPass}}
	s := Compute(Input{
		Runs:         []RunPoint{{ID: "r1", CreatedAt: t0, Run: run}},
		Reports:      []ReportPoint{{ID: "p2", CreatedAt: t0.Add(time.Hour), Report: passed}, {ID: "p1", CreatedAt: t0, Report: blocked}},
		TopEndpoints: []EndpointChange{{Method: "GET", Path: "/users", Count: 3}},
	})
	if len(s.PassRateTrend) != 1 || s.PassRateTrend[0].PassRate != 50 {
		t.Errorf("pass rate wrong: %+v", s.PassRateTrend)
	}
	if len(s.ContractTrend) != 1 || s.ContractTrend[0].Passed != 1 || s.ContractTrend[0].Total != 2 {
		t.Errorf("contract trend wrong: %+v", s.ContractTrend)
	}
	if len(s.GateHistory) != 2 || s.GateHistory[0].ReportID != "p1" || s.GateBlocks != 1 {
		t.Errorf("gate history wrong: %+v blocks=%d", s.GateHistory, s.GateBlocks)
	}
	if len(s.TopChangedEndpoints) != 1 || s.TopChangedEndpoints[0].Count != 3 {
		t.Errorf("top endpoints wrong: %+v", s.TopChangedEndpoints)
	}
}

func TestNilPayloadsAreSkipped(t *testing.T) {
	s := Compute(Input{
		Analyses: []AnalysisPoint{{ID: "a"}},
		Runs:     []RunPoint{{ID: "r"}},
		Reports:  []ReportPoint{{ID: "p"}},
	})
	if s.TotalAnalyses != 0 || len(s.PassRateTrend) != 0 || len(s.GateHistory) != 0 {
		t.Errorf("nil payloads must be skipped: %+v", s)
	}
}
