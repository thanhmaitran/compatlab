package benchmarks

import (
	"fmt"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/compatibility/analyzer"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

func parsePair(tb testing.TB, n int) (*domain.NormalizedAPI, *domain.NormalizedAPI) {
	tb.Helper()
	p := specification.NewParser()
	base, err := p.Parse(BuildSpec(n, 0))
	if err != nil {
		tb.Fatal(err)
	}
	target, err := p.Parse(BuildSpec(n, 1))
	if err != nil {
		tb.Fatal(err)
	}
	return base, target
}

func BenchmarkDiff(b *testing.B) {
	for _, n := range sizes {
		base, target := parsePair(b, n)
		b.Run(fmt.Sprintf("resources=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = analyzer.Diff(base, target)
			}
		})
	}
}

// BenchmarkAnalyze measures the full engine: diff, classification and scoring.
func BenchmarkAnalyze(b *testing.B) {
	for _, n := range sizes {
		base, target := parsePair(b, n)
		engine := compatibility.NewEngine()
		b.Run(fmt.Sprintf("resources=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := engine.Compare(base, target); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
