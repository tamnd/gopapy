a = lambda: 1
b = lambda x: x
c = lambda x, y: x + y
d = lambda x=1, y=2: x + y
e = lambda *args, **kwargs: args
f = lambda x, /, y, *, z: x + y + z
g = lambda *, x, y: x
h = sorted(xs, key=lambda x: x.k)
