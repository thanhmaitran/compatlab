package benchmarks

import (
	"testing"

	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

// The synthetic datasets must parse and must produce real differences,
// otherwise the benchmarks would measure nothing.
func TestSyntheticDatasetsAreValid(t *testing.T) {
	p := specification.NewParser()
	base, err := p.Parse(BuildSpec(30, 0))
	if err != nil {
		t.Fatal(err)
	}
	target, err := p.Parse(BuildSpec(30, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(base.Endpoints) != 30*3 || len(target.Endpoints) >= len(base.Endpoints) {
		t.Errorf("endpoints: base %d, target %d", len(base.Endpoints), len(target.Endpoints))
	}
	analysis, err := compatibility.NewEngine().Compare(base, target)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.SummaryFor(domain.DirectionBackward).Breaking == 0 {
		t.Error("the target variant must contain breaking changes")
	}
	again, _ := p.Parse(BuildSpec(30, 0))
	if again.SourceHash != base.SourceHash {
		t.Error("dataset generation must be deterministic")
	}
}
