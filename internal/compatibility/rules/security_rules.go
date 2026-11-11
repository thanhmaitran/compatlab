package rules

import "github.com/thanhmaitran/compatlab/internal/domain"

func securityRules() []Rule {
	return []Rule{
		{
			ID: "SEC-001", Name: "Authentication requirements changed", ChangeType: domain.ChangeAuthChanged,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				switch c.Attributes["kind"] {
				case "added":
					return []Verdict{
						backward(domain.Breaking, domain.SeverityCritical,
							"The operation now requires authentication that existing consumers do not send.",
							"Roll out authentication behind a feature flag or new version."),
						forward(domain.NonBreaking, domain.SeverityInfo, "New consumers authenticate; the old provider ignores it.", ""),
					}
				case "removed":
					return []Verdict{
						backward(domain.NonBreaking, domain.SeverityInfo,
							"Existing consumers keep working; review the security exposure of the open endpoint.", ""),
						forward(domain.Conditional, domain.SeverityLow, "New consumers may not authenticate against the old provider.", ""),
					}
				}
				if c.Attributes["added_schemes"] != "" || c.Attributes["added_scopes"] != "" {
					return []Verdict{
						backward(domain.Conditional, domain.SeverityHigh,
							"Additional schemes or scopes are required; the impact depends on the security model (alternatives vs. combined requirements).",
							"Confirm existing credentials still satisfy the new requirements."),
						forward(domain.NonBreaking, domain.SeverityInfo, "New consumers present the stronger credentials.", ""),
					}
				}
				return []Verdict{
					backward(domain.NonBreaking, domain.SeverityInfo, "Requirements were relaxed.", ""),
					forward(domain.Conditional, domain.SeverityLow, "The old provider may still require the removed credentials.", ""),
				}
			},
		},
	}
}
