package sentinelgate

// eqFoldASCII reports whether s equals the lowercase ASCII keyword kw,
// ignoring ASCII case only.
func eqFoldASCII(s, kw string) bool {
	if len(s) != len(kw) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != kw[i] {
			return false
		}
	}
	return true
}

// isKeyword reports whether tok is a word token matching kw (ASCII
// case-insensitive).
func isKeyword(tok token, kw string) bool {
	return tok.kind == tokWord && eqFoldASCII(tok.text, kw)
}

// intEqual compares two unsigned decimal integers, ignoring leading
// zeros. Values are not limited by machine integer width.
func intEqual(a, b string) bool {
	a = trimZeros(a)
	b = trimZeros(b)
	if len(a) != len(b) {
		return false
	}
	return a == b
}

func trimZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}

// matchRules applies rules 1 and 2 to the token stream of a single
// parameter value and appends hits. Rule 3 hits come from the lexer.
// Hits never span parameters because matching is per token stream, and
// duplicate hits at the same location are suppressed.
func matchRules(paramIndex int, toks []token, hits *[]Hit) {
	seen := make(map[[3]int]struct{})
	emit := func(rule RuleClass, first, last token) {
		key := [3]int{int(rule), first.rawStart, last.rawEnd}
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		*hits = append(*hits, Hit{
			Rule:       rule,
			ParamIndex: paramIndex,
			Start:      first.rawStart,
			End:        last.rawEnd,
		})
	}

	for i := 0; i < len(toks); i++ {
		// Rule 1: UNION SELECT or UNION ALL SELECT.
		if isKeyword(toks[i], "union") {
			if i+1 < len(toks) && isKeyword(toks[i+1], "select") {
				emit(RuleUnionSelect, toks[i], toks[i+1])
			}
			if i+2 < len(toks) && isKeyword(toks[i+1], "all") &&
				isKeyword(toks[i+2], "select") {
				emit(RuleUnionSelect, toks[i], toks[i+2])
			}
		}
		// Rule 2: OR <int> = <int> with equal numeric values.
		if isKeyword(toks[i], "or") && i+3 < len(toks) &&
			toks[i+1].kind == tokInt &&
			toks[i+2].kind == tokPunct && toks[i+2].text == "=" &&
			toks[i+3].kind == tokInt &&
			intEqual(toks[i+1].text, toks[i+3].text) {
			emit(RuleOrEquals, toks[i], toks[i+3])
		}
	}
}
