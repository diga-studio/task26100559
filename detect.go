package sentinelgate

// eqASCII 按 ASCII 忽略大小写比较关键字。
func eqASCII(s, kw string) bool {
	if len(s) != len(kw) {
		return false
	}
	for i := 0; i < len(s); i++ {
		a, b := s[i], kw[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

// isInteger 判断词元是否全部由 ASCII 数字组成（无符号十进制整数）。
func isInteger(t token) bool {
	if t.kind != tokWord || len(t.text) == 0 {
		return false
	}
	for i := 0; i < len(t.text); i++ {
		if t.text[i] < '0' || t.text[i] > '9' {
			return false
		}
	}
	return true
}

// intEqual 比较两个无符号十进制整数的数值，前导零不影响比较，
// 整数不受机器整数位宽限制（按十进制字符串比较）。
func intEqual(a, b string) bool {
	a = stripZeros(a)
	b = stripZeros(b)
	if len(a) != len(b) {
		return false
	}
	return a == b
}

// stripZeros 去掉十进制数字串的前导零，全零归一为 "0"。
func stripZeros(s string) string {
	i := 0
	for i < len(s) && s[i] == '0' {
		i++
	}
	if i == len(s) {
		return "0"
	}
	return s[i:]
}

// detectTokens 在词元序列上匹配第一、二类规则，返回命中。
// 命中后跳过已匹配词元，同一处命中不重复报告。
func detectTokens(tokens []token, paramIndex int) []Hit {
	var hits []Hit
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		if t.kind == tokWord && eqASCII(t.text, "union") {
			// UNION SELECT 或 UNION ALL SELECT
			if i+1 < len(tokens) && tokens[i+1].kind == tokWord && eqASCII(tokens[i+1].text, "select") {
				hits = append(hits, Hit{Rule: RuleUnionSelect, ParamIndex: paramIndex, Start: t.from, End: tokens[i+1].to})
				i += 2
				continue
			}
			if i+2 < len(tokens) && tokens[i+1].kind == tokWord && eqASCII(tokens[i+1].text, "all") &&
				tokens[i+2].kind == tokWord && eqASCII(tokens[i+2].text, "select") {
				hits = append(hits, Hit{Rule: RuleUnionSelect, ParamIndex: paramIndex, Start: t.from, End: tokens[i+2].to})
				i += 3
				continue
			}
		}
		if t.kind == tokWord && eqASCII(t.text, "or") && i+3 < len(tokens) {
			if isInteger(tokens[i+1]) && tokens[i+2].kind == tokPunct && tokens[i+2].text == "=" &&
				isInteger(tokens[i+3]) && intEqual(tokens[i+1].text, tokens[i+3].text) {
				hits = append(hits, Hit{Rule: RuleOrEquals, ParamIndex: paramIndex, Start: t.from, End: tokens[i+3].to})
				i += 4
				continue
			}
		}
		i++
	}
	return hits
}

// detectParam 检查单个参数值，返回该参数内的全部命中（按原始位置排序）。
func detectParam(p param) []Hit {
	decoded := decodeValue(p.value, p.base)
	if len(decoded) == 0 {
		return nil
	}
	res := lex(decoded)
	hits := detectTokens(res.tokens, p.index)
	for _, ec := range res.execComments {
		hits = append(hits, Hit{Rule: RuleExecComment, ParamIndex: p.index, Start: ec.from, End: ec.to})
	}
	// 按证据起始位置归并排序（命中数量少，插入排序即可）。
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].Start < hits[j-1].Start; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	return hits
}
