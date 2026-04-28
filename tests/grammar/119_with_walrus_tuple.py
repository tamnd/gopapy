# Python 3.10+
# Walrus operator inside parenthesized with: single-tuple context vs two items

# PEP 572: with (x := a, y := b) is a Tuple context manager
with (x := a, y := b):
    pass

# Two separate walrus items (no enclosing parens around pair)
with (x := a), (y := b):
    pass

# Walrus with call exprs in tuple context
with (x := open("f"), y := open("g")):
    pass
