// Package specification parses OpenAPI 3.x documents (JSON or YAML) into the
// normalized domain model. External references are never followed.
package specification

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// DefaultMaxBytes is the default upper bound for an uploaded document.
const DefaultMaxBytes = 5 * 1024 * 1024

// Parser implements domain.SpecificationParser.
type Parser struct {
	MaxBytes int
}

var _ domain.SpecificationParser = (*Parser)(nil)

// NewParser returns a parser with default limits.
func NewParser() *Parser { return &Parser{MaxBytes: DefaultMaxBytes} }

// Parse decodes, validates, extracts and normalizes an OpenAPI document.
func (p *Parser) Parse(data []byte) (*domain.NormalizedAPI, error) {
	if p.MaxBytes > 0 && len(data) > p.MaxBytes {
		return nil, fmt.Errorf("%w: %d bytes exceeds the limit of %d", ErrTooLarge, len(data), p.MaxBytes)
	}
	root, err := decode(data)
	if err != nil {
		return nil, err
	}
	if err := validate(root); err != nil {
		return nil, err
	}
	api, err := extract(root)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	api.SourceHash = hex.EncodeToString(sum[:])
	api.Normalize()
	return api, nil
}
