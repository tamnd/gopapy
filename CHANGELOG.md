# Changelog

All notable changes to gopapy are recorded here. The format follows
[Keep a Changelog 1.1](https://keepachangelog.com/en/1.1.0/). Once
gopapy reaches 1.0 the project will follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); until
then, expect minor version bumps to sometimes include breaking
changes.

## [Unreleased]

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

[Unreleased]: https://github.com/tamnd/gopapy/compare/v0.1.4...HEAD
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
