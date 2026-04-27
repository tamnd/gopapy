package symbols_test

import (
	"testing"

	"github.com/tamnd/gopapy/parser"
	"github.com/tamnd/gopapy/symbols"
)

func parse(t *testing.T, src string) *parser.Module {
	t.Helper()
	mod, err := parser.ParseFile("<test>", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return mod
}

func TestModuleScope(t *testing.T) {
	mod := parse(t, "x = 1\ny = x + 1\n")
	sm := symbols.Build(mod)
	if sm.Root.Kind != symbols.ScopeModule {
		t.Fatalf("root kind %v", sm.Root.Kind)
	}
	x := sm.Root.Symbols["x"]
	if x == nil {
		t.Fatal("x not in module scope")
	}
	if !x.Has(symbols.FlagBound) {
		t.Error("x not bound")
	}
}

func TestFunctionScope(t *testing.T) {
	mod := parse(t, `
def foo(a, b=1):
    c = a + b
    return c
`)
	sm := symbols.Build(mod)
	if len(sm.Root.Children) != 1 {
		t.Fatalf("expected 1 child scope, got %d", len(sm.Root.Children))
	}
	fn := sm.Root.Children[0]
	if fn.Kind != symbols.ScopeFunction {
		t.Fatalf("expected function scope, got %v", fn.Kind)
	}
	if fn.Name != "foo" {
		t.Fatalf("expected scope name 'foo', got %q", fn.Name)
	}
	a := fn.Symbols["a"]
	if a == nil || !a.Has(symbols.FlagParam) {
		t.Error("a should be a param")
	}
	c := fn.Symbols["c"]
	if c == nil || !c.Has(symbols.FlagBound) {
		t.Error("c should be bound")
	}
}

func TestClassScope(t *testing.T) {
	mod := parse(t, `
class Foo:
    x = 1
`)
	sm := symbols.Build(mod)
	if len(sm.Root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(sm.Root.Children))
	}
	cls := sm.Root.Children[0]
	if cls.Kind != symbols.ScopeClass {
		t.Fatalf("expected class scope, got %v", cls.Kind)
	}
}

func TestImport(t *testing.T) {
	mod := parse(t, "import os\nfrom sys import argv\n")
	sm := symbols.Build(mod)
	osB := sm.Root.Symbols["os"]
	if osB == nil || !osB.Has(symbols.FlagImport) {
		t.Error("os should be import-bound")
	}
	argv := sm.Root.Symbols["argv"]
	if argv == nil || !argv.Has(symbols.FlagImport) {
		t.Error("argv should be import-bound")
	}
}

func TestGlobalNonlocal(t *testing.T) {
	mod := parse(t, `
x = 1
def foo():
    global x
    x = 2
def bar():
    y = 1
    def inner():
        nonlocal y
        y = 2
    inner()
`)
	sm := symbols.Build(mod)
	if len(sm.Diagnostics) != 0 {
		t.Errorf("unexpected diagnostics: %v", sm.Diagnostics)
	}
	_ = sm
}

func TestGlobalAndNonlocal(t *testing.T) {
	mod := parse(t, `
def foo():
    global x
    nonlocal x
`)
	sm := symbols.Build(mod)
	if len(sm.Diagnostics) == 0 {
		t.Error("expected S001 diagnostic for global+nonlocal")
	}
}

func TestMatchPatternBindings(t *testing.T) {
	mod := parse(t, `
match cmd:
    case ("go", dest):
        print(dest)
    case {"action": action}:
        print(action)
`)
	sm := symbols.Build(mod)
	dest := sm.Root.Symbols["dest"]
	if dest == nil || !dest.Has(symbols.FlagBound) {
		t.Error("dest should be bound by match pattern")
	}
	action := sm.Root.Symbols["action"]
	if action == nil || !action.Has(symbols.FlagBound) {
		t.Error("action should be bound by match pattern")
	}
}

func TestTypeParams(t *testing.T) {
	mod := parse(t, `
def first[T](xs: list[T]) -> T:
    return xs[0]
type Vector = list[float]
`)
	sm := symbols.Build(mod)
	_ = sm.Root.Symbols["first"]
	_ = sm.Root.Symbols["Vector"]
}

func TestLambdaScope(t *testing.T) {
	mod := parse(t, "f = lambda x: x + 1\n")
	sm := symbols.Build(mod)
	if len(sm.Root.Children) != 1 {
		t.Fatalf("expected 1 child (lambda scope), got %d", len(sm.Root.Children))
	}
	lam := sm.Root.Children[0]
	if lam.Kind != symbols.ScopeLambda {
		t.Fatalf("expected lambda scope, got %v", lam.Kind)
	}
}

func TestComprehensionScope(t *testing.T) {
	mod := parse(t, "[x*2 for x in range(10)]\n")
	sm := symbols.Build(mod)
	if len(sm.Root.Children) != 1 {
		t.Fatalf("expected 1 comp scope, got %d", len(sm.Root.Children))
	}
	comp := sm.Root.Children[0]
	if comp.Kind != symbols.ScopeComprehension {
		t.Fatalf("expected comp scope, got %v", comp.Kind)
	}
}

func TestResolve(t *testing.T) {
	mod := parse(t, `
x = 1
def foo():
    return x
`)
	sm := symbols.Build(mod)
	fn := sm.Root.Children[0]
	sym, _, _ := fn.Resolve("x")
	if sym == nil {
		t.Error("Resolve should find x from enclosing module scope")
	}
}
