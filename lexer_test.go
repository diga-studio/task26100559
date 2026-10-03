package sentinelgate

import (
	"reflect"
	"testing"
)

// lexString decodes (identity) and lexes s, returning token texts and
// exec comment raw ranges.
func lexString(t *testing.T, s string) ([]string, [][2]int) {
	t.Helper()
	rm := rawMap{Start: make([]int, len(s)), End: make([]int, len(s))}
	for i := range s {
		rm.Start[i] = i
		rm.End[i] = i + 1
	}
	res := lex([]byte(s), rm)
	var texts []string
	for _, tok := range res.tokens {
		texts = append(texts, tok.text)
	}
	return texts, res.execComments
}

func TestLexStartsInsideSingleQuote(t *testing.T) {
	// The leading content is inside the assumed string; the first '
	// closes it and UNION SELECT becomes visible.
	toks, _ := lexString(t, "abc' UNION SELECT")
	want := []string{"UNION", "SELECT"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
}

func TestLexUnterminatedInitialString(t *testing.T) {
	toks, _ := lexString(t, "no closing quote here")
	if len(toks) != 0 {
		t.Fatalf("got %v", toks)
	}
}

func TestLexDoubledQuoteStaysInside(t *testing.T) {
	// "''" is an escaped quote: still inside the string, so UNION is hidden.
	toks, _ := lexString(t, "'' UNION ")
	if len(toks) != 0 {
		t.Fatalf("got %v", toks)
	}
	// Three quotes: escaped pair then a closing one.
	toks, _ = lexString(t, "''' UNION")
	want := []string{"UNION"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
}

func TestLexBackslashEscape(t *testing.T) {
	// \' is escaped; the string stays open until the next '.
	toks, _ := lexString(t, `a\'b' UNION`)
	want := []string{"UNION"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
}

func TestLexDoubleQuoteAndBacktick(t *testing.T) {
	toks, _ := lexString(t, `' "UNION" `+"`SELECT`"+` UNION`)
	want := []string{"UNION"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
	// Backtick does not support backslash escapes.
	toks, _ = lexString(t, "' `a\\` UNION")
	want = []string{"UNION"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
}

func TestLexLineComments(t *testing.T) {
	toks, _ := lexString(t, "' UNION# SELECT\nSELECT")
	want := []string{"UNION", "SELECT"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
	// "--" only starts a comment when followed by whitespace or EOI.
	toks, _ = lexString(t, "' a--b")
	want = []string{"a", "-", "-", "b"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
	toks, _ = lexString(t, "' a-- b\nb")
	want = []string{"a", "b"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
	// "--" at end of input is a comment.
	toks, _ = lexString(t, "' a--")
	want = []string{"a"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
}

func TestLexBlockCommentDoesNotGlue(t *testing.T) {
	// UN/**/ION must stay two words.
	toks, _ := lexString(t, "' UN/**/ION")
	want := []string{"UN", "ION"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
	// Unterminated block comment masks to end.
	toks, _ = lexString(t, "' UNION /* SELECT")
	want = []string{"UNION"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
	// Block comments do not nest: first */ ends it.
	toks, _ = lexString(t, "' /* a /* b */ UNION")
	want = []string{"UNION"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("got %v want %v", toks, want)
	}
}

func TestLexExecComment(t *testing.T) {
	s := "' /*!40000 UNION SELECT*/ x"
	toks, execs := lexString(t, s)
	want := []string{"x"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("tokens: got %v want %v", toks, want)
	}
	if len(execs) != 1 || execs[0] != [2]int{2, 25} {
		t.Fatalf("execs: %v", execs)
	}
	// Unterminated exec comment covers to end of input.
	_, execs = lexString(t, "' /*! UNION")
	if len(execs) != 1 || execs[0] != [2]int{2, 11} {
		t.Fatalf("execs: %v", execs)
	}
	// Inside a string, /*! is not a comment.
	toks, execs = lexString(t, `"/*!" ' x`)
	if len(execs) != 0 {
		t.Fatalf("execs: %v", execs)
	}
	want = []string{"x"}
	if !reflect.DeepEqual(toks, want) {
		t.Fatalf("tokens: got %v want %v", toks, want)
	}
}

func TestLexIntAndWordBytes(t *testing.T) {
	res := func(s string) []token {
		rm := rawMap{Start: make([]int, len(s)), End: make([]int, len(s))}
		for i := range s {
			rm.Start[i] = i
			rm.End[i] = i + 1
		}
		return lex([]byte(s), rm).tokens
	}
	toks := res("' 007 _a$b \xff\xfe")
	if toks[0].kind != tokInt || toks[0].text != "007" {
		t.Errorf("tok0: %+v", toks[0])
	}
	if toks[1].kind != tokWord || toks[1].text != "_a$b" {
		t.Errorf("tok1: %+v", toks[1])
	}
	if toks[2].kind != tokWord || toks[2].text != "\xff\xfe" {
		t.Errorf("tok2: %+v", toks[2])
	}
}
