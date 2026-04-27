import contextlib

def f(p, a, b, c):
    with (p / 'x').open('r') as f:
        pass
    with (a if c else b) as v:
        pass
    with (p).joinpath('y') as g:
        pass
