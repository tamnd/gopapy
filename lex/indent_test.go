package lex

import "testing"

func tokenize(t *testing.T, src string) []Token {
	t.Helper()
	it := NewIndent(NewScanner([]byte(src), "<test>"))
	out, err := it.All()
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	return out
}

func TestIndent_FlatProgram(t *testing.T) {
	toks := tokenize(t, "x = 1\ny = 2\n")
	want := []Kind{
		NAME, EQ, NUMBER, NEWLINE,
		NAME, EQ, NUMBER, NEWLINE,
		ENDMARKER,
	}
	if got := kinds(toks); !eqKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIndent_IfBlock(t *testing.T) {
	src := "if x:\n    y = 1\n    z = 2\n"
	toks := tokenize(t, src)
	want := []Kind{
		NAME, NAME, COLON, NEWLINE,
		INDENT,
		NAME, EQ, NUMBER, NEWLINE,
		NAME, EQ, NUMBER, NEWLINE,
		DEDENT,
		ENDMARKER,
	}
	if got := kinds(toks); !eqKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIndent_Nested(t *testing.T) {
	src := "if x:\n    if y:\n        z = 1\n"
	toks := tokenize(t, src)
	want := []Kind{
		NAME, NAME, COLON, NEWLINE,
		INDENT,
		NAME, NAME, COLON, NEWLINE,
		INDENT,
		NAME, EQ, NUMBER, NEWLINE,
		DEDENT, DEDENT,
		ENDMARKER,
	}
	if got := kinds(toks); !eqKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIndent_Dedent_Outer(t *testing.T) {
	src := "if x:\n    y = 1\nz = 2\n"
	toks := tokenize(t, src)
	want := []Kind{
		NAME, NAME, COLON, NEWLINE,
		INDENT,
		NAME, EQ, NUMBER, NEWLINE,
		DEDENT,
		NAME, EQ, NUMBER, NEWLINE,
		ENDMARKER,
	}
	if got := kinds(toks); !eqKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIndent_BracketsSuppressNewline(t *testing.T) {
	// NEWLINE inside (), [], {} is dropped.
	src := "x = (\n    1,\n    2,\n)\n"
	toks := tokenize(t, src)
	want := []Kind{
		NAME, EQ, LPAREN, NUMBER, COMMA, NUMBER, COMMA, RPAREN, NEWLINE,
		ENDMARKER,
	}
	if got := kinds(toks); !eqKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIndent_BlankLinesIgnored(t *testing.T) {
	src := "x = 1\n\n\ny = 2\n"
	toks := tokenize(t, src)
	want := []Kind{
		NAME, EQ, NUMBER, NEWLINE,
		NAME, EQ, NUMBER, NEWLINE,
		ENDMARKER,
	}
	if got := kinds(toks); !eqKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIndent_NoTrailingNewline(t *testing.T) {
	// Source missing a final newline: indent injector adds one before ENDMARKER.
	src := "x = 1"
	toks := tokenize(t, src)
	want := []Kind{NAME, EQ, NUMBER, NEWLINE, ENDMARKER}
	if got := kinds(toks); !eqKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIndent_BadDedent(t *testing.T) {
	// 4-space block then 2-space: doesn't match any outer level.
	src := "if x:\n    y = 1\n  z = 2\n"
	it := NewIndent(NewScanner([]byte(src), "<test>"))
	_, err := it.All()
	if err == nil {
		t.Fatal("expected error for misaligned dedent")
	}
}
