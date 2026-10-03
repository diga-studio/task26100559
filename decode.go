package sentinelgate

// decodedByte 是解码后的一个字节及其在原始查询串中的来源区间。
// 区间左闭右开；一个解码字节可能由多个原始字节（如 %41）折叠而来。
type decodedByte struct {
	b    byte
	from int // 原始起始偏移（含）
	to   int // 原始结束偏移（不含）
}

// hexVal 返回 ASCII 十六进制数字的值，非十六进制返回 -1。
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

// decodeRound1 执行首轮解码：先把 '+' 变为空格，再解码 "%XX" 百分号转义。
// 无效转义逐字节保留。base 为值在原始查询串中的起始偏移。
func decodeRound1(raw string, base int) []decodedByte {
	out := make([]decodedByte, 0, len(raw))
	for i := 0; i < len(raw); {
		c := raw[i]
		switch {
		case c == '+':
			out = append(out, decodedByte{b: ' ', from: base + i, to: base + i + 1})
			i++
		case c == '%' && i+2 < len(raw) && hexVal(raw[i+1]) >= 0 && hexVal(raw[i+2]) >= 0:
			v := byte(hexVal(raw[i+1])<<4 | hexVal(raw[i+2]))
			out = append(out, decodedByte{b: v, from: base + i, to: base + i + 3})
			i += 3
		default:
			out = append(out, decodedByte{b: c, from: base + i, to: base + i + 1})
			i++
		}
	}
	return out
}

// decodeRound2 执行第二轮解码：只解码 "%XX" 百分号转义，不转换 '+'。
// 折叠后的字节继承从 '%' 到第二个十六进制数字的完整原始区间。
func decodeRound2(in []decodedByte) []decodedByte {
	out := make([]decodedByte, 0, len(in))
	for i := 0; i < len(in); {
		if in[i].b == '%' && i+2 < len(in) && hexVal(in[i+1].b) >= 0 && hexVal(in[i+2].b) >= 0 {
			v := byte(hexVal(in[i+1].b)<<4 | hexVal(in[i+2].b))
			out = append(out, decodedByte{b: v, from: in[i].from, to: in[i+2].to})
			i += 3
			continue
		}
		out = append(out, in[i])
		i++
	}
	return out
}

// decodeValue 对一个参数值恰好执行两轮解码。
// base 为该值在原始查询串中的起始字节偏移。
func decodeValue(raw string, base int) []decodedByte {
	return decodeRound2(decodeRound1(raw, base))
}
