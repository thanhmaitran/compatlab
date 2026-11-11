package policies

import (
	"errors"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func analysisWith(changes ...domain.ClassifiedChange) *domain.CompatibilityAnalysis {
	return &domain.CompatibilityAnalysis{Changes: changes}
}

func ch(dir domain.CompatibilityDirection, cl domain.ChangeClassification) domain.ClassifiedChange {
	return domain.ClassifiedChange{Direction: dir, Classification: cl}
}

func evaluate(t *testing.T, p *domain.ReleasePolicy, r *domain.CompatibilityReport) *domain.PolicyEvaluation {
	t.Helper()
	ev, err := Evaluator{}.Evaluate(p, r)
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

func TestBreakingChangeBlocksWhenConfigured(t *testing.T) {
	p := &domain.ReleasePolicy{FailOn: []domain.ChangeClassification{domain.Breaking}}
	r := &domain.CompatibilityReport{Analysis: analysisWith(ch(domain.DirectionBackward, domain.Breaking))}
	if ev := evaluate(t, p, r); ev.Decision != domain.DecisionBlock || len(ev.Reasons) == 0 {
		t.Errorf("unexpected evaluation: %+v", ev)
	}
}

func TestForwardChangesAreIgnoredUnlessRequested(t *testing.T) {
	r := &domain.CompatibilityReport{Analysis: analysisWith(ch(domain.DirectionForward, domain.Breaking))}
	p := &domain.ReleasePolicy{FailOn: []domain.ChangeClassification{domain.Breaking}}
	if ev := evaluate(t, p, r); ev.Decision != domain.DecisionPass {
		t.Errorf("forward-only change must not block by default: %+v", ev)
	}
	p.IncludeForward = true
	if ev := evaluate(t, p, r); ev.Decision != domain.DecisionBlock {
		t.Errorf("include_forward must block: %+v", ev)
	}
}

func TestConditionalChangesWarn(t *testing.T) {
	p := &domain.ReleasePolicy{FailOn: []domain.ChangeClassification{domain.Breaking}}
	r := &domain.CompatibilityReport{Analysis: analysisWith(ch(domain.DirectionBackward, domain.Conditional))}
	if ev := evaluate(t, p, r); ev.Decision != domain.DecisionWarn {
		t.Errorf("expected WARN, got %+v", ev)
	}
}

func TestNonBreakingOnlyPasses(t *testing.T) {
	p := &domain.ReleasePolicy{FailOn: []domain.ChangeClassification{domain.Breaking}}
	r := &domain.CompatibilityReport{Analysis: analysisWith(ch(domain.DirectionBackward, domain.NonBreaking))}
	if ev := evaluate(t, p, r); ev.Decision != domain.DecisionPass {
		t.Errorf("expected PASS, got %+v", ev)
	}
}

func TestContractRequirements(t *testing.T) {
	r := &domain.CompatibilityReport{Analysis: analysisWith()}
	required := &domain.ReleasePolicy{RequireContractTests: true}
	if ev := evaluate(t, required, r); ev.Decision != domain.DecisionBlock {
		t.Errorf("missing contracts must block when required: %+v", ev)
	}
	r.Contracts = []domain.ContractVerification{{Interactions: []domain.InteractionResult{{Passed: false}}}}
	if ev := evaluate(t, required, r); ev.Decision != domain.DecisionBlock {
		t.Errorf("failed contract must block when required: %+v", ev)
	}
	if ev := evaluate(t, &domain.ReleasePolicy{}, r); ev.Decision != domain.DecisionWarn {
		t.Errorf("failed contract must warn when not required: %+v", ev)
	}
}

func TestMinimumPassRate(t *testing.T) {
	p := &domain.ReleasePolicy{MinimumPassRate: 95}
	r := &domain.CompatibilityReport{Analysis: analysisWith()}
	if ev := evaluate(t, p, r); ev.Decision != domain.DecisionWarn {
		t.Errorf("missing test run must warn: %+v", ev)
	}
	r.TestRun = &domain.TestRun{Results: []domain.TestResult{{Status: domain.TestStatusPassed}, {Status: domain.TestStatusFailed}}}
	if ev := evaluate(t, p, r); ev.Decision != domain.DecisionBlock {
		t.Errorf("50%% pass rate must block: %+v", ev)
	}
}

func TestInvalidInputs(t *testing.T) {
	if _, err := (Evaluator{}).Evaluate(nil, nil); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("expected ErrInvalid, got %v", err)
	}
	bad := &domain.ReleasePolicy{MinimumPassRate: 150}
	if _, err := (Evaluator{}).Evaluate(bad, &domain.CompatibilityReport{Analysis: analysisWith()}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("expected ErrInvalid, got %v", err)
	}
}

func TestParsePolicyForms(t *testing.T) {
	nested := "compatlab:\n  policy:\n    fail_on:\n      - breaking\n    require_contract_tests: true\n    minimum_pass_rate: 95\n"
	p, err := Parse([]byte(nested))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.FailOn) != 1 || p.FailOn[0] != domain.Breaking || !p.RequireContractTests || p.MinimumPassRate != 95 {
		t.Errorf("unexpected policy: %+v", p)
	}
	flat := "policy:\n  fail_on: [BREAKING, CONDITIONAL]\n  include_forward: true\n"
	p, err = Parse([]byte(flat))
	if err != nil || len(p.FailOn) != 2 || !p.IncludeForward {
		t.Errorf("unexpected flat policy: %+v %v", p, err)
	}
}

func TestParsePolicyErrors(t *testing.T) {
	for _, doc := range []string{"{{{", "other: 1\n", "policy:\n  fail_on: [SCARY]\n", "policy:\n  minimum_pass_rate: 500\n"} {
		if _, err := Parse([]byte(doc)); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%q: expected ErrInvalid, got %v", doc, err)
		}
	}
}
