package sentinelgate

import "testing"

func toksOf(texts ...string) []token {
	var toks []token
	pos := 0
	for _, tx := range texts {
		kind := tokWord
		allDigit := true
		for i := 0; i < len(tx); i++ {
			if tx[i] < '0' || tx[i] > '9' {
				allDigit = false
				break
			}
		}
		switch {
		case len(tx) == 1 && !isWordByte(tx[0]):
			kind = tokPunct
		case allDigit:
			kind = tokInt
		}
		toks = append(toks, token{kind: kind, text: tx, rawStart: pos, rawEnd: pos + len(tx)})
		pos += len(tx) + 1
	}
	return toks
}

func TestRuleUnionSelect(t *testing.T) {
	var hits []Hit
	matchRules(0, toksOf("union", "select"), &hits)
	if len(hits) != 1 || hits[0].Rule != RuleUnionSelect {
		t.Fatalf("got %+v", hits)
	}
	if hits[0].Start != 0 || hits[0].End != 12 {
		t.Fatalf("span: %+v", hits[0])
	}

	hits = nil
	matchRules(0, toksOf("UnIoN", "ALL", "SeLeCt"), &hits)
	if len(hits) != 1 || hits[0].Rule != RuleUnionSelect {
		t.Fatalf("got %+v", hits)
	}

	// UN and ION are separate words: no hit.
	hits = nil
	matchRules(0, toksOf("un", "ion", "select"), &hits)
	if len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}

	// UNION alone, or UNION ALL without SELECT: no hit.
	hits = nil
	matchRules(0, toksOf("union", "all", "x"), &hits)
	if len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}
}

func TestRuleOrEquals(t *testing.T) {
	var hits []Hit
	matchRules(0, toksOf("or", "007", "=", "7"), &hits)
	if len(hits) != 1 || hits[0].Rule != RuleOrEquals {
		t.Fatalf("got %+v", hits)
	}

	// Unequal values: no hit.
	hits = nil
	matchRules(0, toksOf("or", "1", "=", "2"), &hits)
	if len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}

	// Huge integers beyond machine width, equal after zero trimming.
	big1 := "00012345678901234567890123456789012345678901234567890123"
	big2 := "12345678901234567890123456789012345678901234567890123"
	hits = nil
	matchRules(0, toksOf("or", big1, "=", big2), &hits)
	if len(hits) != 1 {
		t.Fatalf("big int: got %+v", hits)
	}

	// Zero forms.
	hits = nil
	matchRules(0, toksOf("or", "0", "=", "000"), &hits)
	if len(hits) != 1 {
		t.Fatalf("zeros: got %+v", hits)
	}
}

func TestIntEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1", "1", true},
		{"007", "7", true},
		{"0", "000", true},
		{"10", "0100", false},
		{"123456789012345678901234567890", "123456789012345678901234567891", false},
	}
	for _, c := range cases {
		if got := intEqual(c.a, c.b); got != c.want {
			t.Errorf("intEqual(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
