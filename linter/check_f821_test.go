package linter

import "testing"

func TestF821(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "bare-typo",
			src:  "x = 1\nprnt(x)\n",
			want: []string{"F821"},
		},
		{
			name: "builtin-print-ok",
			src:  "print('hi')\n",
			want: nil,
		},
		{
			name: "builtin-len-ok",
			src:  "x = len([1, 2])\n",
			want: nil,
		},
		{
			name: "imported-name-ok",
			src:  "import os\nos.getcwd()\n",
			want: nil,
		},
		{
			name: "from-imported-name-ok",
			src:  "from os.path import join\njoin('a', 'b')\n",
			want: nil,
		},
		{
			name: "parameter-ok",
			src:  "def f(x):\n    return x\n",
			want: nil,
		},
		{
			name: "enclosing-function-ok",
			src:  "def outer(x):\n    def inner():\n        return x\n    return inner\n",
			want: nil,
		},
		{
			name: "class-attr-from-method-fires",
			src:  "class C:\n    x = 1\n    def f(self):\n        return x\n",
			want: []string{"F821"},
		},
		{
			name: "star-import-suppresses",
			src:  "from os import *\nprnt(x)\n",
			want: nil,
		},
		{
			name: "global-decl-ok",
			src:  "def f():\n    global g\n    g = 1\n",
			want: nil,
		},
		{
			name: "noqa-suppresses",
			src:  "prnt(1)  # noqa: F821\n",
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
