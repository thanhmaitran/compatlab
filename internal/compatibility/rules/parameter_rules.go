package rules

import "github.com/thanhmaitran/compatlab/internal/domain"

func parameterRules() []Rule {
	param := []domain.ChangeScope{domain.ScopeParameter}
	return []Rule{
		{
			ID: "PAR-001", Name: "Parameter removed", ChangeType: domain.ChangeParameterRemoved, Scopes: param,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				fwd := forward(domain.NonBreaking, domain.SeverityInfo, "The parameter was optional, so omitting it is safe.", "")
				if isTrue(c, "required") {
					fwd = forward(domain.Breaking, domain.SeverityMedium,
						"Consumers of the new API omit a parameter the old provider requires.", "")
				}
				return []Verdict{
					backward(domain.Conditional, domain.SeverityMedium,
						"Consumers may still send the parameter and rely on its effect.",
						"Keep accepting the parameter, or document the removal."),
					fwd,
				}
			},
		},
		{
			ID: "PAR-002", Name: "Parameter added", ChangeType: domain.ChangeParameterAdded, Scopes: param,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				if isTrue(c, "required") {
					return []Verdict{
						backward(domain.Breaking, domain.SeverityHigh,
							"Existing consumers do not send the new required parameter.",
							"Make the parameter optional or provide a default."),
						forward(domain.NonBreaking, domain.SeverityInfo,
							"The old provider ignores an extra parameter.", ""),
					}
				}
				return []Verdict{
					backward(domain.NonBreaking, domain.SeverityInfo,
						"Existing consumers are unaffected by a new optional parameter.", ""),
					forward(domain.Conditional, domain.SeverityLow,
						"The old provider may ignore the parameter that new consumers rely on.", ""),
				}
			},
		},
		{
			ID: "PAR-003", Name: "Required flag changed", ChangeType: domain.ChangeRequiredChanged,
			Scopes: []domain.ChangeScope{domain.ScopeParameter, domain.ScopeRequest},
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				if c.Attributes["to"] == "true" {
					return []Verdict{
						backward(domain.Breaking, domain.SeverityHigh,
							"Existing consumers that omit this input will be rejected.",
							"Keep the input optional or supply a default value."),
						forward(domain.NonBreaking, domain.SeverityInfo,
							"New consumers always send the input, which the old provider accepts.", ""),
					}
				}
				return []Verdict{
					backward(domain.NonBreaking, domain.SeverityInfo,
						"Making an input optional does not affect existing consumers.", ""),
					forward(domain.Breaking, domain.SeverityMedium,
						"New consumers may omit an input the old provider still requires.", ""),
				}
			},
		},
	}
}
