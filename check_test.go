package sentinelgate

import (
	"strings"
	"testing"
)

func TestCheckDoubleEncodedQuoteWithComment(t *testing.T) {
	// %2527 -> %27 -> ' closes the assumed string; UNION and SELECT are
	// separated by an ordinary comment and still hit, with evidence
	// pointing at the raw (still-encoded) query string bytes.
	q := "a=%2527%2520UNION/**/SELECT"
	hits := Check(q)
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %+v", hits)
	}
	h := hits[0]
	if h.Rule != RuleUnionSelect || h.ParamIndex != 0 {
		t.Fatalf("hit: %+v", h)
	}
	// UNION starts at raw offset 12, SELECT ends at raw offset 27; the
	// span includes the comment between them.
	if h.Start != 12 || h.End != 27 {
		t.Fatalf("span [%d,%d), evidence %q", h.Start, h.End, q[h.Start:h.End])
	}
	if got := q[h.Start:h.End]; got != "UNION/**/SELECT" {
		t.Fatalf("evidence %q", got)
	}
}

func TestCheckDoubleEncodedKeywordLetters(t *testing.T) {
	// Fully double-encoded payload: %2527 -> ', %2555 -> %55 -> 'U' etc.
	// "' UNION SELECT" double-encoded.
	q := "x=%2527%2520%2555%254E%2549%254F%254E%2520%2553%2545%254C%2545%2543%2554"
	hits := Check(q)
	if len(hits) != 1 || hits[0].Rule != RuleUnionSelect {
		t.Fatalf("got %+v", hits)
	}
	h := hits[0]
	// Evidence spans from the first byte of %2555 to the end of %2554.
	start := strings.Index(q, "%2555")
	end := strings.Index(q, "%2554") + len("%2554")
	if h.Start != start || h.End != end {
		t.Fatalf("span [%d,%d) want [%d,%d)", h.Start, h.End, start, end)
	}
}

func TestCheckEncodedSeparatorsDoNotSplit(t *testing.T) {
	// %26 -> '&' and %3D -> '=' after decoding must not create params.
	q := "x=%2527%2520UNION%2520SELECT%26y%3D1"
	hits := Check(q)
	if len(hits) != 1 || hits[0].Rule != RuleUnionSelect || hits[0].ParamIndex != 0 {
		t.Fatalf("got %+v", hits)
	}
	// The raw query has exactly one parameter.
	if strings.Contains(q, "&") {
		t.Fatal("test query should contain no literal &")
	}
}

func TestCheckSameNameParamsCheckedIndividually(t *testing.T) {
	q := "a=%27+UNION+SELECT&a=%27+OR+1=1"
	hits := Check(q)
	if len(hits) != 2 {
		t.Fatalf("got %+v", hits)
	}
	if hits[0].Rule != RuleUnionSelect || hits[0].ParamIndex != 0 {
		t.Fatalf("hit0: %+v", hits[0])
	}
	if hits[1].Rule != RuleOrEquals || hits[1].ParamIndex != 1 {
		t.Fatalf("hit1: %+v", hits[1])
	}
}

func TestCheckNoCrossParamHit(t *testing.T) {
	// UNION in one param, SELECT in the next: no hit.
	q := "a=%27+UNION&b=%27+SELECT"
	if hits := Check(q); len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}
}

func TestCheckDoubledQuoteKeepsStringState(t *testing.T) {
	// "''" is an escaped quote: still inside the string, no hit.
	if hits := Check("a=''+UNION+SELECT"); len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}
	// "'''": escaped pair then a real closing quote -> hit.
	hits := Check("a='''+UNION+SELECT")
	if len(hits) != 1 || hits[0].Rule != RuleUnionSelect {
		t.Fatalf("got %+v", hits)
	}
}

func TestCheckOrdinaryCommentDoesNotGlue(t *testing.T) {
	if hits := Check("a=%27UN/**/ION%20SELECT"); len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}
	if hits := Check("a=%27UNI--%20%0AON%20SELECT"); len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}
}

func TestCheckLongLeadingZeroInts(t *testing.T) {
	zeros := strings.Repeat("0", 5000)
	q := "a=%27+OR+" + zeros + "1%3D1"
	hits := Check(q)
	if len(hits) != 1 || hits[0].Rule != RuleOrEquals {
		t.Fatalf("got %+v", hits)
	}
	// Unequal huge values: no hit.
	q = "a=%27+OR+" + zeros + "1%3D2"
	if hits := Check(q); len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}
}

func TestCheckExecComment(t *testing.T) {
	q := "a=%27/*!40000+UNION+SELECT*/"
	hits := Check(q)
	if len(hits) != 1 || hits[0].Rule != RuleExecComment {
		t.Fatalf("got %+v", hits)
	}
	h := hits[0]
	if got := q[h.Start:h.End]; got != "/*!40000+UNION+SELECT*/" {
		t.Fatalf("evidence %q", got)
	}
	// Interior of an executable comment is not scanned for other rules.
	for _, hh := range hits {
		if hh.Rule != RuleExecComment {
			t.Fatalf("unexpected extra hit %+v", hh)
		}
	}
}

func TestCheckUnterminatedFragments(t *testing.T) {
	// Unterminated executable comment covers to end of the value.
	q := "a=%27/*!+UNION"
	hits := Check(q)
	if len(hits) != 1 || hits[0].Rule != RuleExecComment {
		t.Fatalf("got %+v", hits)
	}
	if hits[0].End != len(q) {
		t.Fatalf("end %d want %d", hits[0].End, len(q))
	}
	// Unterminated block comment masks the tail but keeps earlier hits.
	q = "a=%27+OR+1=1/*+UNION+SELECT"
	hits = Check(q)
	if len(hits) != 1 || hits[0].Rule != RuleOrEquals {
		t.Fatalf("got %+v", hits)
	}
	// Unterminated string masks the tail but keeps earlier hits.
	q = "a=%27+UNION+SELECT+%27+OR+1=1"
	hits = Check(q)
	if len(hits) != 1 || hits[0].Rule != RuleUnionSelect {
		t.Fatalf("got %+v", hits)
	}
}

func TestCheckCaseInsensitiveAndWhitespace(t *testing.T) {
	q := "a=%27%09uNiOn%0D%0AaLl%0BSeLeCt"
	hits := Check(q)
	if len(hits) != 1 || hits[0].Rule != RuleUnionSelect {
		t.Fatalf("got %+v", hits)
	}
}

func TestCheckEmptyAndNoHit(t *testing.T) {
	for _, q := range []string{"", "a", "a=", "a=1&b=2", "a=%27+select+1"} {
		if hits := Check(q); len(hits) != 0 {
			t.Fatalf("q=%q got %+v", q, hits)
		}
	}
}
