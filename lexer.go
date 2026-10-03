package sentinelgate

// tokenKind classifies lexer tokens.
type tokenKind int

const (
	tokWord  tokenKind = iota // letters, digits, '_', '$', non-ASCII bytes
	tokInt                    // a word consisting only of ASCII digits
	tokPunct                  // any other single non-quote, non-comment byte
)

// token is a lexical unit of a decoded parameter value.
type token struct {
	kind     tokenKind
	text     string // decoded token bytes
	rawStart int    // half-open byte range in the raw query string
	rawEnd   int
}

// lexResult is the outcome of lexing one decoded parameter value.
type lexResult struct {
	tokens []token
	// execComments holds raw ranges of executable comments "/*! ... */"
	// found outside quotes (rule 3), already resolved to raw offsets.
	execComments [][2]int
}

// isSpace reports whether c is ASCII whitespace: space or bytes 9-13.
func isSpace(c byte) bool {
	return c == ' ' || (c >= 9 && c <= 13)
}

// isWordByte reports whether c may appear in a word token: ASCII letters,
// digits, '_', '$' and non-ASCII bytes.
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

// lex tokenizes a decoded parameter value. Scanning starts inside a
// single-quoted string. Quoted strings and backtick identifiers never
// produce tokens and never glue tokens across themselves; ordinary
// comments act as whitespace; executable comments are recorded and their
// contents are not scanned. An unterminated string or ordinary block
// comment masks the rest of the value.
func lex(decoded []byte, rm rawMap) lexResult {
	var res lexResult
	n := len(decoded)
	i := 0

	// rawSpan resolves the raw query string range covering decoded
	// bytes [a, b) (a <= b, b >= 1).
	rawSpan := func(a, b int) (int, int) {
		return rm.Start[a], rm.End[b-1]
	}

	// insideString consumes the rest of a quoted string, given that
	// scanning is currently inside it at decoded[i]. kind is '\'', '"'
	// or '`'. Returns the index just past the closing quote (or n if
	// unterminated).
	insideString := func(i int, kind byte) int {
		for i < n {
			c := decoded[i]
			if c == kind {
				if i+1 < n && decoded[i+1] == kind {
					i += 2 // doubled quote: escaped, stays inside
					continue
				}
				return i + 1 // closing quote
			}
			if kind != '`' && c == '\\' && i+1 < n {
				i += 2 // backslash escapes the next byte
				continue
			}
			i++
		}
		return n
	}
	// skipString consumes a quoted string starting at the opening quote
	// decoded[i].
	skipString := func(i int, kind byte) int {
		return insideString(i+1, kind)
	}

	// Initial state: inside a single-quoted string.
	i = insideString(0, '\'')

	for i < n {
		c := decoded[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			i = skipString(i, c)
		case c == '#':
			// Line comment until CR or LF.
			for i < n && decoded[i] != '\r' && decoded[i] != '\n' {
				i++
			}
		case c == '-' && i+1 < n && decoded[i+1] == '-' &&
			(i+2 >= n || isSpace(decoded[i+2])):
			// "--" followed by whitespace or end of input: line comment.
			for i < n && decoded[i] != '\r' && decoded[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && decoded[i+1] == '*':
			exec := i+2 < n && decoded[i+2] == '!'
			start := i
			// Find the first "*/"; block comments do not nest.
			j := i + 2
			for j+1 < n && !(decoded[j] == '*' && decoded[j+1] == '/') {
				j++
			}
			var end int // decoded index just past the comment
			if j+1 < n {
				end = j + 2
			} else {
				end = n // unterminated: masks to end of input
			}
			if exec {
				rs, re := rawSpan(start, end)
				res.execComments = append(res.execComments, [2]int{rs, re})
			}
			i = end
		case isSpace(c):
			i++
		case isWordByte(c):
			j := i + 1
			for j < n && isWordByte(decoded[j]) {
				j++
			}
			text := string(decoded[i:j])
			kind := tokWord
			allDigit := true
			for k := i; k < j; k++ {
				if decoded[k] < '0' || decoded[k] > '9' {
					allDigit = false
					break
				}
			}
			if allDigit {
				kind = tokInt
			}
			rs, re := rawSpan(i, j)
			res.tokens = append(res.tokens, token{kind: kind, text: text, rawStart: rs, rawEnd: re})
			i = j
		default:
			rs, re := rawSpan(i, i+1)
			res.tokens = append(res.tokens, token{kind: tokPunct, text: string(decoded[i : i+1]), rawStart: rs, rawEnd: re})
			i++
		}
	}
	return res
}
