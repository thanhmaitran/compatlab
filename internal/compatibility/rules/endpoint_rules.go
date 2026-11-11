package rules

import "github.com/thanhmaitran/compatlab/internal/domain"

func endpointRules() []Rule {
	return []Rule{
		{
			ID: "EP-001", Name: "Endpoint removed", ChangeType: domain.ChangeEndpointRemoved,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				return []Verdict{
					backward(domain.Breaking, domain.SeverityHigh,
						"Existing consumers calling this endpoint will fail.",
						"Keep the endpoint, or deprecate it first and remove it in a later major version."),
					forward(domain.NonBreaking, domain.SeverityInfo,
						"Consumers written for the new API do not use a removed endpoint.", ""),
				}
			},
		},
		{
			ID: "EP-002", Name: "Endpoint added", ChangeType: domain.ChangeEndpointAdded,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				return []Verdict{
					backward(domain.NonBreaking, domain.SeverityInfo,
						"Existing consumers are unaffected by a new endpoint.", ""),
					forward(domain.Conditional, domain.SeverityLow,
						"Consumers that call the new endpoint fail against the old provider.",
						"Gate use of the new endpoint on the provider version."),
				}
			},
		},
		{
			ID: "EP-003", Name: "HTTP method removed", ChangeType: domain.ChangeMethodRemoved,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				return []Verdict{
					backward(domain.Breaking, domain.SeverityHigh,
						"Existing consumers using this HTTP method will receive errors.",
						"Keep the method or deprecate it before removal."),
					forward(domain.NonBreaking, domain.SeverityInfo,
						"Consumers written for the new API do not use the removed method.", ""),
				}
			},
		},
	}
}
