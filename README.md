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
[`docs/GRAMMAR.md`](docs/GRAMMAR.md).

## License

MIT. See [LICENSE](LICENSE).
