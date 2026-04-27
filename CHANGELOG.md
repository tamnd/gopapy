# Changelog

All notable changes to gopapy are recorded here. The format follows
[Keep a Changelog 1.1](https://keepachangelog.com/en/1.1.0/). Once
gopapy reaches 1.0 the project will follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); until
then, expect minor version bumps to sometimes include breaking
changes.

## [Unreleased]

## [0.2.4] - 2026-04-26

### Added

- **`linter2`**: four new checks:
  - **F403** — star import (`from x import *`)
  - **F631** — `assert` with a non-empty tuple (always truthy)
  - **W291** — trailing whitespace on any line (raw-source scan, via `LintFile`)
  - **F901** — `raise NotImplemented` instead of `raise NotImplementedError`
- Total linter2 checks: 14 (ten ported from v1, four new). 30 tests.

## [0.2.3] - 2026-04-26

### Changed

- **`parse`, `symbols`, `diag` commands** now route through
  parser2+symbols2 instead of v1 parser+ast. Parser2 is ~83x faster on the
  122-line benchmark file (32 µs vs 2.67 ms).
- **`dump`** and **`check`** remain on v1 parser: `dump` needs CPython-
  compatible `ast.dump` field order and quoting; `check` needs complete
  Python 3.14 coverage (large ints, PEP 701 f-string `=`, etc.) that parser2
  does not yet provide.
- **`lint`** command still uses v1 linter.
- CLI version bumped to `0.2.3`.

## [0.2.2] - 2026-04-26

### Added

- **`github.com/tamnd/gopapy/v2/linter2`**: pyflakes-style linter built on
  parser2+symbols2. Ten checks: F401, F501, F541, F632, F811, F821, F841,
  E711, E712, W605. Entry points: `Lint(*parser2.Module)` for AST-only
  checks; `LintFile(filename, src)` for all checks including W605 (raw-source
  invalid-escape scan without any cst/lex dependency).
  22 tests, all green.

## [0.2.1] - 2026-04-26

### Added

- **`github.com/tamnd/gopapy/v2/symbols2`**: symbol table builder for the
  parser2 AST. Accepts `*parser2.Module`; positions use `parser2.Pos{Line, Col}`.
  Same API surface as v1 `symbols`: `Build`, `Module`, `Scope`, `ScopeKind`,
  `Binding`, `BindFlag`, and the `S001`/`S002`/`S003` diagnostic codes.
  Supports all scope kinds (module, function, class, lambda, comprehension),
  PEP 634 match pattern bindings, PEP 695 type params, walrus targets,
  global/nonlocal, free/cell resolution.
- **`github.com/tamnd/gopapy/v2/diag`**: shared v2 `Diagnostic` type using
  `parser2.Pos`. Used by symbols2 and future linter2.

## [0.2.0] - 2026-04-26

v2 declared the recommended import path. parser2 reaches 85/85
grammar fixture parity with v1 (PEP 634 match, PEP 695 type params,
PEP 646 starred subscripts, PEP 758 paren-less except, PEP 701
f-strings, PEP 750 t-strings, full Unicode identifier spec). v1
enters maintenance mode: security and correctness fixes only, no
new features. All new development targets v2.

### Changed

- **README** updated with the v2 usage section, bench numbers, and
  a one-import migration guide.
- **`v2/doc.go`** updated to reflect recommended-path status.
- **`parser/doc.go`** updated with a deprecation notice pointing to
  v2/parser2.
- **`cmd/gopapy/main.go`** version bumped to `0.2.0`. CLI behavior
  is unchanged.

### Performance (v0.1.34 baseline, darwin/arm64, Apple M4)

- v1 ParseFile: 2.67 ms/op, 0.86 MB/s
- v2 ParseFile: 32 us/op, 71 MB/s — **~83x faster**
- v2 ParseExpression: 20 us/op, 35 MB/s — **~177x faster** vs v1

### Notes

- The CLI (`cmd/gopapy`) still routes lint/format/lsp/check through
  v1. Porting those subsystems to v2 is roadmap v9.
- v1 module path `github.com/tamnd/gopapy/v1` continues to ship.
  Nothing forces a migration.

## [0.1.34] - 2026-04-26

Full fixture parity: parser2 now handles all 85 v1 grammar fixtures.
This release closes the four items deferred from v0.1.33: PEP 646
starred subscript and annotation positions, PEP 758 paren-less
except-tuple, and the UAX 31 unicode-tag identifier continuation
block. v1 is unchanged. The path to v0.2.0 (CLI swap) is now clear.

### Added

- `v2/parser2` — full CPython 3.14 grammar parity:

  - **PEP 646** — `*Ts` is now valid inside subscript position
    (`tuple[*Ts]`, `Callable[[*Args], R]`, `dict[str, *Vs]`)
    producing a `Starred` node, matching CPython's AST. Also
    accepted as an annotation on varargs: `def f(*args: *Ts)`.
  - **PEP 758** — paren-less except-tuple (`except A, B:` and
    `except* A, B:`) now accepted; the comma-separated types are
    collected into a `Tuple` expression, identical to the
    parenthesised form (`except (A, B):`).
  - **PEP 3131 / UAX 31** — identifiers may now contain codepoints
    in the `Other_ID_Continue` Unicode property, including the
    variation-selector supplement block (U+E0100..U+E01EF) that
    the fixture `x󠄀` exercises. Non-ASCII letters (`ä`, `蟒`) were
    already accepted; the fix is an `isOtherIDContinue` helper in
    the lexer.

### Performance

End-to-end module parse, darwin/arm64 (Apple M4), shared
`fileBenchSrc` (unchanged from v0.1.33 at 122 lines):

- v1 ParseFile: 2.67 ms/op, 0.86 MB/s
- v2 ParseFile: 32.2 us/op, 71.2 MB/s
- v2 is ~83x faster, ~83x higher throughput than v1.

### Notes

- The CLI still routes through v1. v0.2.0 will flip the default
  once symbols/lint/LSP can consume v2's AST.
- Grammar fixture pass rate: **85/85** (100%).

## [0.1.33] - 2026-04-26

PEP 695 type parameters land in parser2: the `type X = ...`
statement and the `[T, U: bound, *Ts, **P, T = default]` clause on
function and class headers, with PEP 696 defaults. After this
release the v2 parser handles 81 of the 85 v1 grammar fixtures;
the remaining four are explicitly deferred (PEP 646 starred
subscripts and annotations, PEP 758 except groups, the unicode-tag
identifier corner). v1 is unchanged.

### Added

- `v2/parser2` — `ParseFile` accepts the full PEP 695 type
  parameter clause:
  - `type Vector = list[float]` and `type Alias[T] = int`
    statements, with `type` treated as a soft keyword (`type = 1`,
    `type(x)`, `type.x` remain plain name uses).
  - `[T]`, `[T, U]` on function and class headers.
  - bound: `[T: int]`, `[T: (int, str)]`.
  - PEP 696 default: `[T = int]`, also on `*Ts` and `**P`.
  - `*Ts` (`TypeVarTuple`) and `**P` (`ParamSpec`) parameter
    forms.
  - All shapes combine: `def f[T: int, *Ts, **P](x): ...`.
- AST: `TypeAlias`, plus a `TypeParam` interface backed by
  `TypeVar`, `TypeVarTuple`, and `ParamSpec`. `FunctionDef`,
  `AsyncFunctionDef`, and `ClassDef` gain a `TypeParams` slice
  (nil when no clause is present).
- DumpModule renders the new nodes in the parens-explicit format;
  the `type_params=[...]` suffix is omitted when empty so legacy
  test cases stay unchanged.

### Performance

End-to-end module parse, darwin/arm64 (Apple M4), shared
`fileBenchSrc` (now 122 lines including a small generic function,
type alias, and generic class). Numbers from `BenchmarkParseFile`
(v2) and `BenchmarkParseFileV1Compare` (v1):

- v1 ParseFile: 2.45 ms/op, 0.94 MB/s
- v2 ParseFile: 32.0 us/op, 71.7 MB/s
- v2 is ~77x faster, ~76x higher throughput than v1 on the file
  bench. Expression bench (`BenchmarkParseExpression`):
  20.3 us/op v2 vs 3.59 ms/op v1 — ~177x faster.

### Notes

- The CLI still routes through v1. Switching `cmd/gopapy` to
  parser2 is reserved for v0.2.0 once symbols/lint/LSP can consume
  v2's AST.
- Remaining gaps (deferred): PEP 646 `*Ts` in subscript /
  annotation positions (065, 071), PEP 758 parenthesised except
  groups (070), the unicode-tag identifier corner (085).

## [0.1.32] - 2026-04-26

PEP 634 `match` / `case` lands in parser2. With this release the v2
parser handles 77 of the 85 v1 grammar fixtures cleanly; the eight
remaining misses are all features explicitly deferred (PEP 695
type-params, PEP 646 `*Ts` in subscript / annotation positions, PEP
758 except groups, the unicode-tag identifier corner). v1 is
unchanged.

### Added

- `v2/parser2` — `ParseFile` accepts the full PEP 634 match
  statement, including:
  - literal patterns: numbers, signed numbers (`-1`), strings,
    and complex-literal sums (`1+2j`, `-0-0j`),
  - singleton patterns (`None`, `True`, `False`),
  - capture and wildcard patterns (`name`, `_`),
  - value patterns: dotted attribute chains (`Color.RED`,
    `mod.Color.GREEN`),
  - sequence patterns: `[...]`, `(...)`, paren-less open form
    (`case 0, *rest:`), and the star sub-pattern,
  - mapping patterns with optional `**rest` capture,
  - class patterns with positional and keyword sub-patterns
    (`Point(x, y=0)`),
  - or-patterns (`a | b | c`),
  - as-patterns (`pat as name`),
  - guard expressions (`case n if n > 0:`).
- AST: `Match`, `MatchCase`, plus the eight `Pattern` node types
  (`MatchValue`, `MatchSingleton`, `MatchSequence`, `MatchMapping`,
  `MatchClass`, `MatchStar`, `MatchAs`, `MatchOr`).
- Soft-keyword treatment: `match` and `case` remain ordinary names
  unless they appear at statement start in the disambiguating
  positions PEP 634 specifies. `match = 1`, `match(...)`,
  `match.x`, `match[i]` still parse as plain name uses.
- Dump cases for the new nodes follow the existing parens-explicit
  format so match test cases stay inline.

### Performance

End-to-end module parse, darwin/arm64 (Apple M4), shared
`fileBenchSrc` (now 113 lines including the f-string + t-string
sample and a small match block). Numbers from `BenchmarkParseFile`
(v2) and `BenchmarkParseFileV1Compare` (v1):

- v1 ParseFile: 2.28 ms/op, 0.94 MB/s
- v2 ParseFile: 27.8 us/op, 77.2 MB/s
- v2 is ~82x faster, ~82x higher throughput than v1 on the file
  bench. Expression bench (`BenchmarkParseExpression`):
  18.7 us/op v2 vs 3.55 ms/op v1 — ~190x faster.

### Notes

- The CLI still routes through v1. Switching `cmd/gopapy` to
  parser2 is reserved for v0.2.0 once symbols/lint/LSP can consume
  v2's AST.
- Remaining gaps (deferred): PEP 695 type parameters, PEP 646 `*Ts`
  in subscript / annotation position, PEP 758 parenthesised except
  groups, the unicode-tag identifier corner.

## [0.1.31] - 2026-04-26

PEP 701 f-strings and PEP 750 t-strings land in parser2. Both forms
were the only string literals v0.1.30 still rejected; with this
release the v2 parser handles 66 of the 85 v1 grammar fixtures
cleanly, and every remaining miss is a feature explicitly deferred
to a later release (`match`, type-params, PEP 646/758, and the
unicode-tag identifier corner). v1 is unchanged.

### Added

- `v2/parser2` — `ParseFile` and `ParseExpression` accept f-string
  and t-string literals, including:
  - simple interpolation (`f"hi {name}"`),
  - conversion suffix (`!r`, `!s`, `!a`),
  - format spec (`f"{x:>10}"`, recursively nested:
    `f"{x:{w}.{p}}"`),
  - escaped braces (`{{`, `}}`),
  - raw f-strings (`rf"..."` / `fr"..."`),
  - triple-quoted variants (`f"""..."""`, `t'''...'''`),
  - PEP 701 quote/depth/backslash relaxations inside the
    expression part,
  - adjacent-literal concatenation across plain + f (`"a" f"b"`)
    via a single combined `JoinedStr`.
- AST: `JoinedStr`, `FormattedValue`, `TemplateStr`, `Interpolation`.
  Conversion codes use CPython's integer codes (`-1`, 114, 115, 97).
- DumpModule renders the new nodes in the existing parens-explicit
  format, so f-string test cases stay inline.
- Adjacent-literal mixing follows CPython rules: bytes won't combine
  with str/f/t; t-string only combines with another t-string.

### Performance

End-to-end module parse, darwin/arm64 (Apple M4), shared
`fileBenchSrc` (now 99 lines including an f-string + t-string
sample). Numbers from `BenchmarkParseFile` (v2) and
`BenchmarkParseFileV1Compare` (v1):

- v1 ParseFile: 2.01 ms/op, 0.89 MB/s, 4.45 MB/op, 52,363 allocs/op
- v2 ParseFile: 23.3 us/op, 76.9 MB/s, 24.8 KB/op, 579 allocs/op
- v2 is ~86x faster, ~86x higher throughput, ~179x less memory,
  ~90x fewer allocations than v1.

### Notes

- The CLI still routes through v1. Switching `cmd/gopapy` to parser2
  is reserved for v0.2.0 once symbols/lint/LSP can consume v2's AST.
- Remaining gaps (deferred): PEP 634 `match` (v0.1.32 candidate),
  PEP 695 type parameters, PEP 646 `*Ts` in subscript / annotation
  position, PEP 758 parenthesised except groups, the unicode-tag
  identifier corner.

## [0.1.30] - 2026-04-26

Statements land in parser2. The v2 module now parses whole Python
modules end-to-end via a new `ParseFile` entry point, with a
hand-written INDENT/DEDENT lexer modelled on CPython's tokenize and
the full statement AST (assign, aug-assign, ann-assign, control
flow, try/except, with, def/async def, class, decorators, imports,
global/nonlocal, del, assert, raise/from). f-strings, t-strings,
match, type-params, and PEP 646/758 edge forms remain deferred.
v1 is unchanged.

### Added

- `v2/parser2.ParseFile(filename, src)` and `ParseString` — return a
  `*Module` whose `Body` is a list of statement nodes. Backed by a
  new statement-mode scanner that maintains an indent stack, tracks
  bracket depth (no NEWLINE/INDENT/DEDENT inside `()[]{}`), and
  synthesises a trailing NEWLINE + DEDENT chain at EOF.
- Statement AST: `Module`, `ExprStmt`, `Assign` (chained targets),
  `AugAssign`, `AnnAssign`, `Return`, `Raise` (with `from`),
  `Pass`/`Break`/`Continue`, `Import`/`ImportFrom`/`Alias`,
  `Global`/`Nonlocal`, `Delete`, `Assert`, `If` (elif lowered to
  nested If), `While`, `For`/`AsyncFor`, `Try` with
  `ExceptHandler`/`else`/`finally`, `With`/`AsyncWith` with
  `WithItem`, `FunctionDef`/`AsyncFunctionDef`, `ClassDef`,
  decorators on def/async def/class.
- Expression AST: `Await`, `Yield`, `YieldFrom`. `await EXPR` parses
  at unary level; `yield`, `yield EXPR`, `yield from EXPR` parse at
  the top of `parseExpr`.
- Augmented assignment family: `+= -= *= /= //= %= **= &= |= ^= <<= >>= @=`.
- Function-parameter parsing covers positional-only `/`, keyword-only
  `*`, `*args`, `**kwargs`, defaults, and per-param annotations.
  Class bases accept positional, `*args`, `**kwargs`, and keyword
  forms (including `metaclass=`).
- DumpModule and per-statement dumpers using the existing
  parens-explicit single-line format, so test cases stay inline.

### Performance

End-to-end module parse, darwin/arm64 (Apple M4), corpus is the
93-line `fileBenchSrc` shared between
`parser/bench_v2_compare_test.go` and `v2/parser2/bench_test.go`:

- v1 ParseFile: 1.87 ms/op, 0.88 MB/s, 4.24 MB/op, 49,982 allocs/op
- v2 ParseFile: 20.0 us/op, 82.1 MB/s, 19.4 KB/op, 481 allocs/op
- v2 is ~94x faster, ~93x higher throughput, ~219x less memory,
  ~104x fewer allocations than v1.

### Notes

- v0.1.30 is feature-complete enough to parse 47 of 85 v1 grammar
  fixtures cleanly; the 38 misses are entirely accounted for by
  features explicitly deferred to later releases (f/t-strings,
  match, type-params, PEP 646/758, the `x󠄀` unicode-tag identifier
  edge case).
- v1 remains the default in `cmd/gopapy`. Switching the CLI to
  parser2 is its own release once symbols/lint/LSP can consume v2's
  AST.

## [0.1.29] - 2026-04-26

Full expression coverage in parser2. Everything in Python's expression
grammar (except f-strings and t-strings, deferred to v0.1.31) now
parses through the hand-written recursive-descent path. v1 is
unchanged.

### Added

- `v2/parser2` — `ParseExpression` now handles every expression form
  the v1 parser does, except f/t-strings:
  - Literal extensions: hex (`0xFF`), octal (`0o17`), binary
    (`0b1010`), underscore separators (`1_000_000`), complex
    suffix (`3j`), bytes/raw string prefixes (`b"..."`, `r"..."`),
    ellipsis (`...`), adjacent string-literal concatenation.
  - Power (`**`, right-associative; honors Python's `-2 ** 2`
    quirk), floor-div (`//`), modulo (`%`), matmul (`@`).
  - Bitwise: `|`, `&`, `^`, `<<`, `>>`, with full precedence
    ladder.
  - Comparisons including chained form (`1 < 2 < 3`), `in`,
    `not in`, `is`, `is not`.
  - Boolean ops `and`/`or` with short-circuit precedence, `not`.
  - Conditional expression `a if cond else b` (right-associative).
  - Attribute access, subscript including slices (`a[1:2:3]`,
    `a[:]`, `a[::2]`, advanced tuple indexing), call with
    keyword arguments, `*args`, `**kwargs`, single-genexp arg.
  - Collection literals: list, tuple, set, dict, including
    starred unpacking (`[*xs, y]`) and dict-merge (`{**other,
    "k": v}`).
  - Comprehensions: list, set, dict, generator; nested `for`
    clauses with `if` filters and `async for`.
  - Lambda with `/`, `*`, `**`, defaults, kwonly.
  - Walrus (`(x := 5)`).
- f-string and t-string literals raise a clear "not implemented in
  v0.1.29" error rather than silently mis-parsing. They land in
  v0.1.31.

### Changed

- `v2/parser2/bench_test.go` corpus expanded to ~80 entries spanning
  every supported form. The v1-vs-v2 comparison
  (`parser/bench_v2_compare_test.go`) mirrors the same list. Numbers
  on darwin/arm64 (Apple M4):
  - v1: ~3.58 ms/op, 0.20 MB/s, 8.5 MB/op, 98412 allocs/op
  - v2: ~17.4 us/op, 40.5 MB/s, 28 KB/op, 542 allocs/op
  - v2 is ~206x faster, ~200x higher throughput, ~300x less
    memory, ~180x fewer allocations than v1 on this corpus.

### Notes

- v2 still does not parse statements or files. `ParseFile`,
  INDENT/DEDENT handling, and the full `Module` AST land in v0.1.30.
- The bench corpus is now a fixed contract: every parser2 PR
  re-runs both columns against this same list so progress stays
  comparable across versions.

## [0.1.28] - 2026-04-26

The first parser2 version. A new module, `github.com/tamnd/gopapy/v2`,
ships alongside v1 and contains the start of the hand-written
recursive-descent parser. v1 is unchanged and continues to ship from
the existing module path; nothing forces a migration.

### Added

- `v2/` — new module root. `v2/go.mod` declares
  `module github.com/tamnd/gopapy/v2`. `go.work` at the repo root
  links both modules so local development sees them together.
- `v2/parser2/` — hand-written recursive-descent expression parser.
  `ParseExpression(src string) (Expr, error)` covers literals (int,
  float, string, None, True, False), names, parenthesized
  expressions, unary operators (+, -, not, ~), and binary
  arithmetic (+, -, *, /) with correct precedence and
  left-associativity. Anything outside this surface returns a
  position-tagged error.
- `v2/parser2/parser_test.go` — table-driven coverage for every
  implemented form plus precedence corners and error cases.
- `v2/parser2/bench_test.go` and a parallel
  `parser/bench_v2_compare_test.go` benching the same expression
  corpus through v1's `parser.ParseExpression` and v2's
  `parser2.ParseExpression`. On the implemented subset v2 lands
  about 318x faster and 210x fewer allocations than v1; the gap
  will compress as v2's grammar grows.
- CI runs `go vet`, `go build`, `go test`, and `go test -bench`
  across both modules.

### Notes

- v2 is self-contained. It does not import v1's `ast` or `lex`
  packages because v1's module path (`/v1` suffix with v0.x.x
  tags) collides with Go's strict major-version rule. v2 carries
  its own minimal AST types whose shapes mirror v1's; convergence
  on a shared types module is a roadmap-v9 concern.
- Per the parser-swap policy, v1 stays alive indefinitely. v2 is
  experimental today and grows over v0.1.29 (statements + the rest
  of expressions) and v0.1.30 (f-strings, t-strings).

## [0.1.27] - 2026-04-26

Maintenance release that prepares the parser for the v2 split. No
user-visible behavior change; the value is the policy:
`github.com/tamnd/gopapy/v1/parser` is feature-frozen as of this
version, and the hand-written recursive-descent parser will land
under `github.com/tamnd/gopapy/v2` starting at v0.1.28.

### Added

- `parser/doc.go` documenting the v0.1 freeze and the v2 migration
  path. v1 keeps shipping bug fixes; new constructs and the parser
  rewrite target v2.
- `parser/api_test.go` with explicit smoke tests for every exported
  entry point: `ParseFile`, `ParseString`, `ParseExpression`,
  `ParseReader`. The wider parser_test.go covers grammar
  correctness; this file locks in the public surface so a parser2
  build that drops or renames any of these functions fails CI
  immediately.

### Notes

- v1 is the maintenance line going forward. Anyone pinned to the
  current `parser.File` AST shape (including its participle-era
  Pos quirks) stays on v1 indefinitely. v2 ships in parallel and
  is the path for new development.
- Round-trip safety net is already in CI via `gopapy unparse
  --check stdlib`; nothing added on that front.

## [0.1.26] - 2026-04-26

`gopapy lsp` learns `textDocument/codeAction`. The v0.1.25 LSP shipped
the squiggle; this version closes the loop — press the editor's
quick-fix shortcut on any gopapy diagnostic and the buffer rewrites
itself the same way `gopapy lint --fix` would on disk.

The MVP is one CodeAction titled `gopapy: fix all`. Per-diagnostic
targeting (one quick-fix per squiggle, with a precise range edit)
needs the fixer to expose `(range, replacement)` pairs and is its
own follow-up batch.

### Added

- **`textDocument/codeAction`** request handler. Returns a single
  `gopapy: fix all` action of kind `quickfix` whenever
  `linter.FixWithConfig` would change the buffer. The edit is a
  WorkspaceEdit that replaces the document with the post-Fix
  unparse. `isPreferred: true` so editors highlight it as the
  default action on the lightbulb.
- `initialize` response now advertises `codeActionProvider: true`
  in capabilities so clients enable the lightbulb UI.
- Honors LSP's `context.only` filter: a request limited to e.g.
  `["refactor"]` returns `[]` because our action is `quickfix`.

### Notes

- Empty array is the right answer in three cases: the URI isn't
  open, the source can't parse, or the fixer would change nothing.
  Editors treat `[]` as "no actions available" so we never offer
  a lightbulb that wouldn't actually do something.
- The `range.end` covers the whole document. Editors apply this as
  a full-document replacement, which loses cursor position relative
  to a per-edit fix; that's the cost of "fix all" being a single
  WorkspaceEdit. Cursor preservation comes back when we ship
  per-diagnostic targeted edits.
- Out of scope: `source.fixAll.gopapy` source-action kind (runs on
  save), per-diagnostic targeted edits, code actions on unsaved
  ranges that haven't been linted yet,
  `textDocument/codeAction/resolve` two-phase resolve.

## [0.1.25] - 2026-04-26

`gopapy lsp` runs the language-server loop on stdio. Any LSP-aware
editor (VS Code, Neovim, Helix, Zed, Sublime LSP) can wire it up
and get gopapy diagnostics inline as the buffer changes — no save,
no glue script, no file watcher.

Scope is intentionally narrow: diagnostics-only. Code actions
(quick-fix from `--fix`), hover, completion, and definition stay
out so this batch can ship behind a stable interface. Adding any
of them later is a self-contained follow-up that doesn't disturb
the wire format set here.

### Added

- **`gopapy lsp`** subcommand. No flags — the editor manages the
  lifecycle (start when a Python buffer opens in a configured
  workspace, send shutdown+exit on close). Transport is stdio with
  the LSP `Content-Length: N\r\n\r\n` framing prefix and JSON-RPC
  2.0 in the body.
- LSP methods: `initialize` (capabilities advertise full-content
  textDocumentSync and the gopapy serverInfo), `initialized`,
  `shutdown`, `exit`, `textDocument/didOpen`,
  `textDocument/didChange`, `textDocument/didClose`. Anything else
  the client sends gets a JSON-RPC method-not-found response (for
  requests) or is ignored (for notifications).
- `textDocument/publishDiagnostics` notifications fire after every
  successful lint. Diagnostic mapping: gopapy 1-indexed line
  becomes LSP 0-indexed; column passes through (already 0-indexed
  in gopapy); severity SeverityError → 1, SeverityWarning → 2,
  SeverityHint → 4. `source` is `"gopapy"`, `code` carries the
  diagnostic ID ("F401", "E711", …).
- `lsp` package under the v1 module exposing `lsp.Serve(r, w) error`
  for downstream Go programs that want to embed the server in
  their own host process.

### Notes

- Full-content sync (`textDocumentSync.change = 1`) instead of
  incremental. The parser is fast enough on human-scale files that
  resending the buffer per keystroke isn't the bottleneck, and
  full sync keeps the document store a flat `map[uri][]byte`.
- Parse failures surface as a single error-level diagnostic at line
  1, col 0. Mid-edit the buffer is unparseable constantly; we'd
  rather show that gopapy is alive and saw the change than leave
  stale squiggles from the last successful parse.
- `# noqa` suppression and `pyproject.toml` config discovery work
  in LSP mode, identical to `gopapy lint` from the CLI. Workspace
  config is cached per directory so a typing burst doesn't re-walk
  the filesystem on every keystroke.
- Out of scope this batch: `textDocument/codeAction` (quick-fix from
  `--fix`), `textDocument/hover`, completion, definition, workspace
  folders / multi-root, incremental sync, `workspace/configuration`
  pull, semantic tokens, inlay hints. Each is a self-contained
  follow-up.

## [0.1.24] - 2026-04-26

`gopapy lint --format sarif` writes a SARIF 2.1.0 log document.
GitHub code scanning, Azure DevOps Defender, and most enterprise
CI dashboards consume SARIF natively, so a `gopapy lint` step in
a workflow can now feed historical-tracking views without a glue
script. The format also has a stable schema (2.1.0 has been
current since 2020), which keeps the contract from drifting.

Unlike text/json/github (NDJSON, one diagnostic per line), SARIF
is a single JSON object: `results[]` lives inside `runs[0]` inside
the top-level document. The CLI handles this by collecting all
diagnostics for the run and writing once at the end.

### Added

- **`--format sarif`** writes the run as a SARIF 2.1.0 log to
  the configured sink (stdout by default, `--output PATH` writes
  to a file). Severities map: SeverityError → `"error"`,
  SeverityWarning → `"warning"`, SeverityHint → `"note"`. File
  paths are forward-slashed via `filepath.ToSlash` (no-op on
  Unix, normalises Windows paths for the URI form).
- `linter.FormatSARIF` constant + `linter.WriteSARIFLog(w, diags,
  tool)` whole-batch writer + `linter.ToolInfo{Name, Version,
  InformationURI}` struct. The library doesn't bake in the tool
  identity — the CLI passes "gopapy" + the build version, but
  downstream Go programs can supply their own.

### Notes

- `WriteDiagnostic(w, d, FormatSARIF)` returns an error rather
  than emitting a partial document. Use `WriteSARIFLog` instead.
- `tool.driver.rules[]` is intentionally omitted. SARIF allows
  results to reference rule IDs without a corresponding entry,
  and skipping the table keeps the output stable as new codes
  ship.
- This was the originally-planned v0.1.23 slot in roadmap v7;
  it shifted because v0.1.23 took on W605 + source-byte plumbing.
  LSP minimum-viable moves to v0.1.25.

## [0.1.23] - 2026-04-26

W605 (`invalid-escape-sequence`) lands. The check needs raw source
bytes — by the time we have a Constant node, the string-fold has
resolved `"\\p"` and `"\p"` to the same value, so the AST can't
distinguish them. The substrate already exposed the raw bytes via
`cst.File.Tokens()`; what was missing was a path through the linter
that gave a check access to them.

The plumbing is small: `LintFile` already parses through `cst.Parse`,
so the source-aware checks run after the AST pass and merge into
the same diagnostic stream. `Lint(mod)` (the AST-only entry point)
deliberately doesn't run them — there's no source to give. Callers
who want full coverage use `LintFile` / `LintFiles`, which is what
the CLI and the parallel path already do.

### Added

- **W605** (`invalid-escape-sequence`) fires on `"\p"`, `"\d"`, and
  the rest of the unrecognized-escape family in non-raw string
  literals. Raw strings (`r"\p"`, `R"\p"`) are exempt — backslashes
  there are literal. F-strings and t-strings are scanned only
  outside `{...}` placeholders; the placeholder itself is a Python
  expression (or a format spec) and has its own rules.
- Bytes literals get the same treatment with str-only escapes
  excluded: `b"A"`, `b"\N{LATIN ...}"` fire because `\u`,
  `\U`, `\N` are str-only.

### Notes

- No auto-fix. `r"\p"` vs `"\\p"` is an intent question only the
  author can answer; pycodestyle / pyflakes also leave it alone.
- `linter.AllCodes()` returns 10 (was 9).
- `Lint(mod)` does not surface W605 — documented limitation. Use
  `LintFile`. Same shape as `# noqa` not applying via `Lint`.

## [0.1.22] - 2026-04-26

The first two pycodestyle checks land. Both target the same pattern
in different garb: `==` / `!=` against a singleton where Python wants
`is` / `is not`. Comparing to `None` is the textbook PEP 8 example;
comparing to `True` / `False` is the same mistake with a different
constant.

Both checks have safe auto-fixes — operator rewrites (`==` → `is`,
`!=` → `is not`) that preserve identity for the documented cases.
The fix stops short of the truthiness rewrite (`if x == True:` →
`if x:`) because that's behavior-changing for non-bool truthy values.

W605 (`invalid-escape-sequence`) was the third roadmap-v7 starter
but needs raw source-byte access — by the time we have a `Constant`
node, the string-fold has resolved `"\\p"` and `"\p"` to the same
value. Threading bytes into the linter substrate is the v0.1.23
focus.

### Added

- **E711** (`comparison-to-none`) fires on `x == None`, `None == x`,
  `x != None`, and chains like `a == None == b` (each slot is
  checked independently). `is None` / `is not None` stay silent —
  the canonical form is the whole point.
- **E712** (`comparison-to-bool`) fires on `x == True`, `True == x`,
  `x != True`, and the `False` mirrors. `is True` / `is False` stay
  silent (they're the canonical form for E712 — F632 still flags
  them as bool-literal `is` comparisons).
- Auto-fix for both: rewrites the operator slot facing the literal
  from `==` to `is` and `!=` to `is not`. Per-file ignores apply,
  so a project that only wants E711 in `src/` and not in `tests/`
  gets exactly that.

### Notes

- `linter.AllCodes()` returns 9 (was 7).
- Auto-fix preserves operator chain structure: `a == None == b`
  rewrites to `a is None is b` (two slots, two fixes).

## [0.1.21] - 2026-04-26

F632 widens its right-hand-side set to include the named singletons
`Ellipsis` and `NotImplemented`. The bare-dots literal `...` was
already covered (it parses as `Constant(kind=Ellipsis)`), but the
spelled-out names — which parse as `Name`, not `Constant` — slipped
through. Pyflakes flags both; gopapy now matches.

`type(x) is type(y)` is intentionally *not* flagged: that shape is
the canonical "exactly the same type, ignoring inheritance" check
in Python and pyflakes leaves it alone too.

No new codes; F632 just covers more of the same shape.

### Added

- F632 now fires on `x is Ellipsis`, `x is NotImplemented`, and
  the `is not` counterparts. The check stays scoped to the
  right-hand side of `is` / `is not`; identity comparison via `==`
  is fine and stays unflagged.

### Notes

- Two items from roadmap v6's "remaining batch" are not in this
  release because the gaps don't actually exist:
  - F811 on `from X import Y` redefinition has worked since
    v0.1.13 (the symbol table treats every alias as a binding).
  - F541 on t-strings stays suppressed by design — t-strings are
    `Template` objects, not strings, so the `t` prefix has runtime
    semantics that the prefix-only-noise lint doesn't apply to.

## [0.1.20] - 2026-04-26

`gopapy lint` now walks files in parallel by default and ships an
opt-in result cache. The two changes are independent: parallel
helps the cold-cache run (everything has to be parsed once);
caching helps the warm-cache loop (CI on a feature branch, an
editor save loop, anything that re-runs the same lint).

Output is byte-identical to v0.1.19 for any (file set, config).
Determinism is the contract: `--jobs 1` and `--jobs 8` produce the
same stdout, in the same order, for the same input. Diagnostics
are collected per-file then emitted in lexical path order, the
same order single-threaded `filepath.WalkDir` produced before.

### Added

- `linter.LintFiles(paths, cfg, opts) []FileResult` — public
  worker-pool entry point. Workers default to `runtime.GOMAXPROCS(0)`;
  `opts.Jobs = 1` forces serial execution. Per-file errors stay
  per-file: a parse failure on file N doesn't stop work on N+1.
  `linter.SortPaths(paths)` is the small helper for callers that
  want LintFiles' deterministic-output guarantee but haven't sorted
  their input yet.
- `linter.Cache` plus `linter.OpenCache(path, warn)` and
  `linter.DefaultCachePath()` — opt-in result cache keyed on
  `(absolute path, mtime, size, config-hash)`. A miss on any field
  re-lints. Corrupt cache files trigger a warning and an empty
  cache; they never block the run. Safe for concurrent use.
- `gopapy lint --jobs N` selects worker count. `--jobs 1` runs
  serially (handy for debugging output drift). `--jobs 0` is
  rejected so `0` doesn't silently fall back to GOMAXPROCS.
- `gopapy lint --cache [PATH]` enables the cache, defaulting to
  `$XDG_CACHE_HOME/gopapy/lint.cache` when PATH is omitted.
  `--no-cache` disables it for one run regardless of any other
  flag — useful for forcing a clean re-lint without deleting the
  file.

### Notes

- `--fix` keeps the serial loop. Auto-fix mutates files; the cache
  is invalidated by every successful fix anyway, so neither helper
  buys anything in fix mode.
- The cache lives in user cache dir, not the project tree, so
  switching branches doesn't invalidate everything (only files
  that actually differ across branches will miss). Add it to your
  global `.gitignore` only if your home dir is checked in for some
  reason.
- The on-disk format is gob; treat it as opaque. Future versions
  may change the schema with a one-warning, empty-cache reset.

## [0.1.19] - 2026-04-26

Two of the most-cited pyflakes codes were still missing: F821
(undefined name) catches typos and dangling references; F501
(`%`-format mismatch) catches the classic `"%s %s" % (1,)` shape.
Both ship as pure consumers of what the linter substrate already
exposes — F821 reuses `symbols.Scope.Resolve` (which already
implements Python's class-scope skip rule for nested defs); F501
walks `BinOp(Mod)` nodes and counts `%X` codes in the literal.
`linter.AllCodes()` now returns 7 codes.

Neither check has an auto-fix. F821 doesn't know what name the user
*meant*; F501 doesn't know what arguments the user *wanted*.
Pyflakes leaves both alone too.

### Added

- `F821` — undefined name. A `Name` reference in `Load` context
  fires when no scope on its lookup chain binds the identifier and
  it isn't a CPython 3.14 builtin. Class scopes are invisible to
  nested functions (matching Python and pyflakes), so a method that
  references a class-only attribute fires. A module-level `from X
  import *` suppresses F821 across the module — the wildcard could
  bring in any name and we can't know.
- `F501` — `%`-format string with a wrong argument count. Matches
  `"<fmt>" % args` where the right side is a tuple (count must
  equal the number of `%X` conversions in the literal) or a single
  value (must equal exactly one conversion). Dict right-hand sides
  are skipped because they use keyed conversions; that's a separate
  pyflakes code (F502) we haven't shipped.
- Baseline fixtures `linter/testdata/baseline/pct_fmt.py` and
  `undef.py` lock in the expected line/column for each new code.

## [0.1.18] - 2026-04-26

Editors lint unsaved buffers. Until v0.1.18 anyone wiring `gopapy
lint` into a Neovim or VS Code plugin had to write the buffer to a
tempfile per keystroke — fragile and slow. v0.1.18 adds the standard
stdin convention so the editor can pipe the buffer in and read
diagnostics out of one subprocess. The auto-fix path splits cleanly:
rewritten source on stdout, diagnostics on stderr. This is the LSP
precursor; the protocol layer is still a separate epic.

### Added

- `gopapy lint -` reads source from stdin and lints it as a single
  file. Empty stdin produces no diagnostics and exits 0; the parser
  already handles the empty-module case.
- `gopapy lint --stdin-filename PATH` gives the in-memory buffer a
  logical name. The path drives three things: the `Filename` field
  on every emitted diagnostic (so editor jump-to-source works), the
  per-file ignore matcher (so `tests/*` matches an unsaved
  `tests/foo.py` buffer), and the config-discovery anchor (so the
  right `pyproject.toml` is found even when the working dir is
  somewhere else). The path is logical only; gopapy never reads or
  stats it.
- `gopapy lint --fix -` writes the rewritten source to the configured
  output sink (stdout by default; `--output PATH` redirects) and
  the remaining diagnostics to stderr. Editors pipe stdout back into
  the buffer and render stderr as squiggles. When nothing is
  fixable, stdout still receives the source verbatim so the editor
  doesn't have to special-case "no change".

## [0.1.17] - 2026-04-26

A linter that prints to a terminal is half a tool. CI pipelines need
structured output so a `gopapy lint` step can fail a build with
clickable annotations; editors need machine-readable diagnostics so
they don't have to parse the human format. v0.1.17 adds two more
output formats and a `--output PATH` sink so the integration story
matches what ruff and flake8 ship.

The text default is byte-for-byte unchanged. Library callers that
build their own output don't see any of this. Existing v0.1.16
scripts using `gopapy lint --json` keep working — `--json` is now a
deprecated alias for `--format json` that uses the new flat schema.

### Added

- `linter.Format` (`text`, `json`, `github`) and
  `linter.ParseFormat(s)` to convert the CLI string. Unknown values
  produce an error that lists the accepted choices.
- `linter.WriteDiagnostic(w, d, format)` writes one diagnostic in
  the requested format with the trailing newline. Streams one at a
  time so a 50k warning run stays bounded in memory.
- `gopapy lint --format {text,json,github}` chooses the encoding.
  `text` is the default and matches v0.1.16. `json` emits NDJSON
  using a flat `{filename, line, column, end_line, end_column,
  severity, code, message}` schema that overlaps with `ruff
  --output-format json` so a tool that consumes ruff's output can
  consume gopapy's. `github` emits GitHub Actions workflow command
  lines (`::warning file=...,line=...,col=...::CODE message`) so a
  `gopapy lint` step in a workflow surfaces inline PR annotations
  with no glue script.
- `gopapy lint --output PATH` writes the diagnostic stream to a
  file; `-` is stdout (the default). The `loaded config from PATH`
  line and the `N files, M warnings` trailer stay on stderr so
  machine consumers see only diagnostics on the chosen sink.

### Changed

- `gopapy lint --json` now emits the v0.1.17 flat JSON schema
  instead of the v0.1.16 nested-pos shape. Field names switched
  from `{filename, pos:{...}, end:{...}, severity, code, msg}` to
  `{filename, line, column, end_line, end_column, severity, code,
  message}`. The library-level `diag.Diagnostic.MarshalJSON` keeps
  its original shape — the change is CLI-only.

## [0.1.16] - 2026-04-26

A linter without a config story is a linter you fight. v0.1.16 gives
gopapy one: a `[tool.gopapy.lint]` section in `pyproject.toml`, a
`Config` struct that backs it, walk-up discovery so any file inside a
project picks up the right rules, and `--config` / `--no-config` CLI
flags for the cases where discovery is wrong. Per-file ignores arrive
in the same shape ruff and flake8 use, so existing muscle memory
transfers. The auto-fix loop honours the same gates: a project that
ignores F401 for `tests/*` won't have its test imports rewritten
under `gopapy lint --fix`.

### Added

- `linter.Config{Select, Ignore, PerFile}`: a single struct that
  decides which checks fire for which files. `Enabled(code)` answers
  the global question; `EnabledFor(filename, code)` layers per-file
  globs on top. `Validate()` rejects unknown codes at load time so
  typos in a config surface as a clear error rather than silent
  no-ops. `AllCodes()` enumerates every code the linter knows about.
- `linter.LoadConfig(path)` reads a `pyproject.toml` and returns the
  parsed `Config`. Co-existence with non-gopapy sections (`[tool.poetry]`,
  `[tool.ruff]`, `[build-system]`) is mandatory; only the
  `[tool.gopapy.lint]` table is consumed. Malformed TOML, missing
  files, and unknown codes all surface as errors.
- `linter.DiscoverConfig(start)` walks up parent directories from
  `start` (file or dir) until it finds a `pyproject.toml` with a
  `[tool.gopapy.lint]` table. Returns the parsed `Config`, the
  resolved path, and any load error. No config found is not an
  error: callers get a zero `Config` and an empty path.
- `linter.LintWithConfig(mod, cfg)` and
  `linter.LintFileWithConfig(filename, src, cfg)` are config-aware
  twins of `Lint` / `LintFile`. The `File` variant applies per-file
  ignores keyed off `filename`; the bare-module variant only honours
  global `Select` / `Ignore`.
- `linter.FixWithConfig(mod, cfg, filename)` gates each fix kind by
  the config: F401 fix runs only when `F401` is enabled for that
  file, F811 fix only when `F811` is enabled. Per-file-ignore lets a
  project preserve unused imports in `__init__.py` re-export hubs
  while still pruning them everywhere else.
- `gopapy lint --config PATH` loads an explicit config file and skips
  discovery. `gopapy lint --no-config` skips discovery without
  loading anything; CLI default arguments still apply. When a config
  is loaded (whether by discovery or `--config`), the resolved path
  is echoed to stderr as `loaded config from PATH` so it's obvious
  which file is in effect.
- `github.com/BurntSushi/toml v1.6.0` enters `go.mod` as the first
  non-test runtime dependency. The TOML parser is small, dependency-free,
  widely used, and battle-tested against real-world `pyproject.toml`
  files.

## [0.1.15] - 2026-04-26

More pyflakes coverage at zero substrate cost. v0.1.13 shipped three
checks; v0.1.14 wired up the fix loop; v0.1.15 takes the substrate
out for another spin and adds three more checks plus one more fix.
After this version the linter has six checks, two fixes, and parity
with the high-frequency end of pyflakes' catalogue.

### Added

- `linter` check `F541` (f-string without any placeholders): fires
  on `f"hello"`, `f""`, `f'no interpolation'`. Identifies any
  `JoinedStr` whose values contain no `FormattedValue` /
  `Interpolation`. The fix is deferred until `cst`-level prefix
  preservation lands; the unparser today loses original `b` / `r` /
  `u` prefixes too.
- `linter` check `F632` (`is` / `is not` against a literal): fires
  on `x is 1`, `x is "foo"`, `x is True`, `x is (1, 2)`, `x is -1`.
  `is None` and `is not None` stay silent — that's the canonical
  identity idiom. No auto-fix; the right replacement (`==` vs `!=`)
  depends on intent the linter can't see.
- `linter.Fix` now removes `name = CONSTANT` statements in
  function/method bodies when the immediately-next statement rebinds
  the same name. Adjacency is the safety guarantee: no read of `name`
  can sit between the two statements, so dropping the literal store
  preserves observable behavior. Side-effect-bearing right-hand
  sides (`x = expensive(); x = 2`) stay untouched.

## [0.1.14] - 2026-04-26

The substrate's first end-to-end loop. v0.1.13 reported problems;
v0.1.14 *fixes* them. `Transformer` rewrites the AST, the linter
decides what's safe to rewrite, and `cst.Unparse` writes the result
back. `gopapy lint --fix` is the user-visible payoff: pyflakes finds
unused imports, ruff removes them, and now gopapy does both.

Auto-fix is intentionally narrow: only F401. F811 and F841 stay
hands-off because removing the first binding (F811) could drop
intentional registration side effects, and removing an unused local
(F841) could drop a side-effect-bearing right-hand side. Both
pyflakes and ruff also leave them alone.

### Added

- `linter.Fix(mod) (*ast.Module, []FixedDiagnostic)` removes unused
  imports in place. Statements that don't change keep their pointer
  identity so v0.1.9 trivia attachment survives. Recurses into
  module-level control flow (`if`, `try`, `with`, `for`, `while`,
  `match`) but stops at `def` / `class` — those introduce scopes
  the module-level F401 check ignores.
- `gopapy lint --fix PATH` rewrites files in place via temp file +
  rename so a crash mid-write can't truncate the source. Stderr
  summary now includes a `, N files fixed` count when `--fix` is
  set.

### Fixed

- `from __future__ import X` no longer trips F401. The bound name
  is a side effect of the import; removing it would silently change
  parser/compiler behavior. Affected ~20 stdlib files where the
  warning was always wrong.

## [0.1.13] - 2026-04-26

The substrate's first downstream demo. Five versions of API in a row
(`Visitor`, `Trivia`, `Transformer`, `Unparse`, `cst.Unparse`) without
a real consumer was a risk: it could look complete on paper but miss
the one shape an actual analyzer needs. v0.1.13 is the proof — a
pyflakes-style linter built only out of the existing public API.

Three checks ship: `F401` (imported but unused), `F811` (redefinition
without intervening use), `F841` (local assigned but never used).
Each check is one file, table-driven, with positive and negative
cases. `# noqa` and `# noqa: F401` suppression on trailing comments
falls out of the `cst` trivia layer.

The linter is intentionally coarser than pyflakes: F811 doesn't track
`if`/`else` branches separately and F401 doesn't read `__all__`.
Stdlib totals are noisy as a result; they print in CI as an
information channel, not a hard gate.

### Added

- `linter` package with `Lint(mod)` and `LintFile(filename, src)`
  entry points. Three checks: `F401`, `F811`, `F841`. Trailing
  `# noqa` / `# noqa: CODE[, CODE]` suppression is honored by
  `LintFile` (which has source bytes to feed `cst.AttachComments`).
- `gopapy lint PATH` CLI subcommand. Single-file or directory walk;
  `--json` emits one diagnostic per line via `Diagnostic.MarshalJSON`.
  Exit code mirrors pyflakes: warnings never fail; only parse
  failures do.
- CI step `gopapy lint stdlib (informational)` prints F401/F811/F841
  totals over CPython 3.14. Continues on error per the spec.
- `linter/testdata/baseline/` fixtures plus a checked-in
  `baseline.golden` so a check changing its mind is visible in PRs.

### Fixed

- `symbols.Build` now records both a bind *and* a use for
  augmented-assignment targets (`x += 1`). The AST stores the
  target with `Store` ctx, but CPython's `_symtable` treats the
  augmented assign as a load, and so do we now. The existing
  symbols tests stay green; F811 / F841 needed this to stop
  flagging `x = 1; x += 1; return x`.

## [0.1.12] - 2026-04-26

Comment-preserving unparse. v0.1.11 made codemods possible; this
version makes them shippable. The AST doesn't carry `#` comments,
so any tool that round-tripped a file through `ast.Unparse` lost
every comment. v0.1.12 layers the v0.1.9 `Trivia` table onto the
new printer hook surface so leading comments stay above their host
statement, trailing comments stay on the same line, and orphan
end-of-file comments stay at module scope.

Files without comments produce the same byte sequence as plain
`ast.Unparse`, so the comment-preserving path is a strict overlay.

### Added

- `cst.File.Unparse() string` renders the file with comments woven
  back in via `AttachComments`. Internally drives the new
  `ast.UnparseHooks` interface (`LeadingFor`, `TrailingFor`,
  `FileTrailing`).
- `ast.UnparseWith(n Node, h UnparseHooks) string` — the lower-
  level hook entrypoint. `ast.Unparse(n)` is now a thin wrapper
  with a nil hook set.
- `gopapy unparse --comments` emits via `cst.Unparse`. Combined
  with `--check` it round-trips parse + cst-unparse + reparse +
  dump-diff with comment preservation against the local stdlib.
- CI step `gopapy unparse --check --comments stdlib` runs against
  the CPython 3.14 stdlib (1841 files clean, 1 allow-listed for
  the same parser bug as the no-comments path).

## [0.1.11] - 2026-04-26

The inverse of `parser.Parse`: take an AST, get back source. Until
now, codemods that wanted to emit modified Python had to either
string-build by hand or grow their own visitor + precedence logic.
v0.1.11 ships `ast.Unparse(n) string` and a `gopapy unparse` CLI.

The contract follows CPython's `ast.unparse`: output is *not*
byte-identical to the original (whitespace, comment, quote-style and
trailing-comma choices aren't in the AST), but it *is* semantically
equivalent — re-parsing the output reproduces the same `ast.Dump`.

Comment-preserving unparse layers on top of this in v0.1.12.

### Added

- `ast.Unparse(n Node) string` renders any `Node` (`*Module`,
  expression, statement, pattern, ...) as Python source. Tracks
  indent depth and operator precedence. Constants render through
  the existing `ConstantValue` repr path so floats and complex
  literals (including `inf`/`nan`, encoded as `1e309` /
  `(1e309-1e309)` per CPython's `_INFSTR` trick) round-trip exactly.
- `gopapy unparse PATH`. With no flags, prints unparse output to
  stdout. With `--check`, walks a directory, parses + unparses +
  re-parses + dump-diffs every `.py` and exits non-zero on any
  round-trip mismatch. `--allow PATH` (repeatable) skips a known
  parser-limitation file from the failure count.
- CI gate: the stdlib-parse workflow now runs
  `gopapy unparse --check` against the local CPython 3.14 stdlib.
  1841 files pass cleanly; one file is allow-listed for an
  unrelated parser bug tracked separately.

### Fixed

- F-string and t-string escape decoding now handles multi-byte
  escape sequences (`\xHH`, `\uHHHH`, `\UHHHHHHHH`, octal `\NNN`).
  Previously the literal string `\x00` was stored as four characters
  instead of a single null byte.
- F-string and t-string format specs are now escape-decoded the
  same way as the surrounding string body. The raw-string flag is
  threaded through so `rf"{x:\n}"` keeps `\n` as two characters.
- `MatchOr` subpatterns that are themselves `MatchOr` are now
  parenthesised on emit. Without parens, `(0 | 1) | 2` would
  flatten to `0 | 1 | 2` and re-parse as a different AST shape.
- `MatchValue` patterns with a negative numeric right-hand side
  (e.g. our parser's representation of `0 - 0j` as
  `BinOp(Add, 0, Constant(-0j))`) now flip the operator on emit so
  the output stays inside the match-pattern grammar.
- `AugAssign` values that are tuples containing a `Starred` element
  are now parenthesised. `x += 1, *y` only parses with parens.
- Single-element tuples in subscript and `match` subject position
  are now wrapped in parens to preserve the trailing comma on
  re-parse.

## [0.1.10] - 2026-04-26

The rewriting counterpart to v0.1.8's `Visitor`. v0.1.8 closed the
read side; v0.1.10 closes the write side. A `Transformer` whose
`Transform(n)` returns the replacement node — same shape as
CPython's `NodeTransformer`. Constant folding, name renaming,
decorator stripping, and any codemod live on this substrate.

### Added

- `ast.Transformer` interface (`Transform(n Node) Node`). Returning
  the receiver continues the walk (descend into children, replacing
  each child slot). Returning a different node replaces the original
  in its parent slot without recursing into the replacement.
  Returning nil removes the node from a list slot, or sets a scalar
  slot to its zero value.
- `ast.Apply(t Transformer, n Node) Node` drives the rewriter and
  returns the (possibly replaced) root.
- Generated `ast/transform_gen.go` mirrors `walkChildren`'s case
  structure. The generator (`internal/asdlgen`) gained a `genTransform`
  pass, so future ASDL changes regenerate transform code in the same
  go-generate run.
- `ast/transform_test.go` covers identity (no-op), scalar replacement
  (rename), constant folding (BinOp → Constant), list removal
  (drop every Pass), root replacement, and the nil-arg no-op
  contract.

### Verified

- `go test ./... -race` green across every package — no data races
  in the new generic helpers under concurrent transforms.
- Stdlib parse + symbols + diag rates stay 100% on CPython 3.14.

## [0.1.9] - 2026-04-26

Comment-to-AST attachment in the `cst` layer. Comments survived
lexing — `cst.File.Tokens()` already exposed every `COMMENT` and
`TYPE_COMMENT` — but they sat in a flat list with no link back to the
statement they belong to. Every formatter, codemod, or
comment-preserving rewriter that wanted "the comment that explains
this assignment" had to re-implement the same line-arithmetic. v0.1.9
lifts that work into the cst layer.

### Added

- `cst.File.AttachComments() *cst.Trivia` walks the token stream once
  and returns a side table mapping each AST node to the comments that
  attach to it. The CST source bytes and the AST itself are not
  mutated.
- New `cst.Trivia` (`ByNode map[ast.Node][]Comment` + `File []Comment`
  for orphans), `cst.Comment` (Pos / Text / Position), and
  `cst.Position` (Leading, Trailing) types.
- Attachment rules:
  - Trailing: a comment that follows a non-comment token on the same
    line attaches to the innermost statement that ends on that line
    (so `def f(): return 1  # ret` puts the comment on the `Return`,
    not the `FunctionDef`).
  - Leading: a comment alone on its line attaches to the next
    statement that begins on a later line. Consecutive leading
    comments all attach to the same following statement, in source
    order.
  - End of file: comments that match neither rule (no following
    statement) land in `Trivia.File`.
- `TYPE_COMMENT` (`# type: int`) follows the same rules as `COMMENT`.
- `cst/trivia_test.go` covers all of the above plus the "fresh map
  per call" contract and the empty-module edge case.

### Verified

- Stdlib parse + symbols + diag rates stay 100% on CPython 3.14.
- `go test ./...` green across every package.

### Deferred

- A comment-preserving Unparse pass is the consumer of trivia, not
  its producer; lands when the consumer needs it.
- Mutable trivia (insert / remove a comment) — current attachment is
  read-only.
- Attachment to expressions inside a parenthesised continuation
  (`x = (\n  1  # one\n  + 2)`). CPython's tokenize is the only thing
  that gets this fully right; v0.1.9 stays at the statement level.

## [0.1.8] - 2026-04-26

A typed-actor visitor pattern over the AST. `ast.Walk(n, fn)` already
covers the trivial "do X for every node" loop, but every analyzer that
wanted per-type behavior was hand-rolling the same type-switch inside
its closure. v0.1.8 adds the CPython-style `Visitor` shape so a single
visitor object can carry state, dispatch per node type, prune subtrees,
or hand a different visitor to a subtree — all in one place.

### Added

- New `ast.Visitor` interface (`Visit(n Node) Visitor`). The return
  value picks the visitor used for the children: the receiver to keep
  walking, `nil` to prune the subtree, or a different visitor to swap
  in for the subtree (CPython `ast.NodeVisitor` semantics).
- `ast.Visit(v Visitor, n Node)` drives a Visitor in depth-first
  pre-order. Visitor-first arg order matches `io.Copy(dst, src)`: the
  actor first, the target second.
- `ast.WalkPreorder(n, fn)` and `ast.WalkPostorder(n, fn)` —
  flat-callback convenience for traversals that don't need pruning.
  Pre-order calls fn before descending; post-order calls fn after, so
  every descendant is visited before its parent (useful for analyses
  that aggregate child results into the parent).
- `ast/visit_test.go` covers the three Visit return modes (collect,
  prune, swap), the two ordering helpers, and nil-visitor / nil-node
  no-op contracts.

### Fixed

- `ast.Visit` and `ast.WalkPostorder` typed-nil-guard their node
  argument before handing it to the generated `walkChildren`. Several
  AST struct fields (`Arguments.Vararg`, `Arguments.Kwarg`,
  `FunctionDef.Returns`, etc.) are concrete `*Arg` / `*ExprNode`
  pointers that can be nil; once promoted to a `Node` interface they
  become typed-nil values that the bare `n == nil` check inside `Walk`
  doesn't catch. The new `isNilNode` helper uses `reflect.IsNil` to
  catch both forms.

### Deferred

- `ast.Transformer` (in-place AST rewriting via the Visitor protocol)
  was scoped into v0.1.8 originally but moved to v0.1.9. In-place
  rewriting needs either reflection over every struct field or a
  generated child-slot setter table; both are larger than the rest of
  v0.1.8 combined and benefit from focused review on their own. The
  Visitor interface added here is the substrate the Transformer will
  sit on.

### Verified

- Stdlib parse + symbols + diag rates stay 100% on CPython 3.14.
- `go test ./...` green across every package.

## [0.1.7] - 2026-04-26

A standalone `diag` package for analyzer diagnostics, plus a
`gopapy diag` CLI. Until now `symbols` carried its own
`symbols.Diagnostic` type — fine while it was the only analyzer in
the tree, but each new analyzer (the linter, an eventual type
checker) would either reinvent the type or have to import `symbols`
for a type that didn't belong to it. v0.1.7 promotes the shape to
its own package so analyzers and CLI tooling share one type.

### Added

- New `gopapy/v1/diag` package with `Diagnostic` (Filename, Pos, End,
  Severity, Code, Msg) and `Severity` enum
  (`SeverityError`/`SeverityWarning`/`SeverityHint`).
- `Diagnostic.String()` formats as
  `filename:line:col: severity[code]: message` — the conventional
  compiler-output shape that editors already parse for jump-to-source.
- `Diagnostic.MarshalJSON()` emits a stable wire shape with severity
  as a lowercase string for the `--json` CLI flag.
- New `gopapy diag PATH` subcommand. PATH may be a file or directory;
  the directory form walks every `.py` recursively. `--json` switches
  output to JSONL. Exit code is 1 only when a `SeverityError`
  diagnostic is reported (warnings and hints don't fail the run);
  parse failures also fail.
- `cmd/gopapy/diag_test.go` covers the human and JSON output shapes,
  the directory walk, and the missing-PATH error.
- New CI step `gopapy diag stdlib` runs the diag CLI against the
  CPython 3.14 stdlib to make sure analyzers stay clean (zero
  `SeverityError` diagnostics).

### Changed

- `symbols.Diagnostic` is now `type Diagnostic = diag.Diagnostic`.
  Existing callers that read `Pos` and `Msg` compile unchanged; the
  alias adds `Filename`, `End`, `Severity`, and `Code`.
- Symbols-emitted diagnostics now carry `Severity = SeverityWarning`
  and stable codes:
  - `S001` — name declared both `global` and `nonlocal` in the same
    scope (current emit site in `symbols.builder.declare`).
  - `S002`, `S003` — reserved (declared as exported constants
    `CodeNonlocalNoBinding`, `CodeUsedBeforeAssign`) so future
    analyzer extensions can't collide on the next free code.

### Fixed

- `ast.emitDictSetElt` no longer panics on a `**y` item that lands in
  set context (e.g. `{x, **y, z}`). The participle grammar accepts the
  malformed mix; the emitter now mirrors the dict-context fallback in
  `emitDictOrSet` and uses the unpacked expression as the element.
- Each binary-fold emitter (`emitDisjunction`, `emitConjunction`,
  `emitInversion`, `emitComparison`, `emitBitOr` / `BitXor` / `BitAnd`,
  `emitShift` / `Sum` / `Term`, `emitFactor` / `Power` / `AwaitPrimary`
  / `Primary`, plus `emitExpr` itself) nil-guards its required `Head`
  field and returns a placeholder `Constant` when participle hands
  back a partial parse tree. The fuzz contract — "no panic on any
  input" — depended on these guards; both crashes were latent before
  v0.1.7's fuzz pass found them.
- New regression seeds in `ast/testdata/fuzz/FuzzEmit/` pin the inputs
  the fuzzer minimized so the fixes don't regress.

### Verified

- Stdlib parse + symbols + diag rates stay 100% on CPython 3.14.
  `gopapy diag` on the local stdlib reports
  `1842 files, 0 parse-failed, 0 diagnostics`.
- Oracle diff stays at 85 / 85 (diagnostics aren't part of `ast.dump`
  output).
- Local 90-second fuzz pass after the guard additions reports zero
  panics across ~5.7M executions.

## [0.1.6] - 2026-04-26

End positions for AST nodes that map directly to a participle grammar
struct. Every node currently set `EndLineno = Lineno` and
`EndColOffset = ColOffset`; downstream tooling that wants to underline
a span (linters, formatters, language servers) was getting a
zero-width caret. v0.1.6 starts populating real spans.

The end positions come from participle's `EndPos` field — the
position of the token immediately after the construct. That's not
byte-exact with CPython's `end_col_offset` (which points to the
position past the last character of the construct), but it's a real
span instead of a zero-width caret. True CPython-faithful end
positions need a hand-written parser; that lands in v0.2.x.

### Added

- `EndPos plexer.Position` field on every grammar struct in
  `parser/grammar.go` and `parser/grammar_expr.go` that already had
  `Pos plexer.Position`. participle populates the field automatically
  with the position of the token consumed immediately after the
  struct.
- `ast.spanPos(start, end plexer.Position) Pos` emitter helper. Mirror
  of `pos()` but writes both ends.
- `ast/endpos_test.go` table-tests 22 multi-line and multi-column
  constructs (function def, class def, if / else, try / except,
  for / while, with, async def, multi-line list / dict literal,
  return value, call args, binop chain, compare chain, subscript,
  attribute chain, tuple / list / dict literal, lambda, ifexp, unary).
  Each asserts that `(EndLineno, EndColOffset)` is strictly after
  `(Lineno, ColOffset)`.

### Constructs that gained accurate end positions

Every AST node emitted from a participle struct that carries
`Pos`+`EndPos`. Concretely: `SimpleStmt`, `Return`, `Raise`, `Delete`,
`Import`, `ImportFrom`, `Alias` (from `DottedAsName` / `ImportAs`),
`Assign` / `AnnAssign` / `AugAssign`, the inner `Tuple` of an
`AssignTarget` / `TargetList` / `SubscriptList`, `Starred`, `If`,
`While`, `For`, `With`, `Try` / `TryStar`, `ExceptHandler`,
`FunctionDef` / `AsyncFunctionDef`, `ClassDef`, `TypeVar` /
`TypeVarTuple` / `ParamSpec`, `AsyncFor`, `AsyncWith`, `Arg`,
`Keyword`, `NamedExpr`, `Lambda`, `IfExp`, `BoolOp`, `UnaryOp`,
`Compare`, `BinOp` (every level: BitOr / BitXor / BitAnd / Shift /
Sum / Term / Pow), `Await`, `Attribute`, `Call`, `Subscript`, `Slice`,
`Atom` (`Name` / `Constant` / `List` / `Dict` / `Set` / `Tuple` /
`Paren` / `GeneratorExp` / etc.), `Starred` (in dict/set element and
star-or-expr position), `Yield` / `YieldFrom`.

### Constructs that did not (deferred to v0.1.7+)

Synthesized AST nodes — those without a 1:1 participle struct —
keep using `pos()` (end == start) until the constituent-walking code
lands. These are:

- The implicit `Tuple` wrapping multi-value `return 1, 2` and the
  trailing-comma `return 1,` form (`emit.go: emitReturn`).
- The implicit `Tuple` wrapping a multi-value augmented-assign RHS
  (`emit.go: emitAssign`).
- The `*` `Alias` synthesized for `from x import *`
  (`emit.go: emitFrom`).
- The implicit `Tuple` wrapping multi-element `except (A, B)` types
  (`emit.go: emitTry`).
- The implicit `Tuple` wrapping `for x in a, b:` iterators
  (`emit.go: emitForIter`).
- The `Starred` wrapping `*args: *Ts` annotations
  (`emit.go: paramArg`).
- The `GeneratorExp` synthesized from a single bare `f(x for x in xs)`
  call (`emit.go: emitCallArgs`).
- The inner `Name` target of a `NamedExpr` (`emit.go: emitExpr` walrus
  branch).
- The implicit `Tuple` wrapping multi-value `yield a, b`
  (`emit.go: emitYield`).

Each of these needs a "union of children's spans" helper to produce
the right end position. That helper is the v0.1.7+ follow-up.

### Verified

- Stdlib parse and symbols rates stay at 1842 / 1842 on CPython 3.14.
- Oracle diff stays at 85 / 85. End positions are show-empty-skipped
  in `ast.dump` at the default level, so adding them doesn't change
  the dump output for the regression fixtures.
- `pos()` is kept and now documents its "end == start" contract
  explicitly so synthesized-node sites read intentionally rather than
  as oversight.

## [0.1.5] - 2026-04-26

A targeted performance pass. The first version of gopapy that lands
benchmarks in the tree, profiles the hot paths, and ships the cuts
that don't require restructuring the parser. The big remaining cost
— participle's reflection-driven parser core — is documented and
deferred; chasing it is a v0.2.x project.

### Measured (Apple M4, `go test -bench=. -benchtime=3x ./lex ./parser ./symbols`)

| Benchmark              | Before        | After         | Δ      |
|------------------------|---------------|---------------|--------|
| ScanFixtures           | 28.1 MB/s     | 37.8 MB/s     | +35%   |
| IndentFixtures         | 22.0 MB/s     | 32.2 MB/s     | +46%   |
| ParseFixtures          | 0.44 MB/s     | 0.61 MB/s     | +39%   |
| BuildFixtures (symbols)| 80.5 MB/s     | 133.0 MB/s    | +65%   |

Stdlib wall-time (`gopapy bench` over CPython 3.14 `Lib/`) goes from
~24.5 s to ~22.7 s — most of the parser's cost is inside participle
itself, which this version does not rewrite.

### Added

- `parser/bench_test.go`, `lex/bench_test.go`, `symbols/bench_test.go`
  with `b.Loop()` benchmarks that use `b.SetBytes` so reports are in
  MB/s and survive hardware moves.
- New `gopapy bench DIR` CLI subcommand. Walks a directory, runs the
  full `parse + emit` pipeline, and prints throughput numbers in a
  grep-friendly format. Useful for one-shot measurements against a
  corpus that isn't in the fixtures.
- New `bench` CI job that runs the benchmarks on every PR and prints
  the numbers in the log. Hard alloc-budget gating is deferred to a
  later version — see notes/Spec/1100/1132 for why.

### Fixed

- `lex.Indent` queue was sliding the head with `pending = pending[1:]`
  on every dequeue. The slide leaks the prefix and the next append
  reallocates; on a 1800-file corpus the lex package allocated
  6.8 GB / 7.6 GB total in `Indent.queue` alone. Replaced with an
  index-based head pointer that resets the slice when the queue
  empties, preserving the backing array. Heap traffic in `lex` drops
  ~89%.
- `parser.definition.Symbols()` rebuilt the token-name map on every
  `ParseFile` call (participle calls it during per-parse setup).
  Cached once at package init.

### Added (parser front-end fast paths)

- `parser.definition` now implements participle's `BytesDefinition`
  and `StringDefinition` interfaces. Callers using `ParseBytes` or
  `ParseString` skip a `bytes.NewReader` + `io.ReadAll` round-trip
  that participle's default path imposes.

### Deferred to a later version

- Replacing the participle-based parser core. The pprof profile
  attributes 78% of all parser allocations to participle's own
  `parseContext.Branch` (lookahead snapshot), `parseContext.Defer`
  (backtracking deferred actions), and `reflect.unsafe_New` (struct
  creation). None of these can be cut without either a grammar
  restructure that drops lookahead requirements or a hand-written
  parser. Both are larger than v0.1.5's scope.
- Hard alloc-budget gating in CI. The participle path's per-parse
  allocation count varies enough across Go runtime versions that a
  hard floor would produce false positives. The bench output in PR
  logs is enough for reviewer-driven regression detection in the
  meantime.

## [0.1.4] - 2026-04-26

Adds a Python symbol table on top of the AST. Every binding site is
recorded with its source position; every name in every scope is
classified as local, parameter, global, nonlocal, free, cell, or
import. Mirrors what CPython's `_symtable` module exposes, scoped
to what's needed by linters, refactoring tools, and the still-deferred
type checker.

### Added

- New `gopapy/v1/symbols` package. `symbols.Build(*ast.Module)
  *symbols.Module` walks the AST and returns a tree of `*Scope`
  objects (Module / Function / Class / Lambda / Comprehension) with
  per-name `*Binding` entries. Each Binding carries a `BindFlag`
  bitfield (`Bound`, `Used`, `Param`, `Global`, `Nonlocal`,
  `Annotation`, `Import`, `Free`, `Cell`) and the full list of
  bind/use source positions.
- `Scope.Resolve(name)` walks up the scope chain (skipping class
  scopes per Python's nested-scope rule) and returns the binding
  scope plus a flag indicating whether the lookup crossed a function
  boundary — i.e. whether it's a free-variable reference.
- Walrus targets (`:=`) inside a comprehension bind in the enclosing
  function or module scope, matching PEP 572 semantics rather than
  the comprehension's own scope.
- New `gopapy symbols PATH` CLI subcommand. With a file argument it
  dumps the scope tree and per-name flags. With a directory it walks
  every `.py` (skipping `bad_*.py` / `badsyntax_*.py` fixtures) and
  reports a pass / parse-failed / panicked summary, recovering from
  any panic so the harness reports every offender in one run.
- New CI step `gopapy symbols stdlib` runs the above against the
  CPython 3.14 standard library on every push. The contract is
  zero `Build` panics; semantic warnings (e.g. `global x` plus
  `nonlocal x`) land in `Module.Diagnostics` and do not fail CI.

### Classified

- Function and class bodies; lambda expressions; list / set / dict /
  generator comprehensions.
- Assignment targets (including tuple, list, and starred unpack),
  augmented assignment, annotated assignment, for-loop target,
  with-as target, except-as target, walrus.
- Function parameters across all positions: positional-only,
  positional, `*args`, keyword-only, `**kwargs`, plus their
  annotations and defaults (defaults evaluated in the enclosing
  scope, parameters bound in the function scope).
- `def` and `class` definition names; type-alias targets
  (`type X = ...`); type parameters (`def f[T](): ...`).
- `import a.b.c` binds `a`; `from x import y as z` binds `z`;
  `from x import *` is a no-op at the symbol level.
- Match patterns: capture, sequence-rest, mapping-rest, class
  pattern, star pattern, as pattern.

### Deferred

- Type inference; cross-module resolution; runtime semantics
  (`__all__`, `globals()`); stub support. The symbol table is the
  ground for these; building any of them is its own version.

## [0.1.3] - 2026-04-26

Adds a source-faithful concrete syntax tree layer above the AST.
Downstream formatters and codemods need access to the original bytes
and the full token stream — including comments — that the parser
discards on its way to a typed AST. `cst.Parse` exposes both.

### Added

- New `gopapy/v1/cst` package: a thin layer above the AST that
  preserves the original source bytes and the full token stream
  (including `COMMENT` and `TYPE_COMMENT` tokens that the parser
  drops). `cst.Parse(filename, src)` returns a `*cst.File` whose
  `Source()` is byte-equal to the input and whose `Tokens()` exposes
  every token. Foundation for downstream formatters and codemods.
- `lex.AllTokens(filename, src)` returns the indent-injected token
  stream with comments preserved. Used internally by `cst.Parse`.

### Deferred

Trivia attachment to specific AST nodes, per-node end positions, and
a mutation API are planned for later versions — see
notes/Spec/1100/1130 for the rationale on shipping the minimum
useful surface first.

## [0.1.2] - 2026-04-26

Adds Go fuzz harnesses for the lexer, parser, and AST emitter, plus a
CI job that runs each one for 30 s per PR. Three real emitter panics
fell out of the first run; each is now a permanent regression seed.

### Added

- Fuzz harnesses for the lexer (`lex.FuzzScan`), parser
  (`parser.FuzzParseFile`), and AST emitter (`ast.FuzzEmit`). A new
  CI `fuzz` job runs each target for 30 s on every PR.
- `ast.TestRoundTripFixtures` pins the strict parse → unparse → parse
  Dump-equality property over the curated grammar corpus.

### Fixed

- `ast.FromFile` no longer panics on participle parse trees with
  internally inconsistent fields. Three cases caught by the fuzzer:
  - `not` parsed as a bare expression (the `Not` boolean was set on a
    backtracked alternative); the emitter now requires `Inv` to be
    non-nil before treating it as a unary `not`.
  - Generator expression with a starred head (`(*x for ...)`) — emit
    via `emitStarOrExpr` so a nil `Expr` field is safe.
  - Dict literal mixing `key: value` and bare-expression items
    (`{"":0,0}`) — skip the malformed bare item rather than
    dereferencing nil `Value`.

## [0.1.1] - 2026-04-26

Drives `gopapy check` against the CPython 3.14 stdlib to zero
failures and locks it in CI. The fixes target a small set of grammar
and lexer corners that the v0.1.0 release missed; no public API
changes.

### Added

- `stdlib-parse` job in `.github/workflows/ci.yml` that runs
  `gopapy check` against the local Python 3.14 stdlib on every push.
  CI is now red if a stdlib file fails to parse.
- New oracle-diff fixtures `tests/grammar/069_*.py` through
  `tests/grammar/084_*.py`, one per failure category reduced from the
  stdlib survey.

### Fixed

- PEP 617 parenthesized `with` headers (`with (a as x, b as y):`)
  now discriminate from a parenthesized single context expression
  (`with (expr) as x:`) via a `(?= COLON)` lookahead after the
  closing `)`.
- PEP 758 unparenthesized exception tuples
  (`except ValueError, TypeError:`) parse and emit as a Tuple type.
- PEP 646 starred annotations on `*args` (`def f(*args: *Ts):`).
- Augmented assignment with a bare-tuple right-hand side
  (`fds += r, w`).
- `return 1, 2, *z` — starred elements in the implicit return tuple.
- `del x, y,` — trailing comma after a `del` target list.
- `x = yield from f(...)` and `y = yield 1, *rest` — yield as the
  right-hand side of an assignment, optionally with a starred element
  in the implicit yield tuple.
- Match open sequence patterns (`case 0, *rest:`, `case *head, 9:`).
- Match mapping patterns with complex-literal keys
  (`case {-0-0j: x}:`).
- Raw f-strings with `\` followed by a `{{`/`}}` escape
  (`fr'\{{'`).
- Nested string inside an f-string interpolation: a `{` inside the
  inner non-f-string is plain text, not the start of a new
  interpolation (`f'{"{"}'`).
- Nested replacement field inside another field's format spec
  (`f'{x:{y:0}}'`); the inner `{...}` is recursively scanned and the
  spec mode is restored when it closes.
- Dict/set literal inside an f-string interpolation: a `:` inside
  `{ ... }` in expression mode is no longer misread as the start of a
  format spec (`f'{ {1: 2} }'`).
- `for x, in xs:` — single-element tuple target via trailing comma;
  the comma is held for the trailer by a `(?! COMMA 'in')` negative
  lookahead inside the target loop.
- `for x in *a, *b, *c:` — starred elements in the implicit-tuple
  iterable.
- Match or-pattern with deep paren-tuples and signed-number literals;
  parser lookahead bumped from 8 to 96 so the discrimination
  succeeds on the four-alternation forms used in `test_patma.py`.
- PEP 3131 / UAX #31 identifier continuation: combining marks (Mn,
  Mc), connector punctuation (Pc), and the tag-character block
  (U+E0100..U+E01EF) are accepted as identifier continuation
  characters. Previously, encountering one of these mid-identifier
  broke the lexer out of NAME and triggered exponential backtracking
  in the operator alternatives, making `test_unicode_identifiers.py`
  burn 70+ GB of RAM before completing.
- `gopapy check DIR` now forces a `runtime.GC()` every 64 files to
  keep the resident set bounded on large corpora. Without this, the
  `stdlib-parse` CI job exceeded the 7 GB free-runner memory limit.
- Unparser now pads with a space when an f-string interpolation's
  inner expression starts or ends with `{`/`}` (e.g. dict literals)
  so the round-tripped source doesn't lex its braces as `{{`/`}}`
  escapes.

## [0.1.0] - 2026-04-26

First release that promises backwards compatibility. Downstream callers
(goipy, future linters and formatters) can pin to v0.1 and trust the
public surface won't move under them every release. The single
breaking change is the Go module path bump to `/v1`; everything else
is documentation and a contract.

### Changed

- Module path moves from `github.com/tamnd/gopapy` to
  `github.com/tamnd/gopapy/v1`. All internal imports were rewritten in
  the same commit. Downstream replaces

  ```go
  import "github.com/tamnd/gopapy/parser"
  import "github.com/tamnd/gopapy/ast"
  import "github.com/tamnd/gopapy/lex"
  ```

  with

  ```go
  import "github.com/tamnd/gopapy/v1/parser"
  import "github.com/tamnd/gopapy/v1/ast"
  import "github.com/tamnd/gopapy/v1/lex"
  ```

  No source-level changes are required beyond the import path.

### Added

- Stability contract documented in README and per-package doc
  comments. Three guarantees hold from v0.1.0 onward:
  - AST node types in package `ast` are frozen. No renames, no field
    removals, no field-type changes. New optional fields and new node
    variants for upstream-CPython grammar growth land in patch
    releases.
  - Public parser entry points are stable: `parser.ParseFile`,
    `parser.ParseString`, `parser.ParseExpression`,
    `parser.ParseReader`, `ast.Dump`, `ast.Unparse`, `ast.FromFile`,
    `lex.NewScanner`, `lex.NewIndent`. Signatures and behavior are
    frozen.
  - The `/v1` module path itself enforces the contract: future
    breaking changes will move to `/v2`.
- Doc comments on every entry point above so `go doc` renders a usable
  summary without ambiguity.

### Migration

Downstream callers update import paths once (the Go ecosystem's
`/vN` rule means the old path keeps working at v0.0.x; new code
imports `/v1`). No API calls change.

## [0.0.9] - 2026-04-26

`ast.Unparse`. CPython has `ast.unparse`; gopapy now ships the Go
equivalent so downstream tooling can mutate trees and write them
back. The round-trip property (parse, unparse, re-parse, compare
dumps) holds across the full grammar corpus.

### Added

- `ast.Unparse(n Node) string` walks any node (Module, statement,
  expression, pattern, type-param, except-handler) and produces
  Python source. Output is structurally faithful but not byte-equal
  to the original: comments and original parenthesisation are not
  preserved, single quotes are preferred for strings, parens are
  added defensively when precedence would otherwise change the
  parse.
- Precedence table mirrors CPython's `_Unparser`: BoolOp / Compare /
  BitOr / BitXor / BitAnd / Shift / Arith / Term / Factor / Power /
  Await / Atom. Power stays right-associative; comprehensions and
  IfExp wrap their tests at the right level.
- F-strings and t-strings reconstruct from JoinedStr / TemplateStr
  children. `{expr}`, `{expr!conv}`, `{expr:spec}`, `{{` / `}}`
  escapes, and recursive format specs all round-trip.
- Match patterns: every PatternNode constructor (MatchValue,
  MatchSingleton, MatchSequence, MatchMapping, MatchClass,
  MatchStar, MatchAs, MatchOr) renders back into source the parser
  re-accepts.
- Type parameters: `[T: bound = default]`, `[*Ts]`, `[**P]` for
  function, class, and `type` alias definitions.
- Bare-tuple positions (assignment LHS/RHS, return value, yield
  value, for-target, for-iter, comprehension target) emit without
  parens; tuples in expression position get parenthesised.
- Bare `yield` and `yield from` at statement level emit without the
  parens that the expression form requires.

### CLI

- `gopapy unparse FILE` parses the file, runs Unparse on the
  resulting Module, and prints the rendered source. Useful for
  ad-hoc smoke testing.

### Fixtures

- `068_unparse_edge.py` covers the precedence corners (mixed
  power / unary, walrus, IfExp, lambda with every parameter shape,
  recursive f-string format spec, slice steps with Ellipsis,
  starred unpacking, chained comparisons).

`tests/run.sh` reports 68 / 68. The new
`TestUnparse_RoundTrip_Fixtures` table-test in `ast/unparse_test.go`
parses every fixture, unparses, re-parses, and asserts the two
`ast.Dump` outputs match.

## [0.0.8] - 2026-04-26

Stdlib pass. v0.0.7 brought corner cases caught by reading code; v0.0.8
turns the corpus on itself and clears most of what `gopapy check`
flagged when pointed at the CPython 3.14 stdlib. Failure count went
from ~275 to under 20 (remaining items are unparenthesized
context-manager + odd PEP 646 unpack positions, deferred to v0.0.9).

### Added

- `for x in a, b:` — bare-tuple iterator. CPython's grammar allows
  comma-separated star-expressions after `in`; `ForStmt` now captures
  the rest into `IterRest` and the emitter folds them into a Tuple.
  Fixes `for op in '+', '-':` and similar patterns scattered through
  the stdlib.
- `yield a, b` — bare-tuple yield value. `YieldExpr` grows `ValRest`;
  the emitter wraps a single yield value with a comma into a Tuple.
- `return x,` — single-element tuple return. `ReturnStmt` tracks the
  trailing comma so the emitter can wrap a single value as Tuple.
- `class C(A, B,):` — trailing comma in class bases.
- F-string format-spec mode. After a top-level `:` inside an
  interpolation, the scanner switches to literal mode so `{x:#x}` and
  similar format specs scan correctly. Previously the `#` was
  swallowed as a comment, eating the closing `}` and the rest of the
  line.

### Fixed

- `(a, b) = c` and other parenthesized statements at the top of a
  block. `lex/indent.go` was incrementing the bracket counter before
  the line-start indent check, so a `(` opening the line evaded
  INDENT processing. The bracket count is now snapshotted before the
  line-start check.
- `# type: ignore[...]` no longer breaks the parse. The scanner still
  tags TYPE_COMMENT separately from regular comments, but the indent
  layer drops them rather than forwarding to the parser, since no
  grammar rule consumes them yet.
- `Subscript` grammar restructured. The v0.0.7 `Plain | Slice` shape
  could match zero tokens, tripping participle's no-progress guard
  on `]` after a sequence. Three explicit alternatives now: `*expr`,
  `expr (slice-tail)?`, and bare slice.

## [0.0.7] - 2026-04-26

Real-world corner cases. After v0.0.6 the parser claimed a complete
Python 3.14 grammar surface, but pointing `gopapy check` at the
CPython stdlib lit up failures. A reduction pass surfaced three
recurring shapes that no fixture had exercised.

### Added

- Compact-body suite: `def f(): ...`, `class C: pass`,
  `if cond: x = 1`, `for i in xs: print(i)`. Block grows an
  `Inline` alternative that takes a SimpleStmts directly after the
  colon, matching CPython's `simple_stmt` suite shape.
- Starred subscripts (PEP 646 consumer side): `tuple[*Ts]`,
  `Callable[[*Args], R]`, `dict[str, *Vs]`. v0.0.5 deferred this;
  the producer side already worked through TypeVarTuple in a
  type-param list. Subscript gains a Star bool; the emitter wraps
  a single starred element in a Tuple to match CPython.
- Lambda is fixed across the board. The previous `Lambda.Params`
  reused the function `Param` type, whose optional `: annot` slot
  greedily ate the lambda body's COLON. Every lambda has been
  failing to parse since v0.0.1, including the trivial
  `lambda: 1`. The bug went unnoticed because no fixture covered
  lambda. New `LambdaParam` type without annotation, plus fixture
  066 covering positional-only `/`, keyword-only `*`, `*args`,
  `**kwargs`, defaults, and call-site usage.

### Fixtures

- `064_compact_body.py` — def / class / if / while / for inline bodies
- `065_starred_subscript.py` — `tuple[*Ts]` and variants
- `066_lambda.py` — every lambda shape

`tests/run.sh` reports 66 / 66.

### Known limits

A full stdlib pass (zero failures across CPython 3.14's `Lib/`) is
the v0.0.8 target. v0.0.7 closes the recurring shapes; the long
tail of one-off failures lands in the next release.

## [0.0.6] - 2026-04-26

t-strings (PEP 750) and the lexer state machine for nested f-strings
with the same quote character (PEP 701). With this release the
parser covers the full Python 3.14 grammar surface; remaining work
is polish, edge cases, and downstream tooling.

### Added

- t-strings: `t"hello {name}"` lowers to TemplateStr with
  Interpolation values. The Interpolation node carries the original
  expression source text in `str` per the PEP 750 AST shape.
- All t-string prefix variants (`t`, `T`, `rt`, `tr`, `Rt`, `rT`,
  `Tr`, `tR`) and triple-quoted t-strings.
- Conversion specifiers (`!r`, `!s`, `!a`), format specs, recursive
  format-spec parsing (`{x:.{prec}f}`), and the debug `{x=}`
  shorthand all work in t-strings the same way they do in f-strings.
- PEP 701 nested f-strings: `f"{"hello"}"`, `f"{f"{"deep"}"}"`,
  `f"{x + "y"}"`. The lexer now tracks brace depth inside
  interpolations and recurses into nested string literals so an
  inner `"` no longer closes the outer string.
- Cross t-string / f-string nesting: `t"{f"{x}"}"` and
  `f"{t"{x}"}"` both parse and emit the right node mix.

### Lexer

`scanInterpolatedString` replaces the flat-string path for f/t
prefixes. It tracks brace depth (`{` opens, `}` closes, doubled
forms escape) and, when depth > 0, recursively skips nested string
literals via `skipNestedString` / `skipNestedInterpolation`. The
outer scanner still emits a single STRING token; splitting the body
remains the emitter's job.

### Fixtures

- `059_tstring_basic.py` — empty, no-interp, single and multi-interp
- `060_tstring_format.py` — conversions, format specs, recursive
- `061_tstring_triple.py` — triple-quoted, multi-line
- `062_fstring_pep701.py` — same-quote nesting, expression-with-string
- `063_fstring_nested_deep.py` — three-deep, mixed t / f

`tests/run.sh` reports 63 / 63.

## [0.0.5] - 2026-04-25

Type parameters and `type` aliases (PEP 695, with PEP 696 defaults).
After this lands the parser's gap to a full Python 3.14 grammar is
just t-strings (PEP 750) and the lexer state machine for nested
f-strings.

### Added

- `type Name = Expression` statement at module / suite level. Lowers
  to TypeAlias with a Name(Store) target. The `type` keyword is soft:
  `type = 1`, `type += 1`, `def f(type, case)`, and `def type():` all
  still parse as ordinary identifiers because the TypeAlias
  alternative requires NAME after `type`, so single-token `type`
  references fall through to AssignStmt or ExprStmt.
- Optional bracketed type-parameter list on `def`, `class`, and the
  `type` statement: `def f[T](x: T) -> T:`, `class C[T, U](Base):`,
  `type Pair[T, U] = tuple[T, U]`. Empty bases on a parameterised
  class still works (`class D[T, U]:`).
- TypeVar with bound: `def f[T: int](x: T) -> T`. The bound accepts
  arbitrary expressions, so constraint tuples (`T: (int, str)`) come
  for free.
- TypeVar default (PEP 696): `def f[T = int](x: T)`.
- TypeVarTuple `*Ts` and ParamSpec `**P`. Both accept `= default`
  but reject `: bound` at the AST level (the parser is permissive;
  CPython rejects bound on these forms at compile, not parse time).
- Mixed forms: `[T: int, *Ts, **P]` in any combination.

### Fixtures

- `054_type_alias.py` — bare `type X = ...` plus union RHS
- `055_type_param_func.py` — `def f[T](x: T) -> T:` and friends
- `056_type_param_class.py` — `class C[T](Base):` and parameterised
  classes with no bases
- `057_type_param_advanced.py` — bounds, constraint tuples, defaults,
  TypeVarTuple, ParamSpec, mixed forms, parameterised type alias
- `058_type_soft_keyword.py` — `type = 1`, `type += 1`, `def type():`,
  `def f(type, case)` all still work as identifiers

`tests/run.sh` reports 58 / 58.

### Known limits

`tuple[*Ts]` (starred expressions inside subscripts, the consumer
side of PEP 646) is not yet accepted by the subscript grammar — the
producer side (TypeVarTuple in a type-param list) works, but the
unpacking form inside a subscript will be addressed alongside the
other generic-subscript polish in a later release. t-strings (PEP
750) and the lexer state machine that fully handles nested f-strings
inside interpolations remain on the v0.0.6 ticket.

## [0.0.4] - 2026-04-25

Match statements (PEP 634). The full pattern hierarchy lands in
one PR because there is no clean cut point: a partial match parser
either accepts everything or it accepts nothing useful.

### Added

- `match subject:` block with one or more `case pattern:` clauses.
- Literal patterns: signed/unsigned int and float, complex, string,
  bytes (via stringConstant), `True`, `False`, `None`. True / False /
  None lower to MatchSingleton; everything else to MatchValue.
- Capture pattern: a bare NAME binds the subject. `_` is the
  wildcard (MatchAs with no pattern, no name).
- Value pattern: dotted NAME (`Color.RED`, `mod.sub.NAME`) lowers to
  MatchValue wrapping an Attribute chain.
- Group pattern: `(p)` parenthesised.
- Sequence pattern: `[a, b, *rest]`, `(a, b)`, `[]`, `()` all work,
  with star items mapped to MatchStar (`*_` becomes MatchStar with
  no name, matching CPython).
- Mapping pattern: `{"k": v, NAME: p, **rest}`. Keys accept literals
  and dotted names; rest captures into MatchMapping.Rest.
- Class pattern: `Point()`, `Point(0, 0)`, `Point(x=0, y=y)`, and
  `mod.Color(value=v)`. Positional and keyword args separate into
  MatchClass.Patterns / KwdAttrs / KwdPatterns the way the AST
  expects.
- Or pattern: `p1 | p2 | p3`. Lowers to MatchOr.
- As pattern: `pattern as name`, including `_ as x` (which becomes
  MatchAs(pattern=None, name='x'), matching CPython's quirk that
  `_` collapses inside the As).
- Guards: `case p if cond:` populates MatchCase.Guard.

### Soft keywords

`match` and `case` are still NAMEs at the lexer level. The parser
recognises them by position: `match expr:` opens a match statement,
`case pat:` opens a case clause inside the match block. Outside
those contexts, both names lex back as ordinary identifiers, so
`match = 1`, `case = 2`, and `def f(match, case)` keep working.
Fixture 053 locks this behaviour.

### Fixtures

- `045_match_basic.py` — literal and wildcard patterns
- `046_match_capture.py` — capture and value patterns
- `047_match_sequence.py` — list and tuple patterns with star
- `048_match_mapping.py` — dict patterns with `**rest`
- `049_match_class.py` — class patterns positional + keyword
- `050_match_or.py` — or-pattern alternation
- `051_match_as.py` — as-pattern binding
- `052_match_guard.py` — guards
- `053_match_soft_keyword.py` — `match` / `case` as identifiers

`tests/run.sh` reports 53 / 53.

### Known limits

Type parameters and `type` aliases (PEP 695), t-strings (PEP 750),
and the lexer state machine that fully handles nested f-strings
inside interpolations are deferred to v0.0.5 and v0.0.6 per the
roadmap.

## [0.0.3] - 2026-04-25

Completeness pass on the existing surface. The big-rocks releases
(match, type parameters, t-strings) get their own tags; this one
closes the long tail of literal forms and small statement shapes
that were tripping `gopapy check` on real code.

The fixtures in `tests/grammar/` are restored to the
one-construct-per-file convention. Two-file consolidation made the
diff smaller but the failure messages worse, since a single typo
inside a 90-line fixture would mask everything else in the topic.
With one file per construct, a CI failure points straight at the
construct that broke.

### Added

- Multi-target assignment: `a = b = c = 1`. The chain still parses
  through a single AssignStmt and emits one Assign with N targets.
- Starred LHS targets: `x, *rest = xs`, `*head, tail = xs`,
  `first, *middle, last = xs`. The assignment LHS is now a real
  comma-separated target list, not a single Expression.
- Exception groups (PEP 654): `try ... except* TypeError as eg:`.
  The parser keeps `except` and `except*` distinct via a Star bool
  on ExceptClause, and emitTry promotes the wrapper to TryStar when
  any handler uses the star form.
- `assert e` and `assert e, msg` already worked, but are now in the
  fixture corpus to lock the shape.
- `raise X from None` round-trips correctly. None / True / False are
  resolved to Constant in the emitter even when they came through
  the NAME token alternative, fixing the `cause=Name(id='None')`
  vs `cause=Constant(value=None)` regression for free everywhere.
- Numeric literal completeness: `0x_ff_ee` (underscore right after
  the radix prefix), bare-trailing-dot floats `5.`, complex `1j` /
  `0J`, plus the existing `1_000_000` / `1.5e-3` forms re-confirmed.
- All hex / oct / bin literals normalise to decimal in the AST dump,
  matching CPython (`0xff` → `Constant(value=255)`). Big literals
  go through math/big so values past uint64 still come out right.
- Float repr matches CPython: `5.0` not `5`, `10000000000.0` not
  `1e+10`, scientific kept only outside [1e-4, 1e16). Complex imag
  drops the trailing `.0` for whole values (`1j`, not `1.0j`).
- String prefix completeness across all legal cross-products of
  `b`/`B`/`r`/`R`/`u`/`U`/`f`/`F`. Raw strings (`r"..."`) keep their
  backslash escapes literal instead of decoding them. The kind
  field on Constant is set to `'u'` only for the lowercase prefix,
  matching CPython's quirk that `U"x"` produces no kind.
- Triple-quoted strings, including the f-string variant, round trip
  with embedded newlines and mixed quote styles.
- Recursive f-string format spec: `f"{x:>{width}.{prec}f}"` parses
  the inner `{width}` and `{prec}` chunks as real FormattedValues
  inside the spec's JoinedStr.
- Slice step variants (`a[::]`, `a[::-1]`, `a[1::2]`) and subscript
  with Ellipsis (`m[..., 0]`, `m[1:2, ..., ::2]`).

### Fixtures

- `031_multi_assign.py` through `044_subscript_ellipsis.py`. One
  topic per file, deliberately small. Total corpus is now 44/44.

### Known limits

`match` / `case`, type parameters and `type` aliases (PEP 695),
t-strings (PEP 750), and the lexer state machine that fully handles
nested f-strings inside interpolations are deferred to v0.0.4 and
beyond per the roadmap.

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

[Unreleased]: https://github.com/tamnd/gopapy/compare/v0.1.31...HEAD
[0.2.0]: https://github.com/tamnd/gopapy/compare/v0.1.34...v0.2.0
[0.1.34]: https://github.com/tamnd/gopapy/compare/v0.1.33...v0.1.34
[0.1.33]: https://github.com/tamnd/gopapy/compare/v0.1.32...v0.1.33
[0.1.32]: https://github.com/tamnd/gopapy/compare/v0.1.31...v0.1.32
[0.1.31]: https://github.com/tamnd/gopapy/compare/v0.1.30...v0.1.31
[0.1.30]: https://github.com/tamnd/gopapy/compare/v0.1.29...v0.1.30
[0.1.29]: https://github.com/tamnd/gopapy/compare/v0.1.28...v0.1.29
[0.1.28]: https://github.com/tamnd/gopapy/compare/v0.1.27...v0.1.28
[0.1.27]: https://github.com/tamnd/gopapy/compare/v0.1.26...v0.1.27
[0.1.26]: https://github.com/tamnd/gopapy/compare/v0.1.25...v0.1.26
[0.1.25]: https://github.com/tamnd/gopapy/compare/v0.1.24...v0.1.25
[0.1.24]: https://github.com/tamnd/gopapy/compare/v0.1.23...v0.1.24
[0.1.23]: https://github.com/tamnd/gopapy/compare/v0.1.22...v0.1.23
[0.1.22]: https://github.com/tamnd/gopapy/compare/v0.1.21...v0.1.22
[0.1.21]: https://github.com/tamnd/gopapy/compare/v0.1.20...v0.1.21
[0.1.20]: https://github.com/tamnd/gopapy/compare/v0.1.19...v0.1.20
[0.1.19]: https://github.com/tamnd/gopapy/compare/v0.1.18...v0.1.19
[0.1.18]: https://github.com/tamnd/gopapy/compare/v0.1.17...v0.1.18
[0.1.17]: https://github.com/tamnd/gopapy/compare/v0.1.16...v0.1.17
[0.1.16]: https://github.com/tamnd/gopapy/compare/v0.1.15...v0.1.16
[0.1.15]: https://github.com/tamnd/gopapy/compare/v0.1.14...v0.1.15
[0.1.14]: https://github.com/tamnd/gopapy/compare/v0.1.13...v0.1.14
[0.1.13]: https://github.com/tamnd/gopapy/compare/v0.1.12...v0.1.13
[0.1.12]: https://github.com/tamnd/gopapy/compare/v0.1.11...v0.1.12
[0.1.11]: https://github.com/tamnd/gopapy/compare/v0.1.10...v0.1.11
[0.1.10]: https://github.com/tamnd/gopapy/compare/v0.1.9...v0.1.10
[0.1.9]: https://github.com/tamnd/gopapy/compare/v0.1.8...v0.1.9
[0.1.8]: https://github.com/tamnd/gopapy/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/tamnd/gopapy/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/tamnd/gopapy/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/tamnd/gopapy/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/tamnd/gopapy/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/tamnd/gopapy/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/tamnd/gopapy/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/tamnd/gopapy/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/tamnd/gopapy/compare/v0.0.9...v0.1.0
[0.0.9]: https://github.com/tamnd/gopapy/compare/v0.0.8...v0.0.9
[0.0.8]: https://github.com/tamnd/gopapy/compare/v0.0.7...v0.0.8
[0.0.7]: https://github.com/tamnd/gopapy/compare/v0.0.6...v0.0.7
[0.0.6]: https://github.com/tamnd/gopapy/compare/v0.0.5...v0.0.6
[0.0.5]: https://github.com/tamnd/gopapy/compare/v0.0.4...v0.0.5
[0.0.4]: https://github.com/tamnd/gopapy/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/tamnd/gopapy/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/tamnd/gopapy/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/tamnd/gopapy/releases/tag/v0.0.1
