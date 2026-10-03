package sentinelgate

// param 是查询串中一个参数的值及其在原始查询串中的位置。
type param struct {
	index int    // 参数出现序号，从 0 计数
	value string // 未解码的原始值
	base  int    // 值在原始查询串中的起始字节偏移
}

// splitParams 按 '&' 划分参数，再按每项的首个 '=' 分出名称和值。
// 没有 '=' 视为空值；只保留值用于检查。解码在划分之后进行，
// 因此解码产生的 '&'、'=' 不会重新划分参数。
func splitParams(query string) []param {
	var params []param
	start := 0
	idx := 0
	for start <= len(query) {
		end := start
		for end < len(query) && query[end] != '&' {
			end++
		}
		// 段为 query[start:end]，在其内找首个 '='。
		eq := -1
		for i := start; i < end; i++ {
			if query[i] == '=' {
				eq = i
				break
			}
		}
		var p param
		p.index = idx
		if eq >= 0 {
			p.value = query[eq+1 : end]
			p.base = eq + 1
		} else {
			p.value = ""
			p.base = end
		}
		params = append(params, p)
		idx++
		if end == len(query) {
			break
		}
		start = end + 1
	}
	return params
}
