package policies

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

type policyYAML struct {
	Name                 string   `yaml:"name"`
	FailOn               []string `yaml:"fail_on"`
	RequireContractTests bool     `yaml:"require_contract_tests"`
	MinimumPassRate      float64  `yaml:"minimum_pass_rate"`
	IncludeForward       bool     `yaml:"include_forward"`
}

type fileYAML struct {
	Policy    *policyYAML `yaml:"policy"`
	Compatlab *struct {
		Policy *policyYAML `yaml:"policy"`
	} `yaml:"compatlab"`
}

// Parse reads a policy-as-code YAML document. Both a top-level `policy:` key
// and the `compatlab: policy:` form are accepted.
func Parse(data []byte) (*domain.ReleasePolicy, error) {
	var f fileYAML
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%w: invalid policy YAML: %v", domain.ErrInvalid, err)
	}
	p := f.Policy
	if p == nil && f.Compatlab != nil {
		p = f.Compatlab.Policy
	}
	if p == nil {
		return nil, fmt.Errorf("%w: no policy section found", domain.ErrInvalid)
	}
	out := &domain.ReleasePolicy{
		Name:                 p.Name,
		RequireContractTests: p.RequireContractTests,
		MinimumPassRate:      p.MinimumPassRate,
		IncludeForward:       p.IncludeForward,
	}
	for _, c := range p.FailOn {
		out.FailOn = append(out.FailOn, domain.ChangeClassification(strings.ToUpper(strings.TrimSpace(c))))
	}
	if err := out.Validate(); err != nil {
		return nil, err
	}
	return out, nil
}

// LoadFile reads and parses a policy file.
func LoadFile(path string) (*domain.ReleasePolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	p, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}
