package sentinelgate

// param is one occurrence of a name=value pair in the query string.
type param struct {
	index    int // 0-based occurrence index in the query string
	rawStart int // byte offset of the value in the raw query string
	rawEnd   int
}

// splitParams splits a raw query string on '&' and then on the first '='
// per item, returning only the value spans. A missing '=' means an empty
// value. Separators produced by decoding never re-split parameters
// because splitting happens strictly before any decoding.
func splitParams(query string) []param {
	var params []param
	idx := 0
	pos := 0
	for pos <= len(query) {
		segEnd := pos
		for segEnd < len(query) && query[segEnd] != '&' {
			segEnd++
		}
		// Segment is query[pos:segEnd]; find its first '='.
		eq := -1
		for i := pos; i < segEnd; i++ {
			if query[i] == '=' {
				eq = i
				break
			}
		}
		var p param
		p.index = idx
		if eq < 0 {
			// No '=': empty value located at the end of the segment.
			p.rawStart = segEnd
			p.rawEnd = segEnd
		} else {
			p.rawStart = eq + 1
			p.rawEnd = segEnd
		}
		params = append(params, p)
		idx++
		if segEnd == len(query) {
			break
		}
		pos = segEnd + 1
	}
	return params
}

// rawMap maps each byte of a decoded value back to the half-open byte
// range [Start, End) in the raw query string that produced it.
type rawMap struct {
	Start []int
	End   []int
}

// hexVal returns the value of an ASCII hex digit, or -1.
func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// decodeValue performs exactly two decode rounds on the raw value bytes
// and returns the decoded bytes plus the mapping back to raw query string
// offsets (rawBase is the value's offset in the full query string).
//
// Round 1 turns '+' into a space and decodes %XX; round 2 only decodes
// %XX and does not touch '+'. Invalid escapes are preserved byte by byte.
// There is no third round and no Unicode normalization.
func decodeValue(raw string, rawBase int) ([]byte, rawMap) {
	n := len(raw)
	// Round 1: '+' -> space, %XX decoding. Map entries point into raw.
	d1 := make([]byte, 0, n)
	s1 := make([]int, 0, n)
	e1 := make([]int, 0, n)
	for i := 0; i < n; {
		c := raw[i]
		if c == '+' {
			d1 = append(d1, ' ')
			s1 = append(s1, rawBase+i)
			e1 = append(e1, rawBase+i+1)
			i++
			continue
		}
		if c == '%' && i+2 <= n-1 {
			hi := hexVal(raw[i+1])
			lo := hexVal(raw[i+2])
			if hi >= 0 && lo >= 0 {
				d1 = append(d1, byte(hi<<4|lo))
				s1 = append(s1, rawBase+i)
				e1 = append(e1, rawBase+i+3)
				i += 3
				continue
			}
		}
		// Plain byte or invalid escape: preserved byte by byte.
		d1 = append(d1, c)
		s1 = append(s1, rawBase+i)
		e1 = append(e1, rawBase+i+1)
		i++
	}

	// Round 2: only %XX decoding on d1. Map entries compose back to raw.
	d2 := make([]byte, 0, len(d1))
	s2 := make([]int, 0, len(d1))
	e2 := make([]int, 0, len(d1))
	for i := 0; i < len(d1); {
		if d1[i] == '%' && i+2 <= len(d1)-1 {
			hi := hexVal(d1[i+1])
			lo := hexVal(d1[i+2])
			if hi >= 0 && lo >= 0 {
				d2 = append(d2, byte(hi<<4|lo))
				s2 = append(s2, s1[i])
				e2 = append(e2, e1[i+2])
				i += 3
				continue
			}
		}
		d2 = append(d2, d1[i])
		s2 = append(s2, s1[i])
		e2 = append(e2, e1[i])
		i++
	}
	return d2, rawMap{Start: s2, End: e2}
}
