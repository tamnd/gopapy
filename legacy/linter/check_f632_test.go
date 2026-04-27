package linter

import "testing"

func TestF632Extended(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		// New v0.1.21 patterns.
		{
			name: "is-ellipsis",
			src:  "x = 1\nif x is Ellipsis: pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-not-implemented",
			src:  "x = 1\nif x is NotImplemented: pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-not-ellipsis",
			src:  "x = 1\nif x is not Ellipsis: pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-bare-ellipsis-literal",
			src:  "x = 1\nif x is ...: pass\n",
			want: []string{"F632"},
		},

		// Negatives that document the boundary of the new check.
		{
			name: "eq-ellipsis-allowed",
			src:  "x = 1\nif x == Ellipsis: pass\n",
			want: nil,
		},
		{
			name: "type-of-x-is-type-of-y-allowed",
			src:  "x = 1\ny = 2\nif type(x) is type(y): pass\n",
			want: nil,
		},
		{
			name: "type-of-x-is-int-allowed",
			src:  "x = 1\nif type(x) is int: pass\n",
			want: nil,
		},
		{
			name: "is-name-allowed-2",
			src:  "x = 1\nFoo = 1\nif x is Foo: pass\n",
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
