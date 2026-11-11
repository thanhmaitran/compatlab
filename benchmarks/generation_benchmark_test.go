package benchmarks

import (
	"context"
	"fmt"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/execution"
	"github.com/thanhmaitran/compatlab/internal/generation"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

func BenchmarkGenerate(b *testing.B) {
	for _, n := range sizes {
		api, err := specification.NewParser().Parse(BuildSpec(n, 0))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("resources=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := (generation.Generator{}).Generate(api, domain.GenerationConfig{MaxCasesPerEndpoint: 50}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkExecuteOnMock measures end-to-end execution throughput (requests
// per second is cases/op divided by time/op) against the in-process mock.
func BenchmarkExecuteOnMock(b *testing.B) {
	api, err := specification.NewParser().Parse(BuildSpec(10, 0))
	if err != nil {
		b.Fatal(err)
	}
	suite, err := (generation.Generator{}).Generate(api, domain.GenerationConfig{MaxCasesPerEndpoint: 20})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ReportMetric(float64(len(suite.Cases)), "cases/op")
	for i := 0; i < b.N; i++ {
		run, err := execution.RunOnMock(context.Background(), api, suite)
		if err != nil {
			b.Fatal(err)
		}
		if run.PassRate() != 100 {
			b.Fatalf("pass rate %.1f", run.PassRate())
		}
	}
}
