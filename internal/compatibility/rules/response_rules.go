package rules

import (
	"strings"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func responseRules() []Rule {
	res := []domain.ChangeScope{domain.ScopeResponse}
	return []Rule{
		{
			ID: "RES-001", Name: "Response field removed", ChangeType: domain.ChangeFieldRemoved, Scopes: res,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				sev := domain.SeverityMedium
				if isTrue(c, "required") {
					sev = domain.SeverityHigh
				}
				return []Verdict{
					backward(domain.Breaking, sev,
						"Consumers reading this response field will no longer receive it.",
						"Keep returning the field, or deprecate it before removal."),
					forward(domain.NonBreaking, domain.SeverityInfo,
						"Consumers written for the new API do not read the removed field.", ""),
				}
			},
		},
		{
			ID: "RES-002", Name: "Response field added", ChangeType: domain.ChangeFieldAdded, Scopes: res,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				fwd := forward(domain.Conditional, domain.SeverityLow,
					"New consumers may read a field the old provider does not return.", "")
				if isTrue(c, "required") {
					fwd = forward(domain.Conditional, domain.SeverityMedium,
						"New consumers may depend on a required field the old provider does not return.", "")
				}
				return []Verdict{
					backward(domain.Conditional, domain.SeverityLow,
						"Tolerant consumers ignore the new field; strict-schema consumers may reject it.",
						"Verify consumers that validate responses strictly."),
					fwd,
				}
			},
		},
		{
			ID: "RES-003", Name: "Response required flag changed", ChangeType: domain.ChangeRequiredChanged, Scopes: res,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				if c.Attributes["to"] == "true" {
					return []Verdict{
						backward(domain.NonBreaking, domain.SeverityInfo, "The field is now always present.", ""),
						forward(domain.Conditional, domain.SeverityLow, "New consumers may assume a field the old provider omits.", ""),
					}
				}
				return []Verdict{
					backward(domain.Breaking, domain.SeverityHigh,
						"Consumers may assume the field is always present.",
						"Keep the field required, or release it as a new major version."),
					forward(domain.NonBreaking, domain.SeverityInfo, "New consumers already handle a missing field.", ""),
				}
			},
		},
		{
			ID: "RES-004", Name: "Response type changed", ChangeType: domain.ChangeTypeChanged, Scopes: res,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				from, to := c.Attributes["from"], c.Attributes["to"]
				switch {
				case c.Attributes["kind"] == "content_type":
					return []Verdict{
						backward(domain.Breaking, domain.SeverityHigh, "Consumers parse the old content type.", ""),
						forward(domain.Breaking, domain.SeverityHigh, "New consumers parse a content type the old provider does not return.", ""),
					}
				case from == "integer" && to == "number":
					return []Verdict{
						backward(domain.Conditional, domain.SeverityMedium, "Consumers that parse the value as an integer may fail on decimals.", ""),
						forward(domain.NonBreaking, domain.SeverityInfo, "Integers are valid numbers.", ""),
					}
				case from == "number" && to == "integer":
					return []Verdict{
						backward(domain.NonBreaking, domain.SeverityInfo, "Integers are valid numbers.", ""),
						forward(domain.Conditional, domain.SeverityMedium, "New consumers may expect integers the old provider does not guarantee.", ""),
					}
				}
				return []Verdict{
					backward(domain.Breaking, domain.SeverityHigh,
						"Response property type changed from "+from+" to "+to+"; consumers parse the old type.",
						"Keep the old type, or introduce the new type under a new field name."),
					forward(domain.Breaking, domain.SeverityHigh,
						"New consumers expect "+to+" but the old provider returns "+from+".", ""),
				}
			},
		},
		{
			ID: "RES-005", Name: "Response enum changed", ChangeType: domain.ChangeEnumChanged, Scopes: res,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				back := backward(domain.NonBreaking, domain.SeverityInfo, "The provider returns a subset of the previous values.", "")
				if c.Attributes["added"] != "" {
					back = backward(domain.Conditional, domain.SeverityMedium,
						"Consumers with exhaustive handling may not recognise new values.",
						"Announce new values and verify consumer fallbacks.")
				}
				fwd := forward(domain.NonBreaking, domain.SeverityInfo, "New consumers handle every value the old provider returns.", "")
				if c.Attributes["removed"] != "" {
					fwd = forward(domain.Conditional, domain.SeverityMedium,
						"The old provider may return values new consumers no longer handle.", "")
				}
				return []Verdict{back, fwd}
			},
		},
		{
			ID: "RES-006", Name: "Response constraint changed", ChangeType: domain.ChangeConstraintChanged, Scopes: res,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				switch c.Attributes["direction"] {
				case "tightened":
					return []Verdict{
						backward(domain.NonBreaking, domain.SeverityInfo, "The provider now guarantees more.", ""),
						forward(domain.Conditional, domain.SeverityMedium, "New consumers may rely on a guarantee the old provider lacks.", ""),
					}
				case "relaxed":
					if c.Attributes["constraint"] == "nullable" {
						return []Verdict{
							backward(domain.Breaking, domain.SeverityHigh,
								"The value may now be null; consumers may not handle null.",
								"Keep the field non-nullable or document null handling."),
							forward(domain.NonBreaking, domain.SeverityInfo, "New consumers already handle null.", ""),
						}
					}
					return []Verdict{
						backward(domain.Conditional, domain.SeverityMedium, "Consumers may rely on the previous bounds.", ""),
						forward(domain.NonBreaking, domain.SeverityInfo, "New consumers handle the wider range.", ""),
					}
				}
				return []Verdict{
					backward(domain.Conditional, domain.SeverityMedium, "The effect depends on how consumers use the value.", ""),
					forward(domain.Conditional, domain.SeverityMedium, "The effect depends on how consumers use the value.", ""),
				}
			},
		},
		{
			ID: "RES-007", Name: "Response status removed", ChangeType: domain.ChangeStatusCodeRemoved, Scopes: res,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				back := backward(domain.Conditional, domain.SeverityMedium,
					"Consumers handling this status will see different behaviour.",
					"Document which status replaces it.")
				if strings.HasPrefix(c.Attributes["status"], "2") {
					back = backward(domain.Breaking, domain.SeverityHigh,
						"A success status consumers rely on is no longer documented.", "")
				}
				return []Verdict{
					back,
					forward(domain.NonBreaking, domain.SeverityInfo, "New consumers do not rely on the removed status.", ""),
				}
			},
		},
		{
			ID: "RES-008", Name: "Response status added", ChangeType: domain.ChangeStatusCodeAdded, Scopes: res,
			Evaluate: func(c domain.ChangeRecord) []Verdict {
				return []Verdict{
					backward(domain.Conditional, domain.SeverityLow, "Existing consumers may not handle the new status code.", ""),
					forward(domain.Conditional, domain.SeverityLow, "New consumers may rely on a status the old provider never returns.", ""),
				}
			},
		},
	}
}
