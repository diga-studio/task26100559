package sentinelgate

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// buildAdversarial 构造约 1MiB 的对抗查询串：
// 大量双层编码、转义引号、普通/可执行注释、超长前导零整数与行注释片段。
func buildAdversarial() string {
	frag := "%2527%20OR%20" + strings.Repeat("0", 120) + "1%3D1%20" +
		"/*pad*/%20/*!9*/%20UNION/**/SELECT%20" +
		"%23x%0A--%20%0A" + "\\\\%2527%2527"
	var b strings.Builder
	b.WriteString("a=")
	for b.Len() < maxQueryLen-len(frag)-1 {
		b.WriteString(frag)
	}
	return b.String()
}

// TestAdversarialPerf 1MiB 对抗输入：一次检查应在 2 秒内完成，
// 且除输入与返回结果外的额外内存不超过 128MiB。
func TestAdversarialPerf(t *testing.T) {
	q := buildAdversarial()
	if len(q) > maxQueryLen {
		t.Fatalf("对抗输入超长: %d", len(q))
	}
	t.Logf("对抗输入大小: %d 字节", len(q))

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	start := time.Now()
	hits, err := Check(q)
	elapsed := time.Since(start)

	runtime.ReadMemStats(&after)
	alloc := after.TotalAlloc - before.TotalAlloc

	if err != nil {
		t.Fatalf("Check 返回错误: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("对抗输入应产生命中")
	}
	t.Logf("命中数: %d, 耗时: %v, 分配内存: %d 字节 (%.1f MiB)",
		len(hits), elapsed, alloc, float64(alloc)/(1<<20))

	if elapsed > 2*time.Second {
		t.Errorf("耗时 %v 超过 2 秒上限", elapsed)
	}
	if alloc > 128*(1<<20) {
		t.Errorf("额外内存 %d 字节超过 128MiB 上限", alloc)
	}
}

// BenchmarkCheck 供手动压测使用。
func BenchmarkCheck(b *testing.B) {
	q := buildAdversarial()
	b.SetBytes(int64(len(q)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Check(q); err != nil {
			b.Fatal(err)
		}
	}
}
