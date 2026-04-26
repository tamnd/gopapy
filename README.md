<h1 align="center">gopapy</h1>

<p align="center">
  <b>Pure-Go parser for Python 3.14 — full grammar, ast.dump compatible, ~83x faster than participle.</b><br>
  <sub>No CPython at runtime.</sub>
</p>

---

## Using v2 (recommended)

```go
import "github.com/tamnd/gopapy/v2/parser2"

mod, err := parser2.ParseFile("example.py", src)
if err != nil {
    log.Fatal(err)
}
fmt.Println(parser2.DumpModule(mod))
```

**API surface** — four functions, zero dependencies beyond the standard library:

| Function | Description |
|---|---|
| `ParseFile(filename, src string) (*Module, error)` | Parse a whole Python module |
| `ParseExpression(src string) (Expr, error)` | Parse a single expression |
| `DumpModule(*Module) string` | Stable single-line dump (CPython ast.dump compatible) |
| `Dump(Expr) string` | Dump a single expression |

**Performance** vs v1 (darwin/arm64, Apple M4):

| Benchmark | v1 (participle) | v2 (parser2) | speedup |
|---|---|---|---|
| ParseFile (122-line module) | 2.67 ms/op, 0.86 MB/s | 32 us/op, 71 MB/s | ~83x |
| ParseExpression (corpus) | 3.59 ms/op, 0.20 MB/s | 20 us/op, 35 MB/s | ~177x |

**Migration** — if you are on v1, change the import path. The AST dump
format is identical; there are no field renames, no API removals.

```diff
-import "github.com/tamnd/gopapy/v1/parser"
+import "github.com/tamnd/gopapy/v2/parser2"

-f, err := parser.ParseFile(path, src)
-dump := ast.Dump(ast.FromFile(f))
+mod, err := parser2.ParseFile(path, src)
+dump := parser2.DumpModule(mod)
```

**Grammar coverage** — v2 passes all 85 CPython 3.14 grammar fixtures,
including PEP 634 match/case, PEP 695 type parameters, PEP 646 starred
subscripts, PEP 758 paren-less except, PEP 701 f-strings, PEP 750
t-strings, and the full Unicode identifier spec.

---

## v1 (maintenance only)

`github.com/tamnd/gopapy/v1` is maintained for compatibility. It receives
security and correctness fixes. All new features (formatter, type checker,
more lint checks) target v2 only. The CLI (`cmd/gopapy`) still routes
through v1 internally.

## Stability

v2 promises the same three things as v1:

- **AST node types are frozen.** No renames, no field removals, no
  field-type changes within a major version.
- **Public entry points are stable.** `ParseFile`, `ParseExpression`,
  `Dump`, `DumpModule` keep their current signatures.
- **The Go module path is `github.com/tamnd/gopapy/v2`.** Future
  breaking changes will move to `/v3`.

## Quick start (CLI)

```sh
go build ./cmd/gopapy
echo '1 + 2' | tee /tmp/x.py
./gopapy dump /tmp/x.py
# Module(body=[Expr(value=BinOp(left=Constant(value=1), op=Add(), right=Constant(value=2)))])
```

`gopapy check DIR` walks every `.py` under `DIR` and reports a pass/fail
summary.

## Stdlib parse

Both v1 and v2 parse the entire CPython 3.14 standard library without
error. The `stdlib-parse` CI job re-runs this check on every push.

## Tests

```sh
go test ./...        # unit tests: v1 and v2 modules
tests/run.sh         # cross-validate against CPython's ast.dump
```

`tests/run.sh` requires `python3` on PATH.

## License

MIT. See [LICENSE](LICENSE).
