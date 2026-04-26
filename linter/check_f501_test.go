package linter

import "testing"

func TestF501(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "tuple-too-few",
			src:  "x = \"%s %s\" % (1,)\n",
			want: []string{"F501"},
		},
		{
			name: "tuple-too-many",
			src:  "x = \"%s\" % (1, 2)\n",
			want: []string{"F501"},
		},
		{
			name: "tuple-matched",
			src:  "x = \"%s %s\" % (1, 2)\n",
			want: nil,
		},
		{
			name: "single-value-matched",
			src:  "y = 1\nx = \"%s\" % y\n",
			want: nil,
		},
		{
			name: "single-value-too-many-conversions",
			src:  "y = 1\nx = \"%s %s\" % y\n",
			want: []string{"F501"},
		},
		{
			name: "escaped-percent",
			src:  "x = \"100%%\" % ()\n",
			want: nil,
		},
		{
			name: "non-string-lhs",
			src:  "x = 1 % 2\n",
			want: nil,
		},
		{
			name: "dict-rhs-skipped",
			src:  "x = \"%(a)s %(b)s\" % {'a': 1}\n",
			want: nil,
		},
		{
			name: "keyed-format-skipped",
			src:  "y = {'a': 1}\nx = \"%(a)s\" % y\n",
			want: nil,
		},
		{
			name: "format-with-width",
			src:  "x = \"%5d %-10s\" % (1, 'a')\n",
			want: nil,
		},
		{
			name: "noqa-suppresses",
			src:  "x = \"%s\" % (1, 2)  # noqa: F501\n",
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
