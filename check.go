package sentinelgate

import "sort"

// check orchestrates parameter splitting, decoding, lexing and rule
// matching for a whole query string.
func check(query string) []Hit {
	var hits []Hit
	for _, p := range splitParams(query) {
		if p.rawStart == p.rawEnd {
			continue // empty value: nothing to scan
		}
		decoded, rm := decodeValue(query[p.rawStart:p.rawEnd], p.rawStart)
		res := lex(decoded, rm)
		for _, span := range res.execComments {
			hits = append(hits, Hit{
				Rule:       RuleExecComment,
				ParamIndex: p.index,
				Start:      span[0],
				End:        span[1],
			})
		}
		matchRules(p.index, res.tokens, &hits)
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.ParamIndex != b.ParamIndex {
			return a.ParamIndex < b.ParamIndex
		}
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		if a.End != b.End {
			return a.End < b.End
		}
		return a.Rule < b.Rule
	})
	return hits
}
