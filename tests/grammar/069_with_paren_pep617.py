# Python 3.9+: PEP 617: parenthesized list of context managers in a `with` header.
with (open("a") as a, open("b") as b):
    pass

with (
    open("a") as a,
    open("b") as b,
):
    pass
