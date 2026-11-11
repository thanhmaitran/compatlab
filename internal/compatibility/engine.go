// Package compatibility wires the analyzer, rule registry, classifier and
// scoring into one engine that implements domain.CompatibilityAnalyzer.
package compatibility

import (
	"fmt"

	"github.com/thanhmaitran/compatlab/internal/compatibility/analyzer"
	"github.com/thanhmaitran/compatlab/internal/compatibility/classifier"
	"github.com/thanhmaitran/compatlab/internal/compatibility/rules"
	"github.com/thanhmaitran/compatlab/internal/compatibility/scoring"
	"github.com/thanhmaitran/compatlab/internal/domain"
)

// Engine compares two normalized APIs and classifies every difference.
type Engine struct {
	Rules []rules.Rule
}

var _ domain.CompatibilityAnalyzer = (*Engine)(nil)

// NewEngine returns an engine with the built-in rule set.
func NewEngine() *Engine { return &Engine{Rules: rules.Default()} }

// Compare analyses old against new. Equal inputs always produce equal output.
func (e *Engine) Compare(oldAPI, newAPI *domain.NormalizedAPI) (*domain.CompatibilityAnalysis, error) {
	if oldAPI == nil || newAPI == nil {
		return nil, fmt.Errorf("%w: both API versions are required", domain.ErrInvalid)
	}
	oldAPI.Normalize()
	newAPI.Normalize()
	classified := classifier.Classify(analyzer.Diff(oldAPI, newAPI), e.Rules)
	scores := scoring.Compute(classified)
	return &domain.CompatibilityAnalysis{
		BaseVersion:   oldAPI.Version,
		TargetVersion: newAPI.Version,
		Changes:       classified,
		Scores:        &scores,
	}, nil
}
