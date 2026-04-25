package lex

import "testing"

// scanAll drives Scan to EOF and returns the kind list. Use for table-driven
// tests where the exact lexeme is implied by the input.
func scanAll(t *testing.T, src string) []Token {
	t.Helper()
	s := NewScanner([]byte(src), "<test>")
	var out []Token
	for {
		tok, err := s.Scan()
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if tok.Kind == EOF {
			break
		}
		out = append(out, tok)
	}
	return out
}

func kinds(toks []Token) []Kind {
	out := make([]Kind, len(toks))
	for i, t := range toks {
		out[i] = t.Kind
	}
	return out
}

func eqKinds(a, b []Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScan_Operators(t *testing.T) {
	cases := []struct {
		src  string
		want []Kind
	}{
		{"+", []Kind{PLUS}},
		{"++", []Kind{PLUS, PLUS}},
		{"**", []Kind{DOUBLESTAR}},
		{"**=", []Kind{DOUBLESTAREQ}},
		{"...", []Kind{ELLIPSIS}},
		{"..", []Kind{DOT, DOT}},
		{".", []Kind{DOT}},
		{"->", []Kind{ARROW}},
		{":=", []Kind{WALRUS}},
		{"<<=", []Kind{LSHIFTEQ}},
		{"==!=", []Kind{EQEQ, NE}},
		{"a@b", []Kind{NAME, AT, NAME}},
	}
	for _, c := range cases {
		got := kinds(scanAll(t, c.src))
		if !eqKinds(got, c.want) {
			t.Errorf("%q: got %v, want %v", c.src, got, c.want)
		}
	}
}

func TestScan_Numbers(t *testing.T) {
	cases := []struct{ src, want string }{
		{"0", "0"},
		{"123", "123"},
		{"1_000_000", "1_000_000"},
		{"0xFF", "0xFF"},
		{"0xDEAD_BEEF", "0xDEAD_BEEF"},
		{"0o777", "0o777"},
		{"0b1010_1010", "0b1010_1010"},
		{"3.14", "3.14"},
		{".5", ".5"},
		{"1e10", "1e10"},
		{"1.5e-3", "1.5e-3"},
		{"1j", "1j"},
		{"3.14J", "3.14J"},
	}
	for _, c := range cases {
		toks := scanAll(t, c.src)
		if len(toks) != 1 || toks[0].Kind != NUMBER || toks[0].Value != c.want {
			t.Errorf("%q: got %v, want NUMBER(%q)", c.src, toks, c.want)
		}
	}
}

func TestScan_Names(t *testing.T) {
	toks := scanAll(t, "abc _foo bar123")
	if len(toks) != 3 {
		t.Fatalf("got %d tokens, want 3", len(toks))
	}
	for i, want := range []string{"abc", "_foo", "bar123"} {
		if toks[i].Kind != NAME || toks[i].Value != want {
			t.Errorf("[%d] = %v, want NAME(%q)", i, toks[i], want)
		}
	}
}

func TestScan_Strings(t *testing.T) {
	cases := []string{
		`""`,
		`"hello"`,
		`'hello'`,
		`"a \"b\" c"`,
		`'''triple
quoted'''`,
		`"""triple"""`,
		`r"\n"`,
		`b"\xff"`,
		`rb"raw bytes"`,
	}
	for _, src := range cases {
		toks := scanAll(t, src)
		if len(toks) != 1 || toks[0].Kind != STRING {
			t.Errorf("%q: got %v, want STRING", src, toks)
		}
		if toks[0].Value != src {
			t.Errorf("%q: value %q, want %q", src, toks[0].Value, src)
		}
	}
}

func TestScan_PositionTracking(t *testing.T) {
	toks := scanAll(t, "a + b\nfoo")
	want := []struct {
		v       string
		l, c    int
		offset  int
	}{
		{"a", 1, 0, 0},
		{"+", 1, 2, 2},
		{"b", 1, 4, 4},
		{"\n", 1, 5, 5},
		{"foo", 2, 0, 6},
	}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(toks), len(want))
	}
	for i, w := range want {
		if toks[i].Value != w.v {
			t.Errorf("[%d] value = %q, want %q", i, toks[i].Value, w.v)
		}
		if toks[i].Pos.Line != w.l || toks[i].Pos.Col != w.c {
			t.Errorf("[%d] pos = %d:%d, want %d:%d", i, toks[i].Pos.Line, toks[i].Pos.Col, w.l, w.c)
		}
		if toks[i].Pos.Offset != w.offset {
			t.Errorf("[%d] offset = %d, want %d", i, toks[i].Pos.Offset, w.offset)
		}
	}
}

func TestScan_Comments(t *testing.T) {
	toks := scanAll(t, "x = 1  # a comment\n# type: int\n")
	// NAME `=` NUMBER COMMENT NEWLINE TYPE_COMMENT NEWLINE
	wantKinds := []Kind{NAME, EQ, NUMBER, COMMENT, NEWLINE, TYPE_COMMENT, NEWLINE}
	if got := kinds(toks); !eqKinds(got, wantKinds) {
		t.Errorf("kinds = %v, want %v", got, wantKinds)
	}
}

func TestScan_LineContinuation(t *testing.T) {
	// `a + \\ \n b` should lex as NAME PLUS NAME with no NEWLINE.
	toks := scanAll(t, "a + \\\nb")
	want := []Kind{NAME, PLUS, NAME}
	if got := kinds(toks); !eqKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
