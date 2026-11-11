package rules

import "github.com/thanhmaitran/compatlab/internal/domain"

func requestRules() []Rule {
	req := []domain.ChangeScope{domain.ScopeRequest}
	in := []domain.ChangeScope{domain.ScopeRequest, domain.ScopeParameter}
	return []Rule{
		{
			ID: "REQ-001", Name: "Request field added", ChangeType: domain.ChangeFieldAdded, Scopes: req,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				if isTrue(c, "required") {
					return []Verdict{
						backward(domain.Breaking, domain.SeverityHigh,
							"Existing consumers do not send the new required request field.",
							"Make the field optional or provide a server-side default."),
						forward(domain.NonBreaking, domain.SeverityInfo,
							"The old provider accepts the extra field or ignores it.", ""),
					}
				}
				return []Verdict{
					backward(domain.NonBreaking, domain.SeverityInfo,
						"Existing consumers are unaffected by a new optional request field.", ""),
					forward(domain.Conditional, domain.SeverityLow,
						"The old provider may reject or ignore the new field.", ""),
				}
			},
		},
		{
			ID: "REQ-002", Name: "Request field removed", ChangeType: domain.ChangeFieldRemoved, Scopes: req,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				fwd := forward(domain.NonBreaking, domain.SeverityInfo, "The removed field was optional.", "")
				if isTrue(c, "required") {
					fwd = forward(domain.Breaking, domain.SeverityHigh,
						"New consumers omit a field the old provider requires.", "")
				}
				return []Verdict{
					backward(domain.Conditional, domain.SeverityMedium,
						"Consumers still send the field; a strict provider may reject it or lose its behaviour.",
						"Keep accepting the field and ignore it, or document the removal."),
					fwd,
				}
			},
		},
		{
			ID: "REQ-004", Name: "Request type changed", ChangeType: domain.ChangeTypeChanged, Scopes: in,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				if c.Attributes["kind"] == "content_type" {
					return []Verdict{
						backward(domain.Breaking, domain.SeverityHigh, "Existing consumers send the old content type.", ""),
						forward(domain.Breaking, domain.SeverityHigh, "New consumers send a content type the old provider may not accept.", ""),
					}
				}
				if c.Attributes["from"] == "integer" && c.Attributes["to"] == "number" {
					return []Verdict{
						backward(domain.NonBreaking, domain.SeverityInfo, "Every integer is a valid number.", ""),
						forward(domain.Breaking, domain.SeverityMedium, "New consumers may send decimals the old provider rejects.", ""),
					}
				}
				return []Verdict{
					backward(domain.Breaking, domain.SeverityHigh,
						"Existing consumers send the old type and will be rejected.",
						"Accept both types or version the endpoint."),
					forward(domain.Breaking, domain.SeverityHigh,
						"New consumers send a type the old provider does not accept.", ""),
				}
			},
		},
		{
			ID: "REQ-005", Name: "Request enum changed", ChangeType: domain.ChangeEnumChanged, Scopes: in,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				back := backward(domain.NonBreaking, domain.SeverityInfo, "Only new values were allowed; existing values still work.", "")
				if c.Attributes["removed"] != "" {
					back = backward(domain.Breaking, domain.SeverityHigh,
						"Values consumers may send are no longer accepted.",
						"Keep accepting removed values or deprecate them first.")
				}
				fwd := forward(domain.NonBreaking, domain.SeverityInfo, "New consumers only send values the old provider accepts.", "")
				if c.Attributes["added"] != "" {
					fwd = forward(domain.Conditional, domain.SeverityMedium,
						"New consumers may send values the old provider rejects.", "")
				}
				return []Verdict{back, fwd}
			},
		},
		{
			ID: "REQ-006", Name: "Request constraint changed", ChangeType: domain.ChangeConstraintChanged, Scopes: in,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				switch c.Attributes["direction"] {
				case "tightened":
					return []Verdict{
						backward(domain.Breaking, domain.SeverityMedium,
							"Inputs that were valid before may now be rejected.",
							"Relax the constraint or document the stricter validation."),
						forward(domain.NonBreaking, domain.SeverityInfo, "New consumers send stricter values the old provider accepts.", ""),
					}
				case "relaxed":
					return []Verdict{
						backward(domain.NonBreaking, domain.SeverityInfo, "Previously valid inputs remain valid.", ""),
						forward(domain.Conditional, domain.SeverityMedium, "New consumers may send values the old provider rejects.", ""),
					}
				}
				return []Verdict{
					backward(domain.Conditional, domain.SeverityMedium, "The effect of this constraint change depends on the values consumers send.", ""),
					forward(domain.Conditional, domain.SeverityMedium, "The effect of this constraint change depends on the values consumers send.", ""),
				}
			},
		},
	}
}
