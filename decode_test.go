package sentinelgate

import "testing"

func TestSplitParams(t *testing.T) {
	q := "a=1&b=&c&a=%26%3D"
	ps := splitParams(q)
	if len(ps) != 4 {
		t.Fatalf("want 4 params, got %d", len(ps))
	}
	want := [][2]int{{2, 3}, {6, 6}, {8, 8}, {11, 17}}
	for i, p := range ps {
		if p.index != i || p.rawStart != want[i][0] || p.rawEnd != want[i][1] {
			t.Errorf("param %d: got %+v want span %v", i, p, want[i])
		}
		if got := q[p.rawStart:p.rawEnd]; i == 3 && got != "%26%3D" {
			t.Errorf("param 3 value = %q", got)
		}
	}
}

func TestSplitParamsTrailingAndEmpty(t *testing.T) {
	ps := splitParams("")
	if len(ps) != 1 || ps[0].rawStart != 0 || ps[0].rawEnd != 0 {
		t.Fatalf("empty query: %+v", ps)
	}
	ps = splitParams("a=1&")
	if len(ps) != 2 || ps[1].rawStart != 4 || ps[1].rawEnd != 4 {
		t.Fatalf("trailing &: %+v", ps)
	}
}

func TestDecodeTwoRounds(t *testing.T) {
	// "%2527" -> round1 "%27" -> round2 "'"
	d, _ := decodeValue("%2527", 0)
	if string(d) != "'" {
		t.Fatalf("got %q", d)
	}
	// Round 1 turns '+' into space; round 2 must not.
	d, _ = decodeValue("%2B", 0)
	if string(d) != "+" {
		t.Fatalf("round2 plus: got %q", d)
	}
	d, _ = decodeValue("a+b", 0)
	if string(d) != "a b" {
		t.Fatalf("round1 plus: got %q", d)
	}
	// Invalid escapes preserved byte by byte.
	d, _ = decodeValue("%2G%", 0)
	if string(d) != "%2G%" {
		t.Fatalf("invalid escape: got %q", d)
	}
	// No third round.
	d, _ = decodeValue("%252527", 0)
	if string(d) != "%27" {
		t.Fatalf("third round: got %q", d)
	}
}

func TestDecodeRawMap(t *testing.T) {
	raw := "x=%41%2G+"
	base := 2
	d, rm := decodeValue(raw[base:], base)
	if string(d) != "A%2G " {
		t.Fatalf("got %q", d)
	}
	// 'A' came from raw "%41" at [2,5).
	if rm.Start[0] != 2 || rm.End[0] != 5 {
		t.Errorf("A map: [%d,%d)", rm.Start[0], rm.End[0])
	}
	// trailing space came from '+' at raw offset 8.
	last := len(d) - 1
	if rm.Start[last] != 8 || rm.End[last] != 9 {
		t.Errorf("space map: [%d,%d)", rm.Start[last], rm.End[last])
	}
}
