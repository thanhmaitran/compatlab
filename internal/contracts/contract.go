// Package contracts parses consumer contracts, verifies them against a
// provider API and computes which consumers a set of changes affects.
package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// Parse reads a CompatLab contract (JSON) and validates its structure.
// Unknown fields are rejected so typos are not silently ignored.
func Parse(data []byte) (*domain.ConsumerContract, error) {
	var c domain.ConsumerContract
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%w: invalid contract: %v", domain.ErrInvalid, err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}
