package benchmarks

import (
	"fmt"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/compatibility/analyzer"
	"github.com/thanhmaitran/compatlab/internal/compatibility/classifier"
	"github.com/thanhmaitran/compatlab/internal/compatibility/rules"
)

func BenchmarkClassify(b *testing.B) {
	for _, n := range sizes {
		base, target := parsePair(b, n)
		changes := analyzer.Diff(base, target)
		set := rules.Default()
		b.Run(fmt.Sprintf("resources=%d/changes=%d", n, len(changes)), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = classifier.Classify(changes, set)
			}
		})
	}
}
