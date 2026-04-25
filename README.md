<h1 align="center">gopapy</h1>

<p align="center">
  <b>Pure-Go parser for Python 3.14 &mdash; full PEG grammar, ast.dump compatible.</b><br>
  <sub>Built on <a href="https://github.com/alecthomas/participle">participle</a>. No CPython at runtime.</sub>
</p>

---

`gopapy` reads Python 3.14 source and produces an AST that is byte-for-byte
compatible with `ast.dump(ast.parse(src), indent=2, include_attributes=True)`.
Every production in CPython's [PEG grammar](https://docs.python.org/3/reference/grammar.html)
is in scope &mdash; no opt-out subsets, no "we'll get to match-statements
later". Output node shape is generated from
[`Parser/Python.asdl`](https://github.com/python/cpython/blob/3.14/Parser/Python.asdl)
so it cannot drift from upstream.

This is the bootstrap branch. Track scope and progress in
[`docs/GRAMMAR.md`](docs/GRAMMAR.md). For a tour of the pipeline see
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Stability

From v0.1.0 onward, gopapy promises three things to downstream callers
so you can pin to v0.1 and trust the surface won't move under you:

- **AST node types are frozen.** No renames, no field removals, no
  field-type changes. New optional fields and new node variants for
  upstream-CPython grammar growth are allowed and land in patch
  releases.
- **Public parser entry points are stable.** `parser.ParseFile`,
  `parser.ParseString`, `parser.ParseExpression`, `parser.ParseReader`,
  `ast.Dump`, `ast.Unparse`, `ast.FromFile`, `lex.NewScanner`, and
  `lex.NewIndent` keep their current signatures and behavior.
- **The Go module path is `github.com/tamnd/gopapy/v1`.** Future
  breaking changes will move to `/v2`, so the import path itself
  enforces the contract.

Internal helpers under `internal/` are exempt and may move freely.

## Quick start

```sh
go build ./cmd/gopapy
echo '1 + 2' | tee /tmp/x.py
./gopapy dump /tmp/x.py
# Module(body=[Expr(value=BinOp(left=Constant(value=1), op=Add(), right=Constant(value=2)))])
```

`gopapy check DIR` walks every `.py` under `DIR` and reports a pass/fail
summary, which is convenient for pointing the parser at a corpus.

## Tests

```sh
go test ./...        # unit tests across lex, parser, ast
tests/run.sh         # cross-validate against CPython's ast.dump
```

`tests/run.sh` requires `python3` on PATH; it shells out to
`internal/oracle/oracle.py` for the reference output.

## License

MIT. See [LICENSE](LICENSE).
