package parser

import "testing"

func parse(t *testing.T, src string) *File {
	t.Helper()
	f, err := ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return f
}

func TestParse_AtomLiterals(t *testing.T) {
	cases := []string{
		"x\n",
		"42\n",
		"3.14\n",
		"True\n",
		"False\n",
		"None\n",
		"...\n",
		`"hello"` + "\n",
		"[1, 2, 3]\n",
		"{1, 2, 3}\n",
		"{1: 2, 3: 4}\n",
		"(1, 2, 3)\n",
		"(1,)\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParse_BinaryOps(t *testing.T) {
	cases := []string{
		"a + b\n",
		"a + b * c\n",
		"a * b + c\n",
		"a ** b ** c\n",
		"a - b - c\n",
		"a // b\n",
		"a % b\n",
		"a @ b\n",
		"a & b | c ^ d\n",
		"a << b >> c\n",
		"a < b\n",
		"a < b < c\n",
		"a == b\n",
		"a is not b\n",
		"a not in b\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParse_BoolAndUnary(t *testing.T) {
	cases := []string{
		"a and b\n",
		"a or b or c\n",
		"not a\n",
		"not not a\n",
		"-a\n",
		"~a\n",
		"+a + -b\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParse_Conditional(t *testing.T) {
	cases := []string{
		"a if c else b\n",
		"f(a if c else b)\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParse_PrimaryChain(t *testing.T) {
	cases := []string{
		"a.b\n",
		"a.b.c\n",
		"f()\n",
		"f(1, 2)\n",
		"f(x=1, y=2)\n",
		"f(*args, **kwargs)\n",
		"a[0]\n",
		"a[1:2]\n",
		"a[::2]\n",
		"a.b[0]()\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParse_SimpleStatements(t *testing.T) {
	cases := []string{
		"pass\n",
		"break\n",
		"continue\n",
		"return\n",
		"return 1\n",
		"return 1, 2\n",
		"raise\n",
		"raise ValueError\n",
		"raise ValueError from exc\n",
		"del x\n",
		"del x, y\n",
		"global x\n",
		"global x, y\n",
		"nonlocal a\n",
		"assert x\n",
		"assert x, 'msg'\n",
		"import os\n",
		"import os, sys\n",
		"import os.path\n",
		"import os.path as p\n",
		"from os import path\n",
		"from os import path, sep\n",
		"from os.path import *\n",
		"from . import x\n",
		"from .. import x\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParse_Assignments(t *testing.T) {
	cases := []string{
		"x = 1\n",
		"x = y = 1\n",
		"x: int = 1\n",
		"x: int\n",
		"x += 1\n",
		"x -= 1\n",
		"x *= 2\n",
		"x //= 2\n",
		"x **= 2\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParse_If(t *testing.T) {
	src := `if x:
    a
elif y:
    b
elif z:
    c
else:
    d
`
	parse(t, src)
}

func TestParse_While(t *testing.T) {
	parse(t, "while x:\n    pass\n")
}

func TestParse_For(t *testing.T) {
	parse(t, "for i in xs:\n    pass\n")
}

func TestParse_With(t *testing.T) {
	parse(t, "with open(p) as f:\n    pass\n")
}

func TestParse_Try(t *testing.T) {
	src := `try:
    a
except ValueError:
    b
except (TypeError, KeyError) as e:
    c
finally:
    d
`
	parse(t, src)
}

func TestParse_Def(t *testing.T) {
	cases := []string{
		"def f():\n    pass\n",
		"def f(x, y):\n    return x + y\n",
		"def f(x: int = 0) -> int:\n    return x\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParse_Class(t *testing.T) {
	cases := []string{
		"class C:\n    pass\n",
		"class C(B):\n    pass\n",
		"class C(B, metaclass=Meta):\n    pass\n",
	}
	for _, src := range cases {
		if _, err := ParseString("<test>", src); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}
