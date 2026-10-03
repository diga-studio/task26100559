// Package sentinelgate 实现 SQL 注入检测内核。
// 输入原始 URL 查询串（不含开头的问号），输出风险命中及证据位置。
// 规则与工作数据仅保存在内存中，供上层以库形式直接调用。
package sentinelgate

// RuleKind 表示命中的规则类别。
type RuleKind int

const (
	// RuleUnionSelect 相邻词元构成 "UNION SELECT" 或 "UNION ALL SELECT"。
	RuleUnionSelect RuleKind = iota + 1
	// RuleOrEquals 词元序列 OR、整数、=、整数，且两整数数值相等。
	RuleOrEquals
	// RuleExecComment 引号外以 "/*!" 开头的可执行块注释。
	RuleExecComment
)

// String 返回规则类别的可读名称。
func (k RuleKind) String() string {
	switch k {
	case RuleUnionSelect:
		return "union_select"
	case RuleOrEquals:
		return "or_equals"
	case RuleExecComment:
		return "exec_comment"
	default:
		return "unknown"
	}
}

// Hit 表示一次风险命中。
type Hit struct {
	// Rule 为规则类别。
	Rule RuleKind
	// ParamIndex 为参数在查询串中的出现序号，从 0 计数（同名参数逐项独立编号）。
	ParamIndex int
	// Start 为证据在原始查询串中的起始字节偏移（含），从 0 计数。
	Start int
	// End 为证据在原始查询串中的结束字节偏移（不含）。
	End int
}

// maxQueryLen 限定单个查询串最大 1MiB。
const maxQueryLen = 1 << 20

// ErrQueryTooLarge 在查询串超过 1MiB 时返回。
type ErrQueryTooLarge struct {
	Len int
}

func (e *ErrQueryTooLarge) Error() string {
	return "sentinelgate: query string exceeds 1MiB limit"
}

// Check 检查原始查询串（不含开头的问号），返回全部风险命中。
// 命中按参数出现顺序与参数内原始位置排序；同一处命中不重复报告，
// 不跨参数命中。查询串超过 1MiB 时返回 ErrQueryTooLarge。
func Check(query string) ([]Hit, error) {
	if len(query) > maxQueryLen {
		return nil, &ErrQueryTooLarge{Len: len(query)}
	}
	var hits []Hit
	for _, p := range splitParams(query) {
		hits = append(hits, detectParam(p)...)
	}
	return hits, nil
}
