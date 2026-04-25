# Changelog

All notable changes to gopapy are recorded here. The format follows
[Keep a Changelog 1.1](https://keepachangelog.com/en/1.1.0/). Once
gopapy reaches 1.0 the project will follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); until
then, expect minor version bumps to sometimes include breaking
changes.

## [Unreleased]

## [0.0.2] - 2026-04-25

Second cut. The bootstrap surface widens to cover the constructs that
trip up real `.py` files in the wild: comprehensions, decorators, async
statements, walrus, parenthesized with, positional-only parameters,
star-unpacking in collection literals, the full Python escape table,
and a working f-string emitter for the common cases.

### Added

- Walrus assignment (`x := expr`) at any expression position. Parses
  through to a NamedExpr with Store context on the target name.
- Star-unpacking in list, tuple, set, and dict literals (`[*xs, 1]`,
  `{**a, **b}`, `(*xs,)`).
- Decorators on `def` and `class`. Multiple `@expr` lines stack onto
  the following definition. `async def` works under decorators too.
- Parenthesized `with` form (PEP 617): `with (a, b as c):`. Bare
  comma-separated items still work.
- Positional-only `/` marker (PEP 570) and bare-star `*` keyword-only
  marker (PEP 3102). emitArguments splits into Posonlyargs / Args /
  Kwonlyargs / KwDefaults exactly as CPython does.
- Single-element tuple disambiguation: `(x,)` is a Tuple, `(x)` is a
  parenthesized expression. The parser captures the trailing comma.
- List, set, dict, and generator comprehensions, including chained
  `for` clauses and trailing `if` filters. `async for` flips the
  comprehension's `is_async` flag.
- The single-genexp call form: `f(x for x in xs)` folds directly to a
  Call with one GeneratorExp arg, no extra parens needed.
- `async def`, `async for`, `async with` recognised via a soft-keyword
  prefix. `await` was already in place; it composes inside `async def`.
- f-string emission: any string-concat run with an `f` prefix turns
  into a JoinedStr. Interpolation chunks support `{expr}`, `{expr!r}`,
  `{expr!s}`, `{expr!a}`, `{expr:format_spec}`, `{{` / `}}` literal
  braces, and the debug `{x=}` shorthand.
- Octal `\NNN`, `\uHHHH`, `\UHHHHHHHH`, `\a`, `\b`, `\f`, `\v`, and
  backslash-newline line continuations in string literals.
- Sixteen new round-trip fixtures under `tests/grammar/` (015–030)
  exercising every construct above. The harness is now at 30/30.

### Known limits

The f-string emitter does brace-balanced text scanning but does not
yet handle nested f-strings inside an interpolation, triple-quoted
f-strings with embedded triples, or recursive parsing of brace nesting
that crosses string boundaries inside the expression. The lexer state
machine that fixes these is tracked for v0.0.3.

`match` statements, type parameters (PEP 695), `type` aliases, and
t-strings (PEP 750) remain deferred — each warrants its own PR.

## [0.0.1] - 2026-04-25

First public cut. The bootstrap branch covers enough of CPython 3.14's
grammar to parse a real `.py` file end to end and emit an AST that
diffs clean against `python3 -c 'import ast; print(ast.dump(...))'`.

### Added

- Hand-written lexer for the full Python 3.14 token surface, with
  INDENT, DEDENT, NEWLINE, and ENDMARKER injection. Bracket depth
  suppresses the indent dance, so multi-line lists and calls behave.
- Participle-based grammar split across `parser/grammar.go` (statements)
  and `parser/grammar_expr.go` (expressions). Power binds right, the
  rest fold left.
- AST node types, visitor, and dump tables generated from the vendored
  `Parser/Python.asdl`, so the node shape cannot drift from upstream.
- Hand-written emitter that turns the participle parse tree into typed
  AST nodes, including Load/Store/Del context inference.
- `Dump` matches CPython 3.14's `ast.dump` defaults (`show_empty=False`,
  Python repr quoting), so output diffs cleanly against the reference.
- `cmd/gopapy` CLI with `parse`, `dump`, `check`, `version`, and `help`
  subcommands. `check DIR` walks every `.py` and reports a pass/fail
  summary, useful for pointing the parser at a corpus.
- Cross-validation harness: `tests/run.sh` runs every fixture under
  `tests/grammar/` through both `gopapy dump` and `internal/oracle/oracle.py`
  (which calls real CPython) and diffs the output. 14 of 14 pass at
  release time.
- `docs/ARCHITECTURE.md` and `docs/GRAMMAR.md` covering the pipeline
  and which constructs land in this release vs PR2.
- GitHub Actions workflows: `ci` runs `go test` and the oracle diff on
  every PR; `build` cross-compiles for linux amd64+arm64, macOS
  amd64+arm64, and windows amd64; `release` fires on `v*.*.*` tags and
  publishes archives to the GitHub release page.

### Grammar covered

Statements: `pass`, `break`, `continue`, `return` (with tuple values),
`raise` (bare, with exception, `from`), `del`, `import` and `from`
(including relative dots), simple/augmented/annotated assignment,
`if`/`elif`/`else`, `while`, `for`, `try`/`except`/`else`/`finally`,
`def`, `class` (with bases and `metaclass=` keyword).

Expressions: unary `+`/`-`/`~`/`not`, boolean `and`/`or`, chained
comparisons (`<`, `<=`, `>`, `>=`, `==`, `!=`, `is`, `is not`, `in`,
`not in`), bitwise `|`/`^`/`&`/`<<`/`>>`, arithmetic
`+`/`-`/`*`/`/`/`//`/`%`/`@`/`**`, calls with `*args`/`**kwargs`,
subscripts and slices (including `a[::2]` and `a[1:2, 3]`), attribute
access, list/tuple/set/dict literals, name/number (int/float/complex)/
string (with implicit concatenation)/bytes/True/False/None/Ellipsis.

### Deferred to the next release

f-strings and t-strings (still tokenised as STRING), `match` statements,
walrus outside trivial contexts, type parameters, comprehensions and
generator expressions, `async`/`await` outside trivial expressions,
`with` statement, decorators, positional-only marker, star-unpacking in
literals, octal/binary/unicode-name string escapes.

[Unreleased]: https://github.com/tamnd/gopapy/compare/v0.0.2...HEAD
[0.0.2]: https://github.com/tamnd/gopapy/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/tamnd/gopapy/releases/tag/v0.0.1
