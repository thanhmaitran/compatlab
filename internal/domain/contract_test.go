package domain

import (
	"errors"
	"testing"
)

func validContract() *ConsumerContract {
	return &ConsumerContract{
		ContractVersion: "1.0", Consumer: "mobile-app", Provider: "user-service",
		Interactions: []ContractInteraction{{
			Description: "Retrieve user",
			Request:     ContractRequest{Method: "GET", Path: "/users/123"},
			Response:    ContractResponse{Status: 200},
		}},
	}
}

func TestContractValidate(t *testing.T) {
	if err := validContract().Validate(); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	mutations := map[string]func(*ConsumerContract){
		"no version":      func(c *ConsumerContract) { c.ContractVersion = "" },
		"no consumer":     func(c *ConsumerContract) { c.Consumer = "" },
		"no provider":     func(c *ConsumerContract) { c.Provider = "" },
		"no interactions": func(c *ConsumerContract) { c.Interactions = nil },
		"no path":         func(c *ConsumerContract) { c.Interactions[0].Request.Path = "" },
		"bad status":      func(c *ConsumerContract) { c.Interactions[0].Response.Status = 42 },
	}
	for name, mutate := range mutations {
		c := validContract()
		mutate(c)
		if err := c.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: expected ErrInvalid, got %v", name, err)
		}
	}
}

func TestContractVerificationPassed(t *testing.T) {
	v := &ContractVerification{Interactions: []InteractionResult{{Passed: true}, {Passed: true}}}
	if !v.Passed() {
		t.Error("all passed must report Passed")
	}
	v.Interactions = append(v.Interactions, InteractionResult{Passed: false})
	if v.Passed() {
		t.Error("one failure must report not passed")
	}
}
