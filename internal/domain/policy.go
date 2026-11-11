package domain

import "fmt"

// ReleasePolicy states when a release may proceed. Unless IncludeForward is
// set, only backward-direction changes are evaluated.
type ReleasePolicy struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	FailOn               []ChangeClassification `json:"fail_on"`
	RequireContractTests bool                   `json:"require_contract_tests"`
	MinimumPassRate      float64                `json:"minimum_pass_rate"` // percent, 0-100
	IncludeForward       bool                   `json:"include_forward,omitempty"`
}

// Validate checks that the policy only uses known values.
func (p *ReleasePolicy) Validate() error {
	for _, c := range p.FailOn {
		if !c.Valid() {
			return fmt.Errorf("%w: unknown classification %q in fail_on", ErrInvalid, c)
		}
	}
	if p.MinimumPassRate < 0 || p.MinimumPassRate > 100 {
		return fmt.Errorf("%w: minimum_pass_rate must be between 0 and 100", ErrInvalid)
	}
	return nil
}

// PolicyEvaluation is the decision plus the reasons behind it.
type PolicyEvaluation struct {
	Decision PolicyDecision `json:"decision"`
	Reasons  []string       `json:"reasons,omitempty"`
}
