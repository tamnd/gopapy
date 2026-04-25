# Grammar coverage

gopapy targets [CPython 3.14's PEG grammar](https://docs.python.org/3.14/reference/grammar.html).
This file tracks which productions the bootstrap branch covers and which
land in PR2.

## Bootstrap (this PR)

### Statements
- `pass`, `break`, `continue`
- `return` (with tuple return value)
- `raise` (bare, with exception, `raise X from Y`)
- `del` (with Del context)
- `import`, `import ... as ...`, dotted modules
- `from`, including relative dots, `from . import x`, `from .. import x`
- Simple, augmented, and annotated assignment
- `if` / `elif` / `else` (elif folded into nested Orelse)
- `while`, `for`, `try` / `except` / `else` / `finally`
- `def` and `class` (with bases and metaclass keyword)

### Expressions
- Unary `+`, `-`, `~` and boolean `not`
- Boolean `and`, `or`
- Comparisons: `<`, `<=`, `>`, `>=`, `==`, `!=`, `is`, `is not`, `in`,
  `not in` (chained: `a < b < c`)
- Bitwise `|`, `^`, `&`, `<<`, `>>`
- Arithmetic `+`, `-`, `*`, `/`, `//`, `%`, `@`, `**`
- Power is right-associative; the rest fold left.
- Calls with positional, keyword, `*args`, `**kwargs`
- Subscripts and slices, including `a[::2]`, `a[1:2, 3]`
- Attribute access
- List, tuple, set, dict literals
- Name, number (int/float/complex), string (with implicit
  concatenation), bytes (`b'...'`), True, False, None, Ellipsis

### Tokens
- INDENT, DEDENT, NEWLINE, ENDMARKER injection
- Bracket-depth suppression of the indent dance
- Common escape sequences: `\n`, `\t`, `\r`, `\\`, `\'`, `\"`, `\xHH`

## Deferred to PR2 (or later)

- f-strings and t-strings (still tokenised as STRING)
- Match statements
- Walrus (`:=`) outside trivial contexts
- Type parameters (`def f[T]: ...`)
- Comprehensions (`[x for x in xs]`, generator expressions)
- Async (`async def`, `async for`, `async with`, `await` outside
  trivial expressions)
- `with` statement and parenthesised context managers
- Decorators
- Positional-only marker (`/`) in function definitions
- Star-unpacking in tuple/list literals (`(*a, b)`)
- Octal, binary, and unicode-name string escapes
- Soft keywords beyond what bootstrap touches

## How to add coverage

1. Drop a fixture under `tests/grammar/NNN_feature.py`.
2. Run `tests/run.sh`. If it fails, the diff shows what gopapy got wrong.
3. Fix the parser, emitter, or dumper until it passes.
4. Cross-link the new construct here when promoting it from "deferred".
