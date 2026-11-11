package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ChangeScope says which part of an operation a change affects.
type ChangeScope string

const (
	ScopeEndpoint  ChangeScope = "ENDPOINT"
	ScopeParameter ChangeScope = "PARAMETER"
	ScopeRequest   ChangeScope = "REQUEST"
	ScopeResponse  ChangeScope = "RESPONSE"
	ScopeSecurity  ChangeScope = "SECURITY"
)

// ChangeRecord is one raw difference between two API versions, before it is
// classified by the compatibility rules. Attributes carry machine-readable
// facts (for example required=true) that rules use to decide the impact.
type ChangeRecord struct {
	ID          string            `json:"id"`
	Path        string            `json:"path,omitempty"`
	Method      string            `json:"method,omitempty"`
	ChangeType  ChangeType        `json:"change_type"`
	Scope       ChangeScope       `json:"scope"`
	Location    string            `json:"location,omitempty"`
	OldValue    any               `json:"old_value,omitempty"`
	NewValue    any               `json:"new_value,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Description string            `json:"description"`
}

// NewChangeID derives a stable identifier from the change coordinates so that
// repeated analyses of the same input produce identical IDs.
func NewChangeID(path, method string, t ChangeType, location string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{path, method, string(t), location}, "\x00")))
	return hex.EncodeToString(sum[:8])
}
