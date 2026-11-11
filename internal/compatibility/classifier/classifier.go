// Package classifier applies the rule registry to raw changes.
package classifier

import (
	"cmp"
	"slices"

	"github.com/thanhmaitran/compatlab/internal/compatibility/rules"
	"github.com/thanhmaitran/compatlab/internal/domain"
)

// UnclassifiedRuleID marks changes no rule covers. They are reported as
// CONDITIONAL with HIGH severity so they are never silently ignored.
const UnclassifiedRuleID = "GEN-000"

// Classify evaluates every change against the rule set and returns one
// classified entry per change and direction, in a stable order.
func Classify(changes []domain.ChangeRecord, set []rules.Rule) []domain.ClassifiedChange {
	out := make([]domain.ClassifiedChange, 0, len(changes)*2)
	for _, c := range changes {
		rule, ok := rules.Find(set, c)
		if !ok {
			for _, dir := range domain.AllDirections() {
				out = append(out, domain.ClassifiedChange{
					ChangeRecord:   c,
					RuleID:         UnclassifiedRuleID,
					Classification: domain.Conditional,
					Severity:       domain.SeverityHigh,
					Direction:      dir,
					Explanation:    "No compatibility rule covers this change type; review it manually.",
				})
			}
			continue
		}
		for _, v := range rule.Evaluate(c) {
			out = append(out, domain.ClassifiedChange{
				ChangeRecord:   c,
				RuleID:         rule.ID,
				Classification: v.Classification,
				Severity:       v.Severity,
				Direction:      v.Direction,
				Explanation:    v.Explanation,
				Remediation:    v.Remediation,
			})
		}
	}
	slices.SortStableFunc(out, func(a, b domain.ClassifiedChange) int {
		if c := cmp.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Method, b.Method); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Location, b.Location); c != 0 {
			return c
		}
		if c := cmp.Compare(a.ChangeType, b.ChangeType); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Attributes["constraint"], b.Attributes["constraint"]); c != 0 {
			return c
		}
		return cmp.Compare(a.Direction, b.Direction)
	})
	return out
}

// MaxSeverity returns the highest severity among the given changes, or
// SeverityInfo when the list is empty.
func MaxSeverity(changes []domain.ClassifiedChange) domain.Severity {
	best := domain.SeverityInfo
	for _, c := range changes {
		if c.Severity.Rank() > best.Rank() {
			best = c.Severity
		}
	}
	return best
}
