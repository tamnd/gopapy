package linter

import "testing"

func TestW605(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "bad-p",
			src:  `x = "\p"` + "\n",
			want: []string{"W605"},
		},
		{
			name: "bad-d",
			src:  `x = "\d"` + "\n",
			want: []string{"W605"},
		},
		{
			name: "raw-lower",
			src:  `x = r"\p"` + "\n",
			want: nil,
		},
		{
			name: "raw-upper",
			src:  `x = R"\p"` + "\n",
			want: nil,
		},
		{
			name: "escaped-backslash",
			src:  `x = "\\p"` + "\n",
			want: nil,
		},
		{
			name: "newline-escape",
			src:  `x = "\n"` + "\n",
			want: nil,
		},
		{
			name: "hex-escape",
			src:  `x = "\x41"` + "\n",
			want: nil,
		},
		{
			name: "octal-escape",
			src:  `x = "\007"` + "\n",
			want: nil,
		},
		{
			name: "unicode-name-str",
			src:  `x = "\N{LATIN SMALL LETTER A}"` + "\n",
			want: nil,
		},
		{
			name: "unicode-hex-4",
			src:  "x = \"\\u0041\"\n",
			want: nil,
		},
		{
			name: "bytes-bad",
			src:  `x = b"\p"` + "\n",
			want: []string{"W605"},
		},
		{
			name: "bytes-no-unicode-name",
			src:  `x = b"\N{LATIN SMALL LETTER A}"` + "\n",
			want: []string{"W605"},
		},
		{
			name: "bytes-no-u-escape",
			src:  "x = b\"\\u0041\"\n",
			want: []string{"W605"},
		},
		{
			name: "triple-quoted",
			src:  `x = """\p"""` + "\n",
			want: []string{"W605"},
		},
		{
			name: "fstring-bad-outside-brace",
			src:  `x = 1` + "\n" + `y = f"\p {x}"` + "\n",
			want: []string{"W605"},
		},
		{
			name: "fstring-good-with-placeholder",
			src:  `x = 1` + "\n" + `y = f"hi {x}"` + "\n",
			want: nil,
		},
		{
			name: "tstring-bad",
			src:  `x = 1` + "\n" + `y = t"\d {x}"` + "\n",
			want: []string{"W605"},
		},
		{
			name: "good-quote-escape",
			src:  `x = "say \"hi\""` + "\n",
			want: nil,
		},
		{
			name: "good-tab-escape",
			src:  `x = "a\tb"` + "\n",
			want: nil,
		},
		{
			name: "noqa-suppresses",
			src:  `x = "\p"  # noqa: W605` + "\n",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintSrc(t, tc.src)
			if !equalStrings(got, tc.want) {
				t.Errorf("got %v, want %v\nsrc:\n%s", got, tc.want, tc.src)
			}
		})
	}
}

// TestW605LintModeOnly documents the substrate boundary: W605 needs
// raw source, so Lint(mod) can't run it. Only LintFile / LintFiles
// surface the diagnostic.
func TestW605LintModeOnly(t *testing.T) {
	src := []byte(`x = "\p"` + "\n")
	mDiags, err := LintFile("t.py", src)
	if err != nil {
		t.Fatalf("LintFile: %v", err)
	}
	var sawW605 bool
	for _, d := range mDiags {
		if d.Code == "W605" {
			sawW605 = true
		}
	}
	if !sawW605 {
		t.Errorf("LintFile should report W605, got %v", mDiags)
	}
}
