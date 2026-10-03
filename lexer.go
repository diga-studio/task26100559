package sentinelgate

// tokenKind 区分词元类型。
type tokenKind int

const (
	tokWord  tokenKind = iota // 普通词：字母、数字、下划线、美元符、非 ASCII 字节的最长连续序列
	tokPunct                  // 其他非引号、非注释字符，各自独立成词元
)

// token 是引号与注释之外的一个词元，携带原始查询串中的证据区间。
type token struct {
	kind tokenKind
	text string // 解码后的词元文本
	from int    // 原始起始偏移（含）
	to   int    // 原始结束偏移（不含）
}

// isSpace 判断 ASCII 空白：空格及字节 9 至 13。
func isSpace(c byte) bool {
	return c == ' ' || (c >= 9 && c <= 13)
}

// isWordByte 判断普通词可包含的字节：
// ASCII 字母、数字、下划线、美元符和非 ASCII 字节。
func isWordByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '_' || c == '$':
		return true
	case c >= 0x80:
		return true
	default:
		return false
	}
}

// lexResult 是词法分析的结果。
type lexResult struct {
	tokens []token
	// execComments 为可执行块注释（/*! ... */）的原始区间，命中第三类规则。
	execComments []token
}

// lex 对一个解码后的参数值做词法分析。
// 检测开始时假定已经位于单引号字符串内部（值被拼进 SQL 的单引号字符串），
// 因此初始状态在单引号字符串中，直到出现未转义的闭引号才进入引号外状态。
// 字符串与注释内容不产生词元；未闭合的字符串或普通块注释屏蔽至末尾。
func lex(in []decodedByte) lexResult {
	var res lexResult
	n := len(in)
	// 初始状态：位于单引号字符串内部。
	i := scanStringBody(in, 0, '\'')
	for i < n {
		c := in[i].b
		switch {
		case isSpace(c):
			i++
		case c == '\'' || c == '"' || c == '`':
			i = scanStringBody(in, i+1, c)
		case c == '#':
			i = skipLineComment(in, i)
		case c == '-' && i+1 < n && in[i+1].b == '-' && (i+2 >= n || isSpace(in[i+2].b)):
			i = skipLineComment(in, i)
		case c == '/' && i+1 < n && in[i+1].b == '*':
			i = skipBlockComment(in, i, &res)
		case isWordByte(c):
			j := i + 1
			for j < n && isWordByte(in[j].b) {
				j++
			}
			text := make([]byte, 0, j-i)
			for k := i; k < j; k++ {
				text = append(text, in[k].b)
			}
			res.tokens = append(res.tokens, token{kind: tokWord, text: string(text), from: in[i].from, to: in[j-1].to})
			i = j
		default:
			res.tokens = append(res.tokens, token{kind: tokPunct, text: string(c), from: in[i].from, to: in[i].to})
			i++
		}
	}
	return res
}

// scanStringBody 从字符串内容位置 j 扫描至闭引号之后，返回对应下标。
// 同类引号连续出现两次表示转义；单、双引号支持反斜杠转义下一个字节，
// 反引号内不支持反斜杠转义。未闭合时屏蔽至末尾。
func scanStringBody(in []decodedByte, j int, quote byte) int {
	n := len(in)
	for j < n {
		c := in[j].b
		if quote != '`' && c == '\\' {
			// 反斜杠转义下一个字节；末尾无下一个字节则屏蔽至末尾。
			j += 2
			continue
		}
		if c == quote {
			if j+1 < n && in[j+1].b == quote {
				j += 2
				continue
			}
			return j + 1
		}
		j++
	}
	return n
}

// skipLineComment 跳过行注释，到 CR 或 LF 结束（CR/LF 本身按空白处理）。
func skipLineComment(in []decodedByte, i int) int {
	n := len(in)
	j := i
	for j < n && in[j].b != '\r' && in[j].b != '\n' {
		j++
	}
	return j
}

// skipBlockComment 跳过块注释。普通块注释等价于空白；
// 以 "/*!" 开头的可执行块注释记录原始区间并命中规则，内部不再检查。
// 未闭合时覆盖到输入末尾。
func skipBlockComment(in []decodedByte, i int, res *lexResult) int {
	n := len(in)
	executable := i+2 < n && in[i+2].b == '!'
	j := i + 2
	closedEnd := -1 // 首个 "*/" 之后（解码数组下标）
	for j+1 < n {
		if in[j].b == '*' && in[j+1].b == '/' {
			closedEnd = j + 2
			break
		}
		j++
	}
	if executable {
		tok := token{kind: tokPunct, from: in[i].from}
		if closedEnd >= 0 {
			tok.to = in[closedEnd-1].to
		} else {
			tok.to = in[n-1].to
		}
		res.execComments = append(res.execComments, tok)
	}
	if closedEnd >= 0 {
		return closedEnd
	}
	return n
}
