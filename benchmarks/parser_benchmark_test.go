package benchmarks

import (
	"fmt"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/specification"
)

var sizes = []int{10, 100, 500}

func BenchmarkParse(b *testing.B) {
	for _, n := range sizes {
		data := BuildSpec(n, 0)
		b.Run(fmt.Sprintf("resources=%d", n), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := specification.NewParser().Parse(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
