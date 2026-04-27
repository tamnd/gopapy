package symbols

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
)

// build is the test helper: parse src to an AST module and run Build.
func build(t *testing.T, src string) *Module {
	t.Helper()
	if !strings.HasSuffix(src, "\n") {
		src += "\n"
	}
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return Build(ast.FromFile(f))
}

// findChild returns the first descendant scope matching kind+name. It's a
// breadth-first walk so tests that name a deeply nested scope still find it.
func findChild(s *Scope, kind ScopeKind, name string) *Scope {
	queue := []*Scope{s}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.Kind == kind && cur.Name == name {
			return cur
		}
		queue = append(queue, cur.Children...)
	}
	return nil
}

func mustBinding(t *testing.T, scope *Scope, name string) *Binding {
	t.Helper()
	sym, ok := scope.Symbols[name]
	if !ok {
		t.Fatalf("no binding for %q in %s scope %q", name, scope.Kind, scope.Name)
	}
	return sym
}

func TestModuleAssign(t *testing.T) {
	m := build(t, "x = 1")
	sym := mustBinding(t, m.Root, "x")
	if !sym.Has(FlagBound) {
		t.Fatalf("x flags = %b, want bound", sym.Flags)
	}
	if len(sym.BindSites) != 1 {
		t.Fatalf("x bind sites = %d", len(sym.BindSites))
	}
}

func TestTupleUnpack(t *testing.T) {
	m := build(t, "x, y = 1, 2")
	for _, name := range []string{"x", "y"} {
		sym := mustBinding(t, m.Root, name)
		if !sym.Has(FlagBound) {
			t.Errorf("%s not bound", name)
		}
	}
}

func TestStarredUnpack(t *testing.T) {
	m := build(t, "a, *b, c = [1,2,3,4]")
	for _, name := range []string{"a", "b", "c"} {
		if _, ok := m.Root.Symbols[name]; !ok {
			t.Errorf("missing binding for %s", name)
		}
	}
}

func TestAugAssign(t *testing.T) {
	m := build(t, "x = 0\nx += 1")
	sym := mustBinding(t, m.Root, "x")
	if len(sym.BindSites) != 2 {
		t.Fatalf("x bind sites = %d, want 2", len(sym.BindSites))
	}
}

func TestForLoop(t *testing.T) {
	m := build(t, "for i in xs:\n    pass\n")
	if _, ok := m.Root.Symbols["i"]; !ok {
		t.Fatalf("for-target i not bound")
	}
	if !m.Root.Symbols["xs"].Has(FlagUsed) {
		t.Fatalf("xs not marked used")
	}
}

func TestWithAs(t *testing.T) {
	m := build(t, "with f() as g:\n    pass\n")
	if _, ok := m.Root.Symbols["g"]; !ok {
		t.Fatalf("with-target g not bound")
	}
	if !m.Root.Symbols["f"].Has(FlagUsed) {
		t.Fatalf("f not used")
	}
}

func TestExceptAs(t *testing.T) {
	m := build(t, "try:\n    pass\nexcept Exception as e:\n    pass\n")
	if _, ok := m.Root.Symbols["e"]; !ok {
		t.Fatalf("except-as e not bound")
	}
	if !m.Root.Symbols["Exception"].Has(FlagUsed) {
		t.Fatalf("Exception not used")
	}
}

func TestImport(t *testing.T) {
	m := build(t, "import a.b.c\nimport d as dd\nfrom x import y\nfrom x import z as zz\n")
	for _, name := range []string{"a", "dd", "y", "zz"} {
		sym, ok := m.Root.Symbols[name]
		if !ok {
			t.Fatalf("missing import binding %s", name)
		}
		if !sym.Has(FlagImport) {
			t.Errorf("%s not flagged Import", name)
		}
	}
	for _, name := range []string{"d", "z"} {
		if _, ok := m.Root.Symbols[name]; ok {
			t.Errorf("did not expect %s to be bound", name)
		}
	}
}

func TestFunctionDefAndParams(t *testing.T) {
	m := build(t, "def f(a, b=1, *args, c, **kw):\n    return a\n")
	if !m.Root.Symbols["f"].Has(FlagBound) {
		t.Fatalf("f not bound at module")
	}
	fn := findChild(m.Root, ScopeFunction, "f")
	if fn == nil {
		t.Fatalf("no function scope")
	}
	for _, name := range []string{"a", "b", "args", "c", "kw"} {
		sym := mustBinding(t, fn, name)
		if !sym.Has(FlagParam) {
			t.Errorf("%s not flagged Param", name)
		}
	}
	if !fn.Symbols["a"].Has(FlagUsed) {
		t.Fatalf("a not marked used in body")
	}
}

func TestClassDef(t *testing.T) {
	m := build(t, "class C(Base):\n    x = 1\n    def m(self):\n        return self.x\n")
	if !m.Root.Symbols["C"].Has(FlagBound) {
		t.Fatalf("C not bound at module")
	}
	if !m.Root.Symbols["Base"].Has(FlagUsed) {
		t.Fatalf("Base not used at module")
	}
	cls := findChild(m.Root, ScopeClass, "C")
	if cls == nil {
		t.Fatalf("class scope missing")
	}
	if _, ok := cls.Symbols["x"]; !ok {
		t.Fatalf("class attr x not bound")
	}
	mfn := findChild(cls, ScopeFunction, "m")
	if mfn == nil {
		t.Fatalf("method m scope missing")
	}
	if !mfn.Symbols["self"].Has(FlagParam) {
		t.Fatalf("self not param")
	}
}

func TestFreeAndCellAcrossClosure(t *testing.T) {
	src := "def outer():\n    x = 1\n    def inner():\n        return x\n    return inner\n"
	m := build(t, src)
	outer := findChild(m.Root, ScopeFunction, "outer")
	inner := findChild(outer, ScopeFunction, "inner")
	if outer == nil || inner == nil {
		t.Fatalf("scopes missing")
	}
	if !outer.Symbols["x"].Has(FlagCell) {
		t.Fatalf("outer x flags=%b, want Cell", outer.Symbols["x"].Flags)
	}
	if !inner.Symbols["x"].Has(FlagFree) {
		t.Fatalf("inner x flags=%b, want Free", inner.Symbols["x"].Flags)
	}
}

func TestClassScopeSkippedByInnerFunction(t *testing.T) {
	src := "def outer():\n    x = 1\n    class C:\n        x = 2\n        def m(self):\n            return x\n"
	m := build(t, src)
	outer := findChild(m.Root, ScopeFunction, "outer")
	cls := findChild(outer, ScopeClass, "C")
	method := findChild(cls, ScopeFunction, "m")
	if outer == nil || cls == nil || method == nil {
		t.Fatalf("scopes missing")
	}
	if !method.Symbols["x"].Has(FlagFree) {
		t.Fatalf("method x flags=%b, want Free (resolved to outer, skipping class)", method.Symbols["x"].Flags)
	}
	if !outer.Symbols["x"].Has(FlagCell) {
		t.Fatalf("outer x flags=%b, want Cell", outer.Symbols["x"].Flags)
	}
	if cls.Symbols["x"].Has(FlagCell) {
		t.Fatalf("class x must not be Cell — class scope is invisible to nested functions")
	}
}

func TestGlobalDeclaration(t *testing.T) {
	src := "g = 0\ndef f():\n    global g\n    g = 1\n"
	m := build(t, src)
	fn := findChild(m.Root, ScopeFunction, "f")
	if fn == nil {
		t.Fatalf("missing scope")
	}
	if !fn.Symbols["g"].Has(FlagGlobal) {
		t.Fatalf("g not flagged Global in f")
	}
}

func TestNonlocalDeclaration(t *testing.T) {
	src := "def outer():\n    x = 0\n    def inner():\n        nonlocal x\n        x = 1\n"
	m := build(t, src)
	outer := findChild(m.Root, ScopeFunction, "outer")
	inner := findChild(outer, ScopeFunction, "inner")
	if !inner.Symbols["x"].Has(FlagNonlocal) {
		t.Fatalf("x not flagged Nonlocal in inner")
	}
}

func TestGlobalNonlocalConflictDiagnostic(t *testing.T) {
	src := "def outer():\n    x = 0\n    def inner():\n        global x\n        nonlocal x\n"
	m := build(t, src)
	if len(m.Diagnostics) == 0 {
		t.Fatalf("expected diagnostic for conflicting global/nonlocal")
	}
	d := m.Diagnostics[0]
	if d.Code != CodeGlobalAndNonlocal {
		t.Errorf("Code = %q, want %q", d.Code, CodeGlobalAndNonlocal)
	}
	if d.Severity != diag.SeverityWarning {
		t.Errorf("Severity = %v, want SeverityWarning", d.Severity)
	}
	if d.Pos.Lineno == 0 {
		t.Errorf("Pos not populated: %+v", d.Pos)
	}
	if d.Msg == "" {
		t.Errorf("Msg empty")
	}
}

func TestLambdaScope(t *testing.T) {
	m := build(t, "f = lambda x: x + y\n")
	lam := findChild(m.Root, ScopeLambda, "<lambda>")
	if lam == nil {
		t.Fatalf("no lambda scope")
	}
	if !lam.Symbols["x"].Has(FlagParam) {
		t.Fatalf("x not flagged Param in lambda")
	}
	if !lam.Symbols["x"].Has(FlagUsed) {
		t.Fatalf("x not used in lambda body")
	}
	if _, ok := lam.Symbols["y"]; !ok {
		t.Fatalf("y not recorded in lambda")
	}
}

func TestComprehensionScope(t *testing.T) {
	m := build(t, "xs = [i*2 for i in range(10)]\n")
	comp := findChild(m.Root, ScopeComprehension, "<comp>")
	if comp == nil {
		t.Fatalf("no comprehension scope")
	}
	if _, ok := comp.Symbols["i"]; !ok {
		t.Fatalf("i not bound in comprehension")
	}
	if _, ok := m.Root.Symbols["i"]; ok {
		t.Fatalf("i should NOT leak to module scope")
	}
}

func TestWalrusInComprehensionBindsOuter(t *testing.T) {
	src := "def f():\n    xs = [y for x in range(3) if (y := x + 1)]\n"
	m := build(t, src)
	fn := findChild(m.Root, ScopeFunction, "f")
	if fn == nil {
		t.Fatalf("no function scope")
	}
	if _, ok := fn.Symbols["y"]; !ok {
		t.Fatalf("walrus target y must bind in enclosing function scope, not the comprehension")
	}
}

func TestWalrusAtModule(t *testing.T) {
	m := build(t, "if (n := 10) > 5:\n    pass\n")
	if _, ok := m.Root.Symbols["n"]; !ok {
		t.Fatalf("walrus n not bound at module")
	}
}

func TestTypeAlias(t *testing.T) {
	m := build(t, "type Vec = list[int]\n")
	sym, ok := m.Root.Symbols["Vec"]
	if !ok {
		t.Fatalf("Vec not bound")
	}
	if !sym.Has(FlagBound) {
		t.Fatalf("Vec not flagged bound")
	}
}

func TestTypeParam(t *testing.T) {
	m := build(t, "def f[T](x: T) -> T:\n    return x\n")
	fn := findChild(m.Root, ScopeFunction, "f")
	if fn == nil {
		t.Fatalf("no f scope")
	}
	if _, ok := fn.Symbols["T"]; !ok {
		t.Fatalf("type param T not recorded in function scope")
	}
}

func TestAnnAssign(t *testing.T) {
	m := build(t, "x: int = 5\n")
	if !m.Root.Symbols["x"].Has(FlagBound) {
		t.Fatalf("annotated x not bound")
	}
	if !m.Root.Symbols["int"].Has(FlagUsed) {
		t.Fatalf("int annotation not marked used")
	}
}

func TestMatchPatternBindsName(t *testing.T) {
	src := "match v:\n    case [a, *rest]:\n        pass\n    case {1: x, **r}:\n        pass\n"
	m := build(t, src)
	if !m.Root.Symbols["v"].Has(FlagUsed) {
		t.Fatalf("subject v not used")
	}
	for _, name := range []string{"a", "rest", "x", "r"} {
		if _, ok := m.Root.Symbols[name]; !ok {
			t.Errorf("pattern target %s not bound", name)
		}
	}
}

func TestResolveCrossesFunctionBoundary(t *testing.T) {
	src := "x = 1\ndef f():\n    return x\n"
	m := build(t, src)
	fn := findChild(m.Root, ScopeFunction, "f")
	sym, crossed, owner := fn.Resolve("x")
	if sym == nil {
		t.Fatalf("Resolve(x) returned nil")
	}
	if !crossed {
		t.Fatalf("Resolve must report it crossed a function boundary")
	}
	if owner != m.Root {
		t.Fatalf("x should resolve to module scope")
	}
}

func TestResolveSkipsClassScope(t *testing.T) {
	src := "x = 1\nclass C:\n    x = 2\n    def m(self):\n        return x\n"
	m := build(t, src)
	cls := findChild(m.Root, ScopeClass, "C")
	method := findChild(cls, ScopeFunction, "m")
	_, _, owner := method.Resolve("x")
	if owner != m.Root {
		t.Fatalf("Resolve from method must skip class scope and land at module; got %v", owner)
	}
}

func TestImportTopName(t *testing.T) {
	if got := topImportName("a.b.c"); got != "a" {
		t.Errorf("topImportName(a.b.c) = %q, want a", got)
	}
	if got := topImportName("xyz"); got != "xyz" {
		t.Errorf("topImportName(xyz) = %q, want xyz", got)
	}
}

// TestBuildGrammarFixtures asserts that every curated grammar fixture
// produces a symbol table without panicking. Spec contract: "Build
// never panics on stdlib"; the curated corpus is the first line of
// defence for that property.
func TestBuildGrammarFixtures(t *testing.T) {
	matches, err := filepath.Glob("../tests/grammar/*.py")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Skip("no fixtures found")
	}
	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			f, err := parser.ParseString(path, string(data))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_ = Build(ast.FromFile(f))
		})
	}
}

func TestBuildNeverPanicsOnEmpty(t *testing.T) {
	m := build(t, "")
	if m.Root == nil {
		t.Fatalf("nil root for empty module")
	}
	if m.Root.Kind != ScopeModule {
		t.Fatalf("root kind = %v", m.Root.Kind)
	}
}
