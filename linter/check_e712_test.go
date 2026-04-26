package linter

import "testing"

func TestE712(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "eq-true-rhs",
			src:  "x = 1\nif x == True: pass\n",
			want: []string{"E712"},
		},
		{
			name: "eq-true-lhs",
			src:  "x = 1\nif True == x: pass\n",
			want: []string{"E712"},
		},
		{
			name: "noteq-true",
			src:  "x = 1\nif x != True: pass\n",
			want: []string{"E712"},
		},
		{
			name: "eq-false-rhs",
			src:  "x = 1\nif x == False: pass\n",
			want: []string{"E712"},
		},
		{
			name: "eq-false-lhs",
			src:  "x = 1\nif False == x: pass\n",
			want: []string{"E712"},
		},
		{
			// E712 doesn't fire on `is True` (the canonical form).
			// F632 still does, because bool literals are interned-by-
			// implementation, not by language guarantee.
			name: "is-true-not-e712",
			src:  "x = 1\nif x is True: pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-false-not-e712",
			src:  "x = 1\nif x is False: pass\n",
			want: []string{"F632"},
		},
		{
			name: "eq-none-not-e712",
			src:  "x = 1\nif x == None: pass\n",
			want: []string{"E711"},
		},
		{
			name: "eq-int-not-e712",
			src:  "x = 1\nif x == 1: pass\n",
			want: nil,
		},
		{
			name: "noqa-suppresses",
			src:  "x = 1\nif x == True: pass  # noqa: E712\n",
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
