package scoring

import (
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func change(dir domain.CompatibilityDirection, cl domain.ChangeClassification, sev domain.Severity) domain.ClassifiedChange {
	return domain.ClassifiedChange{Direction: dir, Classification: cl, Severity: sev}
}

func TestNoChangesScores100(t *testing.T) {
	got := Compute(nil)
	if got.Backward != 100 || got.Forward != 100 {
		t.Errorf("got %+v, want 100/100", got)
	}
}

func TestNonBreakingChangesDoNotReduceScore(t *testing.T) {
	got := Compute([]domain.ClassifiedChange{change(domain.DirectionBackward, domain.NonBreaking, domain.SeverityInfo)})
	if got.Backward != 100 {
		t.Errorf("backward = %d, want 100", got.Backward)
	}
}

func TestBreakingChangeCapsScoreEvenWhenPenaltyIsSmall(t *testing.T) {
	got := Compute([]domain.ClassifiedChange{change(domain.DirectionBackward, domain.Breaking, domain.SeverityLow)})
	if got.Backward != 89 {
		t.Errorf("backward = %d, want 89 (cap for any breaking change)", got.Backward)
	}
	if got.Forward != 100 {
		t.Errorf("forward = %d, want 100 (direction is independent)", got.Forward)
	}
}

func TestHighSeverityBreakingCapsLower(t *testing.T) {
	got := Compute([]domain.ClassifiedChange{change(domain.DirectionBackward, domain.Breaking, domain.SeverityHigh)})
	if got.Backward != 69 {
		t.Errorf("backward = %d, want 69", got.Backward)
	}
}

func TestScoreNeverGoesBelowZero(t *testing.T) {
	var list []domain.ClassifiedChange
	for i := 0; i < 20; i++ {
		list = append(list, change(domain.DirectionBackward, domain.Breaking, domain.SeverityCritical))
	}
	if got := Compute(list); got.Backward != 0 {
		t.Errorf("backward = %d, want 0", got.Backward)
	}
}

func TestConditionalChangesCostHalf(t *testing.T) {
	got := Compute([]domain.ClassifiedChange{change(domain.DirectionBackward, domain.Conditional, domain.SeverityHigh)})
	if got.Backward != 95 {
		t.Errorf("backward = %d, want 95", got.Backward)
	}
}
