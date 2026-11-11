package reporting

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func sampleReport() *domain.CompatibilityReport {
	return &domain.CompatibilityReport{
		Analysis: &domain.CompatibilityAnalysis{
			BaseVersion: "1.0.0", TargetVersion: "2.0.0",
			Scores: &domain.CompatibilityScores{Backward: 55, Forward: 90},
			Changes: []domain.ClassifiedChange{{
				ChangeRecord: domain.ChangeRecord{Path: "/users/{id}", Method: "DELETE", Description: "<script>alert(1)</script>"},
				RuleID:       "EP-003", Classification: domain.Breaking, Severity: domain.SeverityHigh,
				Direction: domain.DirectionBackward,
			}},
		},
		Contracts: []domain.ContractVerification{{Consumer: "mobile-app", Provider: "users-api",
			Interactions: []domain.InteractionResult{{Description: "Retrieve user", Passed: false, Message: "id has the wrong type"}}}},
		Impacts:    []domain.ConsumerImpact{{Consumer: "mobile-app", Level: domain.ImpactAffected}},
		Evaluation: &domain.PolicyEvaluation{Decision: domain.DecisionBlock, Reasons: []string{"1 BREAKING change(s) violate the policy"}},
	}
}

func TestTextReport(t *testing.T) {
	out := Text(sampleReport())
	for _, want := range []string{"COMPATLAB RELEASE CHECK", "Base:   1.0.0", "BREAKING (backward)", "EP-003",
		"mobile-app: 1 affected", "0 / 1 interactions passed", "Release decision: BLOCKED", "violate the policy"} {
		if !strings.Contains(out, want) {
			t.Errorf("text report is missing %q:\n%s", want, out)
		}
	}
}

func TestTextReportWithoutAnalysis(t *testing.T) {
	if out := Text(&domain.CompatibilityReport{}); !strings.Contains(out, "No analysis") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestJSONReportIsValidAndStable(t *testing.T) {
	a, err := JSON(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(a, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	b, _ := JSON(sampleReport())
	if string(a) != string(b) {
		t.Error("JSON output must be stable")
	}
}

func TestJSONGenerator(t *testing.T) {
	out, err := JSONGenerator{}.Generate(sampleReport().Analysis)
	if err != nil || !strings.Contains(string(out), `"base_version": "1.0.0"`) {
		t.Errorf("unexpected output %q err %v", out, err)
	}
}

func TestHTMLReportEscapesContent(t *testing.T) {
	out, err := HTML(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<script>alert(1)</script>") {
		t.Error("HTML report must escape change descriptions")
	}
	if !strings.Contains(s, "EP-003") || !strings.Contains(s, "BLOCK") {
		t.Errorf("HTML report is missing content:\n%s", s)
	}
}
