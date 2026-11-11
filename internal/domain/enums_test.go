package domain

import "testing"

func TestClassificationValidity(t *testing.T) {
	for _, c := range AllClassifications() {
		if !c.Valid() {
			t.Errorf("%q should be valid", c)
		}
	}
	if ChangeClassification("breaking").Valid() {
		t.Error("lower-case value must be invalid")
	}
}

func TestSeverityRankIsOrdered(t *testing.T) {
	if !(SeverityLow.Rank() < SeverityHigh.Rank()) {
		t.Error("LOW must rank below HIGH")
	}
	if Severity("NOPE").Rank() != -1 {
		t.Error("invalid severity must rank -1")
	}
}

func TestChangeTypesAreUniqueAndValid(t *testing.T) {
	seen := map[ChangeType]bool{}
	for _, ct := range AllChangeTypes() {
		if seen[ct] {
			t.Errorf("duplicate change type %q", ct)
		}
		seen[ct] = true
		if !ct.Valid() {
			t.Errorf("%q should be valid", ct)
		}
	}
	if len(seen) != 14 {
		t.Errorf("expected 14 change types, got %d", len(seen))
	}
}

func TestOtherEnumsRejectUnknownValues(t *testing.T) {
	if TestStatus("x").Valid() || TestCategory("x").Valid() || PolicyDecision("x").Valid() ||
		SpecificationStatus("x").Valid() || CompatibilityDirection("x").Valid() {
		t.Error("unknown enum values must be invalid")
	}
	if !TestStatusPassed.Valid() || !CategoryContract.Valid() || !DecisionBlock.Valid() ||
		!SpecValid.Valid() || !DirectionBackward.Valid() {
		t.Error("known enum values must be valid")
	}
}
