package sentinelgate

import (
	"strings"
	"testing"
)

// check 是测试辅助：调用 Check 并断言无错误。
func check(t *testing.T, query string) []Hit {
	t.Helper()
	hits, err := Check(query)
	if err != nil {
		t.Fatalf("Check(%q) 返回错误: %v", query, err)
	}
	return hits
}

// ev 由证据子串推导期望命中：在 query 中从 from 起查找 evidence，
// 命中区间即该子串的原始字节区间。
func ev(query string, from int, rule RuleKind, paramIndex int, evidence string) Hit {
	start := strings.Index(query[from:], evidence)
	if start < 0 {
		panic("测试数据错误：证据子串不存在")
	}
	start += from
	return Hit{Rule: rule, ParamIndex: paramIndex, Start: start, End: start + len(evidence)}
}

// wantHit 断言命中列表与期望完全一致，且证据区间合法。
func wantHit(t *testing.T, query string, hits []Hit, want []Hit) {
	t.Helper()
	if len(hits) != len(want) {
		t.Fatalf("Check(%q) 命中数 = %d (%v), 期望 %d (%v)", query, len(hits), hits, len(want), want)
	}
	for i := range want {
		if hits[i] != want[i] {
			t.Errorf("Check(%q) 命中[%d] = %+v, 期望 %+v", query, i, hits[i], want[i])
		}
		if hits[i].Start < 0 || hits[i].End > len(query) || hits[i].Start >= hits[i].End {
			t.Errorf("Check(%q) 命中[%d] 区间非法: [%d,%d)", query, i, hits[i].Start, hits[i].End)
		}
	}
}

func wantNoHit(t *testing.T, query string) {
	t.Helper()
	if hits := check(t, query); len(hits) != 0 {
		t.Errorf("Check(%q) 命中 %v, 期望无命中", query, hits)
	}
}

// TestDoubleEncodedKeywordWithComment 双层编码闭引号后穿插注释的关键字仍命中，
// 证据准确回指原始编码片段（包含中间的注释）。
func TestDoubleEncodedKeywordWithComment(t *testing.T) {
	q := "a=%2527%20UNION/**/SELECT"
	wantNoHit(t, "a=UNION/**/SELECT") // 未闭引号，仍在字符串内
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleUnionSelect, 0, "UNION/**/SELECT")})
}

// TestDoubleEncodedKeywordLetter 关键字本身被双层编码时仍命中并回指编码片段。
func TestDoubleEncodedKeywordLetter(t *testing.T) {
	// %2555 -> %55 -> 'U'
	q := "a=%2527%20%2555NION%20SELECT%201"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleUnionSelect, 0, "%2555NION%20SELECT")})
}

// TestUnionAllSelect 覆盖 UNION ALL SELECT 形式及大小写混合。
func TestUnionAllSelect(t *testing.T) {
	q := "a='%20UnIoN%20ALL%20SeLeCt%201"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleUnionSelect, 0, "UnIoN%20ALL%20SeLeCt")})
}

// TestEncodedSeparatorsStayInParam 解码产生的 & 和 = 不得重新划分参数。
func TestEncodedSeparatorsStayInParam(t *testing.T) {
	// %26 -> '&'，%3D -> '='；整段仍属参数 a（序号 0）。
	q := "a=%2527%20UNION%20SELECT%201%26x%3D1%20OR%202%3D2"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{
		ev(q, 0, RuleUnionSelect, 0, "UNION%20SELECT"),
		ev(q, 0, RuleOrEquals, 0, "OR%202%3D2"),
	})
	// 真实的 & 才划分参数：第二个参数序号为 1。
	q2 := "z=1&a=%2527%20UNION%20SELECT%201"
	hits2 := check(t, q2)
	wantHit(t, q2, hits2, []Hit{ev(q2, 0, RuleUnionSelect, 1, "UNION%20SELECT")})
}

// TestPairedQuotesKeepStringState 成对引号（转义）保持字符串状态。
func TestPairedQuotesKeepStringState(t *testing.T) {
	// 值以 '' 开头：转义引号，仍处于初始字符串内，UNION SELECT 不命中。
	wantNoHit(t, "a=''%20UNION%20SELECT%201")
	// 初始字符串闭合后又开新字符串，UNION SELECT 在新字符串内，不命中。
	wantNoHit(t, "a=x'%20'%20UNION%20SELECT%20'")
	// 双引号字符串内的内容不命中。
	wantNoHit(t, "a='%22UNION%20SELECT%22")
	// 反斜杠转义闭引号：仍在字符串内，不命中。
	wantNoHit(t, "a=\\'%20UNION%20SELECT")
	// 反引号内不支持反斜杠转义：\ 不转义闭反引号，UNION SELECT 在引号外命中。
	q := "a='%60x%5C%60%20UNION%20SELECT"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleUnionSelect, 0, "UNION%20SELECT")})
}

// TestPlainCommentCannotJoinKeyword 普通注释等价于空白，不能拼合关键字。
func TestPlainCommentCannotJoinKeyword(t *testing.T) {
	wantNoHit(t, "a='%20UN/**/ION%20SELECT")
	wantNoHit(t, "a='%20UNI#x%0AON%20SELECT")
	// 但注释可出现在完整关键字的相邻词元之间。
	q := "a='%20UNION/**/%20SELECT"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleUnionSelect, 0, "UNION/**/%20SELECT")})
}

// TestOrIntegerEquals OR、整数、=、整数，数值相等（前导零不影响）。
func TestOrIntegerEquals(t *testing.T) {
	q := "a='%20OR%20007%3D7"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleOrEquals, 0, "OR%20007%3D7")})
	wantNoHit(t, "a='%20OR%201%3D2")
	wantNoHit(t, "a='%20AND%201%3D1")
	// 注释与空白可出现在相邻词元之间。
	q2 := "a='%20OR/**/1%09=%0A1"
	hits2 := check(t, q2)
	wantHit(t, q2, hits2, []Hit{ev(q2, 0, RuleOrEquals, 0, "OR/**/1%09=%0A1")})
}

// TestOrIntegerEqualsLongZeros 超长前导零整数比较，不受机器位宽限制。
func TestOrIntegerEqualsLongZeros(t *testing.T) {
	zeros := strings.Repeat("0", 500)
	q := "a='%20OR%20" + zeros + "1%3D1"
	hits := check(t, q)
	if len(hits) != 1 || hits[0].Rule != RuleOrEquals {
		t.Fatalf("Check 命中 = %v, 期望 1 条 or_equals", hits)
	}
	// 两个 600 位整数相等（仅前导零数量不同）。
	big := "1" + strings.Repeat("7", 599)
	q2 := "a='%20OR%20" + big + "%3D000" + big
	if hits2 := check(t, q2); len(hits2) != 1 || hits2[0].Rule != RuleOrEquals {
		t.Fatalf("Check 命中 = %v, 期望 1 条 or_equals", hits2)
	}
	// 末位不同则不命中。
	big2 := "1" + strings.Repeat("7", 598) + "8"
	wantNoHit(t, "a='%20OR%20"+big+"%3D"+big2)
}

// TestExecComment 可执行块注释无条件命中整个块，不检查内部。
func TestExecComment(t *testing.T) {
	q := "a='%20/*!50000%20SELECT*/%20x"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleExecComment, 0, "/*!50000%20SELECT*/")})
	// 普通块注释不命中。
	wantNoHit(t, "a='%20/*%20SELECT%20*/")
	// 可执行注释内部不再检查：其中的 UNION SELECT 不另报。
	q2 := "a='%20/*!%20UNION%20SELECT*/"
	hits2 := check(t, q2)
	wantHit(t, q2, hits2, []Hit{ev(q2, 0, RuleExecComment, 0, "/*!%20UNION%20SELECT*/")})
}

// TestUnclosedFragments 未闭合字符串/注释屏蔽至末尾，但不撤销此前命中。
func TestUnclosedFragments(t *testing.T) {
	// 未闭合可执行注释覆盖到输入末尾。
	q := "a='%20/*!%20UNION"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{{Rule: RuleExecComment, ParamIndex: 0, Start: strings.Index(q, "/*!"), End: len(q)}})
	// 未闭合普通块注释屏蔽其余内容：SELECT 被吞，无 UNION SELECT 命中。
	wantNoHit(t, "a='%20UNION/*%20SELECT")
	// 未闭合字符串屏蔽至末尾，但此前的命中保留。
	q3 := "a='%20UNION%20SELECT%20'"
	hits3 := check(t, q3)
	wantHit(t, q3, hits3, []Hit{ev(q3, 0, RuleUnionSelect, 0, "UNION%20SELECT")})
	// 未闭合行注释同理：此前命中保留。
	q4 := "a='%20UNION%20SELECT%20--%20"
	hits4 := check(t, q4)
	wantHit(t, q4, hits4, []Hit{ev(q4, 0, RuleUnionSelect, 0, "UNION%20SELECT")})
}

// TestLineComments 行注释的开启与结束。
func TestLineComments(t *testing.T) {
	// # 行注释到 LF 结束，之后词元恢复。
	q := "a='%20UNION%23x%0ASELECT"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleUnionSelect, 0, "UNION%23x%0ASELECT")})
	// -- 后随空白开启行注释；-- 后非空白则不是注释。
	wantNoHit(t, "a='%20UNION--%20SELECT")
	q2 := "a='%20UNION--%20%0ASELECT"
	hits2 := check(t, q2)
	wantHit(t, q2, hits2, []Hit{ev(q2, 0, RuleUnionSelect, 0, "UNION--%20%0ASELECT")})
}

// TestNoDuplicateAndNoCrossParam 同一处命中不重复，不跨参数命中。
func TestNoDuplicateAndNoCrossParam(t *testing.T) {
	// UNION UNION SELECT：只报一次（第二处 UNION 与 SELECT）。
	q := "a='%20UNION%20UNION%20SELECT"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 6, RuleUnionSelect, 0, "UNION%20SELECT")})
	// UNION 在参数 a 末尾，SELECT 在参数 b 开头：不得跨参数命中。
	wantNoHit(t, "a='%20UNION&b=%20SELECT")
	// 两组 OR 1=1 各报一次。
	q3 := "a='%20OR%201%3D1%20OR%202%3D2"
	hits3 := check(t, q3)
	wantHit(t, q3, hits3, []Hit{
		ev(q3, 0, RuleOrEquals, 0, "OR%201%3D1"),
		ev(q3, 0, RuleOrEquals, 0, "OR%202%3D2"),
	})
}

// TestMiscParsing 参数划分、空值、无等号、大小写、加号与第二轮不解加号。
func TestMiscParsing(t *testing.T) {
	// 无等号视为空值；空值不产生命中。
	wantNoHit(t, "UNION&SELECT")
	wantNoHit(t, "")
	// 首轮 '+' 变空格；%27 -> ' 闭引号。
	q := "a=x%27+UNION+SELECT+1"
	hits := check(t, q)
	wantHit(t, q, hits, []Hit{ev(q, 0, RuleUnionSelect, 0, "UNION+SELECT")})
	// 第二轮不转换 '+'：%252B -> '+' 而非空格，构成独立标点词元，阻断相邻。
	wantNoHit(t, "a='%27UNION%252BSELECT")
	// 大小写不敏感的关键字 OR。
	q3 := "a='%20oR%201%3D1"
	hits3 := check(t, q3)
	wantHit(t, q3, hits3, []Hit{ev(q3, 0, RuleOrEquals, 0, "oR%201%3D1")})
}

// TestQueryTooLarge 超过 1MiB 的查询串返回错误。
func TestQueryTooLarge(t *testing.T) {
	_, err := Check(strings.Repeat("a", maxQueryLen+1))
	if err == nil {
		t.Fatal("期望 ErrQueryTooLarge")
	}
	if _, ok := err.(*ErrQueryTooLarge); !ok {
		t.Fatalf("错误类型 = %T, 期望 *ErrQueryTooLarge", err)
	}
	if _, err := Check(strings.Repeat("a", maxQueryLen)); err != nil {
		t.Fatalf("1MiB 整应可检查, 得到错误 %v", err)
	}
}
