# `return` may carry a starred element inside the implicit tuple.
def f(z):
    return 1, 2, *z
