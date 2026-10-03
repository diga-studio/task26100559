// Package sentinelgate implements the SQL injection detection kernel for
// SentinelGate. It inspects raw URL query strings whose values are
// concatenated into single-quoted SQL string literals by legacy code, and
// reports risk hits with evidence positions in the original query string.
//
// All rules and working data live in memory only; the library is intended
// to be called directly by an upper layer and is safe for single-threaded
// use.
package sentinelgate

// RuleClass identifies the category of a detected risk.
type RuleClass int

const (
	// RuleUnionSelect matches adjacent tokens UNION SELECT or
	// UNION ALL SELECT outside of quotes.
	RuleUnionSelect RuleClass = iota + 1
	// RuleOrEquals matches OR <int> = <int> with equal numeric values.
	RuleOrEquals
	// RuleExecComment matches an executable block comment "/*! ... */"
	// outside of quotes.
	RuleExecComment
)

// String returns a human-readable name for the rule class.
func (r RuleClass) String() string {
	switch r {
	case RuleUnionSelect:
		return "union-select"
	case RuleOrEquals:
		return "or-equals"
	case RuleExecComment:
		return "exec-comment"
	default:
		return "unknown"
	}
}

// Hit describes a single risk hit.
type Hit struct {
	// Rule is the matched rule category.
	Rule RuleClass
	// ParamIndex is the 0-based occurrence index of the parameter in the
	// query string (same-name parameters are checked per occurrence).
	ParamIndex int
	// Start and End are the byte range in the original query string,
	// 0-based, half-open [Start, End).
	Start int
	End   int
}

// Check inspects a raw URL query string (without the leading '?') and
// returns all risk hits. It never mutates the input.
func Check(query string) []Hit {
	return check(query)
}
