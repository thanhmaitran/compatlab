// Package scoring computes CompatLab's own 0-100 compatibility indicators.
// The score is not an industry standard. Penalties grow with severity, and any
// breaking change caps the score of its direction so it cannot be averaged away.
package scoring

import "github.com/thanhmaitran/compatlab/internal/domain"

// Caps applied when a direction contains breaking changes.
const (
	capAnyBreaking  = 89
	capHighBreaking = 69
)

func penalty(c domain.ClassifiedChange) int {
	weight := map[domain.Severity]int{
		domain.SeverityInfo: 0, domain.SeverityLow: 2, domain.SeverityMedium: 5,
		domain.SeverityHigh: 10, domain.SeverityCritical: 20,
	}[c.Severity]
	switch c.Classification {
	case domain.Breaking:
		return weight
	case domain.Conditional:
		return weight / 2
	}
	return 0
}

func score(changes []domain.ClassifiedChange, dir domain.CompatibilityDirection) int {
	total, ceiling := 0, 100
	for _, c := range changes {
		if c.Direction != dir {
			continue
		}
		total += penalty(c)
		if c.Classification == domain.Breaking {
			ceiling = min(ceiling, capAnyBreaking)
			if c.Severity.Rank() >= domain.SeverityHigh.Rank() {
				ceiling = min(ceiling, capHighBreaking)
			}
		}
	}
	s := max(100-total, 0)
	return min(s, ceiling)
}

// Compute returns the backward and forward scores for classified changes.
func Compute(changes []domain.ClassifiedChange) domain.CompatibilityScores {
	return domain.CompatibilityScores{
		Backward: score(changes, domain.DirectionBackward),
		Forward:  score(changes, domain.DirectionForward),
	}
}
