package benchmarks

import (
	"fmt"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/reporting"
)

func BenchmarkReport(b *testing.B) {
	for _, n := range sizes {
		base, target := parsePair(b, n)
		analysis, err := compatibility.NewEngine().Compare(base, target)
		if err != nil {
			b.Fatal(err)
		}
		report := &domain.CompatibilityReport{Analysis: analysis}
		b.Run(fmt.Sprintf("text/resources=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = reporting.Text(report)
			}
		})
		b.Run(fmt.Sprintf("json/resources=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := reporting.JSON(report); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
