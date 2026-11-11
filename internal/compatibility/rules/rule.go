// Package rules holds the compatibility rule registry. Each rule maps one
// kind of change to a verdict per compatibility direction.
//
// Direction semantics:
//
//	BACKWARD: can existing consumers keep working against the new provider?
//	FORWARD:  can consumers written for the new API work against the old provider?
package rules

import (
	"slices"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// Verdict is one rule's judgement for one direction.
type Verdict struct {
	Direction      domain.CompatibilityDirection
	Classification domain.ChangeClassification
	Severity       domain.Severity
	Explanation    string
	Remediation    string
}

// Rule evaluates changes of one type (optionally restricted to some scopes).
type Rule struct {
	ID         string
	Name       string
	ChangeType domain.ChangeType
	Scopes     []domain.ChangeScope // empty means any scope
	Evaluate   func(c domain.ChangeRecord) []Verdict
}

// Applies reports whether the rule handles the given change.
func (r Rule) Applies(c domain.ChangeRecord) bool {
	if r.ChangeType != c.ChangeType {
		return false
	}
	return len(r.Scopes) == 0 || slices.Contains(r.Scopes, c.Scope)
}

// Find returns the first rule that applies to the change.
func Find(set []Rule, c domain.ChangeRecord) (Rule, bool) {
	for _, r := range set {
		if r.Applies(c) {
			return r, true
		}
	}
	return Rule{}, false
}

// Default returns the built-in rule set in a stable order.
func Default() []Rule {
	var all []Rule
	all = append(all, endpointRules()...)
	all = append(all, parameterRules()...)
	all = append(all, requestRules()...)
	all = append(all, responseRules()...)
	all = append(all, securityRules()...)
	return all
}

func backward(cl domain.ChangeClassification, sev domain.Severity, explanation, remediation string) Verdict {
	return Verdict{domain.DirectionBackward, cl, sev, explanation, remediation}
}

func forward(cl domain.ChangeClassification, sev domain.Severity, explanation, remediation string) Verdict {
	return Verdict{domain.DirectionForward, cl, sev, explanation, remediation}
}

func isTrue(c domain.ChangeRecord, key string) bool { return c.Attributes[key] == "true" }
