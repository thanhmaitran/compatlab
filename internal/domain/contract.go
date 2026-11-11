package domain

import "fmt"

// ContractRequest is the request part of an expected interaction.
type ContractRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// ContractResponse is the response a consumer depends on.
type ContractResponse struct {
	Status int `json:"status"`
	Body   any `json:"body,omitempty"`
}

// ContractInteraction is one request/response expectation.
type ContractInteraction struct {
	Description string           `json:"description"`
	Request     ContractRequest  `json:"request"`
	Response    ContractResponse `json:"response"`
}

// ConsumerContract is what a consumer expects from a provider.
type ConsumerContract struct {
	ContractVersion string                `json:"contract_version"`
	Consumer        string                `json:"consumer"`
	Provider        string                `json:"provider"`
	Interactions    []ContractInteraction `json:"interactions"`
}

// Validate checks structural completeness of the contract.
func (c *ConsumerContract) Validate() error {
	switch {
	case c.ContractVersion == "":
		return fmt.Errorf("%w: contract_version is required", ErrInvalid)
	case c.Consumer == "":
		return fmt.Errorf("%w: consumer is required", ErrInvalid)
	case c.Provider == "":
		return fmt.Errorf("%w: provider is required", ErrInvalid)
	case len(c.Interactions) == 0:
		return fmt.Errorf("%w: at least one interaction is required", ErrInvalid)
	}
	for i, in := range c.Interactions {
		if in.Request.Method == "" || in.Request.Path == "" {
			return fmt.Errorf("%w: interaction %d needs request method and path", ErrInvalid, i)
		}
		if in.Response.Status < 100 || in.Response.Status > 599 {
			return fmt.Errorf("%w: interaction %d has invalid response status %d", ErrInvalid, i, in.Response.Status)
		}
	}
	return nil
}

// InteractionResult is the verification outcome of one interaction.
type InteractionResult struct {
	Description string `json:"description"`
	Passed      bool   `json:"passed"`
	Message     string `json:"message,omitempty"`
}

// ContractVerification is the outcome of verifying a contract against an API.
type ContractVerification struct {
	Consumer     string              `json:"consumer"`
	Provider     string              `json:"provider"`
	Interactions []InteractionResult `json:"interactions"`
}

// Passed reports whether every interaction passed.
func (v *ContractVerification) Passed() bool {
	for _, r := range v.Interactions {
		if !r.Passed {
			return false
		}
	}
	return true
}

// Impact levels describe how a change affects one consumer.
const (
	ImpactAffected = "AFFECTED"
	ImpactReview   = "REVIEW"
)

// ConsumerImpact links a classified change to a consumer whose contract
// touches the changed endpoint or field.
type ConsumerImpact struct {
	Consumer       string               `json:"consumer"`
	Provider       string               `json:"provider"`
	ChangeID       string               `json:"change_id"`
	RuleID         string               `json:"rule_id"`
	Classification ChangeClassification `json:"classification"`
	Level          string               `json:"level"`
	Location       string               `json:"location,omitempty"`
}
