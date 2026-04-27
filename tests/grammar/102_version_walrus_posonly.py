# Python 3.8+: walrus := and positional-only /
def f(x, /, y, *, z):
    return x + y + z

def g(a, b, /):
    return a + b

if n := len([1, 2, 3]):
    pass

result = [y := x + 1 for x in range(3)]
