package sentinelgate

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// buildAdversarialQuery builds a ~1MiB query string packed with
// double-encoded quotes, comment fragments, huge leading-zero integers,
// executable comments and unterminated tails.
func buildAdversarialQuery() string {
	var b strings.Builder
	b.Grow(1 << 20)
	i := 0
	for b.Len() < (1<<20)-256 {
		if i > 0 {
			b.WriteByte('&')
		}
		switch i % 5 {
		case 0:
			// Double-encoded quote + keyword pair split by a comment.
			b.WriteString("u=%2527%2520UNION/**/SELECT")
		case 1:
			// Huge leading-zero integer comparison.
			b.WriteString("n=%27+OR+")
			b.WriteString(strings.Repeat("0", 200))
			b.WriteString("1%3D1")
		case 2:
			// Executable comment with encoded interior.
			b.WriteString("e=%27/*!40000+UNION+SELECT*/")
		case 3:
			// Escapes, line comments and invalid percent sequences.
			b.WriteString("s=%27%2G%\\%27--%20tail%0A%23hash%0D")
		default:
			// Unterminated block comment masking the tail.
			b.WriteString("t=%27+OR+1=1/*")
		}
		i++
	}
	return b.String()
}

func TestCheckAdversarial1MiB(t *testing.T) {
	q := buildAdversarialQuery()
	if len(q) < (1<<20)-512 {
		t.Fatalf("query too small: %d", len(q))
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	hits := Check(q)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	alloc := after.TotalAlloc - before.TotalAlloc
	t.Logf("input=%d bytes hits=%d elapsed=%v allocated=%d bytes",
		len(q), len(hits), elapsed, alloc)
	if elapsed > 2*time.Second {
		t.Fatalf("too slow: %v", elapsed)
	}
	const maxMem = 128 << 20
	if alloc > maxMem {
		t.Fatalf("too much memory: %d > %d", alloc, maxMem)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits in adversarial input")
	}
	for _, h := range hits {
		if h.Start < 0 || h.End > len(q) || h.Start >= h.End {
			t.Fatalf("bad span %+v", h)
		}
	}
}

func BenchmarkCheckAdversarial1MiB(b *testing.B) {
	q := buildAdversarialQuery()
	b.SetBytes(int64(len(q)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Check(q)
	}
}
