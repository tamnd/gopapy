# Architecture

gopapy turns Python 3.14 source into the same `ast.Module` shape that
CPython's `ast.parse` produces, without linking against CPython.

The pipeline has four stages, each in its own package:

```
source bytes -> lex -> parser -> ast -> ast.Dump / FromFile
```

## Packages

### `lex`

Hand-written lexer for the full Python 3.14 token surface (see PEP 617's
"Tokens" section). It is responsible for:

- Producing the basic stream (NAME, NUMBER, STRING, OP, NEWLINE).
- Emitting INDENT and DEDENT tokens by tracking column at the start of
  each logical line. The dance is suppressed inside brackets so that a
  multi-line list or call does not try to indent.
- Emitting a single ENDMARKER at EOF and folding multiple blank
  NEWLINEs the way CPython does.

Bracket depth handling lives in `lex/indent.go`. The fix in
`6fc69f9` is the reason `[\n1,\n]` does not retrigger indentation when
the closing bracket lands back at column 0.

### `parser`

Thin wrapper around [participle v2](https://github.com/alecthomas/participle).
The grammar is split into two files:

- `grammar.go` covers statements, blocks, and the file production.
- `grammar_expr.go` covers expressions in CPython precedence order, with
  right-recursive rules for `**` and left-fold post-passes elsewhere.

A few rules use participle quirks worth knowing about:

- Literal tokens are inlined into capture tags. Participle silently
  drops unexported (`_`) struct fields, so `_ struct{} parser:"'('"` is
  a no-op and does not actually match the paren. Inline form is the
  workaround.
- `FromStmt` uses `(?! 'import')` negative lookahead so the dotted
  module name does not swallow the trailing `import` keyword.
- `Subscript` uses the `( ... )!` non-empty modifier so pure-colon
  slices like `a[::2]` parse with all three sub-fields optional.

### `ast`

Hand-written emitter (`emit.go`) plus generated nodes
(`nodes_gen.go`, `visit_gen.go`, `dump_gen.go`). The generator is
`internal/asdlgen`, driven by the vendored `Python.asdl` so that nodes
and field metadata cannot drift from upstream.

`FromFile` walks the participle parse tree and produces typed AST
nodes. `Dump` renders any node the way CPython 3.14's `ast.dump` does
by default (`show_empty=False`), using `nodeInfoTable` for field
ordering and optionality.

### `cmd/gopapy`

CLI front end with four subcommands: `parse`, `dump`, `check`,
`version`. `check` walks a directory of `.py` files and reports a
pass/fail summary, useful for pointing the parser at a corpus.

## Cross-validation

`tests/run.sh` builds gopapy, runs every fixture under `tests/grammar/`
through both `gopapy dump` and `internal/oracle/oracle.py` (which calls
real CPython), and diffs the output. Adding a fixture is the cheapest
way to assert grammar coverage.

## Generator workflow

When `Python.asdl` changes:

```
go run ./internal/asdlgen/cmd/gen-ast
```

This regenerates `ast/nodes_gen.go`, `ast/visit_gen.go`, and
`ast/dump_gen.go`. The hand-written `emit.go` and `dump.go` are not
touched.
