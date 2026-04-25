# `yield` and `yield from` are valid right-hand sides of an assignment,
# and `yield` may produce an implicit tuple with starred elements.
def gen(it):
    x = yield from it
    y = yield 1, 2, 3
    z = yield 1, *rest
    return x, y, z
