package linter

import "testing"

func TestE711(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "eq-none-rhs",
			src:  "x = 1\nif x == None: pass\n",
			want: []string{"E711"},
		},
		{
			name: "eq-none-lhs",
			src:  "x = 1\nif None == x: pass\n",
			want: []string{"E711"},
		},
		{
			name: "noteq-none",
			src:  "x = 1\nif x != None: pass\n",
			want: []string{"E711"},
		},
		{
			name: "is-none-allowed",
			src:  "x = 1\nif x is None: pass\n",
			want: nil,
		},
		{
			name: "is-not-none-allowed",
			src:  "x = 1\nif x is not None: pass\n",
			want: nil,
		},
		{
			name: "eq-zero-not-e711",
			src:  "x = 1\nif x == 0: pass\n",
			want: nil,
		},
		{
			name: "eq-true-not-e711",
			src:  "x = 1\nif x == True: pass\n",
			want: []string{"E712"},
		},
		{
			name: "chained-none-middle",
			src:  "x = 1\ny = 2\nif x == None == y: pass\n",
			want: []string{"E711", "E711"},
		},
		{
			name: "noqa-suppresses",
			src:  "x = 1\nif x == None: pass  # noqa: E711\n",
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
